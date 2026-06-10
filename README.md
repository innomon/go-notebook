# Go Open Notebook

This repository contains a high-performance Go port of the backend for **Open Notebook**, replacing the original Python backend (FastAPI, LangGraph, and Esperanto) with a structured, compiled, and highly concurrent Go application. The frontend remains the same Next.js-based interface.

The original project can be found at [LuisNovo/open-notebook](https://github.com/LuisNovo/open-notebook).

## Architecture & Subsystems

- **Unified Server** (`cmd/server`): Serves the Web Frontend, REST API, and Background Worker Daemon concurrently inside a single compiled process.
- **Database Layer**: Uses SurrealDB (v3.1.3+) with the official `surrealdb.go` driver. Migrations are managed and applied automatically on startup from standard SQL scripts (`.surrealql`).
- **AI Integrations**: Includes support for OpenAI, Anthropic, Gemini, and Ollama APIs.
- **Content Extractor**: Custom parser for PDFs, web URLs (HTML-to-markdown parsing), and speech-to-text audio transcriptions.

## Getting Started

### Prerequisites

- **Go 1.22+**
- **SurrealDB v3.1.3**

### Running the Services

1. **Start SurrealDB**:
   ```bash
   mkdir -p surreal_data
   surreal start --log info --user root --pass root --bind 0.0.0.0:8000 rocksdb://surreal_data/db
   ```

2. **Run the Unified Go Server**:
   ```bash
   go run ./cmd/server
   ```
   *(Frontend static assets are embedded, database migrations are applied automatically, and background worker starts concurrently)*

   Open [http://localhost:5055](http://localhost:5055) in your browser.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
Original project copyright © 2024 Luis Novo.
