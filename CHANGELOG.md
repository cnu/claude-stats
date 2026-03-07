# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [0.2.0] - 2026-03-08

First feature-complete release. Adds an interactive TUI dashboard, natural language queries, data export, and subagent cost tracking.

### Added

- **TUI dashboard** with 6 interactive tabs:
  - Dashboard — summary cards, 7-day cost chart, model breakdown
  - Sessions — scrollable list with drill-down into message details
  - Costs — daily/weekly/monthly charts, top sessions, project breakdown, cache efficiency
  - Projects — project list with cost/session stats, drill into sessions
  - Heatmap — activity grid by day-of-week and hour (messages or cost)
  - Query — interactive natural language and SQL query interface with history
- **Natural language query engine** with 23 patterns — ask questions like "cost today", "top 5 models", "most expensive session", or "busiest day" from the CLI or TUI
- **Export command** with three subcommands:
  - `export sessions` — all sessions as CSV or JSON
  - `export cost-summary` — Markdown or JSON cost report with monthly trends, model breakdown, and project costs
  - `export dump` — SQLite database backup
- **Subagent ingestion** — automatically detects and ingests Claude Code subagent files, merging their token usage and costs into the parent session (subagent costs are not reflected in parent session token counts and can represent the majority of API spend)
- **Shell completions** for bash, zsh, and fish (`claude-stats completion <shell>`)
- **Environment variable configuration** — `CLAUDE_STATS_DB`, `CLAUDE_STATS_CLAUDE_DIR`, `CLAUDE_STATS_TIMEZONE`, `CLAUDE_STATS_VERBOSE`, and `NO_COLOR`
- **User documentation** — 6 detailed guides covering getting started, TUI usage, commands, queries, configuration, and the data model
- GitHub Actions workflow to sync docs to the GitHub wiki
- Pre-commit hook running go vet, tests, and golangci-lint

### Fixed

- Session duration now computed from first/last message timestamps instead of summing individual durations
- Duplicate message UUIDs in JSONL files (streaming updates) no longer cause ingestion failures
- Added pricing for claude-sonnet-4-5, claude-opus-4-5-20251101, and synthetic/internal models

## [0.1.2] - 2026-03-07

### Fixed

- Resolved linter errors and GoReleaser deprecation warnings

## [0.1.1] - 2026-03-07

### Changed

- Switched from manual release workflow to GoReleaser for cross-platform builds and Homebrew distribution

## [0.1.0] - 2026-03-07

Initial release. Core functionality for ingesting and querying Claude Code usage data.

### Added

- **JSONL parser** — stream-based, lenient parsing of Claude Code session files with support for all content block types (text, tool use, thinking)
- **SQLite storage** — normalized schema with sessions, messages, tool uses, daily stats, and ingestion tracking
- **Ingest command** — incremental file scanning with `--full`, `--dry-run`, `--since`, and `--project` filters
- **Query command** — raw SQL queries with `--format table|json|csv` and `--limit`
- **Model pricing engine** — embedded pricing tables for Opus, Sonnet, and Haiku model families with automatic cost calculation from token counts
- **Version command** — prints version, build date, Go version, and OS/architecture

[0.2.0]: https://github.com/cnu/claude-stats/compare/v0.1.2...v0.2.0
[0.1.2]: https://github.com/cnu/claude-stats/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/cnu/claude-stats/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/cnu/claude-stats/releases/tag/v0.1.0
