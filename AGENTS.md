# Forge Agent Guide

Run `go test ./...` and `go build ./...` before reporting completion. Keep the HTTP API backwards compatible within a phase. Domain changes belong in `internal/arena`; the embedded client in `web` must remain usable without Node. Do not execute submitted code directly on the host: the current service records submissions and results only, and a sandboxed judge worker is a planned module.
