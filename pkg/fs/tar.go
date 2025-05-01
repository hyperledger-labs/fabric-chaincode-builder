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
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func CreateTar(w io.Writer, sourceDir string) error {
	tw := tar.NewWriter(w)
	defer tw.Close()

	return filepath.Walk(sourceDir, func(path string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relpath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		// Ignore the containing directory
		if relpath == "." {
			return nil
		}

		header, err := fileInfoHeader(relpath, fileInfo)
		if err != nil {
			return err
		}
		header.Format = tar.FormatPAX

		err = tw.WriteHeader(header)
		if err != nil {
			return err
		}

		switch {
		case fileInfo.Mode().IsRegular():
			err := writeFileContents(tw, path)
			if err != nil {
				return fmt.Errorf("failed to write file contents for %s: %s", relpath, err)
			}
		case fileInfo.Mode().IsDir():
			// nothing to do
		default:
			return fmt.Errorf("failed to write header for %s: unsupported file mode %04o", relpath, fileInfo.Mode())
		}

		return nil
	})
}

func fileInfoHeader(relpath string, fileInfo os.FileInfo) (*tar.Header, error) {
	header, err := tar.FileInfoHeader(fileInfo, "")
	if err != nil {
		return nil, err
	}

	header.Name = relpath
	if fileInfo.IsDir() {
		header.Name += "/"
	}
	return header, nil
}

func writeFileContents(writer io.Writer, path string) error {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return err
	}
	if file != nil {
		// The following line is nosec because
		// we check for nil pointer & not close it in the code
		defer file.Close() // #nosec
	}

	_, err = io.Copy(writer, file)
	if err != nil {
		return err
	}

	return nil
}

func ExtractTar(r io.Reader, destDir string) error {
	directoryHeaders := map[string]*tar.Header{}

	tr := tar.NewReader(r)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		fi := header.FileInfo()
		target := filepath.Join(destDir, filepath.Clean(header.Name))
		switch header.Typeflag {
		case tar.TypeDir:
			err := os.MkdirAll(target, fi.Mode())
			if err != nil {
				return err
			}
			directoryHeaders[target] = header

		case tar.TypeReg:
			file, err := os.OpenFile(filepath.Clean(target), os.O_RDWR|os.O_CREATE|os.O_TRUNC, fi.Mode())
			if err != nil {
				return err
			}

			err = copyClose(file, tr)
			if err != nil {
				return err
			}

			err = os.Chtimes(target, header.ModTime, header.ModTime)
			if err != nil {
				return err
			}

		default:
			return fmt.Errorf("unsupported tar header type flag: %d", header.Typeflag)
		}
	}

	// Emulate the behavior described in the GNU tar documentation:
	// https://www.gnu.org/software/tar/manual/html_node/Directory-Modification-Times-and-Permissions.html
	for target, header := range directoryHeaders {
		err := os.Chtimes(target, header.AccessTime, header.ModTime)
		if err != nil {
			return err
		}
	}

	return nil
}
