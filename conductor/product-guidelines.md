# Product Guidelines: Go Open Notebook

These guidelines serve as the design, code style, prose style, and interaction system reference for the Go Open Notebook project.

---

## 🎨 Visual Design & Theme Guidelines
*   **Minimalist Slate Aesthetic**: The interface must prioritize high-contrast dark palettes based on deep zinc/slate colors.
*   **Clean Layouts & Thin Borders**: Use fine borders (`border-border` or `border-zinc-800`), consistent padding, and well-aligned grids to create a premium structural look.
*   **Transitions & Micro-Animations**: Implement smooth hover states, fade-in/fade-out transitions, and micro-animations for buttons, cards, and modal dialogs to make the app feel responsive and premium.

## ✍️ Prose & Writing Style
*   **Clear & Action-Oriented**: All user-facing text, labels, and error messages must be direct, precise, and technical. Avoid verbose explanations.
*   **Graceful & Descriptive Error States**: When operations fail (such as database disconnects or background build timeouts), explain exactly what went wrong and provide actionable remedies.
*   **Concise Summaries**: Leverage bullet points, metadata badges, and inline status tags to display detailed info compactly.

## 🧩 User Experience (UX) & Interaction Principles
*   **Low-Latency Feedback**:
    *   Use optimistic UI updates where appropriate (e.g. updating notes or adding source tags immediately in the UI before network completion).
    *   Provide explicit visual indicators (like spinners, progress bars, or logs) for longer background tasks such as compiling PDF files or running GraphRAG build jobs.
*   **Spatial Canvas Focus**:
    *   Ensure the 2D physics-based network canvas operates smoothly, supporting panning, zooming, and clicking nodes.
    *   Map nodes to community colors dynamically and expose detailed metadata in sidebar panels to avoid cluttered tooltips.
*   **Local Control Indicators**:
    *   Expose visible database connectivity badges showing connection status to local SurrealDB.
    *   Provide clear indications that data processing and storage paths are fully local.

## 🛠️ Architecture & Technical Standards
*   **Layered Decoupling**:
    *   Maintain strict separation of responsibilities: HTTP api handlers (`internal/api`), domain business models (`internal/domain`), background command worker jobs (`internal/worker`), and data storage repositories (`internal/db`).
*   **Minimal External Packages**:
    *   Keep backend dependencies minimal. Rely on Go standard libraries (`net/http`, `go/ast`) and avoid large external frameworks. In particular, implement custom handcrafted command registries instead of using heavy libraries like Cobra/Pflag.
*   **Embedded Asset Delivery**:
    *   All compiled frontend output (`frontend/out`) and database migration scripts must be embedded directly into the Go binary (`embed.FS`). The final release artifact must remain a single, self-contained executable.
