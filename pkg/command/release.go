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
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hyperledger-labs/fabric-chaincode-builder/pkg/fs"
)

// A Release is responsible for presenting metadata from a chaincode package
// to the peer.
type Release struct{}

func (r *Release) Run(stderr io.Writer, args ...string) (int, error) {

	builderOutputDir, releaseDir := args[0], args[1]

	isExternal, err := IsExternalCC(builderOutputDir)
	if err != nil {
		return 0, err
	}

	if isExternal {
		err = r.HandleExternalCC(builderOutputDir, releaseDir)
		if err != nil {
			return 0, err
		}
	}

	// for both external and normal cc we need to extract couchdb indexes information

	chaincodeMetadata := filepath.Join(builderOutputDir, "chaincode-metadata.tar")

	tarfile, err := os.Open(filepath.Clean(chaincodeMetadata))
	if os.IsNotExist(err) { // Nothing to copy if does not exist.
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if tarfile != nil {
		// The following line is nosec because
		// we check for nil pointer & not close it in the code
		defer tarfile.Close() // #nosec
	}

	err = fs.ExtractTar(tarfile, releaseDir)
	if err != nil {
		return 0, err
	}

	return 0, nil
}

func (r *Release) HandleExternalCC(args ...string) error {
	builderOutputDir, releaseDir := args[0], args[1]

	destinationDir := filepath.Join(releaseDir, "chaincode/server")
	connectionSrcFile := filepath.Join(builderOutputDir, "connection.json")
	metadataFile := filepath.Join(builderOutputDir, "metadata.json")

	connectionDestFile := filepath.Join(destinationDir, "connection.json")
	metadataDestFile := filepath.Join(destinationDir, "metadata.json")

	// Check and copy connection.json
	_, err := os.Stat(filepath.Join(builderOutputDir, "/connection.json"))
	if os.IsNotExist(err) {
		return fmt.Errorf("connection.json not found in source folder: %s", err)
	}

	err = os.MkdirAll(destinationDir, 0750)
	if err != nil {
		return fmt.Errorf("failed to create target folder for connection.json: %s", err)
	}

	err = fs.CopyFile(connectionSrcFile, connectionDestFile)
	if err != nil {
		return fmt.Errorf("failed to copy connection.json to target folder: %s", err)
	}

	// Copy metadata.json
	err = fs.CopyFile(metadataFile, metadataDestFile)
	if err != nil {
		return fmt.Errorf("failed to copy metadata to target folder: %s", err)
	}

	return nil
}
