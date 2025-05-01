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
	"bytes"
	"context"
	"errors"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hyperledger-labs/fabric-chaincode-builder/pkg/command/fakes"
	"github.com/hyperledger-labs/fabric-chaincode-builder/pkg/fs"
	"github.com/hyperledger-labs/fabric-chaincode-builder/pkg/sidecar"

	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
)

func TestRunFileManagement(t *testing.T) {
	gt := NewGomegaWithT(t)
	tempDir, err := ioutil.TempDir("", "run")
	gt.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tempDir)

	sharedDir, err := ioutil.TempDir(tempDir, "shared")
	gt.Expect(err).NotTo(HaveOccurred())
	saveDir, err := ioutil.TempDir(tempDir, "saved")
	gt.Expect(err).NotTo(HaveOccurred())

	fakeRunner := &fakes.RunnerServer{}
	fakeRunner.RunStub = func(req *sidecar.RunRequest, stream sidecar.Runner_RunServer) error {
		// save contents of what was populated
		return fs.CopyDir(sharedDir, saveDir)
	}

	grpcClient, cleanup := newRunnerClientConn(t, fakeRunner)
	defer cleanup()
	defer grpcClient.Close()

	run := &Run{
		SharedDir:  sharedDir,
		GRPCClient: grpcClient,
	}

	writeFiles(t, tempDir, standardRunFiles)
	outputDir := filepath.Join(tempDir, "output")
	metadataDir := filepath.Join(tempDir, "metadata")

	rc, err := run.Run(ioutil.Discard, outputDir, metadataDir)
	gt.Expect(err).NotTo(HaveOccurred())
	gt.Expect(rc).To(Equal(0))

	gt.Expect(fakeRunner.RunCallCount()).To(Equal(1))

	// Calculate the run temporary directory name
	entries, err := ioutil.ReadDir(saveDir)
	gt.Expect(err).NotTo(HaveOccurred())
	gt.Expect(entries).To(HaveLen(1))

	// Validate the request to the server
	tempRunDir := entries[0].Name()
	req, _ := fakeRunner.RunArgsForCall(0)
	gt.Expect(req).To(Equal(&sidecar.RunRequest{
		ChaincodePath: filepath.Join(tempRunDir, "chaincode-output.tar"),
		MetadataPath:  filepath.Join(tempRunDir, "package-metadata.tar"),
		ArtifactsPath: filepath.Join(tempRunDir, "chaincode.json"),
	}))

	// Validate teh contents of the files placed on the shraed volume
	contents, err := ioutil.ReadFile(filepath.Join(saveDir, tempRunDir, "chaincode-output.tar"))
	gt.Expect(err).NotTo(HaveOccurred())
	gt.Expect(string(contents)).To(Equal("bogus-tar"))
	contents, err = ioutil.ReadFile(filepath.Join(saveDir, tempRunDir, "chaincode.json"))
	gt.Expect(err).NotTo(HaveOccurred())
	gt.Expect(string(contents)).To(Equal("bogus-chaincode-json"))
}

func TestRunMissingInput(t *testing.T) {
	tests := []struct {
		missingFile string
	}{
		{missingFile: "output/chaincode-output.tar"},
		{missingFile: "metadata/chaincode.json"},
	}

	for _, tt := range tests {
		t.Run(tt.missingFile, func(t *testing.T) {
			gt := NewGomegaWithT(t)
			tempDir, err := ioutil.TempDir("", "run")
			gt.Expect(err).NotTo(HaveOccurred())
			defer os.RemoveAll(tempDir)

			writeFiles(t, tempDir, standardRunFiles)
			outputDir := filepath.Join(tempDir, "output")
			metadataDir := filepath.Join(tempDir, "metadata")

			err = os.Remove(filepath.Join(tempDir, tt.missingFile))
			gt.Expect(err).NotTo(HaveOccurred())

			run := &Run{SharedDir: tempDir}
			_, err = run.Run(ioutil.Discard, outputDir, metadataDir)
			gt.Expect(err).To(MatchError(ContainSubstring("no such file or directory")))
			gt.Expect(err).To(MatchError(ContainSubstring(tt.missingFile)))
		})
	}
}

func TestRunTempDirFailure(t *testing.T) {
	gt := NewGomegaWithT(t)

	run := &Run{SharedDir: "missing-dir"}
	_, err := run.Run(ioutil.Discard, "one", "two", "three")
	gt.Expect(err).To(MatchError(HavePrefix("failed to create temporary directory:")))
}

func TestRunRunStreamError(t *testing.T) {
	gt := NewGomegaWithT(t)
	tempDir, err := ioutil.TempDir("", "run")
	gt.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tempDir)

	sharedDir, err := ioutil.TempDir(tempDir, "shared")
	gt.Expect(err).NotTo(HaveOccurred())

	fakeRunner := &fakes.RunnerServer{}
	fakeRunner.RunReturns(errors.New("miserable-failure"))

	grpcClient, cleanup := newRunnerClientConn(t, fakeRunner)
	defer cleanup()
	defer grpcClient.Close()

	run := &Run{
		SharedDir:  sharedDir,
		GRPCClient: grpcClient,
	}

	writeFiles(t, tempDir, standardRunFiles)
	outputDir := filepath.Join(tempDir, "output")
	metadataDir := filepath.Join(tempDir, "metadata")

	_, err = run.Run(ioutil.Discard, outputDir, metadataDir)
	gt.Expect(err).To(MatchError(ContainSubstring("miserable-failure")))

	gt.Expect(fakeRunner.RunCallCount()).To(Equal(1))
}

func TestRunRunMessages(t *testing.T) {
	gt := NewGomegaWithT(t)
	tempDir, err := ioutil.TempDir("", "run")
	gt.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tempDir)

	sharedDir, err := ioutil.TempDir(tempDir, "shared")
	gt.Expect(err).NotTo(HaveOccurred())

	fakeRunner := &fakes.RunnerServer{}
	fakeRunner.RunStub = func(req *sidecar.RunRequest, stream sidecar.Runner_RunServer) error {
		responses := []*sidecar.RunResponse{
			{
				Message: &sidecar.RunResponse_StandardOut{
					StandardOut: "This is a standard out message.\n",
				},
			},
			{
				Message: &sidecar.RunResponse_StandardError{
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

	grpcClient, cleanup := newRunnerClientConn(t, fakeRunner)
	defer cleanup()
	defer grpcClient.Close()

	run := &Run{
		SharedDir:  sharedDir,
		GRPCClient: grpcClient,
	}

	writeFiles(t, tempDir, standardRunFiles)
	outputDir := filepath.Join(tempDir, "output")
	metadataDir := filepath.Join(tempDir, "metadata")

	buf := &bytes.Buffer{}
	_, err = run.Run(buf, outputDir, metadataDir)
	gt.Expect(err).NotTo(HaveOccurred())
	gt.Expect(buf.String()).To(Equal("This is a standard out message.\nThis is a standard error message.\n"))
}

func TestRunRunBadMessage(t *testing.T) {
	gt := NewGomegaWithT(t)
	tempDir, err := ioutil.TempDir("", "run")
	gt.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tempDir)

	sharedDir, err := ioutil.TempDir(tempDir, "shared")
	gt.Expect(err).NotTo(HaveOccurred())

	fakeRunner := &fakes.RunnerServer{}
	fakeRunner.RunStub = func(req *sidecar.RunRequest, stream sidecar.Runner_RunServer) error {
		return stream.Send(&sidecar.RunResponse{})
	}

	grpcClient, cleanup := newRunnerClientConn(t, fakeRunner)
	defer cleanup()
	defer grpcClient.Close()

	run := &Run{
		SharedDir:  sharedDir,
		GRPCClient: grpcClient,
	}

	writeFiles(t, tempDir, standardRunFiles)
	outputDir := filepath.Join(tempDir, "output")
	metadataDir := filepath.Join(tempDir, "metadata")

	_, err = run.Run(ioutil.Discard, outputDir, metadataDir)
	gt.Expect(err).To(MatchError("unexpected server response message <nil>"))
}

func TestRunRunError(t *testing.T) {
	t.Skip() // Skipping this test for now, unstable in travis build job

	gt := NewGomegaWithT(t)
	tempDir, err := ioutil.TempDir("", "run")
	gt.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tempDir)

	sharedDir, err := ioutil.TempDir(tempDir, "shared")
	gt.Expect(err).NotTo(HaveOccurred())

	grpcClient, cleanup := newRunnerClientConn(t, &fakes.RunnerServer{})
	cleanup()
	defer grpcClient.Close()

	run := &Run{
		SharedDir:  sharedDir,
		GRPCClient: grpcClient,
	}

	writeFiles(t, tempDir, standardRunFiles)
	outputDir := filepath.Join(tempDir, "output")
	metadataDir := filepath.Join(tempDir, "metadata")

	_, err = run.Run(ioutil.Discard, outputDir, metadataDir)
	gt.Expect(err).To(MatchError(ContainSubstring("connection error")))
}

var standardRunFiles = map[string]string{
	"output/chaincode-output.tar": "bogus-tar",
	"output/package-metadata.tar": "bogus-tar",
	"metadata/chaincode.json":     "bogus-chaincode-json",
}

func newRunnerClientConn(t *testing.T, runnerServer sidecar.RunnerServer) (grpcClient *grpc.ClientConn, cleaup func()) {
	gt := NewGomegaWithT(t)
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	gt.Expect(err).NotTo(HaveOccurred())

	server := grpc.NewServer()
	sidecar.RegisterRunnerServer(server, runnerServer)

	go server.Serve(lis)

	timeoutContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	grpcClient, err = grpc.DialContext(timeoutContext, lis.Addr().String(), grpc.WithInsecure(), grpc.WithBlock())
	gt.Expect(err).NotTo(HaveOccurred())

	return grpcClient, func() { server.Stop(); lis.Close() }
}

//go:generate counterfeiter -o fakes/runner.go --fake-name RunnerServer . runnerServer
type runnerServer interface{ sidecar.RunnerServer }
