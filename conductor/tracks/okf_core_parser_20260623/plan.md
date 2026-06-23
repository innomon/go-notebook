# Implementation Plan: OKF Integration — Phase 1: Core Foundation & Metadata Parser

## Phase 1: Core Types & Foundational Package Setup

- [ ] Task: Create `pkg/okf/` directory and initialize the Go package structure
    - [ ] Create the directory `pkg/okf/`
    - [ ] Verify the package is accessible from the module root via `go list ./pkg/okf/...`
- [ ] Task: Define OKF type system in `pkg/okf/types.go`
    - [ ] Declare `package okf` and import `time`
    - [ ] Define the `Metadata` struct with all 7 fields and correct YAML struct tags
    - [ ] Define the `BundleNode` struct with `ID`, `Metadata`, and `OutboundLinks` fields and JSON tags
    - [ ] Define and export `ErrNoFrontmatter` and `ErrMissingFields` sentinel errors
    - [ ] Add GoDoc comments to all exported types, fields, and errors
- [ ] Task: Conductor - User Manual Verification 'Phase 1: Core Types & Foundational Package Setup' (Protocol in workflow.md)

## Phase 2: TDD Red Phase — Write Failing Tests

- [ ] Task: Create test file `pkg/okf/parser_test.go` with failing test suite
    - [ ] Write `TestParseDocument_ValidOKF`: assert `meta.Type == "Go Struct"` and body contains expected header
    - [ ] Write `TestParseDocument_MissingMandatoryFields`: assert `ErrMissingFields` when `type` field is absent
    - [ ] Write `TestParseDocument_NoFrontmatter`: assert `ErrNoFrontmatter` for plain markdown without `---` block
    - [ ] Write `TestParseDocument_EmptyBody`: assert valid parse with empty body slice when no body content exists
    - [ ] Write `TestParseDocument_FrontmatterOnly`: assert body is empty but metadata is valid for frontmatter-only docs
    - [ ] Write `TestExtractLinks_ValidPaths`: assert exactly 2 internal links, external HTTPS link excluded
    - [ ] Write `TestExtractLinks_EmptyBody`: assert empty `[]string` for a body with no markdown links
    - [ ] Write `TestExtractLinks_OnlyExternalLinks`: assert empty `[]string` when only `http://`/`https://` links are present
- [ ] Task: Run tests and confirm Red phase — all 8 tests must FAIL before implementation
    - [ ] Execute `go test -v ./pkg/okf/...`
    - [ ] Confirm compilation fails (types/functions not yet implemented) — this is the expected Red state
- [ ] Task: Conductor - User Manual Verification 'Phase 2: TDD Red Phase — Write Failing Tests' (Protocol in workflow.md)

## Phase 3: Green Phase — Implement Parser & Extractor

- [ ] Task: Implement `ParseDocument` in `pkg/okf/parser.go`
    - [ ] Declare `package okf` with imports: `bufio`, `bytes`, `errors`, `io`, `strings`, `gopkg.in/yaml.v3`
    - [ ] Implement the streaming `bufio.Scanner` line-by-line frontmatter detection logic
    - [ ] Handle the `---` open/close boundary detection on line 1
    - [ ] Accumulate frontmatter lines into a `bytes.Buffer` and body lines into a separate buffer
    - [ ] Call `yaml.Unmarshal` on the frontmatter buffer into `*Metadata`
    - [ ] Return `ErrNoFrontmatter` if no valid frontmatter boundary was found
    - [ ] Return `ErrMissingFields` if `meta.Type`, `meta.Title`, or `meta.Description` are empty strings
    - [ ] Add GoDoc comment to `ParseDocument`
- [ ] Task: Implement `ExtractLinks` in `pkg/okf/indexer.go`
    - [ ] Declare `package okf` with imports: `regexp`, `strings`
    - [ ] Declare `LinkRegex` as a package-level compiled `*regexp.Regexp` for `\[[^\]]+\]\(([^)]+)\)`
    - [ ] Implement `ExtractLinks(body []byte) []string` using `FindAllSubmatch`
    - [ ] Filter results: skip entries where the captured group starts with `http://` or `https://`
    - [ ] Return the collected internal link paths slice
    - [ ] Add GoDoc comments to `LinkRegex` and `ExtractLinks`
- [ ] Task: Run tests and confirm Green phase — all 8 tests must PASS
    - [ ] Execute `go test -v -race ./pkg/okf/...`
    - [ ] Confirm all 8 tests pass
- [ ] Task: Conductor - User Manual Verification 'Phase 3: Green Phase — Implement Parser & Extractor' (Protocol in workflow.md)

## Phase 4: Refactor & Quality Gate

- [ ] Task: Refactor for clarity and robustness (optional but recommended)
    - [ ] Review `ParseDocument` for any edge case missed (e.g., Windows-style `\r\n` line endings)
    - [ ] Review `ExtractLinks` regex for correctness against parentheses in link labels
    - [ ] Rerun tests post-refactor to ensure no regression
- [ ] Task: Enforce code quality checks
    - [ ] Run `go fmt ./pkg/okf/...` and commit any formatting changes
    - [ ] Run `go vet ./pkg/okf/...` and resolve any warnings
    - [ ] Run `go test -v -race -coverprofile=coverage.txt ./pkg/okf/...` and verify coverage ≥80% (target: 100%)
    - [ ] Confirm no existing tests in the repository are broken: `go test ./...`
- [ ] Task: Conductor - User Manual Verification 'Phase 4: Refactor & Quality Gate' (Protocol in workflow.md)
