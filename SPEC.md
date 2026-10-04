# Spec: Forge

## 1. Objective

Forge is a cross-platform competitive programming contest host and training workbench, inspired by the offline judging workflow of Project_LemonLime and the Go/Wails desktop architecture of magpie.

Teachers can run Forge as a desktop application (with system tray and auto-discovered local network IP/QR code) or in headless server mode. Through the teacher console, they can configure multiple concurrent contests, edit and validate problem statements, manage independent users and groups, bind participant groups or individual contestants, monitor real-time submissions, and view live scoreboards.

Students access Forge via standard web browsers across the local network or internet to view problem statements in their preferred language, submit code, and track their personal evaluation status and public leaderboard updates in real time.

Both Chinese (`zh-CN`) and English (`en-US`) are first-class locales across frontend UI, backend API messages, judging verdicts, problem statements, and exported documents. Problem statements can be validated for structural completeness and exported as CCF CSP-compliant print-ready A4 PDFs.

---

## 2. Capability Map

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

Build order: `identity` → `contests` / `problems` → `judging` → `realtime` / `i18n` → `pdf` → `web`.

---

## 3. Tech Stack

- **Backend Runtime:** Go 1.26 standard library (`net/http`, `embed`, `sync`, `encoding/json`).
- **Data Persistence:** In-memory repository boundary designed for SQLite pluggability.
- **Frontend Architecture:**
  - *Runtime Distribution:* Embedded semantic HTML5, CSS3, and modern ES modules in `internal/arena/web` bundled via `//go:embed`. Runs zero-dependency with no Node.js required at runtime.
  - *Dev Pipeline (Optional):* Vite-compatible directory layout for future component pre-compilation into `dist/`.
- **Real-Time Communication:** Server-Sent Events (SSE) via `/api/events` with contest-filtering support.
- **PDF Generation Pipeline:**
  - Standard CCF CSP-compliant HTML/CSS print template (A4, 16mm/20mm/17mm margins, SimSun/Consolas typography, cover page, compiler flags, sample boxes, testcase tables).
  - Headless Chromium/Edge via Chrome DevTools Protocol or standard browser Print-to-PDF (`@media print`).
- **Desktop Packaging:** Wails v3 / native system tray compatibility seams for one-click desktop launching.

---

## 4. Commands

```bash
# Run unit and integration tests
go test ./...

# Build all binaries
go build ./...

# Run the Forge contest server
go run ./cmd/arena
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

## 6. Testing Strategy

1. **Service Layer**: Table-driven tests covering user creation, group membership, multi-contest isolation, participant access verification, problem validation rules, submission lifecycle, and ranking calculations.
2. **HTTP Layer**: Endpoint tests verifying REST routes, query parameter handling, JSON request/response formats, static web asset serving, and SSE event streaming.
3. Every increment must cleanly pass `go test ./...` and `go build ./...`.

---

## 7. Boundaries & Rules

- **Always:** Validate inputs, keep user identity independent of contests, broadcast state changes via SSE, run tests and build before claiming completion.
- **Ask First:** Modifying persistence schema, adding heavyweight runtime dependencies, altering PDF layout contracts.
- **Never:** Execute untrusted code directly on the host machine without sandboxing; commit sensitive credentials; overwrite upstream reference clones.
