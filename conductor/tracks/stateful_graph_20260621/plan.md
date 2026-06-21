# Implementation Plan: Stateful Incremental Graph Building in GraphRAG
Track ID: `stateful_graph_20260621`

---

## Phase 1: Database Schema & Domain Model Migration

- [x] Task: Write unit tests verifying RAGEntity and co_occurs serialization and source arrays in rag_graph.go [eb5cfc3]
    - [x] Add test cases in `internal/domain/rag_graph_test.go` covering serialization of the `Sources` array fields.
- [x] Task: Update domain structs and field mappings for RAGEntity and co_occurs [eb5cfc3]
    - [x] Modify `RAGEntity` struct in `internal/domain/rag_graph.go` to include `Sources []string` or `Sources []*models.RecordID` and update JSON tags.
    - [x] Modify `RAGCommunity` struct if needed to support communities.
- [ ] Task: Create database migration scripts for SurrealDB
    - [ ] Create `internal/db/migrations/18.surrealql` containing table field adjustments for `sources` arrays.
    - [ ] Create `internal/db/migrations/18_down.surrealql` to reverse the updates.
- [ ] Task: Update the Go migration manager to run migration 18
    - [ ] Update `internal/db/migration_manager.go` to check and execute migrations up to version 18.
- [ ] Task: Conductor - User Manual Verification 'Phase 1: Database Schema & Domain Model Migration' (Protocol in workflow.md)

## Phase 2: Refactoring Graph Operations

- [ ] Task: Write unit tests for CreateOrUpdateEntity and RelateEntities showing source-array lineage tracking
    - [ ] Add tests in `internal/domain/rag_graph_test.go` asserting correct behavior of entity/edge creation with source tracking.
- [ ] Task: Refactor CreateOrUpdateEntity to upsert and track source lists
    - [ ] Update the `CreateOrUpdateEntity` implementation in `internal/domain/rag_graph.go` to append the source ID to the `sources` field.
- [ ] Task: Refactor RelateEntities to upsert and track source lists in co_occurs relationships
    - [ ] Update the `RelateEntities` implementation in `internal/domain/rag_graph.go` to append the source ID to the relationship edge's `sources` field.
- [ ] Task: Conductor - User Manual Verification 'Phase 2: Refactoring Graph Operations' (Protocol in workflow.md)

## Phase 3: Hash Tracking & Incremental Pipeline

- [ ] Task: Write unit tests verifying incremental pipeline updates and delta indexing
    - [ ] Write mock pipeline test suite in `internal/graphrag/pipeline_test.go`.
- [ ] Task: Add content hash support to the Source domain model and ingestion logic
    - [ ] Add `Hash` string field to `Source` struct in `internal/domain/source.go`.
    - [ ] Compute the SHA256 of source full text during ingestion in `internal/worker/jobs.go` and update the database record.
- [ ] Task: Implement ClearSourceGraphLineage database function
    - [ ] Implement `ClearSourceGraphLineage` in `internal/domain/rag_graph.go` to remove a specific source ID from all entity and relation source lists.
    - [ ] Delete entity and relation records whose source lists become empty.
- [ ] Task: Refactor BuildGraph to use incremental change detection
    - [ ] Modify `BuildGraph` in `internal/graphrag/pipeline.go` to compare content hashes and selectively run lineage cleanup and LLM extraction on new/modified sources.
- [ ] Task: Conductor - User Manual Verification 'Phase 3: Hash Tracking & Incremental Pipeline' (Protocol in workflow.md)
