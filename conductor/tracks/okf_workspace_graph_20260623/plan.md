# Implementation Plan: OKF Integration — Phase 2: Workspace Indexer & Graph Construction

## Phase 1: Database Schema & Migrations [checkpoint: 8990a53]

- [x] Task: Design and create SurrealDB schema migrations c751115
    - [x] Create a new migration file under `internal/db/migrations/` (or determine existing migration conventions)
    - [x] Define the `okf_node` schema table with fields for workspace_path, file_path, metadata, hash, and updated timestamp
    - [x] Define the `okf_link` schema table as relation edge between `okf_node` records
    - [x] Ensure migrations run automatically on app boot and verify table definitions exist using raw queries
- [x] Task: Conductor - User Manual Verification 'Phase 1: Database Schema & Migrations' (Protocol in workflow.md) 8990a53

## Phase 2: TDD Red Phase — Write Failing Tests

- [~] Task: Create tests for Workspace Indexer & API routing
    - [~] Create `pkg/okf/workspace_test.go` and write tests for `WorkspaceIndexer` walking files and writing nodes/links to SurrealDB
    - [ ] Add tests for hash caching (mismatch updates, match skips) and removal of deleted files
    - [ ] Create `internal/api/router/okf_test.go` with tests for `/api/okf/validate` and `/api/okf/graph` endpoints
    - [ ] Include tests for fsnotify background watcher changes updating the database nodes
- [ ] Task: Run tests and confirm Red phase — tests must FAIL/compilation fail before implementation
    - [ ] Execute `go test -v ./pkg/okf/...` and `go test -v ./internal/api/router/...`
    - [ ] Confirm failures due to missing indexer, watcher, and handlers
- [ ] Task: Conductor - User Manual Verification 'Phase 2: TDD Red Phase — Write Failing Tests' (Protocol in workflow.md)

## Phase 3: Green Phase — Implement Indexer, Watcher & API

- [ ] Task: Implement `WorkspaceIndexer` in `pkg/okf/workspace.go`
    - [ ] Implement `WorkspaceIndexer` struct and constructor
    - [ ] Implement recursive directory walking and parsing with hash cache checking
    - [ ] Implement DB transactional queries to create/update `okf_node` records and purge/replace `okf_link` relations
- [ ] Task: Implement dynamic FSNotify Watcher in `pkg/okf/watcher.go`
    - [ ] Implement watcher lifecycle manager pool to dynamically add/remove directory watches
    - [ ] Hook events (Write, Create, Delete, Rename) to update the database records in real-time
    - [ ] Implement inactivity timeout cleanup of watcher resources
- [ ] Task: Implement HTTP Route Handlers in `internal/api/router/okf.go`
    - [ ] Register `POST /api/okf/validate` and `GET /api/okf/graph` routes in the main router
    - [ ] Implement validate handler (scans folder, validates schema, returns file error list)
    - [ ] Implement graph handler (fetches nodes/links from DB, runs indexer dynamically, spawns/updates watcher)
- [ ] Task: Run tests and confirm Green phase — all tests must PASS
    - [ ] Run `go test -v ./pkg/okf/...` and `go test -v ./internal/api/router/...`
    - [ ] Confirm all tests pass successfully
- [ ] Task: Conductor - User Manual Verification 'Phase 3: Green Phase — Implement Indexer, Watcher & API' (Protocol in workflow.md)

## Phase 4: Refactor & Quality Gate

- [ ] Task: Refactor and optimize graph operations
    - [ ] Review query transactions for performance and potential locks
    - [ ] Review concurrency safety of fsnotify watcher and indexer updates
- [ ] Task: Enforce code quality checks
    - [ ] Run `go fmt ./...` and commit any formatting changes
    - [ ] Run `go vet ./...` and resolve any warnings
    - [ ] Generate coverage profile and verify coverage ≥80% for new packages
    - [ ] Run all workspace tests to confirm zero regression
- [ ] Task: Conductor - User Manual Verification 'Phase 4: Refactor & Quality Gate' (Protocol in workflow.md)
