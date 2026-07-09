# Project Goal
Implement a Golang-based engine for "Obsidian Bases", which allows querying, filtering, and displaying database-like views (tables, lists, cards) using Markdown frontmatter (YAML) properties. 

To make this engine highly extensible, implement a WebAssembly (WASM) extension function plugin system. This system will allow users to write custom formula functions or complex filters compiled to WASM. The design pattern must follow the sandboxed WASM tool approach used in `https://github.com/innomon/agentic/tree/main/pkg/wasm`.

# Architecture & Design Pattern
We will use `tetratelabs/wazero` as our WebAssembly runtime to ensure zero dependencies (pure Go). 

Following the `innomon/agentic` WASM design pattern:
1. **Host (Go)**: Manages the `wazero.Runtime`, compiles WASM binaries, instantiates modules, and provides host functions (if the plugin needs to call back to the host).
2. **Guest (WASM)**: Exports a standard ABI for execution (e.g., `malloc`, `free`, and `execute`). 
3. **Memory Management**: The host writes input payloads (JSON-encoded note properties and function arguments) into the guest's memory, calls the exported `execute` function, and reads the returned string/bytes from guest memory.

# Step-by-Step Implementation Tasks

## Phase 1: Core Domain Models for Obsidian Bases
1. Create a `models.go` file.
2. Define a `Note` struct that represents a parsed Markdown file:
   - `FilePath` (string)
   - `Properties` (map[string]any) - Extracted from YAML frontmatter.
   - `Content` (string)
3. Define a `BaseConfig` struct that represents a `.base` configuration file:
   - `Filters` (e.g., conditions based on properties)
   - `ViewType` (table, card, list)
   - `Formulas` (map of custom computed column names to their definitions)

## Phase 2: The WASM Plugin Manager (`pkg/wasm`)
Create a package `pkg/wasm` mimicking the `agentic` design pattern:
1. Initialize a `wazero.Runtime` with context.
2. Create a `Plugin` struct that holds the `wazero.CompiledModule` and `wazero.Module` instances.
3. Implement Memory ABI helpers in the Go Host:
   - `allocate(size uint32) uint64` (calls guest `malloc`)
   - `readMemory(offset uint32, size uint32) []byte`
   - `writeMemory(data []byte) (offset uint32)`
4. Implement an `Execute(ctx context.Context, functionName string, payload []byte) ([]byte, error)` method on the `Plugin` struct.

## Phase 3: The WASM Guest SDK & Example Plugin
1. Create a directory `extensions/guest_sdk` (in Go, meant to be compiled to `GOOS=wasip1 GOARCH=wasm`).
2. Write the standard exports required by the host:
   - `//export malloc`
   - `//export free`
3. Write a sample extension function (e.g., `calculate_days_since`) that:
   - Reads JSON from memory (the note properties).
   - Performs a date calculation using a target property.
   - Writes the JSON result back to memory and returns the pointer/size.

## Phase 4: Integration
1. Write a `main.go` that acts as the Obsidian Bases engine.
2. Load mock `Note` data (e.g., 3-4 notes with a "status" and "created_at" property).
3. Load the compiled WASM extension module.
4. Iterate over the notes, invoke the WASM extension to evaluate a custom formula (e.g., calculating "age_in_days" for each note).
5. Print the resulting tabular data to the console, demonstrating a working Obsidian Bases backend engine utilizing a WASM plugin.

# Constraints & Rules
- Use strictly idiomatic Go 1.21+.
- Use `context.Context` for all wazero runtime calls to allow timeout cancellations.
- Handle memory leaks gracefully: ensure the Go host frees guest memory after reading the response, or leverage wazero's memory instantiation effectively.
- Output modular code separated into `pkg/bases` (domain logic), `pkg/wasm` (wazero engine), and `cmd/engine` (CLI).
