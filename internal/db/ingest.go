package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/cnu/claude-stats/internal/parser"
	"github.com/cnu/claude-stats/internal/pricing"
)

// CheckFileState checks if a file needs ingestion by comparing size and mtime.
func (db *DB) CheckFileState(path string, size int64, modTime time.Time) (bool, error) {
	var storedSize int64
	var storedMtime int64

	err := db.conn.QueryRow(
		"SELECT file_size, mod_time FROM ingest_meta WHERE file_path = ?", path,
	).Scan(&storedSize, &storedMtime)

	if err == sql.ErrNoRows {
		return true, nil // New file
	}
	if err != nil {
		return false, fmt.Errorf("check file state: %w", err)
	}

	// Needs re-ingest if size or mtime changed
	return size != storedSize || modTime.Unix() != storedMtime, nil
}

// UpdateIngestMeta records file ingestion metadata.
func (db *DB) UpdateIngestMeta(path string, size int64, modTime time.Time, lineCount int) error {
	_, err := db.conn.Exec(`
		INSERT OR REPLACE INTO ingest_meta (file_path, file_size, mod_time, line_count, ingested_at)
		VALUES (?, ?, ?, ?, ?)`,
		path, size, modTime.Unix(), lineCount, time.Now().Unix(),
	)
	if err != nil {
		return fmt.Errorf("update ingest meta: %w", err)
	}
	return nil
}

// IngestSession inserts all messages and tool uses from a parsed session into the database.
// It computes costs and aggregates session-level stats.
func (db *DB) IngestSession(sessionFile parser.SessionFile, messages []parser.ParsedMessage) error {
	if len(messages) == 0 {
		return nil
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete existing data for this session (for re-ingestion)
	sessionID := messages[0].SessionID
	if _, err := tx.Exec("DELETE FROM tool_uses WHERE session_id = ?", sessionID); err != nil {
		return fmt.Errorf("delete old tool_uses: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM messages WHERE session_id = ?", sessionID); err != nil {
		return fmt.Errorf("delete old messages: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM sessions WHERE session_id = ?", sessionID); err != nil {
		return fmt.Errorf("delete old session: %w", err)
	}

	// Insert session placeholder first to satisfy FK constraints on messages
	if _, err := tx.Exec(
		"INSERT INTO sessions (session_id, file_path) VALUES (?, ?)",
		sessionID, sessionFile.Path,
	); err != nil {
		return fmt.Errorf("insert session placeholder: %w", err)
	}

	// Prepare statements
	msgStmt, err := tx.Prepare(`
		INSERT INTO messages (uuid, session_id, parent_uuid, timestamp, role, model,
			input_tokens, output_tokens, cache_creation_input_tokens, cache_read_input_tokens,
			cost_usd, duration_ms, content_preview)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare message insert: %w", err)
	}
	defer msgStmt.Close()

	toolStmt, err := tx.Prepare(`
		INSERT INTO tool_uses (message_uuid, session_id, tool_name, tool_input_preview, timestamp)
		VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare tool_use insert: %w", err)
	}
	defer toolStmt.Close()

	// Session aggregation variables
	var (
		projectDir   string
		projectName  string
		gitBranch    string
		version      string
		firstMsg     int64
		lastMsg      int64
		userCount    int
		asstCount    int
		totalInput   int
		totalOutput  int
		totalCCreate int
		totalCRead   int
		totalCost    float64
		totalDurMs   int64
	)

	for i, msg := range messages {
		tsMs := msg.Timestamp.UnixMilli()

		// Calculate cost
		var costUSD float64
		if msg.CostUSD != nil {
			costUSD = *msg.CostUSD
		} else if msg.Role == "assistant" {
			costUSD = pricing.CalculateCost(
				msg.Model,
				msg.Usage.InputTokens,
				msg.Usage.OutputTokens,
				msg.Usage.CacheCreationInputTokens,
				msg.Usage.CacheReadInputTokens,
			)
		}

		if _, err := msgStmt.Exec(
			msg.UUID, sessionID, msg.ParentUUID, tsMs, msg.Role, msg.Model,
			msg.Usage.InputTokens, msg.Usage.OutputTokens,
			msg.Usage.CacheCreationInputTokens, msg.Usage.CacheReadInputTokens,
			costUSD, msg.DurationMs, msg.ContentPreview,
		); err != nil {
			return fmt.Errorf("insert message %s: %w", msg.UUID, err)
		}

		// Insert tool uses
		for _, tu := range msg.ToolUses {
			if _, err := toolStmt.Exec(msg.UUID, sessionID, tu.Name, tu.InputPreview, tsMs); err != nil {
				return fmt.Errorf("insert tool_use: %w", err)
			}
		}

		// Aggregate session stats
		if i == 0 {
			projectDir = msg.CWD
			projectName = extractProjectName(msg.CWD)
			gitBranch = msg.GitBranch
			version = msg.Version
			firstMsg = tsMs
		}
		// Update metadata from any message that has it
		if msg.CWD != "" && projectDir == "" {
			projectDir = msg.CWD
			projectName = extractProjectName(msg.CWD)
		}
		if msg.GitBranch != "" {
			gitBranch = msg.GitBranch
		}
		if msg.Version != "" {
			version = msg.Version
		}

		lastMsg = tsMs

		switch msg.Role {
		case "user":
			userCount++
		case "assistant":
			asstCount++
		}

		totalInput += msg.Usage.InputTokens
		totalOutput += msg.Usage.OutputTokens
		totalCCreate += msg.Usage.CacheCreationInputTokens
		totalCRead += msg.Usage.CacheReadInputTokens
		totalCost += costUSD
		totalDurMs += msg.DurationMs
	}

	// Update session with aggregated stats
	if _, err := tx.Exec(`
		UPDATE sessions SET
			project_dir = ?, project_name = ?, git_branch = ?,
			claude_version = ?, first_message_at = ?, last_message_at = ?,
			message_count = ?, user_message_count = ?, assistant_message_count = ?,
			total_input_tokens = ?, total_output_tokens = ?,
			total_cache_create_tokens = ?, total_cache_read_tokens = ?,
			total_cost_usd = ?, total_duration_ms = ?
		WHERE session_id = ?`,
		projectDir, projectName, gitBranch,
		version, firstMsg, lastMsg, len(messages),
		userCount, asstCount,
		totalInput, totalOutput, totalCCreate, totalCRead,
		totalCost, totalDurMs,
		sessionID,
	); err != nil {
		return fmt.Errorf("update session: %w", err)
	}

	return tx.Commit()
}

// RebuildDailyStats rebuilds the daily_stats table from messages data.
func (db *DB) RebuildDailyStats(tz *time.Location) error {
	if tz == nil {
		tz = time.Local
	}

	// Clear and rebuild
	if _, err := db.conn.Exec("DELETE FROM daily_stats"); err != nil {
		return fmt.Errorf("clear daily_stats: %w", err)
	}

	rows, err := db.conn.Query(`
		SELECT
			m.timestamp,
			m.session_id,
			m.model,
			m.input_tokens,
			m.output_tokens,
			m.cache_creation_input_tokens,
			m.cache_read_input_tokens,
			m.cost_usd
		FROM messages m
		ORDER BY m.timestamp`)
	if err != nil {
		return fmt.Errorf("query messages for daily stats: %w", err)
	}
	defer rows.Close()

	type dayData struct {
		sessions     map[string]bool
		messageCount int
		inputTokens  int
		outputTokens int
		cacheCreate  int
		cacheRead    int
		totalCost    float64
		models       map[string]bool
	}

	days := make(map[string]*dayData)

	for rows.Next() {
		var (
			tsMs                            int64
			sessionID, model                string
			inputTok, outputTok, cc, cr     int
			costUSD                         float64
		)
		if err := rows.Scan(&tsMs, &sessionID, &model, &inputTok, &outputTok, &cc, &cr, &costUSD); err != nil {
			return fmt.Errorf("scan message row: %w", err)
		}

		t := time.UnixMilli(tsMs).In(tz)
		dateKey := t.Format("2006-01-02")

		d, ok := days[dateKey]
		if !ok {
			d = &dayData{
				sessions: make(map[string]bool),
				models:   make(map[string]bool),
			}
			days[dateKey] = d
		}

		d.sessions[sessionID] = true
		d.messageCount++
		d.inputTokens += inputTok
		d.outputTokens += outputTok
		d.cacheCreate += cc
		d.cacheRead += cr
		d.totalCost += costUSD
		if model != "" {
			d.models[model] = true
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate messages: %w", err)
	}

	stmt, err := db.conn.Prepare(`
		INSERT INTO daily_stats (date_key, session_count, message_count,
			input_tokens, output_tokens, cache_create_tokens, cache_read_tokens,
			total_cost_usd, models_used)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare daily_stats insert: %w", err)
	}
	defer stmt.Close()

	for dateKey, d := range days {
		models := make([]string, 0, len(d.models))
		for m := range d.models {
			models = append(models, m)
		}
		modelsJSON, _ := json.Marshal(models)

		if _, err := stmt.Exec(dateKey, len(d.sessions), d.messageCount,
			d.inputTokens, d.outputTokens, d.cacheCreate, d.cacheRead,
			d.totalCost, string(modelsJSON)); err != nil {
			return fmt.Errorf("insert daily_stats %s: %w", dateKey, err)
		}
	}

	return nil
}

// extractProjectName derives a project name from a working directory path.
// Uses the last two path components (e.g., "/Users/cnu/Projects/myapp" -> "Projects/myapp").
func extractProjectName(cwd string) string {
	if cwd == "" {
		return ""
	}
	cwd = filepath.Clean(cwd)
	parts := strings.Split(cwd, string(filepath.Separator))
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], "/")
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return cwd
}
