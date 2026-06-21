package extractor

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ExtractTextFromXlsx extracts sheet names, rows, and cell data from a local XLSX file
func ExtractTextFromXlsx(filePath string) (string, error) {
	zr, err := zip.OpenReader(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open zip reader for xlsx: %w", err)
	}
	defer zr.Close()

	// 1. Locate and parse workbook.xml to extract sheet names
	var workbookFile io.ReadCloser
	for _, f := range zr.File {
		if f.Name == "xl/workbook.xml" {
			workbookFile, err = f.Open()
			if err != nil {
				return "", fmt.Errorf("failed to open xl/workbook.xml: %w", err)
			}
			break
		}
	}

	type WorkbookSheet struct {
		Name    string `xml:"name,attr"`
		SheetID string `xml:"sheetId,attr"`
	}
	type WorkbookSheets struct {
		Sheets []WorkbookSheet `xml:"sheets>sheet"`
	}

	var sheets []WorkbookSheet
	if workbookFile != nil {
		defer workbookFile.Close()
		var wbs WorkbookSheets
		if err := xml.NewDecoder(workbookFile).Decode(&wbs); err == nil {
			sheets = wbs.Sheets
		}
	}

	// 2. Locate and parse sharedStrings.xml (might be absent)
	var sharedStrings []string
	var sstFile io.ReadCloser
	for _, f := range zr.File {
		if f.Name == "xl/sharedStrings.xml" {
			sstFile, err = f.Open()
			if err == nil {
				defer sstFile.Close()
				sharedStrings = parseSharedStrings(sstFile)
			}
			break
		}
	}

	var buf strings.Builder

	// 3. Parse each worksheet
	for i, sheet := range sheets {
		// Try xl/worksheets/sheet{i+1}.xml then xl/worksheets/sheet{sheetId}.xml
		var sheetFile io.ReadCloser
		targetName1 := fmt.Sprintf("xl/worksheets/sheet%d.xml", i+1)
		targetName2 := fmt.Sprintf("xl/worksheets/sheet%s.xml", sheet.SheetID)

		for _, f := range zr.File {
			if f.Name == targetName1 || f.Name == targetName2 {
				sheetFile, err = f.Open()
				break
			}
		}

		if sheetFile == nil {
			continue
		}

		buf.WriteString(fmt.Sprintf("\nSheet: %s\n", sheet.Name))
		parseWorksheet(sheetFile, sharedStrings, &buf)
		sheetFile.Close()
	}

	return buf.String(), nil
}

func parseSharedStrings(r io.Reader) []string {
	var list []string
	dec := xml.NewDecoder(r)
	var inT bool
	var strBuf strings.Builder

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		switch se := tok.(type) {
		case xml.StartElement:
			if se.Name.Local == "t" {
				inT = true
				strBuf.Reset()
			}
		case xml.CharData:
			if inT {
				strBuf.Write(se)
			}
		case xml.EndElement:
			if se.Name.Local == "t" {
				inT = false
				list = append(list, strBuf.String())
			}
		}
	}
	return list
}

func parseWorksheet(r io.Reader, sharedStrings []string, out *strings.Builder) {
	dec := xml.NewDecoder(r)
	var cellType string
	var inV bool
	var valBuf strings.Builder
	var rowCells []string

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		switch se := tok.(type) {
		case xml.StartElement:
			switch se.Name.Local {
			case "c":
				cellType = ""
				for _, attr := range se.Attr {
					if attr.Name.Local == "t" {
						cellType = attr.Value
						break
					}
				}
			case "v":
				inV = true
				valBuf.Reset()
			}
		case xml.CharData:
			if inV {
				valBuf.Write(se)
			}
		case xml.EndElement:
			switch se.Name.Local {
			case "v":
				inV = false
				rawVal := valBuf.String()
				cellVal := rawVal
				if cellType == "s" {
					if idx, parseErr := strconv.Atoi(rawVal); parseErr == nil {
						if idx >= 0 && idx < len(sharedStrings) {
							cellVal = sharedStrings[idx]
						}
					}
				}
				rowCells = append(rowCells, cellVal)
			case "row":
				if len(rowCells) > 0 {
					out.WriteString(" | ")
					out.WriteString(strings.Join(rowCells, " | "))
					out.WriteString(" |\n")
					rowCells = nil
				}
			}
		}
	}
}
