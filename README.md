# Go Open Notebook

This repository contains a high-performance Go port of the backend for **Open Notebook**, replacing the original Python backend (FastAPI, LangGraph, and Esperanto) with a structured, compiled, and highly concurrent Go application. The frontend remains the same Next.js-based interface.

The original project can be found at [LuisNovo/open-notebook](https://github.com/LuisNovo/open-notebook).

## Architecture & Subsystems

- **API Server** (`cmd/api`): Serves the REST HTTP endpoints on port `5055`. Implemented using Go 1.22's enhanced `http.ServeMux` router and standard `net/http` package.
- **Worker Daemon** (`cmd/worker`): Replaces `surreal-commands-worker`. It polls the `command` table in SurrealDB and processes background jobs (document text parsing, content extraction, RAG vector embedding generation, and podcast creation).
- **Database Layer**: Uses SurrealDB (v3.1.3+) with the official `surrealdb.go` driver. Migrations are managed and applied automatically on startup from standard SQL scripts (`.surrealql`).
- **AI Integrations**: Includes support for OpenAI, Anthropic, Gemini, and Ollama APIs.
- **Content Extractor**: Custom parser for PDFs, web URLs (HTML-to-markdown parsing), and speech-to-text audio transcriptions.

## Getting Started

### Prerequisites

- **Go 1.22+**
- **Node.js** (for Next.js frontend development)
- **SurrealDB v3.1.3**

### Running the Services

1. **Start SurrealDB**:
   ```bash
   mkdir -p surreal_data
   surreal start --log info --user root --pass root --bind 0.0.0.0:8000 rocksdb://surreal_data/db
   ```

2. **Run the Go API**:
   ```bash
   go run ./cmd/api
   ```
   *(Migrations are applied automatically on database connection)*

3. **Run the Go Worker Daemon**:
   ```bash
   go run ./cmd/worker
   ```

4. **Start the Frontend**:
   ```bash
   cd frontend
   npm run dev
   ```
   Open [http://localhost:3000](http://localhost:3000) in your browser.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
Original project copyright © 2024 Luis Novo.
