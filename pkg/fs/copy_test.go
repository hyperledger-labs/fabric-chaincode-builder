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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

func TestCopyFileMissingSource(t *testing.T) {
	gt := NewGomegaWithT(t)
	tempdir, err := ioutil.TempDir("", "copyfile")
	gt.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tempdir)

	err = CopyFile("missing-source", tempdir)
	gt.Expect(err).To(MatchError("open missing-source: no such file or directory"))
	gt.Expect(err).To(BeAssignableToTypeOf(&os.PathError{}))
}

func TestCopyFileMissingDestDir(t *testing.T) {
	gt := NewGomegaWithT(t)
	tempdir, err := ioutil.TempDir("", "copyfile")
	gt.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tempdir)

	target := filepath.Join(tempdir, "missing-dir/copy.go")
	err = CopyFile("copy.go", target)
	gt.Expect(err).To(MatchError(fmt.Sprintf("open %s: no such file or directory", target)))
	gt.Expect(err).To(BeAssignableToTypeOf(&os.PathError{}))
}

func TestCopyFileContents(t *testing.T) {
	gt := NewGomegaWithT(t)
	tempdir, err := ioutil.TempDir("", "copyfile")
	gt.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tempdir)

	target := filepath.Join(tempdir, "output.txt")
	err = CopyFile("testdata/input.txt", target)
	gt.Expect(err).NotTo(HaveOccurred())
	assertFileContentsEqual(t, "testdata/input.txt", target)
	assertMetadataEqual(t, "testdata/input.txt", target)
}

func TestCopyFilePermissions(t *testing.T) {
	tests := []struct {
		mode os.FileMode
	}{
		{mode: 0644},
		{mode: 0755},
		{mode: 0600},
		{mode: 0400},
	}

	for _, tt := range tests {
		t.Run(tt.mode.String(), func(t *testing.T) {
			gt := NewGomegaWithT(t)
			tempdir, err := ioutil.TempDir("", "copyfile")
			gt.Expect(err).NotTo(HaveOccurred())
			defer os.RemoveAll(tempdir)

			inputFile := filepath.Join(tempdir, "inputfile")
			outputFile := filepath.Join(tempdir, "outputfile")

			err = ioutil.WriteFile(inputFile, []byte("input"), tt.mode)
			gt.Expect(err).NotTo(HaveOccurred())

			err = CopyFile(inputFile, outputFile)
			gt.Expect(err).NotTo(HaveOccurred())

			fi, err := os.Stat(outputFile)
			gt.Expect(err).NotTo(HaveOccurred())
			gt.Expect(fi.Mode()).To(Equal(tt.mode))
		})
	}
}

func TestCopyDir(t *testing.T) {
	gt := NewGomegaWithT(t)
	tempdir, err := ioutil.TempDir("", "copydir")
	gt.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tempdir)

	err = CopyDir("testdata/tree", tempdir)
	gt.Expect(err).NotTo(HaveOccurred())
	assertDirectoryContentsEqual(t, "testdata/tree", tempdir)
}

func assertDirectoryContentsEqual(t *testing.T, inputDir, outputDir string) {
	gt := NewGomegaWithT(t)

	err := filepath.Walk(inputDir, func(path string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relpath, err := filepath.Rel(inputDir, path)
		if err != nil {
			return err
		}
		if relpath == "." {
			return nil
		}

		target := filepath.Join(outputDir, relpath)
		assertMetadataEqual(t, path, target)
		if !fileInfo.IsDir() {
			assertFileContentsEqual(t, path, target)
		}

		return nil
	})
	gt.Expect(err).NotTo(HaveOccurred())
}

func assertFileContentsEqual(t *testing.T, input, output string) {
	gt := NewGomegaWithT(t)

	inputContents, err := ioutil.ReadFile(input)
	gt.Expect(err).NotTo(HaveOccurred())
	outputContents, err := ioutil.ReadFile(output)
	gt.Expect(err).NotTo(HaveOccurred())
	gt.Expect(inputContents).To(Equal(outputContents), "expected contents of %s to equal the contents of %s", output, input)
}

func assertMetadataEqual(t *testing.T, input, output string) {
	gt := NewGomegaWithT(t)

	inputInfo, err := os.Stat(input)
	gt.Expect(err).NotTo(HaveOccurred())
	outputInfo, err := os.Stat(output)
	gt.Expect(err).NotTo(HaveOccurred())

	gt.Expect(inputInfo.Mode()).To(Equal(outputInfo.Mode()), "expected mode %s but got %s for file %s", inputInfo.Mode(), outputInfo.Mode(), input)
	gt.Expect(inputInfo.ModTime()).To(Equal(outputInfo.ModTime()), "expected modification time %s but got %s for file %s", inputInfo.ModTime(), outputInfo.ModTime(), input)
}
