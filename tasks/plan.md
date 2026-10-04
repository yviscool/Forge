# Implementation plan

1. Establish Go module and domain service with deterministic IDs and lifecycle validation.
2. Add HTTP REST routes, SSE event broker, and embedded browser UI.
3. Add tests for service and HTTP contracts; verify build and test commands.
4. Document the PDF workflow and Wails packaging seam.

The first slice is intentionally self-contained: an in-memory host is useful for local contests and keeps the persistence decision explicit. SQLite adapters can be added without changing the HTTP contract.
