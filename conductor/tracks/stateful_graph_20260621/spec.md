# Specification: Stateful Incremental Graph Building in GraphRAG
Track ID: `stateful_graph_20260621`

---

## 🎯 Objectives
Currently, the GraphRAG pipeline in Go Open Notebook clears the entire graph for a notebook and rebuilds all nodes and edges from scratch on every rebuild. This is slow and expensive in LLM token usage.

This specification details the implementation of a stateful, incremental graph rebuild pipeline that only extracts and updates elements for new or modified sources.

## 🛠️ Requirements & Design

### 1. Database Schema Update
Introduce a `sources` array attribute on `RAGEntity` (table) and `co_occurs` (relation edge). This array contains record links to the source documents that contain or co-occur these entities.
*   `RAGEntity.sources` -> `array<record<source>>`
*   `co_occurs.sources` -> `array<record<source>>`
*   `count` (for entities) and `weight` (for relations) will be derived or maintained dynamically based on the size of the `sources` array.

A SurrealDB migration `18.surrealql` will define these fields.

### 2. Source Hashing
Add a `hash` field (representing the SHA256 sum of the extracted source full text) to the `Source` database schema.
*   During source ingestion (`process_source` task), compute the SHA256 of the extracted full text and store it in `Source.hash`.

### 3. Change Detection & Incremental Pipeline
When `BuildGraph` is triggered:
1.  Fetch all sources linked to the notebook.
2.  Compare each source's current content hash with the last indexed hash.
3.  If a source's hash matches the stored value:
    *   **Skip processing** (reuse its existing entity/relation definitions in SurrealDB).
4.  If a source's hash is missing or changed:
    *   **Delete old lineage**: Remove this `source_id` from the `sources` array of all `RAGEntity` and `co_occurs` records that were associated with it. If an entity or relationship has no remaining sources in its list, delete it entirely.
    *   **Run extraction**: Parse the source text into chunks, call the LLM to extract entities and relations, and write them to the database by appending this `source_id` to their `sources` lists.
    *   **Update cache**: Update the source's content hash inside the `Source` table.
5.  Re-run Community Detection & Summaries on the newly updated graph representation.

## 🧪 Acceptance Criteria
1.  Changing one file in a notebook with 5 files only triggers GraphRAG extraction on the changed file.
2.  Deleting a source removes its associated entities and relations (or decrements their weight/count) dynamically in SurrealDB.
3.  Units tests confirm correct source array insertions, increments, decrements, and cleanups.
