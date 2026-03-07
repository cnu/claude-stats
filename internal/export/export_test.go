package export

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/cnu/claude-stats/internal/db"
	"github.com/cnu/claude-stats/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *db.DB {
	t.Helper()
	database, err := db.OpenMemory()
	require.NoError(t, err)

	ts := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	require.NoError(t, database.IngestSession(
		parser.SessionFile{Path: "/tmp/s1.jsonl", SessionID: "sess-1"},
		[]parser.ParsedMessage{
			{SessionID: "sess-1", UUID: "m1", Timestamp: ts, Role: "user", CWD: "/home/user/Projects/myapp", ContentPreview: "hello"},
			{SessionID: "sess-1", UUID: "m2", Timestamp: ts.Add(30 * time.Second), Role: "assistant", Model: "claude-sonnet-4-6-20250925",
				Usage: parser.UsageStats{InputTokens: 1000, OutputTokens: 200}},
		},
	))
	require.NoError(t, database.RebuildDailyStats(time.UTC))
	return database
}

func TestSessions_CSV(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close() //nolint:errcheck

	var buf bytes.Buffer
	err := Sessions(database, &buf, "csv")
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	assert.GreaterOrEqual(t, len(lines), 2, "should have header + at least 1 row")
	assert.Equal(t, "session_id,project,started_at,messages,cost_usd,duration_s", lines[0])
	assert.Contains(t, lines[1], "sess-1")
}

func TestSessions_JSON(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close() //nolint:errcheck

	var buf bytes.Buffer
	err := Sessions(database, &buf, "json")
	require.NoError(t, err)

	assert.Contains(t, buf.String(), `"session_id"`)
	assert.Contains(t, buf.String(), "sess-1")
}

func TestCostSummary_Markdown(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close() //nolint:errcheck

	var buf bytes.Buffer
	err := CostSummary(database, &buf, "markdown")
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "# Claude Usage Cost Summary")
	assert.Contains(t, output, "## Overview")
	assert.Contains(t, output, "Total Sessions")
	assert.Contains(t, output, "## Cost by Model")
}

func TestCostSummary_JSON(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close() //nolint:errcheck

	var buf bytes.Buffer
	err := CostSummary(database, &buf, "json")
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, `"summary"`)
	assert.Contains(t, output, `"cost_by_model"`)
	assert.Contains(t, output, `"cost_by_project"`)
}

func TestSessions_Empty(t *testing.T) {
	database, err := db.OpenMemory()
	require.NoError(t, err)
	defer database.Close() //nolint:errcheck

	var buf bytes.Buffer
	err = Sessions(database, &buf, "csv")
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	assert.Equal(t, 1, len(lines), "should only have header row")
}
