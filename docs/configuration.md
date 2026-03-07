# Configuration

claude-stats is configured through command-line flags and environment variables. There is no config file — this keeps things simple and predictable.

## Environment Variables

Set these in your shell profile (`~/.zshrc`, `~/.bashrc`, etc.) for persistent configuration:

| Variable | Flag | Default | Description |
|----------|------|---------|-------------|
| `CLAUDE_STATS_DB` | `--db` | `~/.claude-stats/claude-stats.db` | Path to the SQLite database |
| `CLAUDE_STATS_CLAUDE_DIR` | `--claude-dir` | `~/.claude/projects` | Directory containing Claude Code's JSONL files |
| `CLAUDE_STATS_TIMEZONE` | `--timezone` | `Local` | Timezone for date grouping |
| `CLAUDE_STATS_VERBOSE` | `--verbose` | (unset) | Enable debug logging (any non-empty value) |
| `NO_COLOR` | `--no-color` | (unset) | Disable color output (any non-empty value) |

**Precedence:** Command-line flags override environment variables, which override defaults.

### Example Shell Config

```bash
# ~/.zshrc or ~/.bashrc

# Use UTC for consistent date grouping
export CLAUDE_STATS_TIMEZONE="UTC"

# Custom database location
export CLAUDE_STATS_DB="$HOME/data/claude-stats.db"

# Enable debug logging
export CLAUDE_STATS_VERBOSE=1
```

## Settings in Detail

### Database Path (`--db` / `CLAUDE_STATS_DB`)

Where claude-stats stores its SQLite database. The directory is created automatically if it doesn't exist.

**Default:** `~/.claude-stats/claude-stats.db`

Reasons to change this:
- You want the database on a specific volume or partition
- You want to keep it in a synced folder (Dropbox, iCloud, etc.)
- You're running multiple instances with different databases

```bash
# Store in a custom location
claude-stats --db /path/to/my-stats.db

# Use a database in the current directory
claude-stats --db ./local-stats.db
```

The database can be safely deleted — run `claude-stats ingest` to rebuild it from the original JSONL files.

### Claude Directory (`--claude-dir` / `CLAUDE_STATS_CLAUDE_DIR`)

Where to find Claude Code's JSONL session files. This is the directory claude-stats reads from during ingestion.

**Default:** `~/.claude/projects`

Reasons to change this:
- You have Claude Code data in a non-standard location
- You want to analyze a copied/archived set of session files
- You're testing with a specific subset of data

```bash
# Analyze an archived copy of session files
claude-stats --claude-dir /path/to/archived-sessions ingest
```

### Timezone (`--timezone` / `CLAUDE_STATS_TIMEZONE`)

Controls how dates are grouped in the TUI, daily stats, and date-based queries. Accepts any IANA timezone name.

**Default:** `Local` (your system's timezone)

**Common values:**
- `Local` — Use system timezone (default)
- `UTC` — Coordinated Universal Time
- `America/New_York` — US Eastern
- `America/Los_Angeles` — US Pacific
- `Europe/London` — UK
- `Asia/Kolkata` — India
- `Asia/Tokyo` — Japan

```bash
# Group dates by UTC
claude-stats --timezone UTC

# Group dates by US Pacific time
claude-stats --timezone America/Los_Angeles
```

This affects:
- The `daily_stats` table (which day a midnight-boundary session falls on)
- Date-based queries ("cost today", "sessions this week")
- The Heatmap tab (hour-of-day grouping)

It does **not** affect stored timestamps — those are always Unix milliseconds in the database.

### Verbose Mode (`--verbose` / `CLAUDE_STATS_VERBOSE`)

Enables debug-level logging to stderr.

**Default:** Off

Useful for:
- Debugging ingestion issues (see which files are being processed)
- Seeing generated SQL from natural language queries
- Troubleshooting unexpected results

```bash
# See what's happening during ingest
claude-stats ingest --verbose

# See generated SQL for a natural language query
claude-stats query "cost today" --verbose
```

### No Color (`--no-color` / `NO_COLOR`)

Disables color output. Follows the [NO_COLOR](https://no-color.org/) convention.

**Default:** Off (colors enabled)

```bash
# Disable colors
NO_COLOR=1 claude-stats query "total cost"
```

## Multiple Databases

You can maintain separate databases for different purposes:

```bash
# Work projects only
claude-stats --db ~/work-stats.db --claude-dir ~/work-claude-data ingest

# Personal projects
claude-stats --db ~/personal-stats.db --claude-dir ~/personal-claude-data ingest

# View work stats
claude-stats --db ~/work-stats.db
```

## Data Locations Summary

| Path | Purpose | Created By |
|------|---------|------------|
| `~/.claude/projects/` | Claude Code's raw JSONL session files | Claude Code |
| `~/.claude-stats/claude-stats.db` | claude-stats SQLite database | claude-stats |

claude-stats only reads from `~/.claude/projects/` and only writes to the SQLite database. It never modifies Claude Code's files.
