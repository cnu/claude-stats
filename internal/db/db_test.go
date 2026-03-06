package db

import (
	"testing"
	"time"

	"github.com/cnu/claude-stats/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenMemory(t *testing.T) {
	db, err := OpenMemory()
	require.NoError(t, err)
	defer db.Close()

	// Verify schema version is 1
	var version int
	err = db.conn.QueryRow("SELECT MAX(version) FROM schema_version").Scan(&version)
	require.NoError(t, err)
	assert.Equal(t, 1, version)
}

func TestMigrations_Idempotent(t *testing.T) {
	db, err := OpenMemory()
	require.NoError(t, err)
	defer db.Close()

	// Running migrations again should be a no-op
	err = db.RunMigrations()
	require.NoError(t, err)
}

func TestCheckFileState_NewFile(t *testing.T) {
	db, err := OpenMemory()
	require.NoError(t, err)
	defer db.Close()

	needs, err := db.CheckFileState("/tmp/test.jsonl", 1024, time.Now())
	require.NoError(t, err)
	assert.True(t, needs)
}

func TestCheckFileState_UnchangedFile(t *testing.T) {
	db, err := OpenMemory()
	require.NoError(t, err)
	defer db.Close()

	now := time.Now()
	err = db.UpdateIngestMeta("/tmp/test.jsonl", 1024, now, 50)
	require.NoError(t, err)

	needs, err := db.CheckFileState("/tmp/test.jsonl", 1024, now)
	require.NoError(t, err)
	assert.False(t, needs)
}

func TestCheckFileState_ChangedSize(t *testing.T) {
	db, err := OpenMemory()
	require.NoError(t, err)
	defer db.Close()

	now := time.Now()
	err = db.UpdateIngestMeta("/tmp/test.jsonl", 1024, now, 50)
	require.NoError(t, err)

	needs, err := db.CheckFileState("/tmp/test.jsonl", 2048, now)
	require.NoError(t, err)
	assert.True(t, needs)
}

func TestIngestSession(t *testing.T) {
	db, err := OpenMemory()
	require.NoError(t, err)
	defer db.Close()

	ts := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	messages := []parser.ParsedMessage{
		{
			SessionID:      "test-session-1",
			UUID:           "msg-001",
			Timestamp:      ts,
			Role:           "user",
			CWD:            "/Users/test/Projects/myapp",
			ContentPreview: "hello world",
		},
		{
			SessionID: "test-session-1",
			UUID:      "msg-002",
			ParentUUID: "msg-001",
			Timestamp: ts.Add(5 * time.Second),
			Role:      "assistant",
			Model:     "claude-sonnet-4-6-20250925",
			Usage: parser.UsageStats{
				InputTokens:              1500,
				OutputTokens:             200,
				CacheCreationInputTokens: 5000,
			},
			ContentPreview: "I can help",
			ToolUses: []parser.ToolUse{
				{ID: "toolu_01", Name: "Read", InputPreview: `{"file_path":"/tmp/test.go"}`},
			},
		},
	}

	sf := parser.SessionFile{
		Path:      "/tmp/test-session-1.jsonl",
		SessionID: "test-session-1",
	}

	err = db.IngestSession(sf, messages)
	require.NoError(t, err)

	// Verify session
	count, err := db.GetSessionCount()
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify messages
	msgCount, err := db.GetMessageCount()
	require.NoError(t, err)
	assert.Equal(t, 2, msgCount)

	// Verify session aggregates
	var (
		projectName string
		userCount   int
		asstCount   int
		inputTok    int
	)
	err = db.conn.QueryRow(`SELECT project_name, user_message_count, assistant_message_count, total_input_tokens FROM sessions WHERE session_id = ?`, "test-session-1").Scan(&projectName, &userCount, &asstCount, &inputTok)
	require.NoError(t, err)
	assert.Equal(t, "Projects/myapp", projectName)
	assert.Equal(t, 1, userCount)
	assert.Equal(t, 1, asstCount)
	assert.Equal(t, 1500, inputTok)

	// Verify tool uses
	var toolCount int
	err = db.conn.QueryRow("SELECT COUNT(*) FROM tool_uses WHERE session_id = ?", "test-session-1").Scan(&toolCount)
	require.NoError(t, err)
	assert.Equal(t, 1, toolCount)

	// Verify cost was calculated
	cost, err := db.GetTotalCost()
	require.NoError(t, err)
	assert.Greater(t, cost, 0.0)
}

func TestIngestSession_ReIngest(t *testing.T) {
	db, err := OpenMemory()
	require.NoError(t, err)
	defer db.Close()

	ts := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	messages := []parser.ParsedMessage{
		{SessionID: "sess-reingest", UUID: "msg-r1", Timestamp: ts, Role: "user", ContentPreview: "v1"},
	}
	sf := parser.SessionFile{Path: "/tmp/reingest.jsonl", SessionID: "sess-reingest"}

	err = db.IngestSession(sf, messages)
	require.NoError(t, err)

	// Re-ingest with different data
	messages2 := []parser.ParsedMessage{
		{SessionID: "sess-reingest", UUID: "msg-r2", Timestamp: ts, Role: "user", ContentPreview: "v2"},
		{SessionID: "sess-reingest", UUID: "msg-r3", Timestamp: ts.Add(time.Second), Role: "assistant", Model: "claude-haiku-4-5-20251001"},
	}

	err = db.IngestSession(sf, messages2)
	require.NoError(t, err)

	// Should have 2 messages now, not 3
	count, err := db.GetMessageCount()
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestRebuildDailyStats(t *testing.T) {
	db, err := OpenMemory()
	require.NoError(t, err)
	defer db.Close()

	ts := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	messages := []parser.ParsedMessage{
		{SessionID: "sess-daily", UUID: "msg-d1", Timestamp: ts, Role: "user"},
		{
			SessionID: "sess-daily", UUID: "msg-d2", Timestamp: ts.Add(5 * time.Second),
			Role: "assistant", Model: "claude-sonnet-4-6-20250925",
			Usage: parser.UsageStats{InputTokens: 1000, OutputTokens: 500},
		},
	}
	sf := parser.SessionFile{Path: "/tmp/daily.jsonl", SessionID: "sess-daily"}
	err = db.IngestSession(sf, messages)
	require.NoError(t, err)

	err = db.RebuildDailyStats(time.UTC)
	require.NoError(t, err)

	var (
		dateKey      string
		sessionCount int
		msgCount     int
		totalCost    float64
	)
	err = db.conn.QueryRow("SELECT date_key, session_count, message_count, total_cost_usd FROM daily_stats").Scan(&dateKey, &sessionCount, &msgCount, &totalCost)
	require.NoError(t, err)
	assert.Equal(t, "2026-03-01", dateKey)
	assert.Equal(t, 1, sessionCount)
	assert.Equal(t, 2, msgCount)
	assert.Greater(t, totalCost, 0.0)
}

func TestExecuteQuery(t *testing.T) {
	db, err := OpenMemory()
	require.NoError(t, err)
	defer db.Close()

	result, err := db.ExecuteQuery("SELECT COUNT(*) as cnt FROM sessions", 20)
	require.NoError(t, err)
	assert.Equal(t, []string{"cnt"}, result.Columns)
	require.Len(t, result.Rows, 1)
	assert.Equal(t, "0", result.Rows[0][0])
}

func TestExtractProjectName(t *testing.T) {
	tests := []struct {
		cwd      string
		expected string
	}{
		{"/Users/test/Projects/myapp", "Projects/myapp"},
		{"/Users/test", "Users/test"},
		{"myapp", "myapp"},
		{"", ""},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, extractProjectName(tt.cwd), "cwd: %s", tt.cwd)
	}
}
