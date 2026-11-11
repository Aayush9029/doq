# doq

A Go CLI and TUI that searches Apple SDK symbols and the local Apple documentation. It ships through `aayush9029/tap`. Status: shipped.

## Commands

| Task | Command |
|------|---------|
| Build | `go build -o doq .` |
| Test | `go test ./...` |
| Live docs tests (macOS 26 or later) | `DOQ_LIVE_DOCS_TESTS=1 go test ./internal/docs/...` |
| Debug output | `DOQ_DEBUG=1 doq ...` |

## Architecture

- `internal/extract` runs `xcrun swift symbolgraph-extract` on the Xcode SDKs. `internal/symbolgraph` parses the JSON.
- `internal/index` builds and queries a SQLite FTS5 index at `~/.local/share/doq/index.db`.
- `internal/docs` powers `doq docs`. `bridge_darwin.m` calls the system documentation, embedding, and vector search frameworks through cgo. `bridge_stub.go` covers builds without cgo.
- `internal/tui` and `internal/render` hold the TUI and the text output.

## Rules

- `doq docs` needs macOS 26 or later. Keep the symbol index commands working on older macOS versions.
- `doq index` with no arguments indexes a curated set of about 30 frameworks. `--all` indexes every SDK framework and takes much longer.
- Keep cgo code in `internal/docs`, and keep the stub in step with the darwin bridge API.
