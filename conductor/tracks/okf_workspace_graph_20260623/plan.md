# Implementation Plan: OKF Integration — Phase 2: Workspace Indexer & Graph Construction

## Phase 1: Database Schema & Migrations [checkpoint: 8990a53]

- [x] Task: Design and create SurrealDB schema migrations c751115
    - [x] Create a new migration file under `internal/db/migrations/` (or determine existing migration conventions)
    - [x] Define the `okf_node` schema table with fields for workspace_path, file_path, metadata, hash, and updated timestamp
    - [x] Define the `okf_link` schema table as relation edge between `okf_node` records
    - [x] Ensure migrations run automatically on app boot and verify table definitions exist using raw queries
- [x] Task: Conductor - User Manual Verification 'Phase 1: Database Schema & Migrations' (Protocol in workflow.md) 8990a53

## Phase 2: TDD Red Phase — Write Failing Tests [checkpoint: 690ac85]

- [x] Task: Create tests for Workspace Indexer & API routing 8072b70
    - [x] Create `pkg/okf/workspace_test.go` and write tests for `WorkspaceIndexer` walking files and writing nodes/links to SurrealDB
    - [x] Add tests for hash caching (mismatch updates, match skips) and removal of deleted files
    - [x] Create `internal/api/router/okf_test.go` with tests for `/api/okf/validate` and `/api/okf/graph` endpoints
    - [x] Include tests for fsnotify background watcher changes updating the database nodes
- [x] Task: Run tests and confirm Red phase — tests must FAIL/compilation fail before implementation 18a150c
    - [x] Execute `go test -v ./pkg/okf/...`
    - [x] Confirm failures due to missing indexer, watcher, and handlers
- [x] Task: Conductor - User Manual Verification 'Phase 2: TDD Red Phase — Write Failing Tests' (Protocol in workflow.md) 690ac85

## Phase 3: Green Phase — Implement Indexer, Watcher & API [checkpoint: f19ae5a]

- [x] Task: Implement `WorkspaceIndexer` in `pkg/okf/workspace.go` 7b0ff66
    - [x] Implement `WorkspaceIndexer` struct and constructor
    - [x] Implement recursive directory walking and parsing with hash cache checking
    - [x] Implement DB transactional queries to create/update `okf_node` records and purge/replace `okf_link` relations
- [x] Task: Implement dynamic FSNotify Watcher in `pkg/okf/watcher.go` 7b0ff66
    - [x] Implement watcher lifecycle manager pool to dynamically add/remove directory watches
    - [x] Hook events (Write, Create, Delete, Rename) to update the database records in real-time
    - [x] Implement inactivity timeout cleanup of watcher resources
- [x] Task: Implement HTTP Route Handlers in `internal/api/router/okf.go` c43ddfb
    - [x] Register `POST /api/okf/validate` and `GET /api/okf/graph` routes in the main router
    - [x] Implement validate handler (scans folder, validates schema, returns file error list)
    - [x] Implement graph handler (fetches nodes/links from DB, runs indexer dynamically, spawns/updates watcher)
- [x] Task: Run tests and confirm Green phase — all tests must PASS c43ddfb
    - [x] Run `go test -v ./pkg/okf/...` and `go test -v ./internal/api/router/...`
    - [x] Confirm all tests pass successfully
- [x] Task: Conductor - User Manual Verification 'Phase 3: Green Phase — Implement Indexer, Watcher & API' (Protocol in workflow.md) f19ae5a

## Phase 4: Refactor & Quality Gate [checkpoint: 45a9898]

- [x] Task: Refactor and optimize graph operations e67a1f3
    - [x] Review query transactions for performance and potential locks
    - [x] Review concurrency safety of fsnotify watcher and indexer updates
- [x] Task: Enforce code quality checks e67a1f3
    - [x] Run `go fmt ./...` and commit any formatting changes
    - [x] Run `go vet ./...` and resolve any warnings
    - [x] Generate coverage profile and verify coverage ≥80% for new packages
    - [x] Run all workspace tests to confirm zero regression
- [x] Task: Conductor - User Manual Verification 'Phase 4: Refactor & Quality Gate' (Protocol in workflow.md) 45a9898
