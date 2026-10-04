# Forge System Architecture & Layering Blueprint

> Hexagonal, manual DI. HTTP API versioned under `/api/v1`, no compat
> guarantee pre-launch. Contract: `api/openapi.yaml`.

## 1. High-Level System Architecture

```mermaid
flowchart TB
    subgraph TeacherSide [Teacher Deployment & Interaction]
        DesktopApp[Forge Runner (LAN IP + QR Display)]
        TeacherWeb[Teacher Console (Vue3 / Tailwind / Monaco)]
    end

    subgraph StudentSide [Student Access]
        BrowserClient[Student Client (Problem / Submit / Live Board)]
    end

    subgraph GoServer [Forge Go Core (Single Portable Executable)]
        HTTP[Transport (/api/v1, SSE, Bearer auth, SPA)]
        AuthSvc[Auth (bcrypt + token sessions)]
        AppSvc[Use-cases (contest/problem/submit/judge/ranking)]
        JudgePool[Judge workers (compile/run/compare)]
        Sandbox[Runner walls (Job Object / prlimit)]
        PDFPipe[CCF A4 HTML + headless PDF]
        EmbedFS[SPA bundle (//go:embed internal/web/dist)]
    end

    subgraph PortsAdapters [Ports & Adapters]
        StoreIface[ports.Store]
        MemStore[memory]
        SqliteStore[sqlite (WAL, file)]
        TestFiles[testdata files]
    end

    TeacherWeb --> HTTP
    BrowserClient --> HTTP
    HTTP --> AuthSvc & AppSvc & JudgePool & PDFPipe
    AppSvc --> StoreIface
    JudgePool --> Sandbox
    StoreIface -.-> MemStore & SqliteStore
    HTTP --> EmbedFS
```

---

## 2. Layered Responsibilities & Directory Mapping

| Layer | Directory | Responsibilities | Depends on |
|---|---|---|---|
| **Entrypoint** | `cmd/forge/` | Flags/env, manual DI wiring, graceful shutdown | `internal/*` |
| **Domain** | `internal/domain/` | Pure types, validation, ranking (OI/ACM), compare modes, subtask aggregation | stdlib |
| **Ports** | `internal/ports/` | `Store`, `Broadcaster`, `Clock`, `IDGenerator` interfaces | `internal/domain` |
| **Use-cases** | `internal/app/` | Contest/problem/submit/judge/rejudge/statistics/bundle, `NewService` manual DI | `internal/ports` |
| **Auth** | `internal/auth/` | bcrypt credentials, token sessions, admin seed | `internal/ports` |
| **Judge** | `internal/judge/` | Toolchains, runner + walls, checker/interactor, pool, compile cache | `internal/domain` |
| **Testdata** | `internal/testdata/` | Per-problem `.in/.out` file store + pair checks | stdlib |
| **Transport** | `internal/transport/http/` | `/api/v1` routes, Bearer+role gates, SSE, SPA | `internal/app` etc. |
| **PDF** | `internal/pdf/` | CCF A4 template + headless browser PDF | `internal/domain` |
| **i18n** | `internal/i18n/` | Dictionaries, verdict mapping, `Accept-Language` | stdlib |
| **Realtime** | `internal/realtime/` | SSE hub with contest filter | `internal/domain` |
| **Config** | `internal/config/` | Env-based config | stdlib |
| **Web embed** | `internal/web/dist/` | Built SPA (never hand-edit; `npm run build` in `frontend/`) | — |
| **Dev frontend** | `frontend/` | Vite + Vue3 + Tailwind + Monaco + KaTeX source | Node.js (dev only) |
| **Contract** | `api/openapi.yaml` | Frozen v1 API surface | — |

---

## 3. Data Flow & Lifecycles

### 3.1 Independent User & Group Binding Flow
1. Teachers define global `Users` and `Groups` independently of any contest (or CSV bulk import).
2. A `Contest` is created in `Draft` status (`rankingMode: oi/acm`).
3. Groups or individual Users are bound to the `Contest`.
4. Users log in (`/auth/login`) and act with Bearer tokens; writes and judging require teacher role.
5. When the contest is `Running`, authorized participants submit; identity comes from the session, never the request body.

### 3.2 Submission & Safe Evaluation Flow
1. Student submits to `POST /api/v1/contests/{cid}/submissions`.
2. Service validates contest status (`Running`), authorization, problem existence.
3. Submission enters `queued`; `submission.created` broadcasts via SSE.
4. Auto-judge pool picks it up (`FORGE_AUTOJUDGE=1`): compile once, run per case under walls (Job Object / prlimit), compare (or checker/interactor), aggregate by subtask, write back via `JudgeCases` with per-case detail.
5. Problems without test cases stay `queued` for manual judging; teachers can requeue everything via `rejudge`.
6. `submission.judged` + `ranking.updated` broadcast to contest-filtered subscribers.

### 3.3 Sandbox Boundaries
- Every run: fresh temp dir, scrubbed env, stdout cap (16MB → `OLE`), wall-time kill (`TLE`), memory wall (`MLE`), single-process/active-process limit (fork-bomb containment).
- Network isolation is the classroom gateway's job; judges never need outbound access (documented in README).
