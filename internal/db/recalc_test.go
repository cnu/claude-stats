package db

import (
	"testing"
	"time"

	"github.com/cnu/claude-stats/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecalculateCosts(t *testing.T) {
	db, err := OpenMemory()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	recorded := 9.99
	ts := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	msgs := []parser.ParsedMessage{
		{SessionID: "sess-rc", UUID: "rc1", Timestamp: ts, Role: "assistant", Model: "claude-opus-5",
			Usage: parser.UsageStats{InputTokens: 1_000_000}},
		{SessionID: "sess-rc", UUID: "rc2", Timestamp: ts.Add(time.Second), Role: "assistant", Model: "claude-opus-5",
			Usage: parser.UsageStats{InputTokens: 1_000_000}, CostUSD: &recorded},
		{SessionID: "sess-rc", UUID: "rc3", Timestamp: ts.Add(2 * time.Second), Role: "user"},
	}
	require.NoError(t, db.IngestSession(parser.SessionFile{Path: "/tmp/rc.jsonl", SessionID: "sess-rc"}, msgs))

	// Simulate a cost stored with old, wrong pricing.
	_, err = db.conn.Exec("UPDATE messages SET cost_usd = 3.0 WHERE uuid = 'rc1'")
	require.NoError(t, err)

	res, err := db.RecalculateCosts()
	require.NoError(t, err)
	assert.Equal(t, 1, res.Messages, "recorded costUSD is not repriced")
	assert.Equal(t, 1, res.Changed)
	assert.InDelta(t, 3.0+9.99, res.OldTotal, 1e-9)
	assert.InDelta(t, 5.0+9.99, res.NewTotal, 1e-9)

	var sessionCost float64
	require.NoError(t, db.conn.QueryRow("SELECT total_cost_usd FROM sessions WHERE session_id = 'sess-rc'").Scan(&sessionCost))
	assert.InDelta(t, 5.0+9.99, sessionCost, 1e-9)
}
