package command

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

func TestIsExternalCC(t *testing.T) {
	var externalCCFiles = map[string]string{
		"metadata/metadata.json": `{"path":"chaincode", "type":"external"}`,
		"source/connection.json": `{"address":"mycc.ns1:9999", "injectIdentifier":"hostname"}`,
		"output/":                "",
	}

	gt := NewGomegaWithT(t)
	tempDir, err := ioutil.TempDir("", "build")
	gt.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tempDir)

	writeFiles(t, tempDir, externalCCFiles)
	metadataDir := filepath.Join(tempDir, "metadata")

	isExternal, err := IsExternalCC(metadataDir)
	gt.Expect(err).NotTo(HaveOccurred())
	gt.Expect(isExternal).To(BeTrue())
}
