# Specification: Configurable Database Engine with modernc.org/sqlite Support

## Overview
Introduce `modernc.org/sqlite` as a pure Go, CGO-free alternative database backend to SurrealDB, making **SQLite the default engine** (`DB_ENGINE=sqlite`). The active database engine will be dynamically configurable via configuration/environment variables (`DB_ENGINE=sqlite` or `DB_ENGINE=surrealdb`). An abstract database interface/repository layer will decouple business logic from backend storage implementations.

## Functional Requirements
- **Database Abstraction Layer (DAL)**: Define unified repository interfaces for notes, documents, graphs, entities, vectors, and app settings.
- **Configurable DB Engine**: Support selection of `sqlite` (default) or `surrealdb` via configuration files (`.env` / environment variable `DB_ENGINE`).
- **SQLite Engine Implementation**:
  - Implement storage driver using `modernc.org/sqlite` (pure Go, CGO-free).
  - Execute embedded SQL migrations on startup for table/index creation.
  - Store vector embeddings as JSON/BLOB data within SQLite tables and perform similarity search (cosine distance) in Go/SQL helper routines.
- **SurrealDB Engine Refactoring**: Adapt existing SurrealDB interactions to implement the unified DAL interface.
- **Data Parity**: Maintain full feature parity across both database engines (notes CRUD, graph node/edge storage, vector retrieval).

## Non-Functional Requirements
- **Zero CGO Dependency**: Ensure the application remains easily cross-compilable without requiring a C compiler when using `modernc.org/sqlite`.
- **Test Coverage**: Achieve >80% unit test coverage for the repository interfaces and SQLite adapter.
- **Clean Architecture**: Prevent leak of backend-specific types (e.g. SurrealDB RecordIDs or SQL rows) into core domain logic.

## Acceptance Criteria
1. `DB_ENGINE=sqlite` is the default engine, initializing an embedded SQLite database using `modernc.org/sqlite` and creating required tables via automatic migrations.
2. Setting `DB_ENGINE=surrealdb` enables SurrealDB backend.
3. Notebook notes, document chunks, entities, graph structures, and vector similarity search operate seamlessly on both database backends.
4. Unit and integration tests verify repository contracts against both database implementations.

## Out of Scope
- Dual-database live data synchronization/replication.
- External C-based SQLite extensions (e.g. `sqlite-vss`).
