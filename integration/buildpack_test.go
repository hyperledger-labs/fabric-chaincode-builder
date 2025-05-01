/*
 * SPDX-License-Identifier: Apache-2.0
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at:
 *
 * 	  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package integration

import (
	"archive/tar"
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/hyperledger-labs/fabric-chaincode-builder/integration/fakes"
	"github.com/hyperledger-labs/fabric-chaincode-builder/pkg/k8sbuilder"

	pb "github.com/hyperledger/fabric-protos-go/peer"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var _ = Describe("IBP External Builder", func() {
	var (
		ibpSidecar       string
		ibpClient        string
		testDir          string
		buildpackBinPath string

		serverSession    *gexec.Session
		builderClientEnv []string
	)

	BeforeEach(func() {
		var err error
		testDir, err = ioutil.TempDir("", "testdir")
		Expect(err).NotTo(HaveOccurred())

		ibpSidecar, err = gexec.Build("github.com/hyperledger-labs/fabric-chaincode-builder/cmd/ibp-builder")
		Expect(err).NotTo(HaveOccurred())

		ibpClient, err = gexec.Build("github.com/hyperledger-labs/fabric-chaincode-builder/cmd/ibp-builder-client")
		Expect(err).NotTo(HaveOccurred())

		buildpackBinPath = filepath.Join(testDir, "bin")
		err = os.Mkdir(buildpackBinPath, 0777)
		Expect(err).NotTo(HaveOccurred())

		// builder commands are are symbolic links to ibp-builder-client
		err = os.Symlink(ibpClient, filepath.Join(buildpackBinPath, "detect"))
		Expect(err).NotTo(HaveOccurred())
		err = os.Symlink(ibpClient, filepath.Join(buildpackBinPath, "build"))
		Expect(err).NotTo(HaveOccurred())
		err = os.Symlink(ibpClient, filepath.Join(buildpackBinPath, "release"))
		Expect(err).NotTo(HaveOccurred())
		err = os.Symlink(ibpClient, filepath.Join(buildpackBinPath, "run"))
		Expect(err).NotTo(HaveOccurred())

		envTag := "2.2.1-2511004-amd64"
		os.Setenv("FILETRANSFERIMAGE", "us.icr.io/ibp-temp/ibp-init:2.5.0-20200618-amd64")
		os.Setenv("BUILDERIMAGE", fmt.Sprintf("us.icr.io/ibp-temp/ibp-ccenv:%s", envTag))
		os.Setenv("GOENVIMAGE", fmt.Sprintf("us.icr.io/ibp-temp/ibp-goenv:%s", envTag))
		os.Setenv("JAVAENVIMAGE", fmt.Sprintf("us.icr.io/ibp-temp/ibp-javaenv:%s", envTag))
		os.Setenv("NODEENVIMAGE", fmt.Sprintf("us.icr.io/ibp-temp/ibp-nodeenv:%s", envTag))
		os.Setenv("IMAGEPULLSECRETS", "regcred")

		// TODO(mjs): port ranges
		serverCommand := exec.Command(
			ibpSidecar,
			"-fileServerBaseURL", "http://host.docker.internal:22222",
			"-fileServerListenAddress", ":22222",
			"-kubeNamespace", "default",
			"-peerID", k8sbuilder.UUID(),
			"-sharedVolumePath", testDir,
			"-sidecarListenAddress", "127.0.0.1:11111",
		)
		serverSession, err = gexec.Start(serverCommand, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(dialEndpoint("127.0.0.1:11111"), 5*time.Second).Should(Succeed())
		Eventually(dialEndpoint("127.0.0.1:22222"), 5*time.Second).Should(Succeed())

		builderClientEnv = []string{
			"IBP_BUILDER_SHARED_DIR=" + testDir,
			"IBP_BUILDER_ENDPOINT=127.0.0.1:11111",
		}
	})

	AfterEach(func() {
		if serverSession != nil {
			serverSession.Terminate()
			Eventually(serverSession).Should(gexec.Exit())
		}
		gexec.KillAndWait(time.Second)
		gexec.CleanupBuildArtifacts()
		os.RemoveAll(testDir)
	})

	DescribeTable("detect, build, release, and run",
		func(fixtureDir string, outputEntries []string) {
			By("detecting")
			detectCommand := exec.Command(
				filepath.Join(buildpackBinPath, "detect"),
				filepath.Join("testdata", fixtureDir, "source"),
				filepath.Join("testdata", fixtureDir, "metadata"),
			)
			detectCommand.Env = append(os.Environ(), builderClientEnv...)
			detectSess, err := gexec.Start(detectCommand, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(detectSess, 5*time.Second).Should(gexec.Exit(0))

			By("building")
			buildOutputDir, err := ioutil.TempDir(testDir, "build-output")
			Expect(err).NotTo(HaveOccurred())
			buildCommand := exec.Command(
				filepath.Join(buildpackBinPath, "build"),
				filepath.Join("testdata", fixtureDir, "source"),
				filepath.Join("testdata", fixtureDir, "metadata"),
				buildOutputDir,
			)
			buildCommand.Env = append(os.Environ(), builderClientEnv...)
			buildSess, err := gexec.Start(buildCommand, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(buildSess, 3*time.Minute).Should(gexec.Exit(0))
			Expect(filepath.Join(buildOutputDir, "chaincode-output.tar")).To(BeARegularFile())
			Expect(filepath.Join(buildOutputDir, "package-metadata.tar")).To(BeARegularFile())
			Expect(filepath.Join(buildOutputDir, "chaincode-source.tar")).To(BeARegularFile())
			tarEntries := tarEntryNames(filepath.Join(buildOutputDir, "chaincode-output.tar"))
			for _, outputEntry := range outputEntries {
				Expect(outputEntry).To(BeElementOf(tarEntries))
			}
			Expect(filepath.Join(buildOutputDir, "package-metadata.tar")).To(BeARegularFile())
			Expect(filepath.Join(buildOutputDir, "chaincode-source.tar")).To(BeARegularFile())

			By("releasing")
			releaseOutputDir, err := ioutil.TempDir(testDir, "release-output")
			Expect(err).NotTo(HaveOccurred())
			releaseCommand := exec.Command(
				filepath.Join(buildpackBinPath, "release"),
				buildOutputDir,
				releaseOutputDir,
			)
			releaseCommand.Env = append(os.Environ(), builderClientEnv...)
			releaseSess, err := gexec.Start(releaseCommand, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(releaseSess, 5*time.Second).Should(gexec.Exit(0))
			Expect(filepath.Join(releaseOutputDir, "statedb", "couchdb", "indexes", "indexColorSortDoc.json")).To(BeARegularFile())
			Expect(filepath.Join(releaseOutputDir, "statedb", "couchdb", "indexes", "indexSizeSortDoc.json")).To(BeARegularFile())

			By("running")
			lis, err := net.Listen("tcp", ":0")
			Expect(err).NotTo(HaveOccurred())
			_, port, err := net.SplitHostPort(lis.Addr().String())
			Expect(err).NotTo(HaveOccurred())

			rmd := newRunMetadata(net.JoinHostPort("host.docker.internal", port))
			peerMetadataDir, err := ioutil.TempDir(testDir, "peer-metadata")
			Expect(err).NotTo(HaveOccurred())
			md, err := json.Marshal(rmd)
			Expect(err).NotTo(HaveOccurred())
			err = ioutil.WriteFile(filepath.Join(peerMetadataDir, "chaincode.json"), md, 0644)
			Expect(err).NotTo(HaveOccurred())

			grpcServer := newGRPCServer(rmd)
			fakeChaincodeSupport := &fakes.ChaincodeSupportServer{}
			registerComplete := make(chan struct{})
			registerResult := make(chan error, 1)
			fakeChaincodeSupport.RegisterStub = func(stream pb.ChaincodeSupport_RegisterServer) error {
				msg, err := stream.Recv()
				if err == io.EOF {
					return errors.New("unexpected stream termination")
				}
				if err != nil {
					return fmt.Errorf("receive failed: %s", err)
				}
				if msg.Type != pb.ChaincodeMessage_REGISTER {
					return fmt.Errorf("unexpected message type: %v", msg.Type)
				}
				err = stream.Send(&pb.ChaincodeMessage{Type: pb.ChaincodeMessage_REGISTERED})
				if err != nil {
					return err
				}

				close(registerComplete)
				return <-registerResult
			}
			pb.RegisterChaincodeSupportServer(grpcServer, fakeChaincodeSupport)

			grpcServeDone := make(chan error, 1)
			go func() { grpcServeDone <- grpcServer.Serve(lis) }()

			runCommand := exec.Command(
				filepath.Join(buildpackBinPath, "run"),
				buildOutputDir,
				peerMetadataDir,
			)
			runCommand.Env = append(os.Environ(), builderClientEnv...)
			runSess, err := gexec.Start(runCommand, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(registerComplete, time.Minute).Should(BeClosed())

			Consistently(runSess).ShouldNot(gexec.Exit())
			close(registerResult)
			Eventually(runSess, time.Minute).Should(gexec.Exit())

			grpcServer.Stop()
			Eventually(grpcServeDone).Should(Receive(BeNil()))
		},

		Entry("go chaincode", "go-chaincode", []string{
			"chaincode",
		}),
		Entry("java chaincode", "java-chaincode", []string{
			".uberjar",
			"chaincode.jar",
		}),
		Entry("node chaincode", "node-chaincode", []string{
			"chaincode.js",
			"node_modules/@grpc/",
			"package-lock.json",
			"package.json",
		}),
	)
})

func dialEndpoint(address string) func() error {
	return func() error {
		conn, err := net.Dial("tcp", address)
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	}
}

func tarEntryNames(archive string) []string {
	f, err := os.Open(archive)
	Expect(err).NotTo(HaveOccurred())
	defer f.Close()

	var names []string
	tr := tar.NewReader(f)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		Expect(err).NotTo(HaveOccurred())
		names = append(names, hdr.Name)
	}

	return names
}

type runMetadata struct {
	ChaincodeID string `json:"chaincode_id"`
	ClientCert  string `json:"client_cert"`
	ClientKey   string `json:"client_key"`
	RootCert    string `json:"root_cert"`
	PeerAddress string `json:"peer_address"`
	ServerCert  string `json:"-"`
	ServerKey   string `json:"-"`
}

func newRunMetadata(peerAddress string) *runMetadata {
	peerHost, _, err := net.SplitHostPort(peerAddress)
	Expect(err).NotTo(HaveOccurred())
	caCert, caKey := generateCA("test-ca")
	serverCert, serverKey := issueCertificate(caCert, caKey, "server", peerHost)
	clientCert, clientKey := issueCertificate(caCert, caKey, "client", "127.0.0.1")

	return &runMetadata{
		ChaincodeID: "test-chaincode-id",
		RootCert:    string(caCert),
		PeerAddress: peerAddress,
		ClientCert:  string(clientCert),
		ClientKey:   string(clientKey),
		ServerCert:  string(serverCert),
		ServerKey:   string(serverKey),
	}
}

func newGRPCServer(rmd *runMetadata) *grpc.Server {
	peerCertWithKey, err := tls.X509KeyPair([]byte(rmd.ServerCert), []byte(rmd.ServerKey))
	Expect(err).NotTo(HaveOccurred())

	caCertPool := x509.NewCertPool()
	added := caCertPool.AppendCertsFromPEM([]byte(rmd.RootCert))
	Expect(added).To(BeTrue())
	serverTLSConfig := &tls.Config{
		Certificates: []tls.Certificate{peerCertWithKey},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caCertPool,
		RootCAs:      caCertPool,
	}
	serverTLSConfig.BuildNameToCertificate()
	return grpc.NewServer(
		grpc.Creds(credentials.NewTLS(serverTLSConfig)),
	)
}

func newCertificateTemplate(subjectCN string, hosts ...string) x509.Certificate {
	notBefore := time.Now().Add(-1 * time.Minute)
	notAfter := time.Now().Add(time.Duration(365 * 24 * time.Hour))

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	Expect(err).NotTo(HaveOccurred())

	template := x509.Certificate{
		Subject:               pkix.Name{CommonName: subjectCN},
		SerialNumber:          serialNumber,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	return template
}

func pemEncode(derCert []byte, key *ecdsa.PrivateKey) (pemCert, pemKey []byte) {
	certBuf := &bytes.Buffer{}
	err := pem.Encode(certBuf, &pem.Block{Type: "CERTIFICATE", Bytes: derCert})
	Expect(err).NotTo(HaveOccurred())

	keyBytes, err := x509.MarshalPKCS8PrivateKey(key) // Java requires PKCS#8
	Expect(err).NotTo(HaveOccurred())

	keyBuf := &bytes.Buffer{}
	err = pem.Encode(keyBuf, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
	Expect(err).NotTo(HaveOccurred())

	return certBuf.Bytes(), keyBuf.Bytes()
}

func generateCA(subjectCN string) (pemCert, pemKey []byte) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	Expect(err).NotTo(HaveOccurred())
	publicKey := privateKey.Public()

	template := newCertificateTemplate(subjectCN)
	template.KeyUsage |= x509.KeyUsageCertSign
	template.IsCA = true

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, publicKey, privateKey)
	Expect(err).NotTo(HaveOccurred())

	return pemEncode(derBytes, privateKey)
}

func issueCertificate(caCert, caKey []byte, subjectCN string, hosts ...string) (pemCert, pemKey []byte) {
	tlsCert, err := tls.X509KeyPair(caCert, caKey)
	Expect(err).NotTo(HaveOccurred())

	ca, err := x509.ParseCertificate(tlsCert.Certificate[0])
	Expect(err).NotTo(HaveOccurred())

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	Expect(err).NotTo(HaveOccurred())
	publicKey := privateKey.Public()

	template := newCertificateTemplate(subjectCN, hosts...)
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, ca, publicKey, tlsCert.PrivateKey)
	Expect(err).NotTo(HaveOccurred())

	return pemEncode(derBytes, privateKey)
}
