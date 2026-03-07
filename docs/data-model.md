# Data Model

This document describes how claude-stats reads Claude Code's data, stores it in SQLite, and calculates costs.

## Data Flow

```
~/.claude/projects/<hash>/<session>.jsonl
           │
           ▼
     JSONL Parser (internal/parser)
           │
           ▼
     Cost Calculator (internal/pricing)
           │
           ▼
     SQLite Database (internal/db)
           │
           ▼
     TUI / Query / Export
```

1. **Parse** — Read JSONL files line by line, extract messages, tool uses, and metadata
2. **Calculate** — Compute costs from token counts using pricing tables
3. **Store** — Write parsed data into normalized SQLite tables
4. **Aggregate** — Rebuild daily statistics for fast querying
5. **Query** — TUI, CLI queries, and exports read from SQLite

## JSONL Source Files

Claude Code writes one JSONL file per session at:

```
~/.claude/projects/<project-hash>/<session-uuid>.jsonl
```

Each line is a JSON object representing an event. claude-stats only processes lines with `type: "user"` or `type: "assistant"` — other event types (system, result, etc.) are silently skipped.

### Fields Extracted

| Field | Type | Description |
|-------|------|-------------|
| `sessionId` | string | Session identifier |
| `uuid` | string | Unique message identifier |
| `parentUuid` | string | Parent message UUID (for threading) |
| `timestamp` | string | ISO 8601 timestamp |
| `type` | string | Event type — only "user" and "assistant" are processed |
| `cwd` | string | Working directory path (used to derive project name) |
| `gitBranch` | string | Git branch at time of message |
| `version` | string | Claude Code version |
| `costUSD` | float | Pre-calculated cost (used when present, takes priority) |
| `duration` | int | Message processing time in milliseconds |
| `message.role` | string | "user" or "assistant" |
| `message.model` | string | Model name (e.g., `claude-sonnet-4-20250514`) |
| `message.content` | array/string | Message content blocks |
| `message.usage` | object | Token usage statistics |

### Content Blocks

The `message.content` field can be either a string or an array of typed blocks:

- **Text:** `{"type": "text", "text": "..."}` — Regular message text. First 200 characters are stored as `content_preview`.
- **Tool Use:** `{"type": "tool_use", "name": "Read", "input": {...}}` — Tool invocation. Each one becomes a row in `tool_uses`.
- **Thinking:** `{"type": "thinking", "thinking": "..."}` — Model reasoning. Skipped for content preview.

### Token Usage

From `message.usage`:

| Field | Description |
|-------|-------------|
| `input_tokens` | Tokens in the input prompt |
| `output_tokens` | Tokens generated as output |
| `cache_creation_input_tokens` | Tokens written to the prompt cache |
| `cache_read_input_tokens` | Tokens read from the prompt cache |

### Streaming Updates

Claude Code appends to JSONL files as responses stream in. The same message UUID may appear multiple times with updated token counts. claude-stats handles this with `INSERT OR REPLACE` — the last occurrence wins.

### Parsing Behavior

- **Lenient:** Unknown fields are ignored. Malformed lines are logged as warnings and skipped.
- **UTF-8 BOM:** Automatically stripped if present.
- **Large lines:** Lines up to 10 MB are supported.
- **Timestamps:** Both RFC 3339 and ISO 8601 formats are accepted.
- **Content flexibility:** Handles `content` as either a string or an array of blocks.

## Subagent Files

Claude Code spawns subagents for complex tasks. Their session files live in a subdirectory:

```
~/.claude/projects/<hash>/<session-uuid>/subagents/agent-<id>.jsonl
```

claude-stats automatically detects and ingests these files, merging their messages, token counts, and costs into the parent session.

**Why this matters:** Subagent costs are not reflected in the parent session's own token counts. Without subagent ingestion, you'd significantly undercount your actual API spend. In some sessions, subagent costs can exceed the parent session's cost.

## Database Schema

### `sessions`

One row per session. Aggregates all messages and subagent data.

| Column | Type | Description |
|--------|------|-------------|
| `session_id` | TEXT PK | Unique session identifier |
| `file_path` | TEXT | Path to the source JSONL file |
| `project_dir` | TEXT | Working directory (from `cwd` field) |
| `project_name` | TEXT | Derived project name (last path component) |
| `git_branch` | TEXT | Git branch at session time |
| `claude_version` | TEXT | Claude Code version |
| `first_message_at` | INTEGER | Unix ms — when the session started |
| `last_message_at` | INTEGER | Unix ms — when the session ended |
| `message_count` | INTEGER | Total messages (including subagent messages) |
| `user_message_count` | INTEGER | Messages from the user |
| `assistant_message_count` | INTEGER | Messages from the assistant |
| `total_input_tokens` | INTEGER | Sum of all input tokens |
| `total_output_tokens` | INTEGER | Sum of all output tokens |
| `total_cache_create_tokens` | INTEGER | Sum of all cache write tokens |
| `total_cache_read_tokens` | INTEGER | Sum of all cache read tokens |
| `total_cost_usd` | REAL | Total cost in USD |
| `total_duration_ms` | INTEGER | Total duration in milliseconds |

### `messages`

One row per message. Includes both parent session and subagent messages.

| Column | Type | Description |
|--------|------|-------------|
| `uuid` | TEXT PK | Unique message identifier |
| `session_id` | TEXT FK | Parent session |
| `parent_uuid` | TEXT | Parent message UUID |
| `timestamp` | INTEGER | Unix ms timestamp |
| `role` | TEXT | "user" or "assistant" |
| `model` | TEXT | Model name |
| `input_tokens` | INTEGER | Input token count |
| `output_tokens` | INTEGER | Output token count |
| `cache_creation_input_tokens` | INTEGER | Cache write tokens |
| `cache_read_input_tokens` | INTEGER | Cache read tokens |
| `cost_usd` | REAL | Cost for this message |
| `duration_ms` | INTEGER | Processing time in ms |
| `content_preview` | TEXT | First 200 chars of text content |

### `tool_uses`

One row per tool invocation extracted from message content blocks.

| Column | Type | Description |
|--------|------|-------------|
| `id` | INTEGER PK | Auto-increment ID |
| `message_uuid` | TEXT FK | Parent message |
| `session_id` | TEXT FK | Session (for indexing) |
| `tool_name` | TEXT | Name of the tool (e.g., "Read", "Bash", "Edit") |
| `tool_input_preview` | TEXT | First 200 chars of the tool input JSON |
| `timestamp` | INTEGER | Unix ms timestamp |

### `daily_stats`

Pre-aggregated daily statistics for fast chart rendering.

| Column | Type | Description |
|--------|------|-------------|
| `date_key` | TEXT PK | Date as YYYY-MM-DD |
| `session_count` | INTEGER | Sessions that started on this day |
| `message_count` | INTEGER | Messages sent on this day |
| `input_tokens` | INTEGER | Total input tokens |
| `output_tokens` | INTEGER | Total output tokens |
| `cache_create_tokens` | INTEGER | Total cache write tokens |
| `cache_read_tokens` | INTEGER | Total cache read tokens |
| `total_cost_usd` | REAL | Total cost |
| `models_used` | TEXT | JSON array of model names |
| `active_minutes` | INTEGER | Estimated active minutes |

### `ingest_meta`

Tracks which files have been ingested and their state at ingestion time. Used for incremental ingest.

| Column | Type | Description |
|--------|------|-------------|
| `file_path` | TEXT PK | Source file path |
| `file_size` | INTEGER | File size in bytes at ingest time |
| `mod_time` | INTEGER | File modification time at ingest time |
| `line_count` | INTEGER | Number of lines ingested |
| `ingested_at` | INTEGER | When the ingest happened |

### Indexes

- `idx_messages_session` — messages(session_id)
- `idx_messages_timestamp` — messages(timestamp)
- `idx_messages_model` — messages(model)
- `idx_messages_role` — messages(role)
- `idx_tool_uses_name` — tool_uses(tool_name)
- `idx_tool_uses_session` — tool_uses(session_id)
- `idx_sessions_project` — sessions(project_name)
- `idx_sessions_first_msg` — sessions(first_message_at)

### SQLite Pragmas

The database is configured with:

| Pragma | Value | Purpose |
|--------|-------|---------|
| `journal_mode` | WAL | Write-Ahead Logging for concurrent reads |
| `synchronous` | NORMAL | Balanced durability and performance |
| `busy_timeout` | 5000 | 5-second wait on database locks |
| `foreign_keys` | ON | Enforce referential integrity |

## Cost Calculation

### Pricing Table

Costs are calculated from token counts using these rates (USD per million tokens):

| Model | Input | Output | Cache Write | Cache Read |
|-------|-------|--------|-------------|------------|
| Opus 4 / 4.5 / 4.6 | $15.00 | $75.00 | $18.75 | $1.50 |
| Sonnet 4 / 4.5 / 4.6 | $3.00 | $15.00 | $3.75 | $0.30 |
| Haiku 4.5 | $0.80 | $4.00 | $1.00 | $0.08 |

### Supported Model IDs

Full model IDs and their short-name aliases:

| Full ID | Short Name |
|---------|------------|
| `claude-opus-4-5-20251101` | `claude-opus-4-5` |
| `claude-opus-4-6-20250918` | `claude-opus-4-6` |
| `claude-opus-4-20250514` | — |
| `claude-sonnet-4-5-20250929` | `claude-sonnet-4-5` |
| `claude-sonnet-4-6-20250925` | `claude-sonnet-4-6` |
| `claude-sonnet-4-20250514` | — |
| `claude-haiku-4-5-20251001` | `claude-haiku-4-5` |

### Calculation Formula

For each message:

```
cost = (input_tokens      x input_rate      / 1,000,000)
     + (output_tokens     x output_rate     / 1,000,000)
     + (cache_write_tokens x cache_write_rate / 1,000,000)
     + (cache_read_tokens  x cache_read_rate  / 1,000,000)
```

### Priority

1. If the JSONL line includes a `costUSD` field, that value is used directly (Claude Code pre-calculates this in newer versions)
2. Otherwise, cost is calculated from token counts using the pricing table
3. Unknown models fall back to Sonnet pricing (with a warning logged)

### Cache Efficiency

Prompt caching significantly reduces costs. Cache read tokens cost a fraction of regular input tokens (e.g., $0.30/MTok vs $3.00/MTok for Sonnet). The Costs tab in the TUI shows your cache hit ratio.

A high cache read ratio means:
- Your prompts share common prefixes (system prompts, context)
- You're paying 10x less for those tokens
- Longer sessions tend to have better cache efficiency

## Project Name Detection

The project name is derived from the `cwd` field in the JSONL file:

1. If `cwd` is present, the last path component is used (e.g., `/Users/me/Projects/my-app` becomes `my-app`)
2. If `cwd` is not present, the project name is set to `(unknown)`

The full path is stored in `project_dir` for reference.

## Incremental Ingest

When you run `claude-stats ingest`, it:

1. Lists all `.jsonl` files in the Claude directory
2. For each file, checks `ingest_meta` for a matching `file_path`
3. If the file's size and modification time match the stored values, it's skipped
4. Otherwise, the file is fully re-parsed and its data is updated in the database
5. After all files are processed, `daily_stats` is rebuilt

This makes repeated ingests fast — only new or modified files are processed. Use `--full` to force a complete re-ingest.
