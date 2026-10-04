# Spec: LemonLime Arena

## Objective

LemonLime Arena is a cross-platform contest host inspired by Project_LemonLime and the current Go/Wails architecture of magpie. A teacher runs one host, configures multiple contests, edits multiple problems, groups users, starts or evaluates contests, and observes live submissions and rankings. Students use a browser to join independently of a specific contest, submit code, and watch their own status and the public ranking. Chinese and English are first-class locales. Problem statements can be validated and exported as print-ready PDF documents.

## Capability map

| Module | Responsibility | Depends on |
|---|---|---|
| identity | users, roles, groups, contest membership | - |
| contests | contest lifecycle, categories, membership | identity |
| problems | statement editing, validation, versions, export | contests |
| judging | submissions, asynchronous evaluation contract, results | contests, identity, problems |
| realtime | SSE events for submissions, judging, rankings | judging |
| web | teacher and student browser applications, i18n | all |

Build order: identity -> contests/problems -> judging -> realtime -> web.

## Tech Stack

- Go 1.26, standard library HTTP server, SQLite-ready repository boundary
- Wails v3 compatibility boundary for future desktop packaging
- Embedded semantic HTML/CSS/ES modules for the first browser client; no build step required
- Server-Sent Events for live updates; REST/JSON for commands and queries
- PDF export starts as print-ready HTML with a stable document contract; headless Chromium/Playwright is the supported PDF renderer

## Commands

```text
go test ./...
go run ./cmd/arena
go build ./...
```

## Project Structure

```text
cmd/arena/       HTTP host executable
internal/arena/  domain model, in-memory service, HTTP handlers
web/             embedded teacher/student UI
docs/            architecture and PDF workflow
upstream/        cloned reference repositories (not imported at runtime)
```

## Testing Strategy

Table-driven Go tests cover lifecycle commands, validation, ranking, SSE event fan-out, and HTTP contracts. Browser UI is intentionally thin and consumes the same public API. Every increment must pass `go test ./...` and `go build ./...`.

## Boundaries

- Always: validate input, keep contest membership separate from identity, emit an event for state changes, run tests and build before claiming completion.
- Ask first: changing the persistence schema, adding a runtime dependency, publishing a public repository, or changing the PDF layout contract.
- Never: commit credentials, silently execute untrusted submissions, or overwrite the cloned upstream references.

## Success Criteria for phase 1

1. `go test ./...` and `go build ./...` pass.
2. A host serves teacher and student pages from one process.
3. API supports creating contests, users, groups, problems, submissions, and starting contests.
4. `/api/events` streams submission/result/ranking events.
5. Ranking is deterministic and updates after a judged submission.
6. Problem export returns print-ready HTML with title, statement, constraints, examples, and locale.

## Open Questions

- Which sandbox/runtime should execute C++, Python, Rust, and other languages in production?
- Should persistence default to SQLite or PostgreSQL for multi-host deployments?
- Which exact typography and page dimensions should match the supplied CSP PDFs?
