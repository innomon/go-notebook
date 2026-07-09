# Implementation Plan: Obsidian Bases Engine with WASM Extension System & UI

## Phase 1: Core Domain Models & Note Parser [checkpoint: bc522e2]
- [x] Task: Design and implement domain models and frontmatter parsing (d918547)
    - [ ] Write failing unit tests in `pkg/bases/models_test.go` and `pkg/bases/parser_test.go` verifying Note frontmatter extraction and BaseConfig parsing.
    - [ ] Implement `Note`, `BaseConfig`, and `HostPermissions` structures in `pkg/bases/models.go`.
    - [ ] Implement frontmatter YAML extraction and Markdown parser in `pkg/bases/parser.go`.
    - [ ] Run tests and verify they pass (Green phase).
- [x] Task: Implement Base filtering and formula mapping logic (91ee53b)
    - [ ] Write failing unit tests for note filtering and formula evaluation.
    - [ ] Implement query and filtering logic in `pkg/bases/engine.go`.
    - [ ] Implement formula evaluation mapping.
    - [ ] Verify test coverage is >80% for `pkg/bases` package.
- [x] Task: Conductor - User Manual Verification 'Phase 1: Core Domain Models & Note Parser' (Protocol in workflow.md) (bc522e2)

## Phase 2: WASM Plugin Manager & Guest SDK
- [x] Task: Implement Go WASM Host Manager (b2ec902)
    - [ ] Write failing unit tests in `pkg/wasm/manager_test.go` for the wazero runtime manager, testing memory helpers and execution.
    - [ ] Create `pkg/wasm/manager.go` and initialize the wazero runtime with contexts and timeouts.
    - [ ] Implement Go Host helpers: `allocate`, `readMemory`, `writeMemory`, and `free`.
    - [ ] Implement context-aware plugin `Execute` function invoking the guest's `execute` ABI.
    - [ ] Run tests to ensure correct memory manipulation.
- [x] Task: Implement Sandbox Permissions & Host Callbacks (b2ec902)
    - [ ] Write unit tests verifying Host Functions are blocked/allowed based on config permissions.
    - [ ] Implement Host Functions in `pkg/wasm/host.go` allowing guest plugins to read other notes if permitted.
- [ ] Task: Build Guest SDK & Sample Plugin
    - [ ] Create guest SDK directory `extensions/guest_sdk/` containing allocation exports (`malloc`, `free`) in Go.
    - [ ] Create a sample plugin (e.g. `calculate_days_since`) and write a shell command/target to compile it to `extensions/bin/calculate_days_since.wasm` using `GOOS=wasip1 GOARCH=wasm`.
    - [ ] Run integration tests loading the compiled WASM and executing it against notes.
    - [ ] Verify test coverage is >80% for `pkg/wasm` package.
- [ ] Task: Conductor - User Manual Verification 'Phase 2: WASM Plugin Manager & Guest SDK' (Protocol in workflow.md)

## Phase 3: Backend API Endpoints & CLI Engine
- [ ] Task: Build command line engine wrapper
    - [ ] Create CLI interface at `cmd/engine/main.go` loading a workspace of Markdown files and a `.base` config, running the engine, and printing A2UI JSON to stdout.
    - [ ] Write integration test verifying CLI output.
- [ ] Task: Implement HTTP Router Endpoints
    - [ ] Write failing integration tests in `internal/api/router/bases_test.go` checking `/api/bases/plugins` and `/api/bases/evaluate` responses.
    - [ ] Create `internal/api/router/bases.go` containing HTTP handler functions.
    - [ ] Register new routes in `internal/api/router/router.go`.
    - [ ] Ensure API handler uses database settings or local storage to resolve notes.
    - [ ] Run all test suites and verify they pass with >80% coverage.
- [ ] Task: Conductor - User Manual Verification 'Phase 3: Backend API Endpoints & CLI Engine' (Protocol in workflow.md)

## Phase 4: Frontend UI Pages & A2UI Renderer
- [ ] Task: Implement A2UI Declarative Renderer component
    - [ ] Create A2UI client-side renderer `frontend/src/components/bases/A2UIRenderer.tsx` that maps A2UI JSON elements to beautiful Shadcn UI Tables, Lists, and Cards.
    - [ ] Write React unit/integration tests for the A2UI renderer component.
- [ ] Task: Implement Plugins & Permissions Dashboard Page
    - [ ] Create `frontend/src/app/(dashboard)/bases/page.tsx` for workspace file selection, engine execution, and A2UI display.
    - [ ] Add Plugins & Permissions Management UI tab to manage permissions and view compiled plugin states.
    - [ ] Add "Bases" to sidebar navigation in `frontend/src/components/layout/AppSidebar.tsx` and add localized strings.
- [ ] Task: Conductor - User Manual Verification 'Phase 4: Frontend UI Pages & A2UI Renderer' (Protocol in workflow.md)
