# Implementation Plan: Configurable Database Engine (`sqlite_support`)

## Phase 1: Database Abstraction & Configuration Layer [checkpoint: 4251434]
- [x] Task: Define database engine configuration models, environment variables, and factory interfaces. [512b186]
- [x] Task: Design core repository interfaces for domain entities (Notes, Documents, Graph/Entities, Vectors, Settings). [b31ccc2]
- [x] Task: Conductor - User Manual Verification 'Phase 1: Database Abstraction & Configuration Layer' (Protocol in workflow.md) [4251434]

## Phase 2: SQLite Storage Engine Implementation (`modernc.org/sqlite`)
- [x] Task: Integrate `modernc.org/sqlite` dependency and set up embedded migration runner. [cfbeb23]
- [ ] Task: Create SQLite schema definitions and initial migration scripts for all domain entities.
- [ ] Task: Implement SQLite repositories adhering to the domain repository interfaces.
- [ ] Task: Implement in-memory / SQL cosine similarity vector search for SQLite embeddings.
- [ ] Task: Conductor - User Manual Verification 'Phase 2: SQLite Storage Engine Implementation' (Protocol in workflow.md)

## Phase 3: SurrealDB Refactoring & DAL Adaptation
- [ ] Task: Refactor SurrealDB client code to implement domain repository interfaces.
- [ ] Task: Update database initialization and lifecycle manager to support dynamic engine switching (`sqlite` default vs `surrealdb`).
- [ ] Task: Conductor - User Manual Verification 'Phase 3: SurrealDB Refactoring & DAL Adaptation' (Protocol in workflow.md)

## Phase 4: Integration, Testing & Quality Verification
- [ ] Task: Write unit and integration tests covering repository operations for both SQLite and SurrealDB drivers.
- [ ] Task: Update documentation (`README.md`, `tech-stack.md`, `.env.example`) to document SQLite default configuration and engine options.
- [ ] Task: Conductor - User Manual Verification 'Phase 4: Integration, Testing & Quality Verification' (Protocol in workflow.md)
