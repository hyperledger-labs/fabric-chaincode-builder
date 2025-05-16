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
package main

import (
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	tlsgen "github.com/hyperledger-labs/fabric-chaincode-builder/pkg/tls"

	"github.com/hyperledger-labs/fabric-chaincode-builder/pkg/filetransfer"
	"github.com/hyperledger-labs/fabric-chaincode-builder/pkg/k8sbuilder"
	"github.com/hyperledger-labs/fabric-chaincode-builder/pkg/logger"
	"github.com/hyperledger-labs/fabric-chaincode-builder/pkg/pod"
	"github.com/hyperledger-labs/fabric-chaincode-builder/pkg/sidecar"

	"google.golang.org/grpc"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var kubeconfigPath = flag.String(
	"kubeconfig",
	"",
	"(optional) path to the kubeconfig file",
)

// k8s namespace to use; default to contents of
// /var/run/secrets/kubernetes.io/serviceaccount/namespace
var kubeNamespace = flag.String(
	"kubeNamespace",
	"",
	"(optional) kubernetes namespace",
)

var clientTimeout = flag.Duration(
	"clientTimeout",
	5*time.Minute,
	"kubernetes client timeout",
)

var peerID = flag.String(
	"peerID",
	"unknown-peer",
	"unique peer identifer used to label builder pods",
)

var sharedVolumePath = flag.String(
	"sharedVolumePath",
	".",
	"directory where where chaincode asssets are placed by the client",
)

var fileServerListenAddress = flag.String(
	"fileServerListenAddress",
	":22222",
	"file server listen address",
)

var fileServerBaseURL = flag.String(
	"fileServerBaseURL",
	"",
	"(optional) file server base URL",
)

var sidecarListenAddress = flag.String(
	"sidecarListenAddress",
	"127.0.0.1:11111",
	"sidecar server listen address",
)

func main() {
	flag.Parse()

	logLevel := os.Getenv("LOGLEVEL")
	logger.SetupLogging(logLevel)

	log := logger.Logger.Sugar()
	log.Info("Log level set to: ", logger.GetLogLevel(logLevel).CapitalString())

	username := GenerateRandomString(16)
	password := GenerateRandomString(16)

	kubeconfig := k8sConfigPath()
	clientConfig, err := restClientConfig(kubeconfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to establish connection to kubernetes: %s", err)
		os.Exit(10)
	}
	clientConfig.Timeout = *clientTimeout

	clientSet, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to create client set: %s", err)
		os.Exit(11)
	}

	handler := pod.NewHandler(clientSet.CoreV1().Pods(k8sNamespace()))
	pods, err := handler.ListCompletedExecutionPodsForPeer(*peerID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable get completed pods for peer '%s': %s", *peerID, err)
	}

	err = handler.DeletePods(pods, metav1.DeleteOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable get delete completed pods for peer '%s': %s", *peerID, err)
	}

	tlsConfig, _, tlsCertBytes, err := GenerateTLSCrypto(*fileServerBaseURL, *sharedVolumePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to generate tls crypto: %s", err)
		os.Exit(21)
	}

	buildServer := &k8sbuilder.Server{
		SecretClient:      clientSet.CoreV1().Secrets(k8sNamespace()),
		PodClient:         clientSet.CoreV1().Pods(k8sNamespace()),
		PeerID:            *peerID,
		SharedPath:        *sharedVolumePath,
		FSBaseURL:         fsBaseURL(),
		FileTransferImage: getEnvVar("FILETRANSFERIMAGE"),
		BuilderImage:      getEnvVar("BUILDERIMAGE"),
		GoEnvImage:        getEnvVar("GOENVIMAGE"),
		JavaEnvImage:      getEnvVar("JAVAENVIMAGE"),
		NodeEnvImage:      getEnvVar("NODEENVIMAGE"),
		ImagePullSecrets:  parseListEnv(getEnvVar("IMAGEPULLSECRETS")),
		Auth: &k8sbuilder.BasicAuth{
			Username: username,
			Password: password,
		},
		TLSCertString: strings.Join(strings.Split(string(tlsCertBytes), "\n"), "\\n"),
	}

	log.Infof("Build server configuration: %+v\n", buildServer)

	sidecarListener, err := net.Listen("tcp", *sidecarListenAddress)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to listen on sidecar listen address: %s", err)
		os.Exit(12)
	}

	sidecarServer := grpc.NewServer()
	sidecar.RegisterBuilderServer(sidecarServer, buildServer)
	sidecar.RegisterRunnerServer(sidecarServer, buildServer)

	go sidecarServer.Serve(sidecarListener)

	httpServer := &http.Server{
		Addr:              *fileServerListenAddress,
		Handler:           &filetransfer.Handler{Root: *sharedVolumePath, Username: username, Password: password},
		TLSConfig:         tlsConfig,
		ReadHeaderTimeout: 5 * time.Second,
	}

	fileServerListener, err := net.Listen("tcp", *fileServerListenAddress)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to listen on file server listen address: %s", err)
		os.Exit(13)
	}
	go httpServer.ServeTLS(fileServerListener, *sharedVolumePath+"/certs/cert.pem", *sharedVolumePath+"/certs/key.pem")

	select {}
}

func restClientConfig(configPath string) (*rest.Config, error) {
	if configPath == "" {
		return rest.InClusterConfig()
	}
	return clientcmd.BuildConfigFromFlags("", configPath)
}

func k8sConfigPath() string {
	// check the explict command line flag
	if *kubeconfigPath != "" {
		return *kubeconfigPath
	}

	// check the environment variable
	if kubeconfig, ok := os.LookupEnv("KUBECONFIG"); ok {
		return kubeconfig
	}

	// check the home directory
	if home, err := os.UserHomeDir(); err == nil {
		config := filepath.Join(home, ".kube", "config")
		if _, err := os.Stat(config); err == nil {
			return config
		}
	}

	return ""
}

func k8sNamespace() string {
	if *kubeNamespace != "" {
		return *kubeNamespace
	}
	if ns, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		return string(ns)
	}
	return "default"
}

func fsBaseURL() string {
	if *fileServerBaseURL != "" {
		return *fileServerBaseURL
	}
	return "https://" + *fileServerListenAddress + "/"
}

func getEnvVar(name string) string {
	value := os.Getenv(name)
	if value == "" {
		fmt.Fprintf(os.Stderr, "no %s specified", name)
		os.Exit(1)
	}
	return value
}

// Expected format of list env is space delimited, (e.g. name1 name2)
func parseListEnv(list string) []string {
	return strings.Split(list, " ")
}

func GenerateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz" +
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	b := make([]byte, length)
	for i := range b {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[num.Int64()]
	}
	return string(b)
}

func GenerateTLSCrypto(fileServerBaseURL, sharedDir string) (*tls.Config, []byte, []byte, error) {
	u, err := url.Parse(fileServerBaseURL)
	if err != nil {
		return nil, nil, nil, err
	}

	params := tlsgen.Param{
		Hosts:    []string{u.Hostname()},
		NotAfter: time.Now().Add(time.Hour * 8766), // 1 year
	}
	certgen := &tlsgen.Generate{}
	tlsKey, tlsKeyBytes, err := certgen.PrivateKey(elliptic.P256())
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to generate tlskey: %s", err)
		return nil, nil, nil, err
	}
	tlsCertBytes, err := certgen.SelfSignedCert(params, tlsKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to generate tlscert: %s", err)
		return nil, nil, nil, err
	}
	tlsConfig, err := tlsgen.ConfigTLS(tlsCertBytes, tlsKeyBytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse TLS configuration: %s", err)
		return nil, nil, nil, err
	}

	err = os.MkdirAll(sharedDir+"/certs", 0750)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create certs dir : %s", err)
		return nil, nil, nil, err
	}

	err = os.WriteFile(sharedDir+"/certs/cert.pem", tlsCertBytes, 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to save cert : %s", err)
		return nil, nil, nil, err
	}

	err = os.WriteFile(sharedDir+"/certs/key.pem", tlsKeyBytes, 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to save key : %s", err)
		return nil, nil, nil, err
	}

	return tlsConfig, tlsKeyBytes, tlsCertBytes, nil
}
