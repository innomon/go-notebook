# Implementation Plan: OKF Integration — Phase 3: Client Dashboard & Visual Graph Components

## Phase 1: Client Frontmatter Editor Component [checkpoint: ]

- [x] Task: Create tests for the Properties Editor Component (TDD Red Phase) c2ce1ca
    - [x] Create `frontend/src/test/okf/PropertiesEditor.test.tsx` (or matching test folder path)
    - [x] Add tests verifying Form Mode editing, YAML Mode toggle, validation error rendering, and save state propagation
    - [x] Verify test suite fails as expected
- [x] Task: Implement OKF Properties Editor UI (Green Phase) 916d858
    - [x] Create `PropertiesEditor` component supporting Form Fields (Title, Type, Description, Tags)
    - [x] Create toggle view to Raw YAML Code Editor (with Monaco or plain textarea)
    - [x] Handle input validation schema (using Zod or standard state checks) and display inline error list
    - [x] Integrate auto-save debounced callbacks to commit changes back to note files
    - [x] Run tests and verify they pass
- [ ] Task: Conductor - User Manual Verification 'Phase 1: Client Frontmatter Editor Component' (Protocol in workflow.md)

## Phase 2: D3-Based Interactive Visual Graph Component [checkpoint: ]

- [ ] Task: Create tests for Visual Graph Component (TDD Red Phase)
    - [ ] Create component tests verifying rendering of visual nodes, arrow-directed links, tooltips, and click handlers
    - [ ] Confirm test suite failure state
- [ ] Task: Implement Interactive Graph Canvas (Green Phase)
    - [ ] Create `VisualGraph` canvas component using `react-force-graph-2d` (or custom d3-force SVG layout)
    - [ ] Style nodes based on OKF `type` metadata tags using Zinc aesthetic colors
    - [ ] Implement mouse zoom/pan controls, dragging behavior, hover tooltips
    - [ ] Implement node selection highlighting (dims non-adjacent nodes) and trigger callback
    - [ ] Run tests and verify they pass
- [ ] Task: Conductor - User Manual Verification 'Phase 2: D3-Based Interactive Visual Graph Component' (Protocol in workflow.md)

## Phase 3: Dedicated Workspace Dashboard & Explorer Side Panel [checkpoint: ]

- [ ] Task: Create tests for Route and Sidebar Navigation (TDD Red Phase)
    - [ ] Write route-mount tests for `/okf/graph` fetching nodes from `/api/okf/graph`
    - [ ] Write tests for explorer side panel display, search filtering, and link list clicks
    - [ ] Confirm test suite failure state
- [ ] Task: Implement Workspace Route & Side Panel Explorer (Green Phase)
    - [ ] Create `/okf/graph` page route linking to visual graph visualizer
    - [ ] Implement right side collapsible metadata explorer panel listing Title, Description, Type, and Tag badges
    - [ ] Populate clickable lists of Inbound and Outbound references for the selected node to trigger navigation
    - [ ] Implement search bar and type-tag filtering checkboxes on the sidebar
    - [ ] Run tests and verify they pass
- [ ] Task: Conductor - User Manual Verification 'Phase 3: Dedicated Workspace Dashboard & Explorer Side Panel' (Protocol in workflow.md)

## Phase 4: Mermaid and Data Export Toolbar [checkpoint: ]

- [ ] Task: Create tests for Export utility functions (TDD Red Phase)
    - [ ] Write unit tests for Mermaid syntax formatter function
    - [ ] Write tests for CSV/JSON serialization generators and download triggers
    - [ ] Confirm test suite failure state
- [ ] Task: Implement Action Export Toolbar (Green Phase)
    - [ ] Create action toolbar controls on graph canvas view
    - [ ] Implement Export to Mermaid button (converts nodes list to `graph TD` flow representation and copies to clipboard)
    - [ ] Implement Export to JSON button (downloads serialized nodes/links dictionary file)
    - [ ] Implement Export to CSV button (downloads tabular representation of node registry metadata)
    - [ ] Run tests and verify they pass
- [ ] Task: Conductor - User Manual Verification 'Phase 4: Mermaid and Data Export Toolbar' (Protocol in workflow.md)
