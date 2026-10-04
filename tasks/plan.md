# Forge Implementation Plan

## Overview
Forge is being refactored from a preliminary prototype into a full-featured contest and training system.

## Task Breakdown
1. **Domain Model Refactoring (`internal/arena/service.go`)**:
   - Support independent Users and global Groups (M:N mapping).
   - Support multiple concurrent Contests with independent lifecycles (`draft`, `running`, `finished`).
   - Support binding Participant Groups and Individual Users to Contests.
   - Implement problem validation logic (`ValidateProblem`) checking mandatory sections and constraints.
   - Support rich Problem models with multi-locale definitions.
   - Refactor ranking calculation to be per-contest and deterministic.

2. **HTTP API & Endpoints (`internal/arena/http.go`)**:
   - Provide complete REST routes for Users, Groups, Contests, Problems, Submissions, and Rankings.
   - Implement validation endpoint `/api/contests/{cid}/problems/{pid}/validate`.
   - Implement group-to-contest and user-to-contest binding endpoints.
   - Enhance problem export with CCF CSP-styled print HTML.
   - Add `/api/i18n` endpoint providing frontend dictionary translations.

3. **Web Interface Enhancement (`internal/arena/web/`)**:
   - Rebrand UI to **Forge**.
   - Teacher console: Multi-contest switcher, User/Group management panel, Problem editor with validation banner, live submission and scoreboard feed.
   - Student lobby: Contest selector, problem viewer with bilingual toggle, submission box, and live scoreboard.
   - Modern, responsive styling with clean CSS variables and cards.

4. **Testing & Verification**:
   - Expand `service_test.go` and `http_test.go` to cover all new capabilities.
   - Ensure `go test ./...` and `go build ./...` cleanly succeed.

5. **Repository Setup**:
   - Use `gh` CLI to initialize and publish the public repository `yviscool/Forge`.
