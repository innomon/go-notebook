# Technology Stack: Go Open Notebook

This document specifies the official technology stack and dependencies for the Go Open Notebook project.

---

## 💻 Backend Architecture
*   **Language**: Go 1.25.x
*   **HTTP Server & Routing**: Go standard library `net/http` router (native routing support introduced in Go 1.22).
*   **API & Middleware Layer**:
    *   Custom CORS management.
    *   Password authentication middleware.
    *   Model Context Protocol (MCP) streamable HTTP handler endpoint.
*   **Database Engine (Configurable via `DB_ENGINE`)**:
    *   **SQLite (Default)**: Pure Go, CGO-free via `modernc.org/sqlite`. Automatic embedded SQL migrations (`embed.FS`).
    *   **SurrealDB**: Optional multi-model database engine via `github.com/surrealdb/surrealdb.go`.

## 🎨 Frontend Architecture
*   **Framework**: Next.js 15 (TypeScript, React)
*   **Design System & UI**: Tailwind CSS + Shadcn UI (Radix UI primitives under the hood)
*   **State Management & Requests**: Axios for REST API interactions, dynamic force-directed rendering for GraphRAG studio.
*   **Asset Bundling**: Configured to export static files (`next export` / `npm run build`), which are embedded inside the Go executable using `embed.FS`.

## 🤖 AI & NLP Subsystems
*   **LLM Providers**:
    *   Commercial: Anthropic, OpenAI, Gemini
    *   Self-Hosted / Local-first: Ollama
*   **GraphRAG Subsystem**:
    *   Text paragraph parsing and token chunking (implemented in Go).
    *   Semantic entity and relation extraction via LLM JSON prompts.
    *   Label Propagation clustering (LPA) implemented in Go.
    *   Vector Embeddings generation and search queries stored in SurrealDB.
*   **Extraction Engine**:
    *   PDF parsing.
    *   Web page content extraction (HTML to clean Markdown).
    *   Audio transcription.

## 📡 External Connectivity
*   **Model Context Protocol (MCP)**: Exposes tool calls and graph querying via SSE (Server-Sent Events) HTTP transport using `github.com/modelcontextprotocol/go-sdk/mcp`.
