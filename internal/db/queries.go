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
