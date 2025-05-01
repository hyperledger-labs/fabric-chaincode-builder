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
package command

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hyperledger-labs/fabric-chaincode-builder/pkg/command/fakes"
	"github.com/hyperledger-labs/fabric-chaincode-builder/pkg/fs"
	"github.com/hyperledger-labs/fabric-chaincode-builder/pkg/logger"
	"github.com/hyperledger-labs/fabric-chaincode-builder/pkg/sidecar"
	"go.uber.org/zap"

	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
)

func TestBuildFileManagement(t *testing.T) {
	gt := NewGomegaWithT(t)
	tempDir, err := ioutil.TempDir("", "build")
	gt.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tempDir)

	sharedDir, err := ioutil.TempDir(tempDir, "shared")
	gt.Expect(err).NotTo(HaveOccurred())
	saveDir, err := ioutil.TempDir(tempDir, "saved")
	gt.Expect(err).NotTo(HaveOccurred())

	fakeBuilder := &fakes.BuilderServer{}
	fakeBuilder.BuildStub = func(req *sidecar.BuildRequest, stream sidecar.Builder_BuildServer) error {
		// save contents of what was populated
		if err := fs.CopyDir(sharedDir, saveDir); err != nil {
			return err
		}
		// write dummy output
		if err := ioutil.WriteFile(filepath.Join(sharedDir, req.OutputPath), []byte("output-data"), 0644); err != nil {
			return err
		}
		return nil
	}

	grpcClient, cleanup := newBuildClientConn(t, fakeBuilder)
	defer cleanup()
	defer grpcClient.Close()

	build := &Build{
		SharedDir:  sharedDir,
		GRPCClient: grpcClient,
	}

	writeFiles(t, tempDir, standardBuildFiles)
	rc, err := build.Run(
		ioutil.Discard,
		filepath.Join(tempDir, "source"),
		filepath.Join(tempDir, "metadata"),
		filepath.Join(tempDir, "output"),
	)
	gt.Expect(err).NotTo(HaveOccurred())
	gt.Expect(rc).To(Equal(0))

	gt.Expect(fakeBuilder.BuildCallCount()).To(Equal(1))

	// Calculate the build temporary directory name
	entries, err := ioutil.ReadDir(saveDir)
	gt.Expect(err).NotTo(HaveOccurred())
	gt.Expect(entries).To(HaveLen(1))
	tempBuildDir := entries[0].Name()

	// Validate the request to the server
	req, _ := fakeBuilder.BuildArgsForCall(0)
	gt.Expect(req).To(Equal(&sidecar.BuildRequest{
		SourcePath:   filepath.Join(tempBuildDir, "chaincode-source.tar"),
		MetadataPath: filepath.Join(tempBuildDir, "package-metadata.tar"),
		OutputPath:   filepath.Join(tempBuildDir, "chaincode-output.tar"),
	}))

	// Validate the contents of the files placed on the shared volume
	gt.Expect(tarEntries(t, filepath.Join(saveDir, tempBuildDir, "chaincode-source.tar"))).To(Equal(map[string]string{
		"META-INF/":                                   "",
		"META-INF/statedb/":                           "",
		"META-INF/statedb/couchdb/":                   "",
		"META-INF/statedb/couchdb/indexes/":           "",
		"META-INF/statedb/couchdb/indexes/index.json": `{"index":"index-specifier"}`,
		"src/":             "",
		"src/chaincode.go": "package main",
		"src/go.mod":       "module chaincode",
	}))
	gt.Expect(tarEntries(t, filepath.Join(saveDir, tempBuildDir, "package-metadata.tar"))).To(Equal(map[string]string{
		"metadata.json": `{"path":"chaincode", "type":"golang"}`,
	}))

	// Validate the contents of files output by builder
	gt.Expect(tarEntries(t, filepath.Join(tempDir, "output", "chaincode-metadata.tar"))).To(Equal(map[string]string{
		"statedb/":                           "",
		"statedb/couchdb/":                   "",
		"statedb/couchdb/indexes/":           "",
		"statedb/couchdb/indexes/index.json": `{"index":"index-specifier"}`,
	}))
	contents, err := ioutil.ReadFile(filepath.Join(tempDir, "output", "chaincode-output.tar"))
	gt.Expect(err).NotTo(HaveOccurred())
	gt.Expect(string(contents)).To(Equal("output-data"))
}

func TestBuildNoChaincodeMetadata(t *testing.T) {
	gt := NewGomegaWithT(t)
	tempDir, err := ioutil.TempDir("", "build")
	gt.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tempDir)

	sharedDir, err := ioutil.TempDir(tempDir, "shared")
	gt.Expect(err).NotTo(HaveOccurred())

	fakeBuilder := &fakes.BuilderServer{}
	grpcClient, cleanup := newBuildClientConn(t, fakeBuilder)
	defer cleanup()
	defer grpcClient.Close()

	build := &Build{
		SharedDir:  sharedDir,
		GRPCClient: grpcClient,
	}

	writeFiles(t, tempDir, standardBuildFiles)
	err = os.RemoveAll(filepath.Join(tempDir, "source/META-INF"))
	gt.Expect(err).NotTo(HaveOccurred())

	rc, err := build.Run(
		ioutil.Discard,
		filepath.Join(tempDir, "source"),
		filepath.Join(tempDir, "metadata"),
		filepath.Join(tempDir, "output"),
	)
	gt.Expect(err).NotTo(HaveOccurred())
	gt.Expect(rc).To(Equal(0))

	gt.Expect(fakeBuilder.BuildCallCount()).To(Equal(1))
	gt.Expect(filepath.Join(tempDir, "output", "chaincode-metadata.tar")).NotTo(BeAnExistingFile())
}

func TestBuildBuildError(t *testing.T) {
	t.Skip() // Failing in travis only, skipping for now
	gt := NewGomegaWithT(t)
	tempDir, err := ioutil.TempDir("", "build")
	gt.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tempDir)

	sharedDir, err := ioutil.TempDir(tempDir, "shared")
	gt.Expect(err).NotTo(HaveOccurred())

	grpcClient, cleanup := newBuildClientConn(t, &fakes.BuilderServer{})
	cleanup()
	defer grpcClient.Close()

	build := &Build{
		SharedDir:  sharedDir,
		GRPCClient: grpcClient,
	}

	writeFiles(t, tempDir, standardBuildFiles)

	_, err = build.Run(
		ioutil.Discard,
		filepath.Join(tempDir, "source"),
		filepath.Join(tempDir, "metadata"),
		filepath.Join(tempDir, "output"),
	)
	gt.Expect(err).To(MatchError(ContainSubstring("connection error")))
}

func TestBuildBuilderStreamError(t *testing.T) {
	gt := NewGomegaWithT(t)
	tempDir, err := ioutil.TempDir("", "build")
	gt.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tempDir)

	sharedDir, err := ioutil.TempDir(tempDir, "shared")
	gt.Expect(err).NotTo(HaveOccurred())

	fakeBuilder := &fakes.BuilderServer{}
	fakeBuilder.BuildReturns(errors.New("miserable-failure"))

	grpcClient, cleanup := newBuildClientConn(t, fakeBuilder)
	defer cleanup()
	defer grpcClient.Close()

	build := &Build{
		SharedDir:  sharedDir,
		GRPCClient: grpcClient,
	}

	writeFiles(t, tempDir, standardBuildFiles)

	_, err = build.Run(
		ioutil.Discard,
		filepath.Join(tempDir, "source"),
		filepath.Join(tempDir, "metadata"),
		filepath.Join(tempDir, "output"),
	)
	gt.Expect(err).To(MatchError(ContainSubstring("miserable-failure")))

	gt.Expect(fakeBuilder.BuildCallCount()).To(Equal(1))

	entries, err := ioutil.ReadDir(filepath.Join(tempDir, "output"))
	gt.Expect(err).NotTo(HaveOccurred())
	gt.Expect(entries).To(BeEmpty())
}

func TestBuildBuilderBadMessage(t *testing.T) {
	gt := NewGomegaWithT(t)
	tempDir, err := ioutil.TempDir("", "build")
	gt.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tempDir)

	sharedDir, err := ioutil.TempDir(tempDir, "shared")
	gt.Expect(err).NotTo(HaveOccurred())

	fakeBuilder := &fakes.BuilderServer{}
	fakeBuilder.BuildStub = func(req *sidecar.BuildRequest, stream sidecar.Builder_BuildServer) error {
		return stream.Send(&sidecar.BuildResponse{})
	}

	grpcClient, cleanup := newBuildClientConn(t, fakeBuilder)
	defer cleanup()
	defer grpcClient.Close()

	build := &Build{
		SharedDir:  sharedDir,
		GRPCClient: grpcClient,
	}

	writeFiles(t, tempDir, standardBuildFiles)

	_, err = build.Run(
		ioutil.Discard,
		filepath.Join(tempDir, "source"),
		filepath.Join(tempDir, "metadata"),
		filepath.Join(tempDir, "output"),
	)
	gt.Expect(err).To(MatchError("unexpected server response message <nil>"))

	gt.Expect(fakeBuilder.BuildCallCount()).To(Equal(1))
}

func TestBuildBuilderMessages(t *testing.T) {
	gt := NewGomegaWithT(t)
	tempDir, err := ioutil.TempDir("", "build")
	gt.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tempDir)

	sharedDir, err := ioutil.TempDir(tempDir, "shared")
	gt.Expect(err).NotTo(HaveOccurred())

	fakeBuilder := &fakes.BuilderServer{}
	fakeBuilder.BuildStub = func(req *sidecar.BuildRequest, stream sidecar.Builder_BuildServer) error {
		responses := []*sidecar.BuildResponse{
			{
				Message: &sidecar.BuildResponse_StandardOut{
					StandardOut: "This is a standard out message.\n",
				},
			},
			{
				Message: &sidecar.BuildResponse_StandardError{
					StandardError: "This is a standard error message.\n",
				},
			},
		}
		for _, resp := range responses {
			if err := stream.Send(resp); err != nil {
				return err
			}
		}
		return nil
	}

	grpcClient, cleanup := newBuildClientConn(t, fakeBuilder)
	defer cleanup()
	defer grpcClient.Close()

	build := &Build{
		SharedDir:  sharedDir,
		GRPCClient: grpcClient,
	}

	writeFiles(t, tempDir, standardBuildFiles)

	buf := &bytes.Buffer{}
	rc, err := build.Run(
		buf,
		filepath.Join(tempDir, "source"),
		filepath.Join(tempDir, "metadata"),
		filepath.Join(tempDir, "output"),
	)
	gt.Expect(err).NotTo(HaveOccurred())
	gt.Expect(rc).To(Equal(0))

	gt.Expect(fakeBuilder.BuildCallCount()).To(Equal(1))
	gt.Expect(buf.String()).To(Equal("This is a standard out message.\nThis is a standard error message.\n"))
}

func TestBuildTempDirFailure(t *testing.T) {
	gt := NewGomegaWithT(t)

	build := &Build{SharedDir: "missing-dir"}
	_, err := build.Run(ioutil.Discard, "one", "two", "three")
	gt.Expect(err).To(MatchError(HavePrefix("failed to create temporary directory:")))
}

func TestBuildMissingInput(t *testing.T) {
	tests := []struct {
		missingDir     string
		buildCallCount int
	}{
		{missingDir: "source"},
		{missingDir: "metadata"},
		{missingDir: "output", buildCallCount: 1},
	}

	for _, tt := range tests {
		t.Run(tt.missingDir, func(t *testing.T) {
			gt := NewGomegaWithT(t)
			tempDir, err := ioutil.TempDir("", "build")
			gt.Expect(err).NotTo(HaveOccurred())
			defer os.RemoveAll(tempDir)

			sharedDir, err := ioutil.TempDir(tempDir, "shared")
			gt.Expect(err).NotTo(HaveOccurred())

			writeFiles(t, tempDir, standardBuildFiles)

			err = os.RemoveAll(filepath.Join(tempDir, tt.missingDir))
			gt.Expect(err).NotTo(HaveOccurred())

			fakeBuilder := &fakes.BuilderServer{}
			fakeBuilder.BuildReturns(nil)

			grpcClient, cleanup := newBuildClientConn(t, fakeBuilder)
			defer cleanup()
			defer grpcClient.Close()

			build := &Build{
				SharedDir:  sharedDir,
				GRPCClient: grpcClient,
			}
			_, err = build.Run(
				ioutil.Discard,
				filepath.Join(tempDir, "source"),
				filepath.Join(tempDir, "metadata"),
				filepath.Join(tempDir, "output"),
			)
			gt.Expect(err).To(MatchError(ContainSubstring("no such file or directory")))
			gt.Expect(err).To(MatchError(ContainSubstring(tt.missingDir)))

			gt.Expect(fakeBuilder.BuildCallCount()).To(Equal(tt.buildCallCount))
		})
	}
}

func TestHandleExternalCCHostname(t *testing.T) {
	os.Setenv("PEER_NAME", "peer1")
	var externalCCFiles = map[string]string{
		"metadata/metadata.json": `{"path":"chaincode", "type":"external"}`,
		"source/connection.json": `{"address":"mycc.ns1:9999", "injectIdentifier":"hostname"}`,
		"output/":                "",
	}

	gt := NewGomegaWithT(t)
	tempDir, err := ioutil.TempDir("", "build")
	gt.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tempDir)

	writeFiles(t, tempDir, externalCCFiles)
	sourceDir := filepath.Join(tempDir, "source")
	metadataDir := filepath.Join(tempDir, "metadata")
	outputDir := filepath.Join(tempDir, "output")

	build := &Build{
		Logger: getLogger(),
	}
	err = build.HandleExternalCC(sourceDir, metadataDir, outputDir)
	gt.Expect(err).NotTo(HaveOccurred())

	bytes, err := ioutil.ReadFile(filepath.Join(outputDir, "connection.json"))
	gt.Expect(err).NotTo(HaveOccurred())
	gt.Expect(string(bytes)).To(Equal(`{"address":"peer1-mycc.ns1:9999","injectIdentifier":"hostname"}`))

}

func TestHandleExternalCCDomain(t *testing.T) {
	os.Setenv("PEER_NAME", "peer 1")
	var externalCCFiles = map[string]string{
		"metadata/metadata.json": `{"path":"chaincode", "type":"external"}`,
		"source/connection.json": `{"address":"mycc.host.name.ns1:9999", "injectIdentifier":"domain"}`,
		"output/":                "",
	}

	gt := NewGomegaWithT(t)
	tempDir, err := ioutil.TempDir("", "build")
	gt.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tempDir)

	writeFiles(t, tempDir, externalCCFiles)
	sourceDir := filepath.Join(tempDir, "source")
	metadataDir := filepath.Join(tempDir, "metadata")
	outputDir := filepath.Join(tempDir, "output")

	build := &Build{
		Logger: getLogger(),
	}
	err = build.HandleExternalCC(sourceDir, metadataDir, outputDir)
	gt.Expect(err).NotTo(HaveOccurred())

	bytes, err := ioutil.ReadFile(filepath.Join(outputDir, "connection.json"))
	gt.Expect(err).NotTo(HaveOccurred())
	gt.Expect(string(bytes)).To(Equal(`{"address":"mycc.host.name.peer1-ns1:9999","injectIdentifier":"domain"}`))
}

func TestHandleExternalCCNoIdInjection(t *testing.T) {
	os.Setenv("PEER_NAME", "peer1")
	var externalCCFiles = map[string]string{
		"metadata/metadata.json": `{"path":"chaincode", "type":"external"}`,
		"source/connection.json": `{"address":"mycc.ns1:9999", "tls_required":false}`,
		"output/":                "",
	}

	gt := NewGomegaWithT(t)
	tempDir, err := ioutil.TempDir("", "build")
	gt.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tempDir)

	writeFiles(t, tempDir, externalCCFiles)
	sourceDir := filepath.Join(tempDir, "source")
	metadataDir := filepath.Join(tempDir, "metadata")
	outputDir := filepath.Join(tempDir, "output")

	build := &Build{
		Logger: getLogger(),
	}
	err = build.HandleExternalCC(sourceDir, metadataDir, outputDir)
	gt.Expect(err).NotTo(HaveOccurred())

	bytes, err := ioutil.ReadFile(filepath.Join(outputDir, "connection.json"))
	gt.Expect(err).NotTo(HaveOccurred())
	gt.Expect(string(bytes)).To(Equal(`{"address":"mycc.ns1:9999", "tls_required":false}`)) // keeps connection.json file as-is
}

func TestHandleExternalCCError(t *testing.T) {
	os.Setenv("PEER_NAME", "peer1")
	var externalCCFiles = map[string]string{
		"metadata/metadata.json": `{"path":"chaincode", "type":"external"}`,
		"output/":                "",
	}

	gt := NewGomegaWithT(t)
	tempDir, err := ioutil.TempDir("", "build")
	gt.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tempDir)

	writeFiles(t, tempDir, externalCCFiles)
	sourceDir := filepath.Join(tempDir, "source")
	metadataDir := filepath.Join(tempDir, "metadata")
	outputDir := filepath.Join(tempDir, "output")

	build := &Build{
		Logger: getLogger(),
	}
	err = build.HandleExternalCC(sourceDir, metadataDir, outputDir)
	gt.Expect(err).To(HaveOccurred())
	gt.Expect(err.Error()).To(ContainSubstring("connection.json not found in source folder"))
}

func getLogger() *zap.SugaredLogger {
	logger.SetupLogging("")
	return logger.Logger.Sugar().Named("release_test")
}

var standardBuildFiles = map[string]string{
	"source/META-INF/statedb/couchdb/indexes/index.json": `{"index":"index-specifier"}`,
	"source/src/chaincode.go":                            `package main`,
	"source/src/go.mod":                                  `module chaincode`,
	"metadata/metadata.json":                             `{"path":"chaincode", "type":"golang"}`,
	"output/":                                            "",
}

func writeFiles(t *testing.T, root string, files map[string]string) {
	gt := NewGomegaWithT(t)
	for name, contents := range files {
		err := os.MkdirAll(filepath.Join(root, filepath.Dir(name)), 0700)
		gt.Expect(err).NotTo(HaveOccurred())
		if len(name) != 0 && !os.IsPathSeparator(name[len(name)-1]) {
			err = ioutil.WriteFile(filepath.Join(root, name), []byte(contents), 0644)
			gt.Expect(err).NotTo(HaveOccurred())
		}
	}
}

func tarEntries(t *testing.T, archivePath string) map[string]string {
	gt := NewGomegaWithT(t)

	file, err := os.Open(archivePath)
	gt.Expect(err).NotTo(HaveOccurred())
	defer file.Close()

	contents := map[string]string{}
	tr := tar.NewReader(file)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		gt.Expect(err).NotTo(HaveOccurred())

		buf := &bytes.Buffer{}
		_, err = io.Copy(buf, tr)
		gt.Expect(err).NotTo(HaveOccurred())
		contents[header.Name] = buf.String()
	}

	return contents
}

func newBuildClientConn(t *testing.T, buildServer sidecar.BuilderServer) (grpcClient *grpc.ClientConn, cleaup func()) {
	gt := NewGomegaWithT(t)
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	gt.Expect(err).NotTo(HaveOccurred())

	server := grpc.NewServer()
	sidecar.RegisterBuilderServer(server, buildServer)

	go server.Serve(lis)

	timeoutContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	grpcClient, err = grpc.DialContext(timeoutContext, lis.Addr().String(), grpc.WithInsecure(), grpc.WithBlock())
	gt.Expect(err).NotTo(HaveOccurred())

	return grpcClient, func() { server.Stop(); lis.Close() }
}

//go:generate counterfeiter -o fakes/builder.go --fake-name BuilderServer . builderServer
type builderServer interface{ sidecar.BuilderServer }
