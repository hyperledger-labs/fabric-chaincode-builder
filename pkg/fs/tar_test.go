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
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

func TestTarRoundTrip(t *testing.T) {
	gt := NewGomegaWithT(t)
	tempdir, err := ioutil.TempDir("", "tar")
	gt.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tempdir)

	buf := &bytes.Buffer{}
	err = CreateTar(buf, "testdata/tree")
	gt.Expect(err).NotTo(HaveOccurred())
	err = ExtractTar(buf, tempdir)
	gt.Expect(err).NotTo(HaveOccurred())

	assertDirectoryContentsEqual(t, "testdata/tree", tempdir)
}

func TestCreateTarWithSymlink(t *testing.T) {
	gt := NewGomegaWithT(t)
	tempdir, err := ioutil.TempDir("", "tar")
	gt.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tempdir)

	err = os.Symlink(tempdir, filepath.Join(tempdir, "link"))
	gt.Expect(err).NotTo(HaveOccurred())

	err = CreateTar(ioutil.Discard, tempdir)
	gt.Expect(err).To(MatchError(ContainSubstring("failed to write header for link")))
}
