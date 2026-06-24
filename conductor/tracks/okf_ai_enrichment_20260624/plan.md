# Implementation Plan: OKF Integration — Phase 4: Local AI Context/Enrichment Automation

## Phase 1: Backend AI Enrichment Endpoint [checkpoint: ]

- [x] Task: Create tests for `POST /api/okf/enrich` (TDD Red Phase) 0beb3f0
    - [x] Create test cases inside `internal/api/router/okf_test.go` or a new `internal/api/router/okf_enrich_test.go`
    - [x] Write tests verifying in-memory note parsing, LLM prompting, structured JSON extraction, and disk file read/write update behavior
    - [x] Assert failure states when invalid payloads are supplied
    - [x] Run test suite and confirm failure
- [ ] Task: Implement Backend Enrichment Handler (Green Phase)
    - [ ] Register `POST /api/okf/enrich` in `internal/api/router/okf.go`
    - [ ] Parse request payload (accepts `content` and optional `path`)
    - [ ] Resolve default transformation model using `ai.GetClientForDefaultModel(r.Context(), "transformation")`
    - [ ] Invoke `GenerateText` with a strict system prompt instructing JSON formatting of `description` and `tags`
    - [ ] Extract and sanitize JSON from LLM response
    - [ ] If file path is provided, read note, update YAML frontmatter fields (`description`, `tags`) using standard serialization, save back to disk, and trigger file watchers
    - [ ] Respond with JSON payload containing `description` and `tags`
    - [ ] Run tests and verify they pass
- [ ] Task: Conductor - User Manual Verification 'Phase 1: Backend AI Enrichment Endpoint' (Protocol in workflow.md)

## Phase 2: Frontend "AI Suggest" UI Integration [checkpoint: ]

- [ ] Task: Create tests for properties editor AI Suggest button (TDD Red Phase)
    - [ ] Update `frontend/src/test/okf/PropertiesEditor.test.tsx` to assert rendering of "AI Suggest" button
    - [ ] Write mock API hook tests to simulate calling `/api/okf/enrich` and verifying that Title, Description, and Tags fields are populated and saved
    - [ ] Confirm test suite failure state
- [ ] Task: Implement "AI Suggest" Button & Properties Autofill (Green Phase)
    - [ ] Update `frontend/src/components/okf/PropertiesEditor.tsx` props to accept the note's active body content `noteBody`
    - [ ] Add an "AI Suggest" button next to the "Properties" label, styled with a Sparkles icon from `lucide-react`
    - [ ] Implement query handler using `apiClient.post('/okf/enrich', { content: ... })` with active note content
    - [ ] Display loading spinner and animate form inputs (or button) while query is pending
    - [ ] Autofill fields (`title`, `description`, `tags`) upon receiving suggestions and trigger `onSave` debounced save to note editor form
    - [ ] Run tests and verify they pass
- [ ] Task: Conductor - User Manual Verification 'Phase 2: Frontend "AI Suggest" UI Integration' (Protocol in workflow.md)
