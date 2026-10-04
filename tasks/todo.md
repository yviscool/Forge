# Phase 1: Rebrand & Foundation
- [x] Rename and rebrand project to Forge
- [x] Update SPEC.md, README.md, AGENTS.md, docs/pdf.md
- [x] Refactor domain service to support independent Users, Groups, and multi-contest concurrency
- [x] Implement problem schema validation and bilingual locale structure
- [x] Enhance HTTP router with user/group/contest binding and validation routes
- [x] Update embedded web UI for teacher console and student lobby
- [x] Write comprehensive unit and HTTP integration tests
- [x] Verify `go test ./...` and `go build ./...` pass
- [x] Initialize GitHub repository `yviscool/Forge` via `gh` CLI
- [x] Track `.agents` directory in git and push to GitHub

# Phase 2: PDF Engine & Storage
- [x] Implement pure Go / headless browser CCF PDF generation engine (`internal/arena/pdf.go`)
- [x] Expose `/api/contests/{cid}/problems/{pid}/pdf` endpoint for direct A4 download
- [x] Add automated PDF unit tests verifying CCF layout and standard PDF binary output
- [ ] Create `frontend/` modern UI development scaffolding
- [ ] Implement SQLite persistence storage adapter
