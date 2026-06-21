# Specification: Docx and Xlsx Document Parsers

## Overview
Implement in-house document parsers for Microsoft Word (`.docx`) and Excel (`.xlsx`) files within the `go-notebook` multimodal ingestion pipeline. The parsers will extract structured text (headers, table cells, sheet names) using the standard library's `archive/zip` and `encoding/xml` packages to minimize external library dependencies.

## Functional Requirements
1. **In-house Parsers**:
   - Implement a `.docx` parser that reads `word/document.xml` inside the zip archive, extracting paragraphs (`<w:p>`) and tables/cells (`<w:tc>`).
   - Implement a `.xlsx` parser that reads the shared strings (`xl/sharedStrings.xml`), worksheet XMLs (`xl/worksheets/sheet*.xml`), and workbook sheets structure (`xl/workbook.xml`), mapping cell values to structured text.
2. **Structural Information Extraction**:
   - **DOCX**: Preserve structure by extracting headers/paragraphs and representing tables in a simple readable text format (e.g., markdown tables or row-by-row lines).
   - **XLSX**: Prefix worksheets with sheet names (e.g., "Sheet: Sheet1") and represent rows/cells in structured text format.
3. **Integration**:
   - Implement parsing logic inside a new package or file structure within `internal/extractor` (e.g., `internal/extractor/docx.go`, `internal/extractor/xlsx.go`).
   - Hook the extractor into the worker job flow in `internal/worker/jobs.go` under the `process_source` command when files with `.docx` or `.xlsx` extensions are detected.
4. **Resilience & Robustness**:
   - Gracefully handle malformed XML or corrupt ZIP components by skipping errors, extracting as much text as possible, and logging warning messages rather than immediately failing.

## Non-Functional Requirements
- **No Heavy External Dependencies**: Must use Go's standard library (`archive/zip`, `encoding/xml`, `io`, etc.) for document parsing.
- **Performance**: High memory and CPU efficiency by utilizing streaming XML decoders (`xml.Decoder`) where applicable.

## Acceptance Criteria
- Uploading a `.docx` file extracts all text paragraphs, lists, and tables.
- Uploading a `.xlsx` file extracts all sheet names, rows, and cell data.
- The extracted text is saved to the `Source` record's `full_text` field, and hashing is computed correctly.
- Integration tests in `internal/extractor` package verify correct parsing of mock ZIP files.

## Out of Scope
- Support for password-protected/encrypted `.docx` and `.xlsx` files.
- Support for older legacy binary formats (`.doc`, `.xls`).
- Embedding images or diagrams from the docx/xlsx documents into the knowledge base.
