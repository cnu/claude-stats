# Getting Started

This guide walks you through installing claude-stats, running your first ingest, and exploring the TUI dashboard.

## Prerequisites

- **Go 1.26+** (if building from source)
- **Claude Code** installed and used at least once (so there are JSONL session files to analyze)

claude-stats reads the JSONL files that Claude Code writes to `~/.claude/projects/`. If this directory doesn't exist or is empty, there's nothing to analyze yet — just use Claude Code for a bit and come back.

## Installation

### Homebrew (macOS / Linux)

```bash
brew install cnu/tap/claude-stats
```

This installs a pre-built binary. Updates are available via `brew upgrade claude-stats`.

### From Source

```bash
go install github.com/cnu/claude-stats@latest
```

Or clone and build manually:

```bash
git clone https://github.com/cnu/claude-stats.git
cd claude-stats
make build
# Binary is at ./claude-stats
```

### Verify Installation

```bash
claude-stats version
```

You should see output like:

```
claude-stats v0.2.0
  Built:   2026-03-07T18:00:00Z
  Go:      go1.26.0
  OS/Arch: darwin/arm64
```

## First Run

### 1. Ingest Your Data

claude-stats needs to parse your Claude Code session files into a local SQLite database before you can explore them. Run:

```bash
claude-stats ingest
```

This scans `~/.claude/projects/` for all JSONL session files (including subagent files), parses them, calculates costs, and stores everything in `~/.claude-stats/claude-stats.db`.

You'll see output like:

```
Ingested 42 sessions (3 subagent files), 1847 messages in 2.3s
```

The ingest is incremental — running it again only processes new or modified files, so it's fast on subsequent runs.

### 2. Launch the Dashboard

```bash
claude-stats
```

That's it. Running claude-stats with no arguments launches the interactive TUI dashboard and automatically runs a quick ingest first.

You'll land on the **Dashboard** tab showing your total sessions, messages, cost, and a 7-day cost chart.

### 3. Explore the Tabs

Press the number keys `1` through `6` to switch between tabs:

| Key | Tab | What You'll See |
|-----|-----|-----------------|
| `1` | Dashboard | Summary cards, 7-day cost chart, model breakdown |
| `2` | Sessions | Scrollable list of all sessions — press Enter to drill in |
| `3` | Costs | Daily/weekly/monthly cost charts, top sessions, project breakdown |
| `4` | Projects | Projects ranked by cost — press Enter to see sessions |
| `5` | Heatmap | Activity grid by day-of-week and hour |
| `6` | Query | Ask questions in plain English or run SQL |

Press `q` to quit, `?` for help, and `r` to refresh data.

For a detailed walkthrough of each tab, see the [TUI Guide](tui.md).

## Quick One-Shot Queries

Don't need the full dashboard? Ask questions directly from the command line:

```bash
# Plain English
claude-stats query "total cost"
claude-stats query "cost today"
claude-stats query "top 5 models"
claude-stats query "most expensive session"

# Raw SQL
claude-stats query --sql "SELECT count(*) FROM sessions"

# Different output formats
claude-stats query "cost by project" --format json
claude-stats query "cost by project" --format csv
```

See the [Query Guide](queries.md) for all supported patterns.

## Exporting Data

Pull data out for use in spreadsheets, scripts, or backups:

```bash
# All sessions as CSV
claude-stats export sessions > sessions.csv

# Cost report as Markdown
claude-stats export cost-summary > report.md

# Backup the database
claude-stats export dump -o backup.db
```

See the [Commands Reference](commands.md) for full details.

## Where Data Lives

| Path | Contents |
|------|----------|
| `~/.claude/projects/` | Claude Code's raw JSONL session files (read-only input) |
| `~/.claude-stats/claude-stats.db` | SQLite database (created by claude-stats) |

claude-stats never modifies Claude Code's files. The SQLite database can be safely deleted — just run `claude-stats ingest` to rebuild it.

## Next Steps

- [TUI Guide](tui.md) — Detailed walkthrough of every tab and shortcut
- [Commands Reference](commands.md) — All CLI commands, flags, and examples
- [Query Guide](queries.md) — Natural language patterns and SQL examples
- [Configuration](configuration.md) — Environment variables, custom paths, timezone
- [Data Model](data-model.md) — Database schema, JSONL parsing, cost calculation
