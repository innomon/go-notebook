# Implementation Plan: Obsidian-Compatible Vault Export and Import

## Phase 1: Export Feature (Backend)
- [ ] Task: Create Domain and Serialization Logic for Exporting
  - [ ] Implement serialization of notebook metadata, sources, notes, and graph to structures.
  - [ ] Implement conversion of source texts with entity links wrapped in `[[entity_name]]`.
- [ ] Task: Write Tests for Export Serialization & Zip Generation
  - [ ] Write unit tests verifying correct formatting of Markdown files (sources, notes, entities).
  - [ ] Write unit tests verifying correct creation of `graph.json` and the `.zip` archive stream.
- [ ] Task: Implement HTTP Export Handler and Router Setup
  - [ ] Add `GET /api/notebooks/:id/export` route to the server.
  - [ ] Implement streaming zip response.
- [ ] Task: Conductor - User Manual Verification 'Phase 1: Export Feature (Backend)' (Protocol in workflow.md)

## Phase 2: Import Feature (Backend)
- [ ] Task: Create Import Parser Logic
  - [ ] Implement parsing of zipped archives.
  - [ ] Implement file extraction and identification of `sources/`, `notes/`, `entities/`, and `graph.json`.
- [ ] Task: Implement Hash Comparison and Conflict Resolution Logic
  - [ ] Implement SHA256 hash comparison logic for imported sources.
  - [ ] If hash exists, skip importing. If name conflicts but hashes differ, resolve with suffix.
- [ ] Task: Write Tests for Import and Conflict Resolution
  - [ ] Write unit tests for the zip parsing and database insertion logic.
  - [ ] Write unit tests verifying that duplicates are skipped when hashes match.
  - [ ] Write unit tests verifying that name conflicts are resolved with suffixes when hashes differ.
- [ ] Task: Implement HTTP Import Handlers and Router Setup
  - [ ] Add `POST /api/notebooks/import` and `POST /api/notebooks/:id/import` routes.
  - [ ] Implement file upload parsing and invoke import logic.
- [ ] Task: Conductor - User Manual Verification 'Phase 2: Import Feature (Backend)' (Protocol in workflow.md)

## Phase 3: Frontend Integration and Settings UI
- [ ] Task: Design and Implement Import/Export UI Components
  - [ ] Add "Export Vault" and "Import Vault" action controls in the Notebook Settings/Advanced panel.
  - [ ] Implement client-side file upload and download trigger.
  - [ ] Add progress indicators and toast notifications for success/failure.
- [ ] Task: Write Frontend Integration Tests
  - [ ] Mock API requests for import and export endpoints.
  - [ ] Verify UI component renders, fires requests, and handles progress/toasts correctly.
- [ ] Task: Conductor - User Manual Verification 'Phase 3: Frontend Integration and Settings UI' (Protocol in workflow.md)
