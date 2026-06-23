# Functional Specification: OKF Integration — Phase 3: Client Dashboard & Visual Graph Components

## 1. Overview
This track specifies the implementation of the frontend user interface for the Open Knowledge Format (OKF) integration. It introduces:
1. **Interactive Visual Graph Canvas**: A 2D force-directed network graph visualization of the workspace's OKF files.
2. **Dedicated Workspace Graph View**: A full-screen route containing the workspace graph and detailed node explorer side panel.
3. **Editor Frontmatter Panel**: An inline property editor widget inside the Markdown file editor showing real-time OKF compliance status (valid/invalid), support for form-based field modifications (Title, Type, Description, Tags), and a toggleable raw YAML source block editor.
4. **Mermaid & JSON/CSV Export**: Features to copy the graph structure in Mermaid diagram format or export the data as a JSON/CSV payload.

## 2. Functional Requirements
### 2.1 Visual Graph Canvas Component
- **Graph Engine**: Use `d3-force` algorithm directly with React-controlled SVG/Canvas rendering.
- **Node Styling**: Style nodes based on their OKF `type` metadata. Use distinct zinc-based dark theme colors matching Go Open Notebook design guidelines.
- **Interactivity**:
  - Support mouse zoom/pan controls on the canvas.
  - Hovering a node displays a quick tooltip with node title/type.
  - Clicking a node highlights it, dimming unconnected nodes, and updates the active selection in the explorer side panel.
  - Dragging a node updates its force physics position.
- **Adjacency Rendering**: Render directed edge links with small arrowhead indicators representing reference paths between files.

### 2.2 Dedicated Workspace Graph View
- **Route**: Introduce a new route `/okf/graph` (or a toggle tab in the main sidebar) that fetches graph nodes via the backend `GET /api/okf/graph?path=...` endpoint.
- **Side Panel**: Add a collapsible drawer panel on the right side of the screen displaying:
  - Selected node's details: Title, Type, Description, Tags, and relative file path.
  - Outbound links: A list of clickable node paths that navigate to those nodes.
  - Inbound links: A list of other workspace files referencing this node.
- **Filters/Search**: Allow filtering nodes by Type or Tags, and searching nodes by Title/Path.

### 2.3 Editor Frontmatter Panel & Properties Editor
- **UI Integration**: Embed a collapsible "Properties" panel at the top of the markdown editor page.
- **Inline Status Indicator**: Display a status badge (e.g. Zinc badge "Valid OKF" in green/gray, or "Invalid OKF" in amber/red) showing compliance status. If invalid, display a list of validation errors.
- **Toggleable Dual-Mode Editor**:
  - **Form Mode**: Simple text inputs for `Title`, `Description`, and selectors for `Type` (e.g., Concept, Feature, Task) and `Tags` array. Edits are auto-saved back into the Markdown document's YAML frontmatter.
  - **YAML Mode**: A raw text area block with syntax verification checks.
- **File Update Synchronization**: Modifying frontmatter properties automatically updates the underlying markdown file on disk by calling the backend API, triggering fsnotify file events to update the SurrealDB graph.

### 2.4 Mermaid & Data Export
- **Export Actions**: Add an action toolbar in the dedicated graph view with:
  - **Export Mermaid**: Generate and copy standard Mermaid `graph TD` syntax representing the visual workspace graph.
  - **Export JSON**: Download a JSON serialization of the active node graph adjacency matrix.
  - **Export CSV**: Download a flat CSV file of the nodes metadata registry.

## 3. Technical Design & Architecture
- **Tech Stack**: Next.js (React), D3.js (`d3-force`), TailwindCSS, standard markdown editor component hooks.
- **Backend Endpoints**:
  - `GET /api/okf/graph?path=...` (fetches network nodes)
  - `POST /api/okf/validate` (checks raw text validation state)

## 4. Acceptance Criteria
- [ ] Interactive 2D graph successfully renders node networks with directed edge lines.
- [ ] Clicking a node loads details in the side panel.
- [ ] Changing a property inside the editor frontmatter panel updates the disk file.
- [ ] Validation errors are cleanly displayed if frontmatter fields are invalid.
- [ ] Graph data can be copied as Mermaid syntax or downloaded as JSON/CSV.

## 5. Out of Scope
- Dynamic 3D network visualizations.
- Automated AI metadata generation (deferred to Phase 4: Agentic Enrichment).
