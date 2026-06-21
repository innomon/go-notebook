# Implementation Plan: Whiteboard OCR and Image Notes Processing
Track ID: `whiteboard_ocr_20260621`

---

## Phase 1: Implement Whiteboard OCR Parser

- [x] Task: Write unit tests verifying CLI Tesseract invocation and LLM Vision fallback (d2b827a)
    - [x] Add test cases in `internal/extractor/image_test.go` using mock tesseract commands and mock AI responses.
- [x] Task: Implement image parser with Tesseract CLI execution and Vision model fallback (d2b827a)
    - [x] Create `internal/extractor/image.go` containing `ExtractTextFromImage` function checking for local binary and falling back to LLM.
- [ ] Task: Conductor - User Manual Verification 'Phase 1: Implement Whiteboard OCR Parser' (Protocol in workflow.md)

## Phase 2: Ingestion Pipeline Integration

- [ ] Task: Write unit tests verifying image job routing based on extensions
    - [ ] Add test cases in `internal/worker/jobs_test.go` validating job routing for jpg, png, jpeg, and webp.
- [ ] Task: Integrate image parser into process_source command in internal/worker/jobs.go
    - [ ] Modify `handleProcessSource` to detect image file extensions and call `extractor.ExtractTextFromImage`.
- [ ] Task: Conductor - User Manual Verification 'Phase 2: Ingestion Pipeline Integration' (Protocol in workflow.md)
