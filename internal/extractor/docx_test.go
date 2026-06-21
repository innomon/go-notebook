package extractor

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func createMockDocx(t *testing.T, xmlContent string) string {
	t.Helper()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.docx")

	f, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("failed to create mock docx file: %v", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatalf("failed to create document.xml in zip: %v", err)
	}

	_, err = io.WriteString(w, xmlContent)
	if err != nil {
		t.Fatalf("failed to write XML content: %v", err)
	}

	return filePath
}

func TestExtractTextFromDocx(t *testing.T) {
	xmlContent := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:r>
        <w:t>Hello World, from DOCX parser!</w:t>
      </w:r>
    </w:p>
    <w:p>
      <w:r>
        <w:t>Second paragraph.</w:t>
      </w:r>
    </w:p>
    <w:tbl>
      <w:tr>
        <w:tc>
          <w:p><w:r><w:t>Cell A1</w:t></w:r></w:p>
        </w:tc>
        <w:tc>
          <w:p><w:r><w:t>Cell B1</w:t></w:r></w:p>
        </w:tc>
      </w:tr>
      <w:tr>
        <w:tc>
          <w:p><w:r><w:t>Cell A2</w:t></w:r></w:p>
        </w:tc>
        <w:tc>
          <w:p><w:r><w:t>Cell B2</w:t></w:r></w:p>
        </w:tc>
      </w:tr>
    </w:tbl>
  </w:body>
</w:document>`

	filePath := createMockDocx(t, xmlContent)

	txt, err := ExtractTextFromDocx(filePath)
	if err != nil {
		t.Fatalf("ExtractTextFromDocx failed: %v", err)
	}

	expectedParagraphs := []string{
		"Hello World, from DOCX parser!",
		"Second paragraph.",
		"Cell A1",
		"Cell B1",
		"Cell A2",
		"Cell B2",
	}

	for _, exp := range expectedParagraphs {
		if !strings.Contains(txt, exp) {
			t.Errorf("expected extracted text to contain %q, but it did not. Got:\n%s", exp, txt)
		}
	}
}
