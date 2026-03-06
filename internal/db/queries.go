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
