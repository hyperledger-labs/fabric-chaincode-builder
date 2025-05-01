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
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// CopyDir recursively copies the files contained by src to the directory
// rooted at dest with file and directory permissions preserved.
func CopyDir(src, dest string) error {
	dirents, err := ioutil.ReadDir(src)
	if err != nil {
		return err
	}

	for _, de := range dirents {
		sourcePath := filepath.Join(src, de.Name())
		destPath := filepath.Join(dest, de.Name())

		if de.IsDir() {
			err := os.MkdirAll(destPath, de.Mode())
			if err != nil {
				return err
			}
			err = CopyDir(sourcePath, destPath)
			if err != nil {
				return err
			}
			err = chtimes(destPath, de)
			if err != nil {
				return err
			}
			continue
		}

		err := CopyFile(sourcePath, destPath)
		if err != nil {
			return err
		}
	}

	return nil
}

// CopyFile copies a single file from src to dest with file permissions
// presereved.
func CopyFile(src, dest string) error {
	source, err := os.Open(filepath.Clean(src))
	if err != nil {
		return err
	}
	if source != nil {
		// The following line is nosec because
		// we check for nil pointer & not close it in the code
		defer source.Close() // #nosec
	}

	fi, err := source.Stat()
	if err != nil {
		return err
	}

	target, err := os.OpenFile(filepath.Clean(dest), os.O_RDWR|os.O_CREATE|os.O_TRUNC, fi.Mode())
	if err != nil {
		return err
	}

	err = copyClose(target, source)
	if err != nil {
		return err
	}

	err = chtimes(dest, fi)
	if err != nil {
		return err
	}

	return nil
}

// copyClose copies data from the reader to the writer. This function will will
// close the writer before returning, regardless of whether or not the
// operation was successful.
func copyClose(w io.WriteCloser, r io.Reader) (err error) {
	defer func() {
		closeErr := w.Close()
		if closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	_, err = io.Copy(w, r)
	return err
}

func chtimes(target string, fi os.FileInfo) error {
	var atime time.Time
	if sys, ok := fi.Sys().(*syscall.Stat_t); ok {
		atime = time.Unix(sys.Atim.Sec, sys.Atim.Nsec)
	}

	return os.Chtimes(target, atime, fi.ModTime())
}
