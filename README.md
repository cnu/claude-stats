# claude-stats

A Go CLI tool that parses Claude Code's local JSONL session files into SQLite and provides an interactive TUI dashboard for exploring usage, costs, and session analytics.

All data stays local. No network calls, no telemetry.

## Install

```bash
# Homebrew (macOS/Linux)
brew install cnu/tap/claude-stats

# From source
go install github.com/cnu/claude-stats@latest

# Or clone and build
git clone https://github.com/cnu/claude-stats.git
cd claude-stats
make build
```

Requires Go 1.26+.

## Quick Start

```bash
# Launch the interactive TUI (auto-ingests new data)
claude-stats

# Or ingest first, then explore
claude-stats ingest
claude-stats

# One-shot queries (natural language or SQL)
claude-stats query "total cost"
claude-stats query "cost this week"
claude-stats query --sql "SELECT count(*) FROM sessions"
```

## TUI Dashboard

Running `claude-stats` with no arguments launches an interactive terminal dashboard with 6 tabs:

| Tab | Description | Key |
|-----|-------------|-----|
| **Dashboard** | Summary cards, 7-day cost chart, model breakdown | `1` |
| **Sessions** | Scrollable session list, drill into message details | `2` |
| **Costs** | Daily/weekly/monthly cost charts, top sessions, project breakdown | `3` |
| **Projects** | Project list with cost/session stats, drill into sessions | `4` |
| **Heatmap** | Activity grid by day-of-week and hour | `5` |
| **Query** | Natural language or SQL queries with inline results | `6` |

**Keyboard shortcuts:** `1`-`6` switch tabs, `j`/`k` navigate lists, `enter` drills in, `esc` goes back, `s` cycles sort, `r` refreshes, `q` quits.

## Commands

### `ingest`

Scans `~/.claude/projects/` for JSONL session files (including subagent files) and loads them into SQLite. Incremental by default — only new or modified files are processed.

```bash
claude-stats ingest              # Incremental ingest
claude-stats ingest --full       # Force full re-ingest
claude-stats ingest --dry-run    # Show what would be ingested
claude-stats ingest --since 2026-03-01   # Only files modified after this date
claude-stats ingest --project myapp      # Only sessions matching project name
```

### `query`

Run natural language or SQL queries against the database.

```bash
# Natural language queries
claude-stats query "total cost"
claude-stats query "cost today"
claude-stats query "cost this week"
claude-stats query "top 5 models"
claude-stats query "cost by project"
claude-stats query "most expensive session"
claude-stats query "busiest day"
claude-stats query "most used tools"
claude-stats query "average cost per session"

# Raw SQL queries
claude-stats query --sql "SELECT * FROM daily_stats ORDER BY date_key DESC LIMIT 7"
claude-stats query --sql "SELECT tool_name, count(*) as uses FROM tool_uses GROUP BY tool_name ORDER BY uses DESC" --format csv

# Output formats: table (default), json, csv
claude-stats query "total cost" --format json
```

### `export`

Export structured reports for external use.

```bash
# Export all sessions as CSV or JSON
claude-stats export sessions                          # CSV to stdout
claude-stats export sessions --format json -o sessions.json

# Export cost summary report (Markdown or JSON)
claude-stats export cost-summary                      # Markdown to stdout
claude-stats export cost-summary -o report.md
claude-stats export cost-summary --format json -o report.json

# Backup the database
claude-stats export dump -o backup.db
```

### `completion`

Generate shell completion scripts.

```bash
# Bash
claude-stats completion bash > /etc/bash_completion.d/claude-stats

# Zsh
claude-stats completion zsh > "${fpath[1]}/_claude-stats"

# Fish
claude-stats completion fish > ~/.config/fish/completions/claude-stats.fish
```

### `version`

```bash
claude-stats version
```

## Global Flags

```
--db <path>           SQLite database path (default: ~/.claude-stats/claude-stats.db)
--claude-dir <path>   Claude data directory (default: ~/.claude/projects)
--verbose             Enable debug logging
--no-color            Disable color output
--timezone <tz>       Timezone for date grouping (default: Local)
```

## Environment Variables

All global flags can be set via environment variables:

| Variable | Corresponding Flag |
|----------|-------------------|
| `CLAUDE_STATS_DB` | `--db` |
| `CLAUDE_STATS_CLAUDE_DIR` | `--claude-dir` |
| `CLAUDE_STATS_VERBOSE` | `--verbose` |
| `CLAUDE_STATS_TIMEZONE` | `--timezone` |
| `NO_COLOR` | `--no-color` |

Flags take precedence over environment variables.

## Subagent Support

Claude Code spawns subagents for complex tasks. Their JSONL files are stored at `~/.claude/projects/<hash>/<session-id>/subagents/agent-<id>.jsonl`. claude-stats automatically detects and ingests subagent files, merging their token usage and costs into the parent session. This is important — subagent costs are **not** reflected in the parent session's token counts and can represent the majority of API spend.

## Database Schema

| Table | Description |
|-------|-------------|
| `sessions` | One row per session — project, model, token totals, cost |
| `messages` | One row per message — role, model, tokens, cost, content preview |
| `tool_uses` | One row per tool invocation — tool name, input preview |
| `daily_stats` | Aggregated daily stats — session/message counts, tokens, cost |
| `ingest_meta` | Tracks which files have been ingested (for incremental ingest) |

## Cost Calculation

Costs are calculated from token counts using embedded pricing tables:

| Model | Input ($/MTok) | Output ($/MTok) | Cache Write | Cache Read |
|-------|---------------|-----------------|-------------|------------|
| Opus 4 / 4.5 / 4.6 | $15.00 | $75.00 | $18.75 | $1.50 |
| Sonnet 4 / 4.5 / 4.6 | $3.00 | $15.00 | $3.75 | $0.30 |
| Haiku 4.5 | $0.80 | $4.00 | $1.00 | $0.08 |

If a JSONL line includes a pre-calculated `costUSD` field, that value is used instead. Unknown models fall back to Sonnet pricing.

## Project Structure

```
cmd/claude-stats/main.go       Entrypoint
internal/cli/                   Cobra commands (ingest, query, export, tui, completion, version)
internal/parser/                JSONL parsing (stream-based, lenient)
internal/db/                    SQLite schema, migrations, queries
internal/pricing/               Model pricing lookup
internal/nlquery/               Natural language to SQL pattern matching
internal/tui/                   Bubble Tea TUI screens (6 tabs)
internal/export/                CSV/JSON/Markdown export
```

## Testing

```bash
make test                       # Run all tests
make lint                       # Run linter
make build                      # Build binary
go test ./internal/parser/ -v   # Test specific package
```

## Documentation

- [Getting Started](docs/getting-started.md) — Installation, first run, quick tour
- [TUI Guide](docs/tui.md) — Detailed walkthrough of every tab and keyboard shortcut
- [Commands Reference](docs/commands.md) — All CLI commands, flags, and examples
- [Query Guide](docs/queries.md) — Natural language patterns and SQL examples
- [Configuration](docs/configuration.md) — Environment variables, custom paths, timezone
- [Data Model](docs/data-model.md) — Database schema, JSONL parsing, cost calculation

## License

MIT
