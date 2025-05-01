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
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

func TestDetect(t *testing.T) {
	tests := []struct {
		cm ChaincodeMetadata
		rc int
	}{
		{cm: ChaincodeMetadata{Type: "GOLANG"}, rc: 0},
		{cm: ChaincodeMetadata{Type: "Golang"}, rc: 0},
		{cm: ChaincodeMetadata{Type: "goLang"}, rc: 0},
		{cm: ChaincodeMetadata{Type: "golang"}, rc: 0},
		{cm: ChaincodeMetadata{Type: "JAVA"}, rc: 0},
		{cm: ChaincodeMetadata{Type: "Java"}, rc: 0},
		{cm: ChaincodeMetadata{Type: "java"}, rc: 0},
		{cm: ChaincodeMetadata{Type: "NODE"}, rc: 0},
		{cm: ChaincodeMetadata{Type: "Node"}, rc: 0},
		{cm: ChaincodeMetadata{Type: "node"}, rc: 0},
		{cm: ChaincodeMetadata{Type: ""}, rc: 1},
		{cm: ChaincodeMetadata{Type: "unknown"}, rc: 1},
	}

	for _, tt := range tests {
		t.Run(tt.cm.Type, func(t *testing.T) {
			gt := NewGomegaWithT(t)
			detect := &Detect{}

			tempDir, err := ioutil.TempDir("", "detect")
			gt.Expect(err).NotTo(HaveOccurred())
			defer os.RemoveAll(tempDir)

			srcDir, err := ioutil.TempDir(tempDir, "source")
			gt.Expect(err).NotTo(HaveOccurred())
			metaDir, err := ioutil.TempDir(tempDir, "metadata")
			gt.Expect(err).NotTo(HaveOccurred())

			md, err := json.Marshal(tt.cm)
			gt.Expect(err).NotTo(HaveOccurred())
			err = ioutil.WriteFile(filepath.Join(metaDir, "metadata.json"), md, 0600)
			gt.Expect(err).NotTo(HaveOccurred())

			rc, err := detect.Run(ioutil.Discard, srcDir, metaDir)
			gt.Expect(err).NotTo(HaveOccurred())
			gt.Expect(rc).To(Equal(tt.rc))
		})
	}
}

func TestDetectBadPath(t *testing.T) {
	gt := NewGomegaWithT(t)
	detect := &Detect{}

	_, err := detect.Run(ioutil.Discard, "", "bad-path")
	gt.Expect(err).To(MatchError("open bad-path/metadata.json: no such file or directory"))
	gt.Expect(err).To(BeAssignableToTypeOf(&os.PathError{}))
}

func TestDetectBadJSON(t *testing.T) {
	gt := NewGomegaWithT(t)
	detect := &Detect{}

	tempDir, err := ioutil.TempDir("", "detect")
	gt.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tempDir)

	err = ioutil.WriteFile(filepath.Join(tempDir, "metadata.json"), []byte("bad-json:$&$&$:"), 0644)
	gt.Expect(err).NotTo(HaveOccurred())

	_, err = detect.Run(ioutil.Discard, "", tempDir)
	gt.Expect(err).To(HaveOccurred())
	gt.Expect(err).To(BeAssignableToTypeOf(&json.SyntaxError{}))
}
