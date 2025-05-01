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
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/hyperledger-labs/fabric-chaincode-builder/pkg/fs"
	"github.com/hyperledger-labs/fabric-chaincode-builder/pkg/sidecar"

	"google.golang.org/grpc"
)

type Run struct {
	SharedDir  string
	GRPCClient *grpc.ClientConn
}

func (r *Run) Run(stderr io.Writer, args ...string) (int, error) {
	buildOutputDir, peerMetadataDir := args[0], args[1]

	isExternal, err := IsExternalCC(buildOutputDir)
	if err != nil {
		return 0, err
	}

	if isExternal {
		return 0, fmt.Errorf("Peer should not call run for external chaincode type")
	}

	tempDir, err := ioutil.TempDir(r.SharedDir, "run")
	if err != nil {
		return 0, fmt.Errorf("failed to create temporary directory: %s", err)
	}
	defer os.RemoveAll(tempDir)

	runRoot, err := filepath.Rel(r.SharedDir, tempDir)
	if err != nil {
		return 0, fmt.Errorf("failed to determine build root: %s", err)
	}
	err = fs.CopyFile(filepath.Join(buildOutputDir, "chaincode-output.tar"), filepath.Join(tempDir, "chaincode-output.tar"))
	if err != nil {
		return 0, err
	}
	err = fs.CopyFile(filepath.Join(buildOutputDir, "package-metadata.tar"), filepath.Join(tempDir, "package-metadata.tar"))
	if err != nil {
		return 0, err
	}
	err = fs.CopyFile(filepath.Join(peerMetadataDir, "chaincode.json"), filepath.Join(tempDir, "chaincode.json"))
	if err != nil {
		return 0, err
	}

	client := sidecar.NewRunnerClient(r.GRPCClient)
	responseStream, err := client.Run(context.Background(), &sidecar.RunRequest{
		ChaincodePath: filepath.Join(runRoot, "chaincode-output.tar"),
		MetadataPath:  filepath.Join(runRoot, "package-metadata.tar"),
		ArtifactsPath: filepath.Join(runRoot, "chaincode.json"),
	})
	if err != nil {
		return 0, err
	}

	for {
		runResponse, err := responseStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}

		switch message := runResponse.Message.(type) {
		case *sidecar.RunResponse_StandardOut:
			fmt.Fprintf(stderr, message.StandardOut)

		case *sidecar.RunResponse_StandardError:
			fmt.Fprintf(stderr, message.StandardError)

		default:
			return 0, fmt.Errorf("unexpected server response message %T", message)
		}
	}

	return 0, nil
}
