# Forge

Cross-platform competitive programming contest hosting, training workbench, and live evaluation platform.

Inspired by the evaluation model of [Project LemonLime](https://github.com/Project-LemonLime/Project_LemonLime) and the modern Go architecture of [yetone/magpie](https://github.com/yetone/magpie).

---

## Highlights

- **Multi-Contest Concurrency:** Create, configure, run, and evaluate multiple independent contests in parallel.
- **Independent Identity & Grouping:** Global users and reusable groups (classes, training camps). Assign entire groups or individual students to contests.
- **Modern Web Interfaces:**
  - **Teacher Console (`/teacher`):** Contest management, rich problem editor, statement schema validator, group assignment, submission monitoring, and live scoreboard.
  - **Student Lobby (`/`):** View accessible contests, bilingual problem statements, submit code in multiple languages, and follow live rankings.
- **Zero-Dependency Distribution:** Core web assets are embedded into a single Go binary via `//go:embed`. No Node.js runtime needed by end users.
- **Full-Stack i18n:** Built-in bilingual support (`zh-CN` / `en-US`) across frontend interfaces, API messages, verdicts, and problem statement locales.
- **CCF CSP-Compliant PDF Export:** Generate standard A4 contest papers (cover page, compiler parameters, instructions, sample formatting, testcase specifications) with clean print styles and headless browser automation.

---

## Quick Start

```powershell
# Run the Forge host
go run ./cmd/arena
```

- Open `http://localhost:8080/` for the Student Lobby.
- Open `http://localhost:8080/teacher` for the Teacher Console.
- Real-time events are streamed via Server-Sent Events at `http://localhost:8080/api/events`.

---

## Test & Build

```powershell
go test ./...
go build ./...
```

---

## Architecture & Documents

- [SPEC.md](SPEC.md) - Architectural specification and capability map.
- [docs/pdf.md](docs/pdf.md) - CCF CSP PDF specification and generation pipeline.
- [AGENTS.md](AGENTS.md) - Agent guidelines and safety boundaries.
