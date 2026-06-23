# Track Spec: OKF Integration — Phase 1: Core Foundation & Metadata Parser

## Overview

This track implements **Phase 1** of the Open Knowledge Format (OKF) integration into `go-notebook`. The goal is to establish the complete foundational layer: the Go type definitions, the YAML frontmatter parser, and the link extractor. This is the prerequisite for all subsequent OKF phases (indexer, UI, AI enrichment).

The implementation is fully backend-only and introduces a new, isolated `pkg/okf` package. All code must be idiomatic Go with no new external dependencies beyond `gopkg.in/yaml.v3` (already available in the project via the existing go.mod).

## Source Reference

- Based on: `docs/OKF_4_NOTEBOOK.md` (SPECS-OKF-001)

## Functional Requirements

### FR-1: OKF Type Definitions (`pkg/okf/types.go`)
- Define the `Metadata` struct with all OKF-standard fields:
  - `Type` (required, `string`)
  - `Title` (required, `string`)
  - `Description` (required, `string`)
  - `Resource` (optional, `string`)
  - `Tags` (optional, `[]string`)
  - `Timestamp` (optional, `time.Time`)
  - `Properties` (optional, `map[string]interface{}`)
- Define the `BundleNode` struct with fields:
  - `ID` (`string`) — unique node identifier (relative filepath or UUID)
  - `Metadata` (`Metadata`) — parsed OKF metadata
  - `OutboundLinks` (`[]string`) — extracted relative link paths
- Export sentinel error variables:
  - `ErrNoFrontmatter` — for documents lacking valid OKF YAML blocks
  - `ErrMissingFields` — for documents missing required `type`, `title`, or `description` fields

### FR-2: Document Parser (`pkg/okf/parser.go`)
- Implement `ParseDocument(r io.Reader) (*Metadata, []byte, error)` that:
  - Reads the file stream line-by-line using `bufio.Scanner` (streaming, not full read-into-memory)
  - Detects the `---` frontmatter opening boundary on line 1
  - Strips the frontmatter block and passes it to `gopkg.in/yaml.v3` for unmarshalling into `Metadata`
  - Returns the clean markdown body as a `[]byte` (excluding the frontmatter block)
  - Returns `ErrNoFrontmatter` if no valid YAML front-matter boundary is found
  - Returns `ErrMissingFields` if `type`, `title`, or `description` are absent after parsing
  - Does **not** alter the original file content on disk

### FR-3: Link Extractor (`pkg/okf/indexer.go`)
- Implement `ExtractLinks(body []byte) []string` that:
  - Uses a compiled `regexp.Regexp` to find all standard Markdown links: `[Label](./path.md)`
  - Returns **only** internal (non-HTTP/HTTPS) relative link paths
  - Filters out external URLs (`http://`, `https://` prefixes)
  - Returns links in the order they appear in the document body

### FR-4: Test Suite (`pkg/okf/parser_test.go`)
- Use TDD: tests written **before** implementation (Red-Green-Refactor cycle)
- Baseline test cases from `OKF_4_NOTEBOOK.md` §6, expanded with additional edge cases:
  - `TestParseDocument_ValidOKF`: Parse a fully valid document; assert all fields and body content
  - `TestParseDocument_MissingMandatoryFields`: Assert `ErrMissingFields` when `type` is absent
  - `TestParseDocument_NoFrontmatter`: Assert `ErrNoFrontmatter` for plain markdown without YAML block
  - `TestParseDocument_EmptyBody`: Parse a valid frontmatter with an empty body
  - `TestParseDocument_FrontmatterOnly`: Parse a doc where body content is missing after closing `---`
  - `TestExtractLinks_ValidPaths`: Assert exactly 2 internal links are extracted, external URLs excluded
  - `TestExtractLinks_EmptyBody`: Assert empty slice returned for a body with no links
  - `TestExtractLinks_OnlyExternalLinks`: Assert empty slice when only HTTP links are present

## Non-Functional Requirements

- **Zero new external dependencies** beyond `gopkg.in/yaml.v3` (already in `go.mod`)
- **>80% code coverage** on the `pkg/okf` package via `go test -coverprofile`
- **No changes to existing packages** — this is a strictly additive implementation
- All public symbols must have GoDoc comments following the project's `godoc-canonical` standard
- Pass `go vet ./pkg/okf/...` and `go fmt ./pkg/okf/...` with zero warnings

## Acceptance Criteria

1. `go test -v -race -coverprofile=coverage.txt ./pkg/okf/...` passes with 100% path coverage on the parser and extractor
2. `go vet ./pkg/okf/...` outputs no errors
3. All 8 test cases pass (Green phase confirmed)
4. No existing tests in the repository are broken
5. All exported symbols in `pkg/okf/` have GoDoc documentation

## Out of Scope (for this track)

- Background filesystem watcher (FSNotify) — Phase 2
- Workspace graph indexer and in-memory graph construction — Phase 2
- API endpoints (`/api/okf/graph`, `/api/okf/validate`) — Phase 2
- Frontend UI (frontmatter editor panel, graph visualization) — Phase 3
- AI/Agentic enrichment auto-generation — Phase 4
