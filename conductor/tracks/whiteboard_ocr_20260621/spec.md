# Specification: Whiteboard OCR and Image Notes Processing

## Overview
Implement an image ingestion parser for png, jpeg, and jpg files in `go-notebook` to support whiteboard drawings and image notes. The parser will run local Tesseract OCR if available on the system; otherwise, it will fall back to using the configured LLM Vision API (such as Gemini or GPT-4o) to transcribe text and generate structured markdown summaries/descriptions of non-text whiteboard elements (like drawings and diagrams). The extracted markdown is saved as the source's full text and ingested into the knowledge graph.

## Functional Requirements
1. **OCR Engine Routing**:
   - Check if `tesseract` binary is installed on the host system using `exec.LookPath("tesseract")`.
   - If `tesseract` is available, execute a local command line call (`tesseract input.png output.txt`) to extract raw text.
   - If `tesseract` is missing, or fails, call the configured LLM Vision API to extract text and analyze drawings.
2. **Visual Content Analysis**:
   - Use the LLM Vision model (e.g. `gemini-1.5-flash` or similar active vision model) to describe drawing structures, diagrams, flowcharts, or text layout.
   - Format the final output as a structured Markdown document containing both the transcribed text and the visual description.
3. **Integration**:
   - Create a separate parser package `internal/extractor/image.go`.
   - Hook it into `internal/worker/jobs.go` under the `process_source` command when image suffixes (`.png`, `.jpg`, `.jpeg`, `.webp`) are detected.
4. **Resilience**:
   - If both local Tesseract and cloud LLM Vision fail (e.g., due to offline or invalid keys), return an informative extraction error.

## Non-Functional Requirements
- **No Cgo Bindings**: Invoke Tesseract via CLI commands (`os/exec`) to avoid complex Cgo bindings and static compilation issues.
- **API Model Detection**: Determine and use the default active model for vision queries.

## Acceptance Criteria
- Uploading an image file runs the parser flow.
- If Tesseract is installed, it extracts raw text and saves it.
- If Tesseract is absent, the system calls the LLM Vision API, returning structured Markdown with both text and drawing descriptions.
- The output text is stored in `Source.full_text` and indexed into the GraphRAG pipeline.
- Integration tests mock CLI execution and API payloads.

## Out of Scope
- Direct bounding box coordinate mapping on the visualizer canvas.
- Support for vector format images (`.svg`).
