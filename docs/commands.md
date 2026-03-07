# Commands Reference

Complete reference for all claude-stats CLI commands, flags, and usage examples.

## Global Flags

These flags are available on every command:

| Flag | Default | Env Variable | Description |
|------|---------|--------------|-------------|
| `--db <path>` | `~/.claude-stats/claude-stats.db` | `CLAUDE_STATS_DB` | Path to the SQLite database |
| `--claude-dir <path>` | `~/.claude/projects` | `CLAUDE_STATS_CLAUDE_DIR` | Directory containing Claude Code JSONL files |
| `--timezone <zone>` | `Local` | `CLAUDE_STATS_TIMEZONE` | Timezone for date grouping (e.g., `America/New_York`, `UTC`) |
| `--verbose` | `false` | `CLAUDE_STATS_VERBOSE` | Enable debug logging to stderr |
| `--no-color` | `false` | `NO_COLOR` | Disable color output |

Flags take precedence over environment variables. See [Configuration](configuration.md) for details.

---

## `claude-stats` (no subcommand)

Launches the interactive TUI dashboard. Equivalent to `claude-stats tui`.

```bash
claude-stats
```

---

## `ingest`

Scans Claude Code's JSONL session files and loads them into the SQLite database.

```bash
claude-stats ingest [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--full` | `false` | Force full re-ingest, ignoring file change detection |
| `--dry-run` | `false` | Show what would be ingested without writing to database |
| `--since <date>` | (none) | Only process files modified after this date (YYYY-MM-DD) |
| `--project <name>` | (none) | Only process sessions matching this project name |

### How Ingestion Works

1. Scans `--claude-dir` for `.jsonl` files
2. Checks each file against `ingest_meta` table — skips files with unchanged size and modification time
3. Parses each JSONL file line by line, extracting messages, tool uses, and token counts
4. Calculates costs from token counts using embedded pricing tables (or uses pre-calculated `costUSD` if present)
5. Detects subagent files in `<session-id>/subagents/` directories and merges their data into the parent session
6. Rebuilds the `daily_stats` aggregation table

### Examples

```bash
# Standard incremental ingest (fast — skips unchanged files)
claude-stats ingest

# Full re-ingest (useful after pricing updates or bug fixes)
claude-stats ingest --full

# Preview what would be ingested
claude-stats ingest --dry-run

# Only ingest recent files
claude-stats ingest --since 2026-03-01

# Only ingest a specific project
claude-stats ingest --project my-app

# Combine flags
claude-stats ingest --since 2026-03-01 --project my-app --verbose
```

### When to Re-Ingest

- **Normally: never.** The TUI auto-ingests on launch, and `ingest` is incremental.
- **After upgrading claude-stats:** If a new version changes cost calculation or schema, run `ingest --full`.
- **After deleting the database:** Run `ingest` to rebuild from scratch.
- **Debugging:** Use `--verbose` to see which files are being processed.

---

## `tui`

Launches the interactive TUI dashboard.

```bash
claude-stats tui [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--skip-ingest` | `false` | Skip the automatic incremental ingest on launch |

### Examples

```bash
# Launch with auto-ingest (default)
claude-stats tui

# Skip ingest for faster startup
claude-stats tui --skip-ingest

# Use a specific database
claude-stats tui --db /path/to/stats.db
```

See the [TUI Guide](tui.md) for detailed tab descriptions and keyboard shortcuts.

---

## `query`

Run a one-shot query against the database from the command line. Supports natural language questions and raw SQL.

```bash
claude-stats query <question> [flags]
```

### Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `question` | Yes | The question to ask (natural language or SQL) |

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--sql` | `false` | Interpret input as raw SQL instead of natural language |
| `--format <fmt>` | `table` | Output format: `table`, `json`, `csv` |
| `--limit <n>` | `20` | Maximum rows to return |

### Natural Language Examples

```bash
claude-stats query "total cost"
claude-stats query "cost today"
claude-stats query "cost this week"
claude-stats query "cost this month"
claude-stats query "cost last 30 days"
claude-stats query "cost yesterday"
claude-stats query "most expensive session"
claude-stats query "how many sessions"
claude-stats query "how many messages"
claude-stats query "top 5 models"
claude-stats query "cost by project"
claude-stats query "cost for my-project"
claude-stats query "sessions today"
claude-stats query "sessions this week"
claude-stats query "average cost per session"
claude-stats query "longest session"
claude-stats query "most used tools"
claude-stats query "busiest day"
claude-stats query "daily cost"
```

See the [Query Guide](queries.md) for the full list of supported patterns.

### SQL Examples

```bash
# Simple counts
claude-stats query --sql "SELECT count(*) as total FROM sessions"

# Cost by day
claude-stats query --sql "SELECT date_key, total_cost_usd FROM daily_stats ORDER BY date_key DESC LIMIT 14"

# Most used tools
claude-stats query --sql "SELECT tool_name, count(*) as uses FROM tool_uses GROUP BY tool_name ORDER BY uses DESC LIMIT 10"

# Sessions with high cache efficiency
claude-stats query --sql "SELECT session_id, project_name, total_cache_read_tokens * 100.0 / total_input_tokens as cache_pct FROM sessions WHERE total_input_tokens > 0 ORDER BY cache_pct DESC LIMIT 10"
```

### Output Formats

**Table** (default) — Human-readable ASCII table with column alignment:

```
project_name    | total_cost
----------------+-----------
my-app          | 45.23
other-project   | 12.87

(2 rows)
```

**JSON** — Array of objects, useful for piping to `jq`:

```bash
claude-stats query "cost by project" --format json | jq '.[0]'
```

**CSV** — RFC 4180 CSV, useful for spreadsheets:

```bash
claude-stats query "cost by project" --format csv > costs.csv
```

### Verbose Mode

Use `--verbose` with natural language queries to see the generated SQL:

```bash
claude-stats query "cost today" --verbose
# Prints the SQL to stderr, results to stdout
```

---

## `export`

Export structured reports and data for use outside the TUI.

### `export sessions`

Export all sessions as CSV or JSON.

```bash
claude-stats export sessions [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--format <fmt>` | `csv` | Output format: `csv`, `json` |
| `-o, --output <file>` | stdout | Write to file instead of stdout |

**CSV columns:** `session_id`, `project`, `started_at`, `messages`, `cost_usd`, `duration_s`

```bash
# CSV to stdout (pipe to file or another tool)
claude-stats export sessions

# JSON to file
claude-stats export sessions --format json -o sessions.json

# CSV to file
claude-stats export sessions -o sessions.csv
```

### `export cost-summary`

Generate a cost summary report.

```bash
claude-stats export cost-summary [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--format <fmt>` | `markdown` | Output format: `markdown`, `json` |
| `-o, --output <file>` | stdout | Write to file instead of stdout |

The Markdown report includes:
- **Overview** — Total sessions, messages, cost, tokens, most active project, primary model
- **Monthly Costs** — Last 12 months with cost, sessions, and messages
- **Cost by Model** — Breakdown by model with cost and message count
- **Cost by Project** — Breakdown by project with cost and session count

```bash
# Markdown to stdout
claude-stats export cost-summary

# Markdown to file
claude-stats export cost-summary -o report.md

# JSON for programmatic use
claude-stats export cost-summary --format json -o report.json
```

### `export dump`

Copy the SQLite database file for backup or sharing.

```bash
claude-stats export dump -o <file>
```

| Flag | Default | Description |
|------|---------|-------------|
| `-o, --output <file>` | (required) | Output path for the database copy |

```bash
claude-stats export dump -o backup.db
claude-stats export dump -o ~/Dropbox/claude-stats-backup.db
```

The dump is a full copy of the SQLite file. You can open it with any SQLite client (`sqlite3`, DB Browser, etc.).

---

## `completion`

Generate shell completion scripts for tab-completion of commands and flags.

```bash
claude-stats completion <shell>
```

| Argument | Required | Description |
|----------|----------|-------------|
| `shell` | Yes | One of: `bash`, `zsh`, `fish` |

### Installation

**Bash:**
```bash
claude-stats completion bash > /etc/bash_completion.d/claude-stats
# Or for current user only:
claude-stats completion bash > ~/.local/share/bash-completion/completions/claude-stats
```

**Zsh:**
```bash
claude-stats completion zsh > "${fpath[1]}/_claude-stats"
# Then restart your shell or run:
compinit
```

**Fish:**
```bash
claude-stats completion fish > ~/.config/fish/completions/claude-stats.fish
```

After installing, restart your shell. Then type `claude-stats ` and press Tab to see available commands and flags.

---

## `version`

Print version, build date, Go version, and OS/architecture.

```bash
claude-stats version
```

Output:
```
claude-stats v0.2.0
  Built:   2026-03-07T18:00:00Z
  Go:      go1.26.0
  OS/Arch: darwin/arm64
```

---

## Common Workflows

### Daily Check-In

```bash
# Quick overview
claude-stats query "cost today"
claude-stats query "cost this week"

# Or launch the TUI
claude-stats
```

### Monthly Report

```bash
# Generate a cost report
claude-stats export cost-summary -o "report-$(date +%Y-%m).md"

# Or as JSON for processing
claude-stats export cost-summary --format json | jq '.summary.total_cost'
```

### Data Analysis in a Spreadsheet

```bash
# Export sessions
claude-stats export sessions -o sessions.csv

# Or query specific data
claude-stats query --sql "SELECT date_key, total_cost_usd FROM daily_stats" --format csv > daily.csv
```

### Backup Before Upgrade

```bash
claude-stats export dump -o "backup-$(date +%Y%m%d).db"
```

### Debugging Ingestion Issues

```bash
# See what files would be processed
claude-stats ingest --dry-run --verbose

# Force full re-ingest
claude-stats ingest --full --verbose
```
