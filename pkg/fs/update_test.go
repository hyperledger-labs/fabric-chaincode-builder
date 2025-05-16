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
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

func TestUpdateFileMissingFile(t *testing.T) {
	gt := NewGomegaWithT(t)
	tempdir, err := ioutil.TempDir("", "copyfile")
	gt.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tempdir)

	err = UpdateFile("src", "dest", nil)
	gt.Expect(err).To(HaveOccurred())
}

func TestUpdateFile(t *testing.T) {
	gt := NewGomegaWithT(t)
	tempdir, err := ioutil.TempDir("", "copyfile")
	gt.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tempdir)

	src := filepath.Join(tempdir, "test.txt")
	err = ioutil.WriteFile(src, []byte("test file"), 0755)
	gt.Expect(err).NotTo(HaveOccurred())

	updateFunc := func(file string) ([]byte, error) {
		return []byte("updated file"), nil
	}
	dest := filepath.Join(tempdir, "updated.txt")
	err = UpdateFile(src, dest, updateFunc)
	gt.Expect(err).NotTo(HaveOccurred())

	bytes, err := ioutil.ReadFile(dest)
	gt.Expect(err).NotTo(HaveOccurred())
	gt.Expect(string(bytes)).To(Equal("updated file"))
}
