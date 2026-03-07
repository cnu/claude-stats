package db

import (
	"database/sql"
	"fmt"
)

// QueryResult holds the results of a raw SQL query.
type QueryResult struct {
	Columns []string
	Rows    [][]string
}

// ExecuteQuery runs an arbitrary SQL query and returns the results as strings.
func (db *DB) ExecuteQuery(query string, limit int) (*QueryResult, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("get columns: %w", err)
	}

	result := &QueryResult{Columns: cols}
	count := 0

	for rows.Next() && count < limit {
		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}

		row := make([]string, len(cols))
		for i, v := range values {
			row[i] = formatValue(v)
		}

		result.Rows = append(result.Rows, row)
		count++
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}

	return result, nil
}

func formatValue(v interface{}) string {
	if v == nil {
		return "NULL"
	}
	switch val := v.(type) {
	case []byte:
		return string(val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// GetSessionCount returns the total number of sessions.
func (db *DB) GetSessionCount() (int, error) {
	var count int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count sessions: %w", err)
	}
	return count, nil
}

// GetMessageCount returns the total number of messages.
func (db *DB) GetMessageCount() (int, error) {
	var count int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM messages").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count messages: %w", err)
	}
	return count, nil
}

// GetTotalCost returns the sum of all estimated costs.
func (db *DB) GetTotalCost() (float64, error) {
	var cost sql.NullFloat64
	err := db.conn.QueryRow("SELECT SUM(total_cost_usd) FROM sessions").Scan(&cost)
	if err != nil {
		return 0, fmt.Errorf("sum cost: %w", err)
	}
	if !cost.Valid {
		return 0, nil
	}
	return cost.Float64, nil
}

// DashboardSummary holds aggregate stats for the dashboard display.
type DashboardSummary struct {
	TotalSessions          int
	TotalMessages          int
	TotalInputTokens       int64
	TotalOutputTokens      int64
	TotalCacheCreateTokens int64
	TotalCacheReadTokens   int64
	TotalCost              float64
	AvgDailyCost           float64
	MostActiveProject      string
	PrimaryModel           string
}

// GetDashboardSummary returns aggregate stats for the dashboard.
func (db *DB) GetDashboardSummary() (*DashboardSummary, error) {
	s := &DashboardSummary{}

	err := db.conn.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(message_count), 0),
			COALESCE(SUM(total_input_tokens), 0), COALESCE(SUM(total_output_tokens), 0),
			COALESCE(SUM(total_cache_create_tokens), 0), COALESCE(SUM(total_cache_read_tokens), 0),
			COALESCE(SUM(total_cost_usd), 0)
		FROM sessions`).Scan(
		&s.TotalSessions, &s.TotalMessages,
		&s.TotalInputTokens, &s.TotalOutputTokens,
		&s.TotalCacheCreateTokens, &s.TotalCacheReadTokens,
		&s.TotalCost,
	)
	if err != nil {
		return nil, fmt.Errorf("query session totals: %w", err)
	}

	var avgCost sql.NullFloat64
	_ = db.conn.QueryRow("SELECT AVG(total_cost_usd) FROM daily_stats").Scan(&avgCost)
	if avgCost.Valid {
		s.AvgDailyCost = avgCost.Float64
	}

	var project sql.NullString
	_ = db.conn.QueryRow(`
		SELECT project_name FROM sessions
		WHERE project_name != ''
		GROUP BY project_name ORDER BY COUNT(*) DESC LIMIT 1`).Scan(&project)
	if project.Valid {
		s.MostActiveProject = project.String
	}

	var model sql.NullString
	_ = db.conn.QueryRow(`
		SELECT model FROM messages
		WHERE model != ''
		GROUP BY model ORDER BY COUNT(*) DESC LIMIT 1`).Scan(&model)
	if model.Valid {
		s.PrimaryModel = model.String
	}

	return s, nil
}

// DailyCostEntry represents one day's cost for the bar chart.
type DailyCostEntry struct {
	Date string
	Cost float64
}

// GetRecentDailyCosts returns cost per day for the last N days.
func (db *DB) GetRecentDailyCosts(days int) ([]DailyCostEntry, error) {
	rows, err := db.conn.Query(`
		SELECT date_key, total_cost_usd FROM daily_stats
		ORDER BY date_key DESC LIMIT ?`, days)
	if err != nil {
		return nil, fmt.Errorf("query daily costs: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var entries []DailyCostEntry
	for rows.Next() {
		var e DailyCostEntry
		if err := rows.Scan(&e.Date, &e.Cost); err != nil {
			return nil, fmt.Errorf("scan daily cost: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate daily costs: %w", err)
	}

	// Reverse to chronological order
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}

	return entries, nil
}

// ModelCostBreakdown represents cost by model.
type ModelCostBreakdown struct {
	Model        string
	Cost         float64
	MessageCount int
}

// GetModelCostBreakdown returns cost grouped by model.
func (db *DB) GetModelCostBreakdown() ([]ModelCostBreakdown, error) {
	rows, err := db.conn.Query(`
		SELECT model, SUM(cost_usd) as total_cost, COUNT(*) as msg_count
		FROM messages WHERE model != ''
		GROUP BY model ORDER BY total_cost DESC`)
	if err != nil {
		return nil, fmt.Errorf("query model breakdown: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var results []ModelCostBreakdown
	for rows.Next() {
		var m ModelCostBreakdown
		if err := rows.Scan(&m.Model, &m.Cost, &m.MessageCount); err != nil {
			return nil, fmt.Errorf("scan model breakdown: %w", err)
		}
		results = append(results, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate model breakdown: %w", err)
	}

	return results, nil
}

// SessionListEntry represents a session in the sessions list.
type SessionListEntry struct {
	SessionID    string
	ProjectName  string
	FirstMsgAt   int64
	MessageCount int
	CostUSD      float64
	DurationMs   int64
}

// GetSessionList returns sessions sorted by the given field.
func (db *DB) GetSessionList(sortBy string, limit int) ([]SessionListEntry, error) {
	if limit <= 0 {
		limit = 200
	}

	orderClause := "first_message_at DESC"
	switch sortBy {
	case "cost":
		orderClause = "total_cost_usd DESC"
	case "messages":
		orderClause = "message_count DESC"
	}

	rows, err := db.conn.Query(fmt.Sprintf(`
		SELECT session_id, COALESCE(project_name, ''), COALESCE(first_message_at, 0),
			COALESCE(message_count, 0), COALESCE(total_cost_usd, 0), COALESCE(total_duration_ms, 0)
		FROM sessions ORDER BY %s LIMIT ?`, orderClause), limit)
	if err != nil {
		return nil, fmt.Errorf("query session list: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var entries []SessionListEntry
	for rows.Next() {
		var e SessionListEntry
		if err := rows.Scan(&e.SessionID, &e.ProjectName, &e.FirstMsgAt,
			&e.MessageCount, &e.CostUSD, &e.DurationMs); err != nil {
			return nil, fmt.Errorf("scan session list: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// SessionDetail holds full details for a single session.
type SessionDetail struct {
	SessionID     string
	ProjectName   string
	GitBranch     string
	ClaudeVersion string
	FirstMsgAt    int64
	LastMsgAt     int64
	MessageCount  int
	UserMsgCount  int
	AsstMsgCount  int
	InputTokens   int64
	OutputTokens  int64
	CacheCreate   int64
	CacheRead     int64
	CostUSD       float64
	DurationMs    int64
}

// GetSessionDetail returns full details for a session.
func (db *DB) GetSessionDetail(sessionID string) (*SessionDetail, error) {
	d := &SessionDetail{}
	err := db.conn.QueryRow(`
		SELECT session_id, COALESCE(project_name, ''), COALESCE(git_branch, ''),
			COALESCE(claude_version, ''),
			COALESCE(first_message_at, 0), COALESCE(last_message_at, 0),
			COALESCE(message_count, 0), COALESCE(user_message_count, 0),
			COALESCE(assistant_message_count, 0),
			COALESCE(total_input_tokens, 0), COALESCE(total_output_tokens, 0),
			COALESCE(total_cache_create_tokens, 0), COALESCE(total_cache_read_tokens, 0),
			COALESCE(total_cost_usd, 0), COALESCE(total_duration_ms, 0)
		FROM sessions WHERE session_id = ?`, sessionID).Scan(
		&d.SessionID, &d.ProjectName, &d.GitBranch, &d.ClaudeVersion,
		&d.FirstMsgAt, &d.LastMsgAt,
		&d.MessageCount, &d.UserMsgCount, &d.AsstMsgCount,
		&d.InputTokens, &d.OutputTokens, &d.CacheCreate, &d.CacheRead,
		&d.CostUSD, &d.DurationMs,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	if err != nil {
		return nil, fmt.Errorf("query session detail: %w", err)
	}
	return d, nil
}

// MessageEntry represents a message in a session's message list.
type MessageEntry struct {
	UUID           string
	Timestamp      int64
	Role           string
	Model          string
	InputTokens    int
	OutputTokens   int
	CostUSD        float64
	DurationMs     int64
	ContentPreview string
}

// GetSessionMessages returns all messages for a session ordered by timestamp.
func (db *DB) GetSessionMessages(sessionID string) ([]MessageEntry, error) {
	rows, err := db.conn.Query(`
		SELECT uuid, timestamp, role, COALESCE(model, ''),
			COALESCE(input_tokens, 0), COALESCE(output_tokens, 0),
			COALESCE(cost_usd, 0), COALESCE(duration_ms, 0),
			COALESCE(content_preview, '')
		FROM messages WHERE session_id = ?
		ORDER BY timestamp ASC`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("query session messages: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var entries []MessageEntry
	for rows.Next() {
		var e MessageEntry
		if err := rows.Scan(&e.UUID, &e.Timestamp, &e.Role, &e.Model,
			&e.InputTokens, &e.OutputTokens, &e.CostUSD, &e.DurationMs,
			&e.ContentPreview); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// WeeklyCostEntry represents aggregated cost for one week.
type WeeklyCostEntry struct {
	WeekStart string
	Cost      float64
	Sessions  int
	Messages  int
}

// GetWeeklyCosts returns cost aggregated by week for the last N weeks.
func (db *DB) GetWeeklyCosts(weeks int) ([]WeeklyCostEntry, error) {
	rows, err := db.conn.Query(`
		SELECT date(date_key, 'weekday 0', '-6 days') as week_start,
			SUM(total_cost_usd), SUM(session_count), SUM(message_count)
		FROM daily_stats
		WHERE date_key >= date('now', ? || ' days')
		GROUP BY week_start
		ORDER BY week_start ASC`, fmt.Sprintf("-%d", weeks*7))
	if err != nil {
		return nil, fmt.Errorf("query weekly costs: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var entries []WeeklyCostEntry
	for rows.Next() {
		var e WeeklyCostEntry
		if err := rows.Scan(&e.WeekStart, &e.Cost, &e.Sessions, &e.Messages); err != nil {
			return nil, fmt.Errorf("scan weekly cost: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// MonthlyCostEntry represents aggregated cost for one month.
type MonthlyCostEntry struct {
	Month    string
	Cost     float64
	Sessions int
	Messages int
}

// GetMonthlyCosts returns cost aggregated by month for the last N months.
func (db *DB) GetMonthlyCosts(months int) ([]MonthlyCostEntry, error) {
	rows, err := db.conn.Query(`
		SELECT strftime('%Y-%m', date_key) as month,
			SUM(total_cost_usd), SUM(session_count), SUM(message_count)
		FROM daily_stats
		WHERE date_key >= date('now', ? || ' months')
		GROUP BY month
		ORDER BY month ASC`, fmt.Sprintf("-%d", months))
	if err != nil {
		return nil, fmt.Errorf("query monthly costs: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var entries []MonthlyCostEntry
	for rows.Next() {
		var e MonthlyCostEntry
		if err := rows.Scan(&e.Month, &e.Cost, &e.Sessions, &e.Messages); err != nil {
			return nil, fmt.Errorf("scan monthly cost: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// GetTopExpensiveSessions returns the most expensive sessions.
func (db *DB) GetTopExpensiveSessions(limit int) ([]SessionListEntry, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := db.conn.Query(`
		SELECT session_id, COALESCE(project_name, ''), COALESCE(first_message_at, 0),
			COALESCE(message_count, 0), COALESCE(total_cost_usd, 0), COALESCE(total_duration_ms, 0)
		FROM sessions ORDER BY total_cost_usd DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query top sessions: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var entries []SessionListEntry
	for rows.Next() {
		var e SessionListEntry
		if err := rows.Scan(&e.SessionID, &e.ProjectName, &e.FirstMsgAt,
			&e.MessageCount, &e.CostUSD, &e.DurationMs); err != nil {
			return nil, fmt.Errorf("scan top session: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// ProjectCostEntry represents cost grouped by project.
type ProjectCostEntry struct {
	ProjectName  string
	Cost         float64
	SessionCount int
}

// GetCostByProject returns cost grouped by project name.
func (db *DB) GetCostByProject() ([]ProjectCostEntry, error) {
	rows, err := db.conn.Query(`
		SELECT COALESCE(project_name, '(unknown)'),
			COALESCE(SUM(total_cost_usd), 0), COUNT(*)
		FROM sessions
		GROUP BY project_name
		ORDER BY SUM(total_cost_usd) DESC`)
	if err != nil {
		return nil, fmt.Errorf("query cost by project: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var entries []ProjectCostEntry
	for rows.Next() {
		var e ProjectCostEntry
		if err := rows.Scan(&e.ProjectName, &e.Cost, &e.SessionCount); err != nil {
			return nil, fmt.Errorf("scan project cost: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// CacheEfficiency holds aggregate cache statistics.
type CacheEfficiency struct {
	TotalCacheCreate int64
	TotalCacheRead   int64
	HitRatio         float64
}

// GetCacheEfficiency returns aggregate cache statistics.
func (db *DB) GetCacheEfficiency() (*CacheEfficiency, error) {
	c := &CacheEfficiency{}
	err := db.conn.QueryRow(`
		SELECT COALESCE(SUM(total_cache_create_tokens), 0),
			COALESCE(SUM(total_cache_read_tokens), 0)
		FROM sessions`).Scan(&c.TotalCacheCreate, &c.TotalCacheRead)
	if err != nil {
		return nil, fmt.Errorf("query cache efficiency: %w", err)
	}
	total := c.TotalCacheCreate + c.TotalCacheRead
	if total > 0 {
		c.HitRatio = float64(c.TotalCacheRead) / float64(total)
	}
	return c, nil
}
