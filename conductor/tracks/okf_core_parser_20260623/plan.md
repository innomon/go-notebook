# Implementation Plan: OKF Integration — Phase 1: Core Foundation & Metadata Parser

## Phase 1: Core Types & Foundational Package Setup [checkpoint: b4e6c84]

- [x] Task: Create `pkg/okf/` directory and initialize the Go package structure 230b9ae
    - [x] Create the directory `pkg/okf/`
    - [x] Verify the package is accessible from the module root via `go list ./pkg/okf/...`
- [x] Task: Define OKF type system in `pkg/okf/types.go` 199d721
    - [x] Declare `package okf` and import `time`
    - [x] Define the `Metadata` struct with all 7 fields and correct YAML struct tags
    - [x] Define the `BundleNode` struct with `ID`, `Metadata`, and `OutboundLinks` fields and JSON tags
    - [x] Define and export `ErrNoFrontmatter` and `ErrMissingFields` sentinel errors
    - [x] Add GoDoc comments to all exported types, fields, and errors
- [x] Task: Conductor - User Manual Verification 'Phase 1: Core Types & Foundational Package Setup' (Protocol in workflow.md) b4e6c84

## Phase 2: TDD Red Phase — Write Failing Tests [checkpoint: c431348]

- [x] Task: Create test file `pkg/okf/parser_test.go` with failing test suite 775300b
    - [x] Write `TestParseDocument_ValidOKF`: assert `meta.Type == "Go Struct"` and body contains expected header
    - [x] Write `TestParseDocument_MissingMandatoryFields`: assert `ErrMissingFields` when `type` field is absent
    - [x] Write `TestParseDocument_NoFrontmatter`: assert `ErrNoFrontmatter` for plain markdown without `---` block
    - [x] Write `TestParseDocument_EmptyBody`: assert valid parse with empty body slice when no body content exists
    - [x] Write `TestParseDocument_FrontmatterOnly`: assert body is empty but metadata is valid for frontmatter-only docs
    - [x] Write `TestExtractLinks_ValidPaths`: assert exactly 2 internal links, external HTTPS link excluded
    - [x] Write `TestExtractLinks_EmptyBody`: assert empty `[]string` for a body with no markdown links
    - [x] Write `TestExtractLinks_OnlyExternalLinks`: assert empty `[]string` when only `http://`/`https://` links are present
- [x] Task: Run tests and confirm Red phase — all 8 tests must FAIL before implementation ec58208
    - [x] Execute `go test -v ./pkg/okf/...`
    - [x] Confirm compilation fails (types/functions not yet implemented) — this is the expected Red state
- [x] Task: Conductor - User Manual Verification 'Phase 2: TDD Red Phase — Write Failing Tests' (Protocol in workflow.md) c431348

## Phase 3: Green Phase — Implement Parser & Extractor [checkpoint: 304f685]

- [x] Task: Implement `ParseDocument` in `pkg/okf/parser.go` 5f67ec5
    - [x] Declare `package okf` with imports: `bufio`, `bytes`, `errors`, `io`, `strings`, `gopkg.in/yaml.v3`
    - [x] Implement the streaming `bufio.Scanner` line-by-line frontmatter detection logic
    - [x] Handle the `---` open/close boundary detection on line 1
    - [x] Accumulate frontmatter lines into a `bytes.Buffer` and body lines into a separate buffer
    - [x] Call `yaml.Unmarshal` on the frontmatter buffer into `*Metadata`
    - [x] Return `ErrNoFrontmatter` if no valid frontmatter boundary was found
    - [x] Return `ErrMissingFields` if `meta.Type`, `meta.Title`, or `meta.Description` are empty strings
    - [x] Add GoDoc comment to `ParseDocument`
- [x] Task: Implement `ExtractLinks` in `pkg/okf/indexer.go` 5f67ec5
    - [x] Declare `package okf` with imports: `regexp`, `strings`
    - [x] Declare `LinkRegex` as a package-level compiled `*regexp.Regexp` for `\[[^\]]+\]\(([^)]+)\)`
    - [x] Implement `ExtractLinks(body []byte) []string` using `FindAllSubmatch`
    - [x] Filter results: skip entries where the captured group starts with `http://` or `https://`
    - [x] Return the collected internal link paths slice
    - [x] Add GoDoc comments to `LinkRegex` and `ExtractLinks`
- [x] Task: Run tests and confirm Green phase — all 8 tests must PASS 0587764
    - [x] Execute `go test -v -race ./pkg/okf/...`
    - [x] Confirm all 8 tests pass
- [x] Task: Conductor - User Manual Verification 'Phase 3: Green Phase — Implement Parser & Extractor' (Protocol in workflow.md) 304f685

## Phase 4: Refactor & Quality Gate

- [~] Task: Refactor for clarity and robustness (optional but recommended)
    - [~] Review `ParseDocument` for any edge case missed (e.g., Windows-style `\r\n` line endings)
    - [ ] Review `ExtractLinks` regex for correctness against parentheses in link labels
    - [ ] Rerun tests post-refactor to ensure no regression
- [ ] Task: Enforce code quality checks
    - [ ] Run `go fmt ./pkg/okf/...` and commit any formatting changes
    - [ ] Run `go vet ./pkg/okf/...` and resolve any warnings
    - [ ] Run `go test -v -race -coverprofile=coverage.txt ./pkg/okf/...` and verify coverage ≥80% (target: 100%)
    - [ ] Confirm no existing tests in the repository are broken: `go test ./...`
- [ ] Task: Conductor - User Manual Verification 'Phase 4: Refactor & Quality Gate' (Protocol in workflow.md)
