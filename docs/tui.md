# TUI Guide

The interactive terminal dashboard is the primary way to explore your Claude Code usage. Launch it with:

```bash
claude-stats
# or explicitly:
claude-stats tui
```

By default, the TUI runs a quick incremental ingest before launching. Skip it with `--skip-ingest` if you've already ingested recently and want a faster start.

## Navigation

### Global Shortcuts

These work from any tab:

| Key | Action |
|-----|--------|
| `1`-`6` | Switch tabs |
| `r` | Refresh current tab's data |
| `?` | Toggle help overlay |
| `q` / `Ctrl+C` | Quit |

### List Navigation

Tabs with scrollable lists (Sessions, Projects) share these controls:

| Key | Action |
|-----|--------|
| `j` / `Down` | Move down |
| `k` / `Up` | Move up |
| `Enter` | Drill into selected item |
| `Esc` / `Backspace` | Back to list |
| `s` | Cycle sort order |
| `g` | Jump to top |
| `G` | Jump to bottom |

---

## Tab 1: Dashboard

The overview tab. Shows at-a-glance stats for your entire usage history.

### What You See

- **Summary cards** — Total sessions, total messages, total cost, input/output tokens, most active project, primary model
- **7-day cost chart** — Bar chart of daily cost for the last 7 days
- **Model breakdown** — Cost and message count per model

This tab is view-only — no interactive controls beyond global shortcuts.

### What to Look For

- **Total cost** gives you a quick reality check on API spend
- **Most active project** and **primary model** tell you where your time goes
- The 7-day chart helps spot trends — is spending going up, down, or steady?

---

## Tab 2: Sessions

A scrollable list of every session, sortable by date, cost, or message count.

### List View

Each row shows:

| Column | Description |
|--------|-------------|
| Project | Derived project name |
| Date | When the session started (local time) |
| Messages | Total messages in the session |
| Cost | Total cost in USD |
| Duration | Session length |

**Sort options** (press `s` to cycle):
1. **Date** (default) — Most recent first
2. **Cost** — Most expensive first
3. **Messages** — Highest message count first

### Detail View

Press `Enter` on any session to see its full message list.

Each message shows:
- Role (user/assistant)
- Timestamp
- Model used
- Token counts (input, output, cache read, cache write)
- Cost
- Content preview (first 200 characters)

Scroll with `j`/`k` and press `Esc` to go back.

### Use Cases

- Find your most expensive session to understand what drove the cost
- Review message flow in a session to see how a conversation progressed
- Check which model was used for specific interactions

---

## Tab 3: Costs

Deep dive into cost analytics with multiple time horizons.

### Views

Switch between views with keyboard shortcuts:

| Key | View | Range |
|-----|------|-------|
| `d` | Daily | Last 30 days |
| `w` | Weekly | Last 12 weeks |
| `m` | Monthly | Last 6 months |

Each view shows a bar chart of cost over time.

### Additional Sections

Scroll down (`j`/`k`) to see:

- **Top 10 Most Expensive Sessions** — Quick reference for cost outliers
- **Cost by Project** — Which projects cost the most
- **Cache Efficiency** — Cache hit ratio and how much caching saves you

### Use Cases

- Track spending trends across days, weeks, or months
- Identify which projects drive the most cost
- Check if prompt caching is working effectively — a high cache read ratio means you're saving money

---

## Tab 4: Projects

Projects grouped and ranked, with drill-down into per-project sessions.

### List View

Each project shows:

| Column | Description |
|--------|-------------|
| Name | Project name (derived from working directory) |
| Sessions | Number of sessions |
| Messages | Total messages across all sessions |
| Cost | Total cost |
| Last Active | When the project was last used |
| Top Model | Most-used model in this project |

**Sort options** (press `s` to cycle):
1. **Cost** (default) — Highest cost first
2. **Sessions** — Most sessions first
3. **Name** — Alphabetical
4. **Recent** — Most recently active first

### Detail View

Press `Enter` on a project to see all sessions for that project, showing date, messages, cost, and duration for each.

### Use Cases

- Compare cost across projects to see where your Claude usage goes
- Find projects you haven't touched in a while
- See if certain projects consistently use more expensive models

---

## Tab 5: Heatmap

A 7x24 grid showing activity patterns by day of week and hour.

### Reading the Heatmap

- **Rows** — Days of the week (Monday through Sunday)
- **Columns** — Hours of the day (0-23, in your local timezone)
- **Cell color** — Intensity of activity

Color scale:
- Bright (peak) — More than 75% of maximum
- Medium-bright — 50-75% of maximum
- Medium — 25-50% of maximum
- Dim — Less than 25% of maximum
- Dark — No activity

### Toggle

Press `t` to switch between:
- **Message count** — How many messages were sent in each time slot
- **Cost** — How much was spent in each time slot

### Use Cases

- Discover your peak coding hours
- See if weekends look different from weekdays
- Identify patterns — do you use Claude more in the morning or evening?

---

## Tab 6: Query

An interactive query interface for ad-hoc analysis. Supports both natural language and raw SQL.

### Input Mode

Type your query and press `Enter` to execute.

| Key | Action |
|-----|--------|
| `Enter` | Execute query |
| `Tab` | Toggle between NL and SQL mode |
| `Esc` | Clear results or input |
| `Ctrl+U` | Clear entire input line |
| `Ctrl+A` / `Home` | Jump to start of input |
| `Ctrl+E` / `End` | Jump to end of input |
| `Up` / `Down` | Browse query history (when input is empty) |

The mode indicator in the prompt shows `[NL]` for natural language or `[SQL]` for raw SQL.

### Natural Language Mode

Ask questions in plain English:

```
total cost
cost today
cost this week
top 5 models
most expensive session
cost for my-project
busiest day
```

The TUI shows the generated SQL below your query, which is useful for learning the database schema.

### SQL Mode

Press `Tab` to switch to SQL mode and write queries directly:

```sql
SELECT project_name, count(*) as sessions, sum(total_cost_usd) as cost
FROM sessions
GROUP BY project_name
ORDER BY cost DESC
```

### Results

Results appear as a scrollable table below the query input. Scroll with `j`/`k`. Press `Esc` to dismiss results.

### Query History

The TUI remembers your last 50 queries. Use `Up`/`Down` arrow keys to navigate through them when the input field is empty.

### Use Cases

- Quick lookups without leaving the TUI ("cost today", "how many sessions")
- Exploratory SQL for questions the pre-built tabs don't answer
- Learning the database schema by seeing generated SQL from NL queries

---

## Tips

- **Refresh frequently** — Press `r` to pull in new data if you've been using Claude Code while the TUI is open
- **Start with Dashboard** — Get the big picture, then drill into specific tabs
- **Use Query for edge cases** — If a tab doesn't show what you need, the Query tab with SQL mode can answer almost anything
- **Sort is your friend** — In Sessions and Projects, press `s` to cycle through sort options and find what you're looking for
- **Skip ingest for speed** — If you're iterating quickly, `claude-stats tui --skip-ingest` launches faster
