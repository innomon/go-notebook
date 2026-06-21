# Implementation Plan: Docx and Xlsx Document Parsers
Track ID: `docx_xlsx_parsers_20260621`

---

## Phase 1: Implement In-House Document Parsers

- [x] Task: Write unit tests verifying docx parsing from mock ZIP/XML structure [24aace4]
    - [x] Add test cases in `internal/extractor/docx_test.go` using mock docx ZIP data.
- [x] Task: Implement docx XML parsing using archive/zip and encoding/xml [24aace4]
    - [x] Create `internal/extractor/docx.go` to parse paragraphs and tables from docx zip.
- [~] Task: Write unit tests verifying xlsx parsing from mock ZIP/XML structure
    - [~] Add test cases in `internal/extractor/xlsx_test.go` using mock xlsx ZIP data.
- [ ] Task: Implement xlsx XML parsing using archive/zip and encoding/xml
    - [ ] Create `internal/extractor/xlsx.go` to parse worksheets and shared strings from xlsx zip.
- [ ] Task: Conductor - User Manual Verification 'Phase 1: Implement In-House Document Parsers' (Protocol in workflow.md)

## Phase 2: Ingestion Pipeline Integration

- [ ] Task: Write unit tests verifying parser routing inside worker job based on file extensions
    - [ ] Add test cases in `internal/worker/jobs_test.go` validating job routing for docx and xlsx.
- [ ] Task: Integrate DOCX and XLSX parsers into process_source command in internal/worker/jobs.go
    - [ ] Modify `handleProcessSource` to detect `.docx` and `.xlsx` file extensions and call the respective parsers.
- [ ] Task: Conductor - User Manual Verification 'Phase 2: Ingestion Pipeline Integration' (Protocol in workflow.md)
