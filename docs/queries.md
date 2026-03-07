# Query Guide

claude-stats includes a natural language query engine that translates plain English questions into SQL. You can also write raw SQL for more complex analysis.

## Using Queries

### From the Command Line

```bash
# Natural language
claude-stats query "cost today"

# Raw SQL
claude-stats query --sql "SELECT count(*) FROM sessions"

# With output format
claude-stats query "top models" --format json
```

### From the TUI

Press `6` to switch to the Query tab. Type your question and press `Enter`. Press `Tab` to toggle between NL and SQL mode.

## Natural Language Patterns

The query engine recognizes these patterns. They're case-insensitive and flexible — you don't need to match them exactly.

### Cost Queries

| You Can Say | What It Returns |
|-------------|-----------------|
| `total cost` | Sum of all session costs |
| `cost today` | Today's total cost |
| `cost yesterday` | Yesterday's total cost |
| `cost this week` / `cost current week` | Cost since Monday |
| `cost this month` / `cost current month` | Cost since the 1st |
| `cost last 7 days` / `cost past 30 days` | Cost for the last N days |
| `cost by project` / `cost per project` | Cost breakdown by project |
| `cost for <project-name>` | Cost for a specific project |
| `daily cost` | Daily costs for the last 14 days |
| `average cost per session` / `average session cost` | Mean cost across all sessions |

**Examples:**

```bash
claude-stats query "total cost"
claude-stats query "how much did I spend this week"
claude-stats query "cost last 7 days"
claude-stats query "cost for my-app"
claude-stats query "daily cost"
```

### Session Queries

| You Can Say | What It Returns |
|-------------|-----------------|
| `how many sessions` | Total session count |
| `sessions today` | Sessions started today |
| `sessions this week` | Sessions started this week |
| `most expensive session` | The single highest-cost session |
| `longest session` | Session with the longest duration |

**Examples:**

```bash
claude-stats query "how many sessions do I have"
claude-stats query "sessions today"
claude-stats query "most expensive session"
claude-stats query "longest session"
```

### Model & Tool Queries

| You Can Say | What It Returns |
|-------------|-----------------|
| `top 5 models` / `top N models` | Top N models by message count |
| `top models` | Top 10 models by message count |
| `most used tools` / `top tools` | Tools ranked by usage count |

**Examples:**

```bash
claude-stats query "top 5 models"
claude-stats query "most used tools"
```

### Activity Queries

| You Can Say | What It Returns |
|-------------|-----------------|
| `how many messages` | Total message count |
| `busiest day` | Day with the most messages |

**Examples:**

```bash
claude-stats query "how many messages"
claude-stats query "busiest day"
```

### Tips

- Patterns are flexible: "what's my total cost", "total cost", and "show me total cost" all work
- Numbers are extracted automatically: "top 5 models", "last 30 days", "cost past 7 days"
- Project names in "cost for X" match the project_name column in the database
- If a pattern isn't recognized, you'll get an error with a list of examples

## Raw SQL Queries

For anything the natural language engine doesn't cover, use `--sql` to query the database directly.

### Available Tables

| Table | Description |
|-------|-------------|
| `sessions` | One row per session — project, model, token totals, cost, duration |
| `messages` | One row per message — role, model, tokens, cost, content preview |
| `tool_uses` | One row per tool invocation — tool name, input preview |
| `daily_stats` | Aggregated daily stats — counts, tokens, cost |
| `ingest_meta` | File ingestion tracking (mainly internal) |

For full column details, see the [Data Model](data-model.md).

### Useful SQL Queries

#### Cost Analysis

```bash
# Daily cost trend for the last 30 days
claude-stats query --sql "
  SELECT date_key, total_cost_usd, session_count, message_count
  FROM daily_stats
  ORDER BY date_key DESC
  LIMIT 30
"

# Cost by model
claude-stats query --sql "
  SELECT model, count(*) as messages, sum(cost_usd) as total_cost
  FROM messages
  WHERE model IS NOT NULL AND model != ''
  GROUP BY model
  ORDER BY total_cost DESC
"

# Most expensive sessions with project context
claude-stats query --sql "
  SELECT session_id, project_name, total_cost_usd, message_count,
         total_duration_ms / 1000 as duration_s
  FROM sessions
  ORDER BY total_cost_usd DESC
  LIMIT 10
"

# Hourly cost distribution
claude-stats query --sql "
  SELECT strftime('%H', datetime(timestamp/1000, 'unixepoch', 'localtime')) as hour,
         count(*) as messages, sum(cost_usd) as cost
  FROM messages
  GROUP BY hour
  ORDER BY hour
"
```

#### Token Usage

```bash
# Total tokens consumed
claude-stats query --sql "
  SELECT sum(total_input_tokens) as input,
         sum(total_output_tokens) as output,
         sum(total_cache_create_tokens) as cache_write,
         sum(total_cache_read_tokens) as cache_read
  FROM sessions
"

# Cache efficiency by project
claude-stats query --sql "
  SELECT project_name,
         sum(total_cache_read_tokens) as cache_reads,
         sum(total_input_tokens) as total_input,
         round(sum(total_cache_read_tokens) * 100.0 / max(sum(total_input_tokens), 1), 1) as cache_pct
  FROM sessions
  GROUP BY project_name
  HAVING total_input > 0
  ORDER BY cache_pct DESC
"

# Average tokens per message by model
claude-stats query --sql "
  SELECT model,
         count(*) as messages,
         avg(input_tokens) as avg_input,
         avg(output_tokens) as avg_output
  FROM messages
  WHERE model IS NOT NULL AND model != ''
  GROUP BY model
  ORDER BY messages DESC
"
```

#### Session Patterns

```bash
# Sessions per day of week
claude-stats query --sql "
  SELECT case strftime('%w', datetime(first_message_at/1000, 'unixepoch', 'localtime'))
           when '0' then 'Sunday'
           when '1' then 'Monday'
           when '2' then 'Tuesday'
           when '3' then 'Wednesday'
           when '4' then 'Thursday'
           when '5' then 'Friday'
           when '6' then 'Saturday'
         end as day,
         count(*) as sessions,
         round(sum(total_cost_usd), 2) as cost
  FROM sessions
  GROUP BY strftime('%w', datetime(first_message_at/1000, 'unixepoch', 'localtime'))
  ORDER BY strftime('%w', datetime(first_message_at/1000, 'unixepoch', 'localtime'))
"

# Long sessions (over 30 minutes)
claude-stats query --sql "
  SELECT session_id, project_name, total_duration_ms / 60000 as minutes,
         message_count, total_cost_usd
  FROM sessions
  WHERE total_duration_ms > 1800000
  ORDER BY total_duration_ms DESC
  LIMIT 20
"

# Sessions with the most tool usage
claude-stats query --sql "
  SELECT t.session_id, s.project_name, count(*) as tool_calls, s.total_cost_usd
  FROM tool_uses t
  JOIN sessions s ON t.session_id = s.session_id
  GROUP BY t.session_id
  ORDER BY tool_calls DESC
  LIMIT 10
"
```

#### Tool Analysis

```bash
# Tool usage frequency
claude-stats query --sql "
  SELECT tool_name, count(*) as uses
  FROM tool_uses
  GROUP BY tool_name
  ORDER BY uses DESC
"

# Tools by cost (which tools are used in expensive sessions)
claude-stats query --sql "
  SELECT t.tool_name, count(distinct t.session_id) as sessions,
         count(*) as uses, round(sum(s.total_cost_usd), 2) as total_session_cost
  FROM tool_uses t
  JOIN sessions s ON t.session_id = s.session_id
  GROUP BY t.tool_name
  ORDER BY total_session_cost DESC
  LIMIT 15
"

# Tool usage over time (last 7 days)
claude-stats query --sql "
  SELECT date(timestamp/1000, 'unixepoch', 'localtime') as day,
         tool_name, count(*) as uses
  FROM tool_uses
  WHERE timestamp > (strftime('%s', 'now', '-7 days') * 1000)
  GROUP BY day, tool_name
  ORDER BY day DESC, uses DESC
"
```

### Output Formats with SQL

```bash
# Pipe JSON to jq for further processing
claude-stats query --sql "SELECT * FROM sessions" --format json | jq '.[] | .project_name' | sort | uniq -c | sort -rn

# Export specific data as CSV
claude-stats query --sql "SELECT date_key, total_cost_usd FROM daily_stats ORDER BY date_key" --format csv > daily_costs.csv

# Limit results
claude-stats query --sql "SELECT * FROM messages" --limit 5
```

### Timestamps

All timestamps in the database are stored as **Unix milliseconds**. Use SQLite's `datetime()` function to convert:

```sql
-- Convert to readable datetime
datetime(timestamp/1000, 'unixepoch', 'localtime')

-- Convert to date only
date(first_message_at/1000, 'unixepoch', 'localtime')

-- Filter by date
WHERE timestamp > (strftime('%s', '2026-03-01') * 1000)
```
