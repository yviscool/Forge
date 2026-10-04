# Spec: Forge (V2 Architecture Specification)

## 1. Objective

Forge is a cross-platform competitive programming contest hosting, training workbench, and live evaluation platform. It fuses the offline judging workflow of [Project LemonLime](https://github.com/Project-LemonLime/Project_LemonLime) with the modern Go/Wails desktop architecture of [yetone/magpie](https://github.com/yetone/magpie).

Teachers run Forge with one click as a desktop application (with system tray and auto-discovered local network IP/QR code) or in headless server mode. Through the teacher console, they configure multiple concurrent contests, edit and validate problem statements, manage independent users and groups, bind participant groups or individual contestants, monitor real-time submissions, and view live scoreboards.

Students access Forge via standard web browsers across the local network or internet to view problem statements in their preferred language, submit code, and track their personal evaluation status and public leaderboard updates in real time.

Both Chinese (`zh-CN`) and English (`en-US`) are first-class locales across frontend UI, backend API messages, judging verdicts, problem statements, and exported documents. Problem statements are strictly validated for structural completeness and exported as CCF CSP-compliant print-ready A4 PDFs.

---

## 2. Architecture Stratification & Review Alignment

### 2.1 Frontend Build vs Runtime Decoupling
- **Development Mode (`frontend/`):** Built with modern frontend tooling (Vite + Vue 3 / React + Tailwind CSS + Monaco Editor + KaTeX). Provides rich Markdown live preview, syntax highlighting, split-pane editing, and responsive tables.
- **Runtime Distribution (`internal/web/dist/`):** Pre-compiled production bundles are embedded directly into the Go executable via `//go:embed`. End users and teachers require **zero Node.js or npm dependencies** to run Forge.

### 2.2 Desktop GUI vs Headless Mode
- **Teacher Desktop Mode (Default):** Runs with a native system tray, auto-detects the local IP/port, shows a QR code/URL for students to join, and automatically opens the teacher console.
- **Headless Server Mode:** Supports `forge serve --port 8080` for continuous daemon operation on Linux servers or lab hosts.

### 2.3 Full-Stack i18n
1. **Frontend UI i18n:** Navigation, action buttons, table headers, toasts, and localized date/time.
2. **Backend API i18n:** REST error messages and notices localized via `Accept-Language` headers.
3. **Verdict Terminology i18n:** Canonical mapping between standard verdicts (`Accepted`, `Wrong Answer`, `Time Limit Exceeded`) and Chinese equivalents (`答案正确`, `答案错误`, `运行超时`).
4. **Problem Statement Locales:** Multi-language problem definitions (`Problem.Locales`) supporting seamless language switching.
5. **PDF Paper i18n:** Localized CCF cover notices, candidate instructions, and headers.

### 2.4 PDF Export Pipeline
- **Tier 1 (Default / Zero-Dependency):** Pure Go Chrome DevTools Protocol (CDP) controls the pre-installed system Edge or Chrome browser to output CCF-compliant A4 PDFs with TOC outline injection via pure Go PDF tooling.
- **Tier 2 (Print Fallback):** Web-based `@media print` layout allows direct `Ctrl+P` export.
- **Tier 3 (Advanced Integration):** External Playwright + PyMuPDF pipeline for batch CLI rendering.

---

## 3. Capability Map

| Module ID | Responsibility | Depends On |
|---|---|---|
| `identity` | Independent global users (admin, teacher, student), passwords, roles, and global user groups (classes/teams) | — |
| `contests` | Multiple concurrent contest lifecycles (draft, running, finished), binding by group or individual contestant | `identity` |
| `problems` | Multi-problem management, rich Markdown statements, multi-language statement locales, schema validation, testcase limits | `contests` |
| `judging` | Submission queue, asynchronous evaluation contract (safe non-host execution), verdict tracking, per-contest scoreboards | `contests`, `identity`, `problems` |
| `realtime` | SSE event broker for contest-filtered real-time submissions, judge results, rankings, and lifecycle changes | `judging` |
| `i18n` | Full-stack internationalization: UI dictionaries, localized API error responses, verdict mappings, problem locales | — |
| `pdf` | CCF CSP-standardized A4 HTML template generation, Chromium/Edge CDP headless PDF rendering, bookmark injection | `problems` |
| `web` | Embedded modern responsive web applications for both Teacher Console and Student Contest Lobby | all |

---

## 4. Commands

```bash
# Run unit and integration tests
go test ./...

# Build all binaries
go build ./...

# Run the Forge contest server
go run ./cmd/forge
```

Default access points:
- Student Contest Lobby: `http://localhost:8080/`
- Teacher Management Console: `http://localhost:8080/teacher`
- Live SSE Event Stream: `http://localhost:8080/api/events`

---

## 5. Domain Models & Relational Architecture

### 5.1 Identity & Grouping (Independent of Contests)
- **User**: Global identity with `ID`, `Username`, `Name`, `Role` (`admin`, `teacher`, `student`), and `Groups`.
- **Group**: Reusable organization entity with `ID`, `Name`, `Description`, and `UserIDs`.
- Users can belong to multiple groups; groups can be assigned to contests as a whole.

### 5.2 Contests (Multi-Contest Concurrency)
- **Contest**: Has `ID`, `Name`, `Description`, `Status` (`draft`, `running`, `finished`), `ProblemIDs`, `GroupIDs`, and `ParticipantUserIDs`.
- Supports configuring, starting, and judging multiple contests in parallel.
- Participant access control: a user is authorized if their individual ID is in `ParticipantUserIDs` OR if any of their `Groups` is in `GroupIDs`.

### 5.3 Problems & Validation
- **Problem**: Has `ID`, `ContestID`, `Code` (e.g. "A", "reverse"), `Title`, `Statement`, `Constraints`, `Input`, `Output`, `Examples`, `Locales` (bilingual statements), and `TestData`.
- **Validation Engine**: Enforces presence of required sections (Description, Input, Output, Constraints, paired Samples), preventing invalid statements from entering contests or PDF exports.

### 5.4 Submissions & Ranking
- **Submission**: Records `ID`, `ContestID`, `ProblemID`, `UserID`, `Language`, `Code`, `Verdict`, `Score`, and `SubmittedAt`.
- **Ranking**: Deterministic OI-style scoring per contest (best score per problem aggregated, sorted by score descending, then contestant name ascending).

---

## 6. Boundaries & Rules

- **Always:** Validate inputs, keep user identity independent of contests, broadcast state changes via SSE, run tests and build before claiming completion.
- **Ask First:** Modifying persistence schema, adding heavyweight runtime dependencies, altering PDF layout contracts.
- **Never:** Execute untrusted code directly on the host machine without sandboxing; commit sensitive credentials; overwrite upstream reference clones.
