# LemonLime Arena

Cross-platform contest hosting for teachers and students, based on the current Go/Wails direction of magpie and the contest workflow of Project_LemonLime.

## Run

```powershell
go run ./cmd/arena
```

Open `http://localhost:8080/` for the student view and `http://localhost:8080/teacher` for the teacher view. The server exposes REST JSON under `/api` and an SSE stream at `/api/events`.

## Current phase

The first phase provides a working in-memory host: users, groups, contests, problems, submissions, deterministic ranking, live events, i18n labels, and print-ready problem export. It deliberately does not execute untrusted code. See [SPEC.md](SPEC.md), [tasks/plan.md](tasks/plan.md), and [docs/pdf.md](docs/pdf.md).

## References

The upstream repositories are cloned under `upstream/` for local reference only. They are not vendored into the runtime binary.
