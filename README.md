# Forge Contest Workshop

[English](README.md) | [简体中文](README.zh-CN.md)

Forge is a modern competitive-programming platform for schools, training teams,
and contest organizations. A single Go binary serves the API, SQLite
persistence, judging orchestration, real-time SSE events, and an embedded SPA;
the frontend is built with React 19, TypeScript, Vite, Tailwind CSS, and the
Monaco Editor.

## Capabilities

- Concurrent contest management: `draft`, `running`, and `finished` lifecycles
  are fully isolated.
- Users and groups: an independent account system, bulk CSV import, temporary
  exam tickets, and password resets.
- Multiple problem types: traditional programming, answer-only, interactive,
  and GESP single/multiple-choice questions.
- Statements and data: Markdown/KaTeX, bilingual statements, a test-data file
  repository, subtask DAGs with per-case limits.
- Judging: toolchain probing, a compile cache, per-case results, OI/ACM
  scoreboards, rejudging, and manual review.
- Exam-room safety: strict time windows, IP seat binding, paper passphrases,
  targeted broadcasts, and hard submission cutoffs.
- Delivery: CCF A4 PDF/HTML papers, score CSV exports, and full contest bundle
  import/export.
- Observability: health/readiness probes, request trace IDs, real-time SSE
  status, and structured logs.

## Quick Start

Requires Go 1.27.3+ (including stdlib security fixes). Node.js 24.15+ is only
needed to rebuild the frontend (the repo pins 24.21.0).

### Desktop Mode (recommended for teacher machines)

```powershell
go build -tags "desktop wails" -ldflags="-H windowsgui" -o forge-desktop.exe ./cmd/forge
./forge-desktop.exe
```

Desktop mode is built on Wails v3 (the same stack as magpie — a truly native
window for the teacher's machine):

- The **main window** is the teacher console (WebView connected to the local
  HTTP service; SSE/cookies share the same channel as the student side).
- **Tray left-click** pops a frameless quick-view panel anchored to the icon:
  contest countdown, online users/submissions/judge queue, seat-conflict
  alerts, and recent submissions; the right-click menu jumps straight to the
  teacher console, student lobby, or the QR page.
- **Closing the main window hides to tray**; exit goes through the tray menu
  or the quick-view panel's "Exit service" (graceful shutdown).
- Single instance: launching again wakes the running instance.

Students are unaffected (they use a browser against the LAN address). Double
-clicking starts desktop mode by default; `--server` forces headless.
`-tags desktop` (without `wails`) builds a tray-only fallback without WebView
dependencies (the teacher console opens in a browser). Linux/Darwin server
deployments keep using headless mode.

### Server Mode (server / lab host)

```powershell
cd frontend
npm.cmd ci
npm.cmd run build
cd ..

$env:FORGE_ADMIN_PASSWORD = "change-this-before-first-start"
go run ./cmd/forge
```

Open:

- Student lobby: `http://localhost:8080/`
- Teacher console: `http://localhost:8080/teacher`
- Health probe: `http://localhost:8080/healthz`
- Readiness probe: `http://localhost:8080/readyz`

On first start without `FORGE_ADMIN_PASSWORD`, the service generates a one
-time random administrator password and writes it to
`FORGE_DATA_DIR/bootstrap-admin-password.txt` (mode 0600); the startup log only
prints the file path. That password should be used once for the first login
and the file deleted right after changing the password.

While developing the frontend you can run Vite standalone; in production the
Go service hosts the build output embedded from `internal/web/dist`. After
login the browser only keeps an `HttpOnly`, `SameSite=Strict` `forge_session`
cookie; JavaScript never touches or persists long-lived credentials. The
database only stores the SHA-256 digest of session tokens, so a leaked
database cannot replay valid sessions.

## Configuration

| Environment variable | Default | Description |
|---|---|---|
| `FORGE_ADDR` | `127.0.0.1:8080` | HTTP listen address (set explicitly for LAN access) |
| `FORGE_DATA_DIR` | `./data` | SQLite and test-data directory |
| `FORGE_STORE` | `sqlite` | `sqlite` or `memory` |
| `FORGE_ADMIN_PASSWORD` | random | Password used to create the first admin |
| `FORGE_LOCALE` | `zh-CN` | Server default locale |
| `FORGE_AUTOJUDGE` | `false` | Enable the local auto judge |
| `FORGE_ALLOW_UNSAFE_LOCAL_JUDGE` | `false` | Explicitly acknowledge that the local judge has no full sandbox |
| `FORGE_JUDGE_WORKERS` | physical cores | Judge concurrency, minimum 1 |
| `FORGE_TOOLCHAINS` | built-in config | Toolchain JSON path |
| `FORGE_WIN_SANDBOX` | `auto` | Windows judge sandbox: `auto` = LowBox token + workdir ACL (falls back to Job-only walls when the probe fails), `off` = Job walls only |
| `FORGE_TRUST_PROXY` | `false` | Only enable forwarded IP headers behind a trusted reverse proxy |

`FORGE_AUTOJUDGE=1` must be paired with `FORGE_ALLOW_UNSAFE_LOCAL_JUDGE=1`.
On Windows (the primary teacher-machine platform) the judge additionally runs
every submission inside a LowBox/AppContainer sandbox: filesystem access is
denied outside the working directory (explicit ACL for a per-machine package
SID), network sockets are denied without capabilities, and the Job memory/
process walls stack on top. The sandbox self-probes at startup and falls back
to Job-only walls when the probe fails (`FORGE_WIN_SANDBOX=off` opts out).
Linux runs submissions inside bwrap (`--unshare-all`) with prlimit walls when
available. Even so, production deployments handling untrusted code at scale
should still prefer an isolated judge cluster.

The toolchain JSON (`FORGE_TOOLCHAINS`) supports, besides
`command/args/std/compileTimeoutSec/compileMemoryMiB`, per-language limit
ratios (mirroring LemonLime's Compiler settings): `timeLimitRatio` (2~3 is
recommended for interpreted languages), `memoryLimitRatio`, and
`disableMemoryLimitCheck` (turn off the hard memory wall for runtimes with
their own memory management). Example:

```json
{
  "python": { "command": "python", "timeLimitRatio": 3, "memoryLimitRatio": 2 }
}
```

Per-problem overrides (mirroring LemonLime's per-task compiler configuration,
stacked multiplicatively with toolchain ratios): `compileStd` (C/C++ only,
e.g. `"c++17"`), `timeLimitRatio`, `memoryLimitRatio`, and
`disableMemoryLimitCheck`. The `external_tool` comparison mode additionally
takes `diffCommand`/`diffArgs` (mirroring LemonLime's external diff tool;
defaults to `--ignore-space-change --text --brief`).

Judge foundation platform semantics: the `ignore_space` comparison mode
mirrors LemonLime's token comparison — identical tokens with different line
layout yield `PE`, extra trailing tokens yield `OLE`. The soft time limit hits
when wall clock or CPU time exceeds it; the hard kill wall is
limit + max(2s, 2×limit) × 25%, and borderline runs between the two walls are
re-run automatically. The memory wall uses a Job Object on Windows, prlimit on
Linux (optionally wrapped in a bwrap namespace sandbox), and a degraded
`sh ulimit` on macOS; oversized BSS images are rejected as MLE before the run
via static footprint analysis. Interactors and checkers run inside their own
resource walls with a minimal environment; an interactor may write testlib
score markers to stderr on exit 0 (`ok` / `partially correct (N)` /
`points X`) to award partial credit. Compiled artifacts are cached by
(source, effective toolchain, compiler version, platform) with LRU eviction;
workers default to physical core count (an explicit `FORGE_JUDGE_WORKERS`
wins), and Windows teacher machines suppress system sleep while judging.

Special judges support three protocols: `special_judge` (exit code
0=AC/1=WA/2=PE), `lemon_special_judge` (the Lemon scoreFile protocol, partial
credit 0..full), and `testlib_special_judge` (the testlib stderr protocol:
`ok` / `partially correct (N)` / `points X` with partial credit).

## Testing & Acceptance

```powershell
gofmt -l internal cmd
go mod verify
go vet ./...
go test ./...
go build ./...

cd frontend
npm.cmd audit --audit-level=high
npm.cmd run typecheck
npm.cmd test -- --run
npx.cmd playwright install chromium
npm.cmd run test:e2e
npm.cmd run build

cd ..
npx.cmd --yes @redocly/cli@2.67.1 lint api/openapi.yaml --format=stylish
```

The current frontend tests cover the login gate, session restore, SSE
reconnect, countdown, the answer-sheet flow, teacher statement/data workflows,
answer-only problems, contest bundle import, security settings,
Markdown/KaTeX, and accessibility components.

## Operations & Docs

- [System specification](SPEC.md) (Chinese)
- [Architecture](docs/architecture.md) (Chinese)
- [Deployment manual](docs/deployment.md) (Chinese)
- [CCF paper layout](docs/pdf.md) (Chinese)
- [OpenAPI contract](api/openapi.yaml)
- [Developer guide](AGENTS.md) (Chinese)
- [简体中文说明](README.zh-CN.md)

APIs live under `/api/v1`. The project makes no pre-launch backwards
-compatibility promise: back up `FORGE_DATA_DIR` before upgrading and verify
bundle import, the start-contest precheck, and the full submission pipeline on
a throwaway database first.
