# Initial Concept
A high-performance Go port of the backend for Open Notebook, replacing the original Python backend with a structured, compiled, and highly concurrent Go application while maintaining the Next.js frontend.

# Product Guide: Go Open Notebook

## Vision & Core Mission
Go Open Notebook is a high-performance, local-first personal knowledge management (PKM) and research assistant. It aims to empower **General Knowledge Workers** to seamlessly organize personal notes, web documents, audio, and files. By combining vector database search with structured knowledge graphs (GraphRAG), users can explore their records semantically and interactively, turning raw text and media into connected, easily queryable knowledge.

## Target Audience
*   **General Knowledge Workers**: Professionals, writers, researchers, and domain experts who need to maintain, synthesize, and query deep repositories of personal notes, articles, books, and web content.
*   **Developers & Tech Enthusiasts**: Users who prefer self-hosted, local-first applications that do not lock their private data in proprietary cloud silos.

## Priority Product Capabilities

### 1. Enhanced GraphRAG & Visualizer
*   **Graph Ingest & Extraction**: Deep text chunking and LLM-driven entity/relationship extraction.
*   **Modularity & Community Detection**: High-quality topological clustering and automated summarization of thematic areas.
*   **Hybrid Retrieval Canvas**: A force-directed 2D interactive canvas allowing users to browse their knowledge base visually while executing Local, Global, or Hybrid retrieval search queries.

### 2. Multimodal Ingestion Pipeline
*   **Document Parsers**: Support for PDFs, HTML URLs, and Markdown notes, expanding to `.docx` and `.xlsx` files.
*   **Audio/Video Transcription**: Native background transcription processing for voice notes, lectures, and videos.
*   **Visual Assets & OCR**: Vision-enabled processing for image notes, whiteboard captures, and screenshots.

### 3. High Performance & Scalability
*   **Local Concurrency**: Compiled, multi-threaded Go application executing APIs and worker tasks concurrently in a single binary.
*   **Database Engine**: SurrealDB rocksdb storage engine for robust local storage.
*   **Smart Caching**: Content hashing to allow incremental graph builds, avoiding duplicate processing.

## Storage & Deployment Architecture
*   **Local-First Database**: Runs locally using SurrealDB. All user files, indexes, and database tables reside on the user's local machine, providing absolute data privacy, offline-friendly access, and fast operations.
*   **Zero-Dependency Binary**: Multi-stage build process compiling Next.js assets directly into the Go executable, creating a single distribution artifact.

## Key Integrations
*   **Developer Assistants & MCP**: Exposing Model Context Protocol (MCP) SSE endpoints so external AI assistants (like Claude, Cursor, and Gemini CLI) can query the notebook's knowledge graph.
*   **Multimodal API Providers**: Connects to OpenAI, Anthropic, Gemini, and local-first self-hosted model providers (Ollama).
*   **Diverse Data Ingestion**: Direct link imports from external web URLs, arXiv papers, and file system notes.
