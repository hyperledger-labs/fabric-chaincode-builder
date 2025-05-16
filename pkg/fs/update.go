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

package fs

import (
	"io/ioutil"
	"os"
)

// UpdateFile updates a source file with a given update function and writes the updated
// file to a destination file.
func UpdateFile(src, dest string, updateFunc func(file string) ([]byte, error)) error {
	file, err := os.Stat(src)
	if os.IsNotExist(err) {
		return err
	}

	updatedFileBytes, err := updateFunc(src)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(dest, updatedFileBytes, file.Mode())
	if err != nil {
		return err
	}

	return nil
}
