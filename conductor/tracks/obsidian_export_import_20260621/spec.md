# Specification: Obsidian-Compatible Vault Export and Import

## Overview
This feature adds the capability to export an entire notebook (including its sources, notes, knowledge graph entities, and relationships) as a zipped Obsidian-compatible vault, and to import a previously exported zipped vault to create a new notebook or merge into an existing notebook.

## Functional Requirements

### 1. Zipped Export (Backend)
- Add a REST API endpoint `GET /api/notebooks/:id/export` that generates a `.zip` archive containing:
  - `sources/`: Folder containing all notebook sources converted/saved as Markdown files.
    - Inside sources, search the text for known entity names and wrap them in Obsidian double-brackets `[[entity_name]]`.
  - `notes/`: Folder containing all user-created notes as Markdown files.
  - `entities/`: Folder containing each `RAGEntity` as a Markdown file detailing its name, description, and list of backlinks to sources where it is mentioned.
  - `graph.json`: File containing the serialized database records of the RAG graph (entities and `co_occurs` relationships) for external visualizations.
- Stream the zip file to the client for download.

### 2. Zipped Import (Backend)
- Add a REST API endpoint `POST /api/notebooks/import` to create a new notebook from a zipped vault.
- Add a REST API endpoint `POST /api/notebooks/:id/import` to merge a zipped vault into an existing notebook.
- The endpoints will accept a `multipart/form-data` file upload containing the `.zip` archive.
- The import logic will:
  - Extract the contents of the zip file.
  - Parse the Markdown files in `sources/` and `notes/` to create source/note records.
  - Parse `graph.json` or extract entities and relationships to populate the RAG graph in SurrealDB.
  - Compute hashes for the imported sources.
  - **Conflict Resolution Strategy**:
    - If the source hash already exists in the notebook, keep the existing file (skip importing the duplicate file).
    - If a name conflict exists but the hashes don't match, resolve the conflict by adding a suffix (e.g., `File_1.md`).

### 3. User Interface (Frontend)
- Add "Export Vault" and "Import Vault" controls inside the Notebook Settings / Advanced panel.
- The Export action triggers the browser download of the zip file.
- The Import action opens a file picker to upload a zipped vault, displaying progress and status toasts.

## Acceptance Criteria
- Exporting a notebook produces a valid ZIP file that can be opened in Obsidian as a vault.
- Links in sources use the standard `[[entity_name]]` format and resolve correctly to pages in the `entities/` folder within Obsidian.
- Importing the exported ZIP file creates a functional replica of the notebook.
- Duplicate sources with the same content (hash match) are skipped; those with different content but conflicting names are imported with a suffix (e.g., `source_1.txt`).
