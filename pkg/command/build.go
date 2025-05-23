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
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/hyperledger-labs/fabric-chaincode-builder/pkg/fs"
	"github.com/hyperledger-labs/fabric-chaincode-builder/pkg/sidecar"
	"go.uber.org/zap"

	"google.golang.org/grpc"
)

// Build is responsible for invoking the external-builder sidecar to build
// chaincode.
type Build struct {
	SharedDir  string
	GRPCClient *grpc.ClientConn

	Logger *zap.SugaredLogger
}

func (b *Build) Run(stderr io.Writer, args ...string) (int, error) {
	sourceDir, metadataDir, outputDir := args[0], args[1], args[2]

	isExternal, err := IsExternalCC(metadataDir)
	if err != nil {
		return 0, err
	}

	if isExternal {
		return 0, b.HandleExternalCC(sourceDir, metadataDir, outputDir)
	}

	tempDir, err := ioutil.TempDir(b.SharedDir, "build")
	if err != nil {
		return 0, fmt.Errorf("failed to create temporary directory: %s", err)
	}
	defer os.RemoveAll(tempDir)

	buildRoot, err := filepath.Rel(b.SharedDir, tempDir)
	if err != nil {
		return 0, fmt.Errorf("failed to determine build root: %s", err)
	}
	err = createTar(filepath.Join(tempDir, "chaincode-source.tar"), sourceDir)
	if err != nil {
		return 0, err
	}
	if dirExists(filepath.Join(sourceDir, "META-INF")) {
		err = createTar(filepath.Join(tempDir, "chaincode-metadata.tar"), filepath.Join(sourceDir, "META-INF"))
		if err != nil {
			return 0, err
		}
	}
	err = createTar(filepath.Join(tempDir, "package-metadata.tar"), metadataDir)
	if err != nil {
		return 0, err
	}

	client := sidecar.NewBuilderClient(b.GRPCClient)
	responseStream, err := client.Build(context.Background(), &sidecar.BuildRequest{
		SourcePath:   filepath.Join(buildRoot, "chaincode-source.tar"),
		MetadataPath: filepath.Join(buildRoot, "package-metadata.tar"),
		OutputPath:   filepath.Join(buildRoot, "chaincode-output.tar"),
	})
	if err != nil {
		return 0, err
	}

	for {
		buildResponse, err := responseStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}

		switch message := buildResponse.Message.(type) {
		case *sidecar.BuildResponse_StandardOut:
			fmt.Fprintf(stderr, "%s", message.StandardOut)

		case *sidecar.BuildResponse_StandardError:
			fmt.Fprintf(stderr, "%s", message.StandardError)

		default:
			return 0, fmt.Errorf("unexpected server response message %T", message)
		}
	}

	err = fs.CopyDir(tempDir, outputDir)
	if err != nil {
		return 0, fmt.Errorf("failed to copy build result to peer: %s", err)
	}

	return 0, nil
}

func (b *Build) HandleExternalCC(args ...string) error {
	sourceDir, metadataDir, outputDir := args[0], args[1], args[2]
	connectionSrcFile := filepath.Join(sourceDir, "/connection.json")
	connectionDestFile := filepath.Join(outputDir, "/connection.json")

	_, err := os.Stat(connectionSrcFile)
	if os.IsNotExist(err) {
		return fmt.Errorf("connection.json not found in source folder: %s", err)
	}

	// Inject peer identifier in connection.json if needed and write to output folder
	err = fs.UpdateFile(connectionSrcFile, connectionDestFile, b.injectPeerIdentifier)
	if err != nil {
		return fmt.Errorf("failed to update connection.json and write to output folder: %s", err)
	}

	err = fs.CopyDir(metadataDir, outputDir)
	if err != nil {
		return fmt.Errorf("failed to copy build metadata folder: %s", err)
	}

	if dirExists(filepath.Join(sourceDir, "META-INF")) {
		err = createTar(filepath.Join(outputDir, "chaincode-metadata.tar"), filepath.Join(sourceDir, "META-INF"))
		if err != nil {
			return err
		}
	}

	return nil
}

func createTar(targetArchive, sourceDir string) error {
	// The following line is nosec because
	// other containers with different users will need access to this folder
	err := os.MkdirAll(filepath.Dir(targetArchive), 0755) // #nosec
	if err != nil {
		return fmt.Errorf("tar directory create failed: %s", err)
	}

	archiveFile, err := os.Create(filepath.Clean(targetArchive))
	if err != nil {
		return fmt.Errorf("file create failed: %s", err)
	}
	if archiveFile != nil {
		// The following line is nosec because
		// we check for nil pointer & not close it in the code
		defer archiveFile.Close() // #nosec
	}

	err = fs.CreateTar(archiveFile, sourceDir)
	if err != nil {
		return fmt.Errorf("failed to create archive %s: %s", archiveFile.Name(), err)
	}

	return nil
}

func dirExists(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fi.IsDir()
}

func (b *Build) injectPeerIdentifier(connectionFile string) ([]byte, error) {
	connectionBytes, err := ioutil.ReadFile(filepath.Clean(connectionFile))
	if err != nil {
		return nil, fmt.Errorf("failed to read connection.json: %s", err)
	}

	var connectionJSON map[string]interface{}
	err = json.Unmarshal(connectionBytes, &connectionJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshall connection.json: %s", err)
	}

	injectIdentifier, ok := connectionJSON["injectIdentifier"].(string)
	if !ok {
		b.Logger.Info("injectIdentifier not specified, persisting existing connection.json")
		return connectionBytes, nil
	}

	address, ok := connectionJSON["address"].(string)
	if !ok {
		return nil, fmt.Errorf("address unable to be parsed as a string")
	}

	peerName := getPeerID()
	if peerName == "" {
		return nil, fmt.Errorf("failed to get peer name from PEER_NAME env var")
	}
	b.Logger.Infof("Peer identifier to inject into connection.json['address']: %s", peerName)

	hostname, domain, port, err := parseAddress(address)
	if err != nil {
		return nil, fmt.Errorf("invalid address provided in connection.json: %s", err)
	}

	switch injectIdentifier {
	case "hostname":
		b.Logger.Info("Injecting peer identifier into hostname of connection.json['address']")

		// <peername>-<hostname>.<domain>:<port>
		newAddress := peerName + "-" + address
		connectionJSON["address"] = newAddress

		b.Logger.Infof("Updated address in connection.json: %s", newAddress)

	case "domain":
		b.Logger.Info("Injecting peer identifier into domain of connection.json['address']")

		// <hostname>.<peername>-<domain>:<port>
		newAddress := fmt.Sprintf("%s.%s-%s:%s", hostname, peerName, domain, port)
		connectionJSON["address"] = newAddress

		b.Logger.Infof("Updated address in connection.json: %s", newAddress)

	default:
		b.Logger.Info("connection.json['injectIdentifier'] not set, persisting existing connection.json")
		// Maintain existing URL
	}

	updatedConnectionBytes, err := json.Marshal(connectionJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal updated connection.json file: %s", err)
	}

	return updatedConnectionBytes, nil
}

// Address is in the format <hostname>.<domain>:<port>
func parseAddress(addr string) (hostname, domain, port string, err error) {
	addressItems := strings.Split(addr, ":")
	if len(addressItems) != 2 {
		err = fmt.Errorf("address not in format <host-url>:<port>")
		return
	}

	port = addressItems[1]     // <port>
	hostURL := addressItems[0] // <hostname>.<domain>

	hostURLItems := strings.Split(hostURL, ".") // [<hostname>, <domain>]
	domain = hostURLItems[len(hostURLItems)-1]
	hostname = strings.TrimSuffix(hostURL, "."+domain)

	return
}

func getPeerID() string {
	peerName := os.Getenv("PEER_NAME")

	// Remove any whitespace to ensure that url address will remain valid
	id := strings.ReplaceAll(peerName, " ", "")
	return id
}
