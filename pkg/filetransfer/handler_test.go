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

package filetransfer

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hyperledger-labs/fabric-chaincode-builder/pkg/fs"

	. "github.com/onsi/gomega"
)

func TestHandlerGet(t *testing.T) {
	tests := []struct {
		path        string
		status      int
		contentType string
	}{
		{path: "/healthz", status: http.StatusOK},
		{path: "/some.json", status: http.StatusOK, contentType: "application/json; charset=utf-8"},
		{path: "/empty.tar", status: http.StatusOK, contentType: "application/x-tar"},
		{path: "/file.txt", status: http.StatusOK, contentType: "text/plain; charset=utf-8"},
		{path: "/missing.txt", status: http.StatusNotFound, contentType: "text/plain; charset=utf-8"},
	}

	handler := &Handler{Root: "testdata", Username: "testuser", Password: "testpassword"}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			gt := NewGomegaWithT(t)

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.SetBasicAuth("testuser", "testpassword")
			resp := httptest.NewRecorder()
			handler.ServeHTTP(resp, req)

			gt.Expect(resp.Code).To(Equal(tt.status))
			gt.Expect(resp.Header().Get("content-type")).To(Equal(tt.contentType))

			if resp.Code == http.StatusOK && tt.path != "/healthz" {
				expectedContents, err := ioutil.ReadFile(filepath.Join("testdata", tt.path))
				gt.Expect(err).NotTo(HaveOccurred())
				gt.Expect(resp.Body.Bytes()).To(Equal(expectedContents))
			}
		})
	}
}

func TestHandlerPut(t *testing.T) {
	gt := NewGomegaWithT(t)

	tempdir, err := ioutil.TempDir("", "handler-put")
	gt.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tempdir)

	err = fs.CopyDir("testdata", tempdir)
	gt.Expect(err).NotTo(HaveOccurred())

	// Step 3: Create a no-perm directory
	noPermPath := filepath.Join(tempdir, "no-perm")
	err = os.Mkdir(noPermPath, 0555)
	gt.Expect(err).NotTo(HaveOccurred())

	// Step 4: Define test cases
	tests := []struct {
		path   string
		status int
	}{
		{path: "/", status: http.StatusConflict},
		{path: "/empty.tar", status: http.StatusConflict},
		{path: "/file.txt", status: http.StatusConflict},
		{path: "/missing-dir/file.txt", status: http.StatusNotFound},
		{path: "/no-perm/file.txt", status: http.StatusForbidden},
		{path: "/new-file.txt", status: http.StatusCreated},
	}

	handler := &Handler{
		Root:     tempdir,
		Username: "testuser",
		Password: "testpassword",
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			gt := NewGomegaWithT(t)

			if os.Geteuid() == 0 && tt.path == "/no-perm/file.txt" {
				t.Skip("Skipping /no-perm/file.txt test when running as root")
			}

			req := httptest.NewRequest(http.MethodPut, tt.path, strings.NewReader("content"))
			req.SetBasicAuth("testuser", "testpassword")
			resp := httptest.NewRecorder()

			handler.ServeHTTP(resp, req)

			gt.Expect(resp.Code).To(Equal(tt.status), "unexpected status for path %s", tt.path)

			if resp.Code == http.StatusCreated {
				newfile := filepath.Join(tempdir, tt.path)
				gt.Expect(newfile).To(BeARegularFile())

				contents, err := ioutil.ReadFile(newfile)
				gt.Expect(err).NotTo(HaveOccurred())
				gt.Expect(contents).To(BeEquivalentTo("content"))
			}
		})
	}
}

func TestHandlerBadPaths(t *testing.T) {
	tests := []struct {
		method         string
		path           string
		expectedStatus int
	}{
		{method: http.MethodGet, path: "/..", expectedStatus: http.StatusBadRequest},
		{method: http.MethodGet, path: "http://example.com/%2e%2e/foo", expectedStatus: http.StatusBadRequest},
		{method: http.MethodGet, path: "http://example.com//buck%00rogers", expectedStatus: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s %s", tt.method, tt.path), func(t *testing.T) {
			gt := NewGomegaWithT(t)

			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.SetBasicAuth("testuser", "testpassword")
			resp := httptest.NewRecorder()

			handler := &Handler{Root: ".", Username: "testuser", Password: "testpassword"}
			handler.ServeHTTP(resp, req)
			gt.Expect(resp.Code).To(Equal(tt.expectedStatus))
		})
	}
}

func TestHandlerBadMethod(t *testing.T) {
	methods := []string{
		http.MethodHead,
		http.MethodPost,
		http.MethodPatch,
		http.MethodTrace,
		http.MethodDelete,
		http.MethodOptions,
		http.MethodConnect,
		"unkwnon",
	}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			gt := NewGomegaWithT(t)

			req := httptest.NewRequest(method, "/", nil)
			req.SetBasicAuth("testuser", "testpassword")
			resp := httptest.NewRecorder()

			handler := &Handler{Root: "testdata", Username: "testuser", Password: "testpassword"}
			handler.ServeHTTP(resp, req)

			gt.Expect(resp.Code).To(Equal(http.StatusBadRequest))
			gt.Expect(resp.Body.Bytes()).To(BeEquivalentTo(fmt.Sprintf("unsupported method: %s", method)))
		})
	}
}
