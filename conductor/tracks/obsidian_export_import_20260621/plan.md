# Implementation Plan: Obsidian-Compatible Vault Export and Import

## Phase 1: Export Feature (Backend) [checkpoint: 1bdf146]
- [x] Task: Create Domain and Serialization Logic for Exporting 8d06117
  - [x] Implement serialization of notebook metadata, sources, notes, and graph to structures.
  - [x] Implement conversion of source texts with entity links wrapped in `[[entity_name]]`.
- [x] Task: Write Tests for Export Serialization & Zip Generation 8d06117
  - [x] Write unit tests verifying correct formatting of Markdown files (sources, notes, entities).
  - [x] Write unit tests verifying correct creation of `graph.json` and the `.zip` archive stream.
- [x] Task: Implement HTTP Export Handler and Router Setup 8d06117
  - [x] Add `GET /api/notebooks/:id/export` route to the server.
  - [x] Implement streaming zip response.
- [x] Task: Conductor - User Manual Verification 'Phase 1: Export Feature (Backend)' (Protocol in workflow.md)

## Phase 2: Import Feature (Backend)
- [x] Task: Create Import Parser Logic 8d06117
  - [x] Implement parsing of zipped archives.
  - [x] Implement file extraction and identification of `sources/`, `notes/`, `entities/`, and `graph.json`.
- [x] Task: Implement Hash Comparison and Conflict Resolution Logic 8d06117
  - [x] Implement SHA256 hash comparison logic for imported sources.
  - [x] If hash exists, skip importing. If name conflicts but hashes differ, resolve with suffix.
- [x] Task: Write Tests for Import and Conflict Resolution 8d06117
  - [x] Write unit tests for the zip parsing and database insertion logic.
  - [x] Write unit tests verifying that duplicates are skipped when hashes match.
  - [x] Write unit tests verifying that name conflicts are resolved with suffixes when hashes differ.
- [x] Task: Implement HTTP Import Handlers and Router Setup 8d06117
  - [x] Add `POST /api/notebooks/import` and `POST /api/notebooks/:id/import` routes.
  - [x] Implement file upload parsing and invoke import logic.
- [ ] Task: Conductor - User Manual Verification 'Phase 2: Import Feature (Backend)' (Protocol in workflow.md)

## Phase 3: Frontend Integration and Settings UI
- [x] Task: Design and Implement Import/Export UI Components dbb044a
  - [x] Add "Export Vault" and "Import Vault" action controls in the Notebook Settings/Advanced panel.
  - [x] Implement client-side file upload and download trigger.
  - [x] Add progress indicators and toast notifications for success/failure.
- [x] Task: Write Frontend Integration Tests dbb044a
  - [x] Mock API requests for import and export endpoints.
  - [x] Verify UI component renders, fires requests, and handles progress/toasts correctly.
- [ ] Task: Conductor - User Manual Verification 'Phase 3: Frontend Integration and Settings UI' (Protocol in workflow.md)
