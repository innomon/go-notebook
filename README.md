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
- **Node.js 18+ & npm** (for building/running the frontend)
- **SurrealDB v3.1.3+**

### Step-by-Step Setup

#### 1. Build the Frontend
Because the Go backend embeds the compiled Next.js frontend pages directly into the final executable, you must build the frontend at least once before compiling or running the Go server.

```bash
# Navigate to the frontend directory
cd frontend

# Install Node.js dependencies
npm install

# Build and export static assets (outputs to frontend/out)
npm run build

# Return to the project root
cd ..
```

#### 2. Start SurrealDB
Start the SurrealDB instance to store database records:
```bash
# Create directory for SurrealDB data storage
mkdir -p surreal_data

# Start SurrealDB
surreal start --log info --user root --pass root --bind 0.0.0.0:8000 rocksdb://surreal_data/db
```

#### 3. Configure Environment Variables
Copy the `.env` template or ensure `.env` exists in the root directory. At a minimum, set `OPEN_NOTEBOOK_ENCRYPTION_KEY` to a secure random string (used for securing API keys):

```env
OPEN_NOTEBOOK_ENCRYPTION_KEY=some-secure-random-string
SURREAL_URL=ws://localhost:8000/rpc
SURREAL_USER=root
SURREAL_PASSWORD=root
SURREAL_NAMESPACE=open_notebook
SURREAL_DATABASE=open_notebook
DATA_FOLDER=notebook_data
UPLOADS_FOLDER=uploads
PORT=5055
```

#### 4. Run the Go Server
Run the unified server (which embeds the frontend, runs database migrations automatically, and starts the background worker daemon):

```bash
go run ./cmd/server
```
Once started, open [http://localhost:5055](http://localhost:5055) in your web browser.

---

### Development Mode (Hot-Reloading Frontend)

If you are developing the frontend, rebuilding the static files and restarting the Go server on every change is slow. Instead, you can run the frontend and backend concurrently:

1. **Start the Go Server (Backend Only)**:
   Make sure you have built the frontend at least once (so `frontend/out` exists and Go can compile/run), then run the Go server:
   ```bash
   go run ./cmd/server
   ```
   This starts the Go server on port `5055` (REST APIs, migrations, and worker).

2. **Start Next.js Development Server (Frontend)**:
   In a separate terminal, start the Next.js development server:
   ```bash
   cd frontend
   npm run dev
   ```
   Open [http://localhost:3000](http://localhost:3000) in your browser. Next.js is configured via `next.config.ts` to automatically proxy all requests matching `/api/*` to the Go backend on `http://localhost:5055`.

### Production Build

To compile a self-contained, standalone production binary:

1. Build/export the frontend:
   ```bash
   cd frontend && npm run build && cd ..
   ```
2. Build the Go binary:
   ```bash
   go build -o open-notebook ./cmd/server
   ```
   This produces a single, zero-dependency executable (`open-notebook`) containing both the backend code and all static web assets embedded inside.



## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
Original project copyright © 2024 Luis Novo.
