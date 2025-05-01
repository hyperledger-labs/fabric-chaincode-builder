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
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"
)

type ChaincodeMetadata struct {
	Type string `json:"type"`
}

// Detect is responsible for determining if our builder can process the
// chaincode package that has been provided.
type Detect struct{}

func (d *Detect) Run(stderr io.Writer, args ...string) (int, error) {
	// 0: source directory - ignored
	// 1: metadata - hosts metadata.json from the chaincode package
	mdbytes, err := ioutil.ReadFile(filepath.Join(args[1], "metadata.json"))
	if err != nil {
		return 0, err
	}

	var metadata ChaincodeMetadata
	err = json.Unmarshal(mdbytes, &metadata)
	if err != nil {
		return 0, err
	}

	switch strings.ToLower(metadata.Type) {
	case "golang":
		return 0, nil
	case "java":
		return 0, nil
	case "node":
		return 0, nil
	case "executable":
		return 0, nil
	case "external":
		return 0, nil
	default:
		fmt.Fprintf(stderr, "chaincode type not supported: %s", metadata.Type)
		return 1, nil
	}
}
