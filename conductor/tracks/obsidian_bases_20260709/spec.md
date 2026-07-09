# Specification: Obsidian Bases Engine with WASM Extension System & UI

## Overview
Implement a "Obsidian Bases" feature for Open Notebook. This includes a Golang-based backend engine to query, filter, and apply custom WASM formula functions to Markdown note properties, and a Next.js frontend dashboard. The engine outputs results using the declarative **A2UI** (Agent-to-User Interface) protocol (https://a2ui.org/), which is dynamically rendered in the UI. Users can manage WASM plugin files and configure sandboxing permissions (e.g., read files, access env) via the UI.

## Functional Requirements
1. **Domain Models & Engine (`pkg/bases`)**:
   - `Note`: Model representing parsed markdown files: filepath, content, frontmatter properties (YAML).
   - `BaseConfig`: Model representing `.base` config (YAML/JSON) with filters, view type (table, list, card), custom formulas (mapping column names to WASM exports), and plugin sandbox permissions.
2. **WASM Plugin Manager (`pkg/wasm`)**:
   - Compiles and runs pre-compiled WASM files from `extensions/bin/` using `tetratelabs/wazero`.
   - Guest ABI exports: `malloc`, `free`, and `execute(payloadPtr, payloadSize)`.
   - Host functions: Exposed contextually based on plugin-specific permissions (e.g. reading other notes).
3. **Backend API Endpoints (`internal/api/router/bases.go`)**:
   - `GET /api/bases/plugins`: List available compiled WASM plugins and their current permission configurations.
   - `POST /api/bases/plugins/permissions`: Update sandboxing permissions for a specific plugin.
   - `POST /api/bases/evaluate`: Load a `.base` config, execute the engine over notes, and return A2UI JSON payload.
4. **Frontend UI Components (`frontend/src/app/(dashboard)/bases/`)**:
   - **Bases Dashboard**: UI to select/load a `.base` configuration, choose notes/notebook context, and view rendering.
   - **A2UI Renderer Component**: Renders the declarative JSON output from `/api/bases/evaluate` into beautiful Tailwind/Shadcn tables, card grids, or lists.
   - **Plugins & Permissions Manager**: A configuration panel to see all available WASM plugins and toggle their sandbox permissions (read files, system access).
5. **Guest SDK & Sample Plugin (`extensions/guest_sdk`)**:
   - Standard Go-based SDK compileable to `GOOS=wasip1 GOARCH=wasm` with helper macros and memory exports.
   - Sample plugin: Calculates dates/days since a property value.

## Non-Functional Requirements
- Strictly pure Go + wazero on the backend (no CGO).
- Respect timeouts and cancel execution cleanly.
- High test coverage (>80% unit/integration tests).
- High visual aesthetics for the Next.js pages, utilizing Radix/Shadcn components and responsive layouts.

## Out of Scope
- File system watchers for hot reloading.
- Execution of non-WASM arbitrary scripts (only sandboxed WASM binaries allowed).
