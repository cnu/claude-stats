package db

import (
	"fmt"

	"github.com/cnu/claude-stats/internal/pricing"
)

// RecalcResult summarizes a cost recalculation.
type RecalcResult struct {
	Messages int     // assistant messages repriced
	Changed  int     // messages whose cost changed
	OldTotal float64 // total cost of all messages before
	NewTotal float64 // total cost of all messages after
}

// RecalculateCosts reprices every assistant message from the token counts and
// model stored in the database, using the active pricing table, and then
// updates the session totals. It does not need the JSONL files. Costs that came
// from the JSONL costUSD field are kept. Call RebuildDailyStats afterwards.
func (db *DB) RecalculateCosts() (RecalcResult, error) {
	var res RecalcResult

	tx, err := db.conn.Begin()
	if err != nil {
		return res, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if err := tx.QueryRow("SELECT COALESCE(SUM(cost_usd), 0) FROM messages").Scan(&res.OldTotal); err != nil {
		return res, fmt.Errorf("sum old cost: %w", err)
	}

	rows, err := tx.Query(`
		SELECT uuid, COALESCE(model, ''), input_tokens, output_tokens,
			cache_creation_input_tokens, cache_read_input_tokens, cost_usd
		FROM messages
		WHERE role = 'assistant' AND COALESCE(cost_recorded, 0) = 0`)
	if err != nil {
		return res, fmt.Errorf("query messages: %w", err)
	}

	type update struct {
		uuid string
		cost float64
	}
	var updates []update
	for rows.Next() {
		var (
			uuid, model                   string
			input, output, cCreate, cRead int
			oldCost                       float64
		)
		if err := rows.Scan(&uuid, &model, &input, &output, &cCreate, &cRead, &oldCost); err != nil {
			_ = rows.Close()
			return res, fmt.Errorf("scan message: %w", err)
		}
		res.Messages++
		cost := pricing.CalculateCost(model, input, output, cCreate, cRead)
		if cost != oldCost {
			updates = append(updates, update{uuid, cost})
		}
	}
	if err := rows.Close(); err != nil {
		return res, fmt.Errorf("close rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return res, fmt.Errorf("iterate messages: %w", err)
	}

	stmt, err := tx.Prepare("UPDATE messages SET cost_usd = ? WHERE uuid = ?")
	if err != nil {
		return res, fmt.Errorf("prepare update: %w", err)
	}
	defer stmt.Close() //nolint:errcheck
	for _, u := range updates {
		if _, err := stmt.Exec(u.cost, u.uuid); err != nil {
			return res, fmt.Errorf("update message %s: %w", u.uuid, err)
		}
	}
	res.Changed = len(updates)

	if _, err := tx.Exec(`
		UPDATE sessions SET total_cost_usd =
			(SELECT COALESCE(SUM(cost_usd), 0) FROM messages WHERE messages.session_id = sessions.session_id)`); err != nil {
		return res, fmt.Errorf("update session totals: %w", err)
	}

	if err := tx.QueryRow("SELECT COALESCE(SUM(cost_usd), 0) FROM messages").Scan(&res.NewTotal); err != nil {
		return res, fmt.Errorf("sum new cost: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return res, fmt.Errorf("commit: %w", err)
	}
	return res, nil
}
