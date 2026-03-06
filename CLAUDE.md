# CLAUDE.md

## Project: claude-stats

Go CLI tool that parses Claude Code JSONL usage data into SQLite and provides a TUI for analytics.

## Tech Stack
- Go 1.26+, Cobra CLI, Bubble Tea TUI, lipgloss, modernc.org/sqlite (pure Go, no CGo)
- SQLite for persistence, TOML for config

## Key Commands
- `make build` - Build binary
- `make test` - Run tests
- `make lint` - Run linter (golangci-lint)
- `go run ./cmd/claude-stats` - Run from source

## Architecture
- `cmd/claude-stats/main.go` - Entrypoint
- `internal/cli/` - Cobra commands
- `internal/parser/` - JSONL parsing (stream-based, lenient)
- `internal/db/` - SQLite schema, migrations, queries
- `internal/analyzer/` - Stats computation
- `internal/nlquery/` - Natural language to SQL pattern matching
- `internal/pricing/` - Model pricing lookup
- `internal/tui/` - Bubble Tea screens
- `internal/export/` - CSV/JSON/Markdown export
- `internal/config/` - Viper config management

## Conventions
- Use `modernc.org/sqlite`, NOT `mattn/go-sqlite3` (pure Go, no CGo needed)
- All timestamps stored as Unix milliseconds in SQLite
- Use `slog` for logging (Go stdlib)
- Test files go in the same package with `_test.go` suffix
- Test fixtures in `testdata/` at the repo root
- Error wrapping: always use `fmt.Errorf("context: %w", err)`
- All exported functions need doc comments

## Data Source
- JSONL files at ~/.claude/projects/<project-hash>/<session-uuid>.jsonl
- Schema is undocumented and evolving. Parser must be lenient (ignore unknown fields).
- Key fields: sessionId, uuid, timestamp, model, message.role, message.usage, costUSD

## Testing
- `go test ./...` runs all tests
- Use testify for assertions
- Test fixtures in `testdata/`
- Mock SQLite with in-memory databases for db tests
