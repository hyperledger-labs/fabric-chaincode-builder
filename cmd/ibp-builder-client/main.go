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
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/hyperledger-labs/fabric-chaincode-builder/pkg/command"
	"github.com/hyperledger-labs/fabric-chaincode-builder/pkg/logger"

	"google.golang.org/grpc"
)

// A Command represents an object that executes an external builder
// command.
type Command interface {
	Run(stderr io.Writer, args ...string) (rc int, err error)
}

func main() {
	logLevel := os.Getenv("LOGLEVEL")
	logger.SetupLogging(logLevel)

	log := logger.Logger.Sugar()
	log.Info("Log level set to: ", logger.GetLogLevel(logLevel).CapitalString())

	var cmd Command

	sharedDir := os.Getenv("IBP_BUILDER_SHARED_DIR")
	builderEndpoint := os.Getenv("IBP_BUILDER_ENDPOINT")

	grpcConn, err := newGRPCClientConn(builderEndpoint)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create connection to %s: %s", builderEndpoint, err)
		os.Exit(10)
	}
	defer grpcConn.Close()

	switch filepath.Base(os.Args[0]) {
	case "detect":
		cmd = &command.Detect{}

	case "build":
		cmd = &command.Build{
			SharedDir:  sharedDir,
			GRPCClient: grpcConn,
			Logger:     log,
		}

	case "release":
		cmd = &command.Release{}

	case "run":
		cmd = &command.Run{
			SharedDir:  sharedDir,
			GRPCClient: grpcConn,
		}

	default:
		fmt.Fprintf(os.Stderr, "Unexpected command: %s", filepath.Base(os.Args[0]))
		os.Exit(99)
	}

	rc, err := cmd.Run(os.Stderr, os.Args[1:]...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "External builder failed: %s\n", err)
		os.Exit(3)
	}

	os.Exit(rc)
}

func newGRPCClientConn(endpoint string) (*grpc.ClientConn, error) {
	timeoutContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return grpc.DialContext(timeoutContext, endpoint, grpc.WithInsecure(), grpc.WithBlock())
}
