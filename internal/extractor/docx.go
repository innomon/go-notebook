package extractor

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// ExtractTextFromDocx extracts structured plain text and tables from a local DOCX file
func ExtractTextFromDocx(filePath string) (string, error) {
	zr, err := zip.OpenReader(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open zip reader for docx: %w", err)
	}
	defer zr.Close()

	var docXML io.ReadCloser
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			docXML, err = f.Open()
			if err != nil {
				return "", fmt.Errorf("failed to open word/document.xml inside zip: %w", err)
			}
			break
		}
	}

	if docXML == nil {
		return "", fmt.Errorf("word/document.xml not found in the DOCX archive")
	}
	defer docXML.Close()

	var buf strings.Builder
	dec := xml.NewDecoder(docXML)

	var inCell bool
	var cellText strings.Builder

	for {
		t, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Skip XML decoding errors if we've successfully read some content, to follow the resilience guidelines
			if buf.Len() > 0 {
				break
			}
			return "", fmt.Errorf("failed to parse XML token: %w", err)
		}

		switch se := t.(type) {
		case xml.StartElement:
			switch se.Name.Local {
			case "p":
				if !inCell {
					if buf.Len() > 0 && !strings.HasSuffix(buf.String(), "\n") {
						buf.WriteByte('\n')
					}
				}
			case "tc":
				inCell = true
				cellText.Reset()
			}
		case xml.CharData:
			if inCell {
				cellText.Write(se)
			} else {
				buf.Write(se)
			}
		case xml.EndElement:
			switch se.Name.Local {
			case "p":
				if !inCell {
					buf.WriteByte('\n')
				}
			case "tc":
				inCell = false
				val := strings.TrimSpace(cellText.String())
				buf.WriteString(" | ")
				buf.WriteString(val)
			case "tr":
				buf.WriteString(" |\n")
			}
		}
	}

	return buf.String(), nil
}
