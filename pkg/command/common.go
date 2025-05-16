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
	"io/ioutil"
	"path/filepath"
	"strings"
)

func IsExternalCC(metadataDir string) (bool, error) {
	metadataFileContents, err := ioutil.ReadFile(filepath.Clean(filepath.Join(metadataDir, "metadata.json")))
	if err != nil {
		// deliberately returing nil instead of err here
		// if metadata.json not found, we still need to continue with rest of code
		return false, nil
	}

	var metadata ChaincodeMetadata
	err = json.Unmarshal(metadataFileContents, &metadata)
	if err != nil {
		return false, err
	}

	if strings.ToLower(metadata.Type) == "external" {
		return true, nil
	} else {
		return false, nil
	}
}
