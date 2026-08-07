# Architecture & System Overview: Go Open Notebook

This document provides a comprehensive technical blueprint of **Go Open Notebook**, detailing system architecture, data abstraction layers, multimodal ingestion pipelines, GraphRAG indexing, WebAssembly plugin extensions, and deployment packaging.

---

## 1. System Vision & High-Level Architecture

Go Open Notebook is a high-performance, local-first personal knowledge management (PKM) and research assistant. It compiles the backend REST API, background worker daemon, and static Next.js frontend into a single zero-dependency executable.

```mermaid
flowchart TB
    subgraph Client ["Client Layer"]
        WebUI["Next.js Web Interface<br>(Radix UI + Tailwind CSS)"]
        GraphUI["GraphRAG Visualizer Canvas<br>(2D Force Graph)"]
        MCPAgents["External MCP Clients<br>(Claude, Cursor, Gemini CLI)"]
    end

    subgraph Server ["Go Application Server (Single Binary)"]
        Router["HTTP Mux Router<br>(net/http)"]
        Middleware["Auth & CORS Middleware"]
        SSEHandler["MCP SSE Transport<br>(/api/mcp)"]
        FrontendFS["Embedded Frontend Assets<br>(embed.FS 'out')"]

        subgraph Core ["Core Subsystems"]
            IngestPipe["Multimodal Ingestion Pipeline<br>(PDF, HTML, Audio, Code)"]
            GraphEngine["GraphRAG Subsystem<br>(Chunking, Extraction, LPA, Summaries)"]
            BasesEngine["Obsidian Bases Engine<br>(YAML Properties + WASM Formulas)"]
            WorkerDaemon["Background Worker Daemon"]
        end

        subgraph DAL ["Database Abstraction Layer (DAL)"]
            DBConfig["Config Manager<br>(DB_ENGINE=sqlite|surrealdb)"]
            DBFactory["Repository Factory"]
            
            subgraph Drivers ["Storage Engines"]
                SQLiteEngine["SQLite Engine<br>(modernc.org/sqlite, CGO-Free)"]
                SurrealEngine["SurrealDB Engine<br>(surrealdb.go WebSocket/RPC)"]
            end
        end
    end

    WebUI --> Router
    GraphUI --> Router
    MCPAgents --> SSEHandler
    Router --> Middleware
    Middleware --> Core
    Router --> FrontendFS
    Core --> DBFactory
    DBConfig --> DBFactory
    DBFactory --> SQLiteEngine
    DBFactory --> SurrealEngine
```

---

## 2. Database Abstraction Layer (DAL) & Storage Engines

Go Open Notebook supports a decoupled, dual-backend database architecture configured via the `DB_ENGINE` environment variable (default: `sqlite`).

```mermaid
classDiagram
    class RepositoryFactory {
        <<interface>>
        +Notes() NoteRepository
        +Documents() DocumentRepository
        +Vectors() VectorRepository
        +Graph() GraphRepository
        +Settings() SettingsRepository
        +Close(ctx) error
    }

    class NoteRepository {
        <<interface>>
        +Get(ctx, id) *NoteRecord
        +List(ctx) []NoteRecord
        +Create(ctx, note) *NoteRecord
        +Update(ctx, id, note) *NoteRecord
        +Delete(ctx, id) error
    }

    class VectorRepository {
        <<interface>>
        +Save(ctx, vec) error
        +Search(ctx, queryVector, limit) []VectorRecord
        +DeleteBySource(ctx, sourceID) error
    }

    class SQLiteFactory {
        -db *sql.DB
        +Notes() NoteRepository
        +Documents() DocumentRepository
        +Vectors() VectorRepository
        +Graph() GraphRepository
        +Settings() SettingsRepository
    }

    class SurrealFactory {
        +Notes() NoteRepository
        +Documents() DocumentRepository
        +Vectors() VectorRepository
        +Graph() GraphRepository
        +Settings() SettingsRepository
    }

    RepositoryFactory <|.. SQLiteFactory
    RepositoryFactory <|.. SurrealFactory
    SQLiteFactory --> NoteRepository
    SQLiteFactory --> VectorRepository
```

### Storage Engine Implementations

1. **SQLite (`modernc.org/sqlite`) — Default**:
   - **Characteristics**: Pure Go, zero-CGO dependency, embedded file-based (`notebook.db`).
   - **Migrations**: Executed automatically on startup via `embed.FS` (`schema/*.sql`).
   - **Vector Embeddings**: Vectors stored as JSON strings in SQLite tables; similarity search executed via in-memory/SQL cosine similarity functions.

2. **SurrealDB (`surrealdb.go`) — Multi-Model**:
   - **Characteristics**: Client-server mode connecting via WebSockets/RPC (`ws://localhost:8000/rpc`).
   - **Graph Native**: Executes SurrealQL graph traversals (`RELATE source -> reference -> notebook`).

---

## 3. Multimodal Ingestion Pipeline

The ingestion pipeline handles heterogeneous data sources (PDFs, URLs, Audio notes, images, and Obsidian vaults), transforming raw assets into cleaned Markdown content and tokenized chunks.

```mermaid
flowchart LR
    Input[Raw Input Asset] --> TypeCheck{Source Type?}

    TypeCheck -->|PDF| PDFExtract["PDF Text Parser<br>(ledongthuc/pdf)"]
    TypeCheck -->|HTML URL| HTMLExtract["HTML Parser & Markdown Formatter"]
    TypeCheck -->|Audio/Voice| Whisper["Speech-to-Text Transcriber"]
    TypeCheck -->|Image / Screenshot| OCR["Tesseract OCR / Vision LLM Fallback"]
    TypeCheck -->|Docx / Xlsx| OfficeExtract["Office XML Parser"]

    PDFExtract --> CleanMD[Clean Markdown Output]
    HTMLExtract --> CleanMD
    Whisper --> CleanMD
    OCR --> CleanMD
    OfficeExtract --> CleanMD

    CleanMD --> Storage["Store Document & Chunks in DAL"]
```

---

## 4. GraphRAG Subsystem (Knowledge Graph & Vector Search)

Go Open Notebook integrates a GraphRAG knowledge graph extraction, clustering, and retrieval engine.

```mermaid
flowchart TD
    Doc[Document Source] --> Chunking["Paragraph Token Chunking"]
    Chunking --> Embedding["Vector Embedding Generation"]
    Chunking --> Extraction["LLM Named Entity & Relation Extraction"]

    Embedding --> VectorDB["Vector Index (DAL)"]
    Extraction --> GraphDB["Knowledge Graph Nodes & Edges (DAL)"]

    GraphDB --> LPA["Label Propagation Clustering (LPA in Go)"]
    LPA --> Community["Community Detection & Thematic Summaries"]

    subgraph Retrieval ["Hybrid Retrieval Engine"]
        Query([User Query]) --> LocalSearch["Local Mode: Vector Similarity + Graph Neighbors"]
        Query --> GlobalSearch["Global Mode: Vector Search on Community Summaries"]
        Query --> HybridSearch["Hybrid Mode: RRF Reranking & Context Synthesis"]
    end

    VectorDB --> LocalSearch
    Community --> GlobalSearch
    LocalSearch --> HybridSearch
    GlobalSearch --> HybridSearch
    HybridSearch --> LLMResponse[LLM Synthesis Output]
```

---

## 5. Obsidian Bases & WASM Extension System

To provide dynamic database-like views over Markdown frontmatter properties without native code execution security risks, Go Open Notebook integrates a WebAssembly (WASM) plugin extension system.

```mermaid
sequenceDiagram
    participant Host as Go Host (pkg/bases)
    participant Wazero as Wazero WASM Runtime (pkg/wasm)
    participant Guest as WASM Plugin (extensions/guest_sdk)

    Host->>Wazero: Instantiate Compiled Plugin Module
    Host->>Wazero: Allocate Guest Memory (malloc)
    Wazero-->>Host: Memory Offset Pointer
    Host->>Wazero: Write JSON Payload (Note Properties & Args)
    Host->>Wazero: Invoke Exported `execute` Function
    Wazero->>Guest: Execute Formula / Filter Logic
    Guest-->>Wazero: Return Result Pointer & Size
    Wazero-->>Host: Read JSON Result from Memory
    Host->>Wazero: Free Guest Memory
    Host->>Host: Render Evaluated Column / Filter View
```

---

## 6. External Connectivity & Model Context Protocol (MCP)

External AI coding assistants (such as Claude Code, Cursor, or Gemini CLI) can connect directly to Go Open Notebook using the **Model Context Protocol (MCP)** endpoint:

- **SSE Transport Endpoint**: `GET /api/mcp/sse`
- **JSON-RPC Message Endpoint**: `POST /api/mcp/message`
- **Exposed Tools**:
  - `search_graph`: Executes hybrid vector/graph queries against a notebook.
  - `get_entity_connections`: Returns 1st-degree topological neighbor nodes for an entity.
  - `get_community_summary`: Returns pre-computed thematic community outlines.

---

## 7. Embedded Web Frontend & Production Build

The Next.js 15 frontend application is compiled as a static single-page application (`npm run build`, `output: 'export'`) into `frontend/out`.

```mermaid
flowchart LR
    NextSrc[Next.js Source] -->|npm run build| NextOut[frontend/out]
    NextOut -->|go:embed all:out| EmbeddedFS[embed.FS Assets]
    EmbeddedFS -->|fs.Sub out| ServerMux[Go http.ServeMux Router]
    ServerMux -->|Catch-all SPA Handler| ClientBrowser[Web Browser]
```

To compile the self-contained production binary:
```bash
cd frontend && npm run build && cd ..
go build -o open-notebook ./cmd/server
```

---

## 8. Directory & Package Structure

```
go-notebook/
├── cmd/
│   ├── engine/          # CLI tool for Obsidian Bases WASM engine
│   └── server/          # Main unified application entrypoint
├── conductor/           # Conductor spec-driven development tracks & context
├── docs/                # Architecture & design documentation
├── extensions/          # WASM guest SDK and example plugin source files
├── frontend/            # Next.js 15 frontend application
│   ├── frontend.go      # embed.FS wrapper declaration for 'out'
│   └── out/             # Compiled static web assets
├── internal/
│   ├── ai/              # Provider clients (OpenAI, Anthropic, Gemini, Ollama)
│   ├── api/             # HTTP router, middleware, and MCP endpoints
│   ├── db/              # Database configuration and storage implementations
│   │   ├── factory/     # Dynamic repository factory loader
│   │   ├── repository/  # Domain repository interfaces
│   │   ├── sqlite/      # SQLite modernc driver & embedded migrations
│   │   └── surrealdb/   # SurrealDB driver adapter
│   ├── domain/          # Core domain models (Notebook, Source, Note, RAGGraph)
│   ├── extractor/       # Document & audio parsing pipeline
│   ├── graphrag/        # Chunking, entity extraction, LPA, and retrieval
│   └── worker/          # Background task execution daemon
├── pkg/
│   ├── bases/           # Obsidian Bases engine & view evaluator
│   ├── okf/             # Open Knowledge Format parser & graph indexer
│   └── wasm/            # Wazero WebAssembly host runtime manager
├── .env                 # Environment configuration template
├── go.mod               # Go module definition
└── README.md            # Project overview & getting started guide
```
