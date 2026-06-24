# Functional Specification: OKF Integration — Phase 4: Local AI Context/Enrichment Automation

## 1. Overview
This track specifies the implementation of Phase 4: Local AI Context/Enrichment Automation of the Open Knowledge Format (OKF) integration for `go-notebook`. It introduces:
1. **Enrichment REST Endpoint**: A backend `POST /api/okf/enrich` endpoint that accepts note content or paths, queries the configured default local or cloud LLM via the existing `internal/ai` manager, and generates suggested `description` and `tags` formatted as OKF YAML metadata.
2. **On-Demand "AI Suggest" Feature**: An interactive trigger inside the `PropertiesEditor` metadata sidebar. This button sends the note body/content to the enrichment endpoint, shows a premium active loading state, and instantly updates/autofills the Title, Description, and Tags fields.

## 2. Functional Requirements
### 2.1 Backend Enrichment Engine (`POST /api/okf/enrich`)
- **API Signature**:
  - Method: `POST`
  - URL: `/api/okf/enrich`
  - Payload Schema:
    ```json
    {
      "content": "string (optional raw note content)",
      "path": "string (optional absolute or relative note file path on disk)"
    }
    ```
- **Behavior**:
  - If a `path` is specified, read the note file from disk, parse its frontmatter and body, enrich the metadata using the default AI client, serialize the updated frontmatter, and save the updated file on disk without altering the Markdown body.
  - If only `content` is specified, parse it in-memory, enrich the metadata using the default AI client, and return the suggested metadata parameters.
- **LLM Integration**:
  - Resolve the default AI client using the existing `ai.GetClientForDefaultModel(ctx, "transformation")` or "chat" which natively supports Ollama, Gemini, and OpenAI.
  - Formulate a system prompt that guides the model to analyze the note body and produce a concise summary (up to 2 sentences) and 3-5 relevant lowercase tags.
  - Format the prompt to strictly output a valid JSON response block.
  - Handle JSON decoding with fallbacks to avoid schema validation failures.

### 2.2 Frontend "AI Suggest" Button
- **Placement**: Add a premium "AI Suggest" button inside the `PropertiesEditor` header next to the "Properties" title, styled with a Sparkles icon.
- **Loading State**: Show a beautiful loading spinner (with a subtle pulse animation) on the button and form inputs while the API request is pending.
- **Form Integration**:
  - When suggestions are returned, instantly update the form values for `title`, `description`, and `tags` in the `PropertiesEditor` state.
  - Seamlessly propagate these updates to the parent markdown editor hook form so they can be saved to the database on "Save Note".

## 3. Technical Design & Architecture
- **Backend Components**:
  - Register route handler `handleOKFEnrich` in `internal/api/router/okf.go`.
  - Reuse `pkg/okf/parser.go`'s `ParseDocument` for separating frontmatter and body.
  - Use `internal/ai` package for LLM generation.
- **Frontend Components**:
  - Update `frontend/src/components/okf/PropertiesEditor.tsx` to include an optional `noteBody` or `content` prop.
  - Add "AI Suggest" button and hook calling `/api/okf/enrich`.

## 4. Acceptance Criteria
- [ ] Backend `POST /api/okf/enrich` successfully parses note content and returns suggested description and tags in JSON.
- [ ] Backend endpoint supports file-based enrichment and saves back to disk when given a file path.
- [ ] Clicking the "AI Suggest" button in the form editor queries the enrichment endpoint.
- [ ] The button shows a clear, active loading spinner while the API query is running.
- [ ] On successful response, the Title, Description, and Tags form fields are instantly autofilled and synchronized with the editor state.

## 5. Out of Scope
- Fully automated background indexing of all notes with LLMs without user intent/interaction.
- Entity-relationship (link) extraction via LLMs.
