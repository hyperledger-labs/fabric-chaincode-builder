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
	"archive/tar"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

func TestReleaseWithoutMetadata(t *testing.T) {
	gt := NewGomegaWithT(t)
	tempDir, err := ioutil.TempDir("", "release")
	gt.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tempDir)

	outputDir := filepath.Join(tempDir, "build-output")
	releaseDir := filepath.Join(tempDir, "release")

	err = os.Mkdir(outputDir, 0755)
	gt.Expect(err).NotTo(HaveOccurred())
	err = os.Mkdir(releaseDir, 0755)
	gt.Expect(err).NotTo(HaveOccurred())

	release := &Release{}
	rc, err := release.Run(ioutil.Discard, outputDir, releaseDir)
	gt.Expect(err).NotTo(HaveOccurred())
	gt.Expect(rc).To(Equal(0))

	entries, err := ioutil.ReadDir(releaseDir)
	gt.Expect(err).NotTo(HaveOccurred())
	gt.Expect(entries).To(BeEmpty())
}

func TestReleaseWithMetadata(t *testing.T) {
	gt := NewGomegaWithT(t)
	tempDir, err := ioutil.TempDir("", "release")
	gt.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tempDir)

	outputDir := filepath.Join(tempDir, "build-output")
	releaseDir := filepath.Join(tempDir, "release")

	err = os.Mkdir(outputDir, 0755)
	gt.Expect(err).NotTo(HaveOccurred())
	err = os.Mkdir(releaseDir, 0755)
	gt.Expect(err).NotTo(HaveOccurred())

	archiveFile, err := os.Create(filepath.Join(outputDir, "chaincode-metadata.tar"))
	gt.Expect(err).NotTo(HaveOccurred())

	tw := tar.NewWriter(archiveFile)
	err = tw.WriteHeader(&tar.Header{
		Name:       "test-metadata.json",
		Size:       int64(len(`{"metadata":"document"}`)),
		Mode:       0644,
		Uid:        1000,
		Gid:        1000,
		ModTime:    time.Now(),
		AccessTime: time.Now(),
		Format:     tar.FormatPAX,
	})
	gt.Expect(err).NotTo(HaveOccurred())
	fmt.Fprint(tw, `{"metadata":"document"}`)
	err = tw.Close()
	gt.Expect(err).NotTo(HaveOccurred())
	err = archiveFile.Close()
	gt.Expect(err).NotTo(HaveOccurred())

	release := &Release{}
	rc, err := release.Run(ioutil.Discard, outputDir, releaseDir)
	gt.Expect(err).NotTo(HaveOccurred())
	gt.Expect(rc).To(Equal(0))

	gt.Expect(filepath.Join(releaseDir, "test-metadata.json")).To(BeARegularFile())
	contents, err := ioutil.ReadFile(filepath.Join(releaseDir, "test-metadata.json"))
	gt.Expect(err).NotTo(HaveOccurred())
	gt.Expect(string(contents)).To(Equal(`{"metadata":"document"}`))
}
