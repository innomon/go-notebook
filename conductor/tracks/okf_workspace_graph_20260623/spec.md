# Track Spec: OKF Integration — Phase 2: Workspace Indexer & Graph Construction

## Overview

This track implements **Phase 2** of the Open Knowledge Format (OKF) integration into `go-notebook`. It builds the workspace crawling, database persistence, and directory-watching layer. The OKF knowledge graph (nodes, metadata, and links) will be persisted in SurrealDB, enabling fast on-demand querying, incremental updates via file watches, and future graph queries.

## Functional Requirements

### FR-1: Database Schema & Migrations (`internal/db/migrations/`)
- Define schema migration scripts (`.surrealql`) to support the OKF graph:
  - `okf_node` table storing:
    - `id`: unique composite record ID (e.g. `okf_node:[workspace_path, relative_file_path]`)
    - `workspace_path`: `string`
    - `file_path`: `string` (relative file path)
    - `metadata`: object containing parsed OKF Metadata fields
    - `hash`: `string` (SHA-256 of file contents for incremental change detection)
    - `updated`: `datetime`
  - `okf_link` relation table (`DEFINE TABLE okf_link TYPE RELATION`) connecting `okf_node` records.

### FR-2: Workspace Graph Indexer (`pkg/okf/workspace.go`)
- Implement a `WorkspaceIndexer` that:
  - Walks the target workspace folder recursively using Go's `os.WalkDir`.
  - For each `.md` file, checks its hash against the stored database node:
    - **Hash Match**: Skip parsing (cache hit).
    - **Hash Mismatch / Missing**: Re-parse frontmatter/links, update the database record, delete existing outbound links from this node, and create new relation edges to target nodes.
  - Detects files that are no longer on disk and removes their node/link records from SurrealDB.
  - Protects index operations with transactional safety.

### FR-3: Dynamic Watcher Lifecycle Manager (`pkg/okf/watcher.go`)
- Implement a background directory watcher using `github.com/fsnotify/fsnotify`:
  - Dynamically registers recursive watchers when a workspace path is first accessed.
  - On file write/creation: Calculates hash, parses, and updates the database node and relations.
  - On file delete: Removes the node and relations from SurrealDB.
  - Releases watcher resources after a timeout of inactivity.

### FR-4: API Routing & Handlers (`internal/api/router/okf.go`)
- Register the following REST endpoints on the native Go router:
  - `POST /api/okf/validate`
    - Payload: `{"path": "/absolute/or/relative/path"}`
    - Behavior: Crawls the directory, validates all markdown files against OKF schema criteria, and returns lists of errors per file.
  - `GET /api/okf/graph?path=/absolute/or/relative/path`
    - Behavior: Retrieves nodes and relations for the path from SurrealDB and starts/updates the fsnotify watcher.

### FR-5: Test Suite (`pkg/okf/workspace_test.go` & `internal/api/router/okf_test.go`)
- Implement a unit and integration test suite (using mock files and a test database instance) verifying:
  - Table schemas compile and allow relation traversals.
  - Database nodes/edges update incrementally on hash mismatches.
  - File watching maps disk changes directly to SurrealDB records.
  - Endpoints validate and return graph data.

## Non-Functional Requirements

- **Dependency**: Add `github.com/fsnotify/fsnotify` to `go.mod`.
- **Quality Gates**: Maintain code coverage >=80% on all new code. Vetted clean with `go vet`.
- **Docs**: Fully compliant with `godoc-canonical` comments on all exported API symbols.
