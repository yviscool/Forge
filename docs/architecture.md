# Forge System Architecture & Layering Blueprint

## 1. High-Level System Architecture

```mermaid
flowchart TB
    subgraph TeacherSide [Teacher Deployment & Interaction]
        DesktopApp[Forge Desktop Runner (System Tray + LAN IP/QR Display)]
        TeacherWeb[Teacher Management Console (Vue3 / Tailwind / Monaco Editor)]
    end

    subgraph StudentSide [Student Access]
        BrowserClient[Student Web Client (Problem View / Code Submit / Live Board)]
    end

    subgraph GoServer [Forge Go Core (Single Portable Executable)]
        HTTPRouter[Embedded HTTP & REST Server (net/http)]
        SSERouter[SSE Real-time Event Broadcaster (/api/events)]
        AuthCore[Identity & Role System (Admin / Teacher / Student)]
        GroupCore[Global Grouping & Membership Manager]
        ContestEngine[Multi-Contest Scheduling & Lifecycle Engine]
        ProblemEngine[Problem Schema Validation & Multilingual Store]
        JudgingQueue[Safe Evaluation Queue & Result Recorder]
        PDFPipeline[Pure Go CDP / Edge Headless PDF Engine]
        EmbeddedFS[Static Asset Embed (//go:embed internal/arena/web)]
    end

    DesktopApp --> HTTPRouter
    TeacherWeb --> HTTPRouter
    BrowserClient --> HTTPRouter
    HTTPRouter --> AuthCore & GroupCore & ContestEngine & ProblemEngine & JudgingQueue & SSERouter & PDFPipeline
    HTTPRouter --> EmbeddedFS
```

---

## 2. Layered Responsibilities & Directory Mapping

| Layer | Directory | Responsibilities | Dependencies |
|---|---|---|---|
| **Entrypoint** | `cmd/forge/` & `cmd/arena/` | Process initialization, environment flags, graceful shutdown, listener binding | `internal/arena` |
| **Domain Services** | `internal/arena/service.go` | Business logic, state consistency, user/group mapping, contest concurrency, deterministic scoring | Standard Library |
| **Transport / API** | `internal/arena/http.go` | HTTP REST endpoints, JSON codec, SSE subscription, static asset handler | `internal/arena` domain, `embed` |
| **Document Export** | `internal/arena/http.go` / `docs/pdf.md` | CCF CSP A4 HTML print generator, Edge/Chrome CDP headless PDF generation | `internal/arena` |
| **Embedded Frontend** | `internal/arena/web/` | Production distribution assets embedded directly into the Go executable via `//go:embed` | None (Self-contained) |
| **Development Frontend** | `frontend/` (Optional source) | Source code for rich Vue3/Tailwind/Monaco web interface with full npm/Vite tooling | Node.js (Dev only) |
| **Specifications** | `SPEC.md`, `docs/`, `tasks/` | Architecture blueprints, single-source-of-truth specs, task tracking | None |

---

## 3. Data Flow & Lifecycles

### 3.1 Independent User & Group Binding Flow
1. Teachers define global `Users` and `Groups` independently of any contest.
2. A `Contest` is created in `Draft` status.
3. Groups or individual Users are bound to the `Contest`.
4. When the contest is started (`Running`), only authorized participants may submit code.

### 3.2 Submission & Safe Evaluation Flow
1. Student submits code to `POST /api/contests/{cid}/submissions`.
2. Service validates contest status (`Running`), user authorization, and problem existence.
3. Submission enters `queued` state; a `submission.created` event is broadcast via SSE.
4. An external or sandboxed judge worker (or teacher manual input) updates the verdict; a `submission.judged` event and an updated `ranking.updated` event are broadcast to all connected clients.
5. In accordance with safety rules, submission code is never directly executed in the host server process without a sandbox worker.
