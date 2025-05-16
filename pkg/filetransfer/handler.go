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
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// TODO: In the future, we may wish to use signed paths in the URL.

type Handler struct {
	Root     string
	Username string
	Password string
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	upath := path.Clean(req.URL.Path)

	if upath == "/healthz" {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Chaincode Launcher running"))
		return
	}

	username, password, ok := req.BasicAuth()
	if !ok {
		fmt.Println("Error parsing basic auth")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if username != h.Username && password != h.Password {
		fmt.Println("Username or password are not correct")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if strings.Contains(upath, "..") || strings.Contains(upath, "\x00") {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	location := filepath.Join(h.Root, upath)

	switch req.Method {

	// Download the chaincode or source
	case http.MethodGet:
		if ct, ok := contentType(location); ok {
			w.Header().Set("Content-Type", ct)
		}
		http.ServeFile(w, req, location)

	// Upload the chaincode
	case http.MethodPut:
		// The following line is "nosec" because
		// 666 is required as other containers can use the file
		output, err := os.OpenFile(filepath.Clean(location), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0666) // #nosec
		if err != nil {
			sendErrorResponse(w, req, err)
			return
		}
		if output != nil {
			// The following line is nosec because
			// we check for nil pointer & not close it in the code
			defer output.Close() // #nosec
		}

		_, err = io.Copy(output, req.Body)
		if err != nil {
			sendErrorResponse(w, req, err)
			return
		}

		w.WriteHeader(http.StatusCreated)

	default:
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "unsupported method: %s", req.Method)
	}
}

func sendErrorResponse(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case os.IsExist(err):
		w.WriteHeader(http.StatusConflict)
	case os.IsNotExist(err):
		w.WriteHeader(http.StatusNotFound)
	case os.IsPermission(err):
		w.WriteHeader(http.StatusForbidden)
	default:
		w.WriteHeader(http.StatusBadRequest)
	}
	return
}

func contentType(filename string) (string, bool) {
	ext := filepath.Ext(filename)
	switch strings.ToLower(ext) {
	case ".tar":
		return "application/x-tar", true
	case ".json":
		return "application/json; charset=utf-8", true
	default:
		return "", false
	}
}
