# claude-stats

A Go CLI tool that parses Claude Code's local JSONL session files into SQLite and provides analytics on usage, costs, and sessions.

All data stays local. No network calls, no telemetry.

## Install

```bash
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
# Ingest your Claude Code session data into SQLite
claude-stats ingest

# Query your data
claude-stats query --sql "SELECT count(*) FROM sessions"
claude-stats query --sql "SELECT date_key, total_cost_usd FROM daily_stats ORDER BY date_key DESC LIMIT 7"

# See version info
claude-stats version
```

## Commands

### `ingest`

Scans `~/.claude/projects/` for JSONL session files and loads them into SQLite. Incremental by default — only new or modified files are processed.

```bash
claude-stats ingest              # Incremental ingest
claude-stats ingest --full       # Force full re-ingest
claude-stats ingest --dry-run    # Show what would be ingested
claude-stats ingest --since 2026-03-01   # Only files modified after this date
claude-stats ingest --project myapp      # Only sessions matching project name
```

### `query`

Run raw SQL queries against the database.

```bash
# Table output (default)
claude-stats query --sql "SELECT project_name, count(*) as sessions, sum(total_cost_usd) as cost FROM sessions GROUP BY project_name ORDER BY cost DESC LIMIT 10"

# JSON output
claude-stats query --sql "SELECT * FROM daily_stats ORDER BY date_key DESC LIMIT 7" --format json

# CSV output
claude-stats query --sql "SELECT tool_name, count(*) as uses FROM tool_uses GROUP BY tool_name ORDER BY uses DESC" --format csv
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

## Database Schema

The SQLite database contains these tables:

| Table | Description |
|-------|-------------|
| `sessions` | One row per JSONL file — project, model, token totals, cost |
| `messages` | One row per message — role, model, tokens, cost, content preview |
| `tool_uses` | One row per tool invocation — tool name, input preview |
| `daily_stats` | Aggregated daily stats — session/message counts, tokens, cost |
| `ingest_meta` | Tracks which files have been ingested (for incremental ingest) |

### Useful Queries

```sql
-- Total spend
SELECT sum(total_cost_usd) FROM sessions;

-- Daily cost for the last 30 days
SELECT date_key, total_cost_usd, message_count
FROM daily_stats ORDER BY date_key DESC LIMIT 30;

-- Cost by project
SELECT project_name, count(*) as sessions, sum(total_cost_usd) as cost
FROM sessions GROUP BY project_name ORDER BY cost DESC;

-- Cost by model
SELECT model, count(*) as messages, sum(cost_usd) as cost
FROM messages WHERE model != '' GROUP BY model ORDER BY cost DESC;

-- Most used tools
SELECT tool_name, count(*) as uses
FROM tool_uses GROUP BY tool_name ORDER BY uses DESC LIMIT 20;

-- Most expensive sessions
SELECT session_id, project_name, total_cost_usd, message_count
FROM sessions ORDER BY total_cost_usd DESC LIMIT 10;

-- Cache efficiency
SELECT
  sum(cache_read_input_tokens) as cache_reads,
  sum(input_tokens) as inputs,
  round(100.0 * sum(cache_read_input_tokens) / (sum(input_tokens) + sum(cache_read_input_tokens)), 1) as cache_hit_pct
FROM messages;
```

## Testing

```bash
# Run all tests
make test

# Run tests for a specific package
go test ./internal/parser/ -v
go test ./internal/pricing/ -v
go test ./internal/db/ -v

# Test against real data (uses a temp DB)
claude-stats ingest --db /tmp/test.db
claude-stats query --db /tmp/test.db --sql "SELECT count(*) FROM sessions"
rm /tmp/test.db
```

## How It Works

1. Claude Code stores all conversation data as JSONL files in `~/.claude/projects/<project-hash>/<session-uuid>.jsonl`
2. `claude-stats ingest` parses these files, extracts messages/tool usage/token counts, calculates costs using model pricing tables, and stores everything in SQLite
3. `claude-stats query` lets you run SQL against that database

### Cost Calculation

Costs are calculated from token counts using embedded pricing tables:

| Model | Input ($/MTok) | Output ($/MTok) | Cache Write | Cache Read |
|-------|---------------|-----------------|-------------|------------|
| Opus 4 / 4.6 | $15.00 | $75.00 | $18.75 | $1.50 |
| Sonnet 4 / 4.6 | $3.00 | $15.00 | $3.75 | $0.30 |
| Haiku 4.5 | $0.80 | $4.00 | $1.00 | $0.08 |

If a JSONL line includes a pre-calculated `costUSD` field, that value is used instead. Unknown models fall back to Sonnet pricing.

## Project Structure

```
cmd/claude-stats/main.go       Entrypoint
internal/cli/                   Cobra commands (ingest, query, version)
internal/parser/                JSONL parsing (stream-based, lenient)
internal/db/                    SQLite schema, migrations, queries
internal/pricing/               Model pricing lookup
```

## License

MIT
