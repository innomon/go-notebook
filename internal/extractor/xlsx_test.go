package extractor

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func createMockXlsx(t *testing.T, workbookXML, sharedStringsXML, sheet1XML string) string {
	t.Helper()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.xlsx")

	f, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("failed to create mock xlsx file: %v", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	files := map[string]string{
		"xl/workbook.xml":          workbookXML,
		"xl/sharedStrings.xml":     sharedStringsXML,
		"xl/worksheets/sheet1.xml": sheet1XML,
	}

	for path, content := range files {
		w, err := zw.Create(path)
		if err != nil {
			t.Fatalf("failed to create %s inside zip: %v", path, err)
		}
		_, err = io.WriteString(w, content)
		if err != nil {
			t.Fatalf("failed to write content to %s: %v", path, err)
		}
	}

	return filePath
}

func TestExtractTextFromXlsx(t *testing.T) {
	workbookXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheets>
    <sheet name="Sales Data" sheetId="1" r:id="rId1"/>
  </sheets>
</workbook>`

	sharedStringsXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="2" uniqueCount="2">
  <si><t>Revenue</t></si>
  <si><t>Q1 Total</t></si>
</sst>`

	sheet1XML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row r="1">
      <c r="A1" t="s"><v>0</v></c>
      <c r="B1"><v>50000</v></c>
    </row>
    <row r="2">
      <c r="A2" t="s"><v>1</v></c>
      <c r="B2"><v>999.99</v></c>
    </row>
  </sheetData>
</worksheet>`

	filePath := createMockXlsx(t, workbookXML, sharedStringsXML, sheet1XML)

	txt, err := ExtractTextFromXlsx(filePath)
	if err != nil {
		t.Fatalf("ExtractTextFromXlsx failed: %v", err)
	}

	expectedContents := []string{
		"Sheet: Sales Data",
		"Revenue",
		"50000",
		"Q1 Total",
		"999.99",
	}

	for _, exp := range expectedContents {
		if !strings.Contains(txt, exp) {
			t.Errorf("expected extracted text to contain %q, but it did not. Got:\n%s", exp, txt)
		}
	}
}
