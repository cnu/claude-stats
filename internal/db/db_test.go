package db

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cnu/claude-stats/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenMemory(t *testing.T) {
	db, err := OpenMemory()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	// Verify schema version is 1
	var version int
	err = db.conn.QueryRow("SELECT MAX(version) FROM schema_version").Scan(&version)
	require.NoError(t, err)
	assert.Equal(t, 1, version)
}

func TestMigrations_Idempotent(t *testing.T) {
	db, err := OpenMemory()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	// Running migrations again should be a no-op
	err = db.RunMigrations()
	require.NoError(t, err)
}

func TestCheckFileState_NewFile(t *testing.T) {
	db, err := OpenMemory()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	needs, err := db.CheckFileState("/tmp/test.jsonl", 1024, time.Now())
	require.NoError(t, err)
	assert.True(t, needs)
}

func TestCheckFileState_UnchangedFile(t *testing.T) {
	db, err := OpenMemory()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

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
	defer db.Close() //nolint:errcheck

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
	defer db.Close() //nolint:errcheck

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
	defer db.Close() //nolint:errcheck

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
	defer db.Close() //nolint:errcheck

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
	defer db.Close() //nolint:errcheck

	result, err := db.ExecuteQuery("SELECT COUNT(*) as cnt FROM sessions", 20)
	require.NoError(t, err)
	assert.Equal(t, []string{"cnt"}, result.Columns)
	require.Len(t, result.Rows, 1)
	assert.Equal(t, "0", result.Rows[0][0])
}

func TestIngestSession_EmptyMessages(t *testing.T) {
	db, err := OpenMemory()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	sf := parser.SessionFile{Path: "/tmp/empty.jsonl", SessionID: "sess-empty"}
	err = db.IngestSession(sf, nil)
	require.NoError(t, err)

	count, err := db.GetSessionCount()
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestIngestSession_WithCostUSD(t *testing.T) {
	db, err := OpenMemory()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	ts := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	cost := 0.0042
	messages := []parser.ParsedMessage{
		{
			SessionID: "sess-cost", UUID: "msg-c1", Timestamp: ts,
			Role: "assistant", Model: "claude-sonnet-4-6-20250925",
			CostUSD: &cost,
			Usage:   parser.UsageStats{InputTokens: 100, OutputTokens: 50},
		},
	}
	sf := parser.SessionFile{Path: "/tmp/cost.jsonl", SessionID: "sess-cost"}
	err = db.IngestSession(sf, messages)
	require.NoError(t, err)

	// Should use the pre-calculated costUSD, not compute from tokens
	var storedCost float64
	err = db.conn.QueryRow("SELECT cost_usd FROM messages WHERE uuid = ?", "msg-c1").Scan(&storedCost)
	require.NoError(t, err)
	assert.InDelta(t, 0.0042, storedCost, 0.0001)
}

func TestIngestSession_MetadataFromLaterMessages(t *testing.T) {
	db, err := OpenMemory()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	ts := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	messages := []parser.ParsedMessage{
		{SessionID: "sess-meta", UUID: "msg-m1", Timestamp: ts, Role: "user"},
		{
			SessionID: "sess-meta", UUID: "msg-m2", Timestamp: ts.Add(time.Second),
			Role: "assistant", Model: "claude-sonnet-4-6-20250925",
			CWD: "/Users/test/Projects/webapp", GitBranch: "feature-x", Version: "2.1.70",
		},
	}
	sf := parser.SessionFile{Path: "/tmp/meta.jsonl", SessionID: "sess-meta"}
	err = db.IngestSession(sf, messages)
	require.NoError(t, err)

	var projectName, gitBranch, version string
	err = db.conn.QueryRow("SELECT project_name, git_branch, claude_version FROM sessions WHERE session_id = ?", "sess-meta").Scan(&projectName, &gitBranch, &version)
	require.NoError(t, err)
	assert.Equal(t, "Projects/webapp", projectName)
	assert.Equal(t, "feature-x", gitBranch)
	assert.Equal(t, "2.1.70", version)
}

func TestRebuildDailyStats_NilTimezone(t *testing.T) {
	db, err := OpenMemory()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	// Should use local timezone and not panic
	err = db.RebuildDailyStats(nil)
	require.NoError(t, err)
}

func TestRebuildDailyStats_MultipleDays(t *testing.T) {
	db, err := OpenMemory()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	day1 := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 3, 2, 14, 0, 0, 0, time.UTC)

	msgs1 := []parser.ParsedMessage{
		{SessionID: "sess-d1", UUID: "msg-dd1", Timestamp: day1, Role: "user"},
		{SessionID: "sess-d1", UUID: "msg-dd2", Timestamp: day1.Add(time.Second), Role: "assistant", Model: "claude-sonnet-4-6-20250925", Usage: parser.UsageStats{InputTokens: 500, OutputTokens: 100}},
	}
	msgs2 := []parser.ParsedMessage{
		{SessionID: "sess-d2", UUID: "msg-dd3", Timestamp: day2, Role: "user"},
		{SessionID: "sess-d2", UUID: "msg-dd4", Timestamp: day2.Add(time.Second), Role: "assistant", Model: "claude-opus-4-6-20250918", Usage: parser.UsageStats{InputTokens: 1000, OutputTokens: 200}},
	}

	require.NoError(t, db.IngestSession(parser.SessionFile{Path: "/tmp/d1.jsonl", SessionID: "sess-d1"}, msgs1))
	require.NoError(t, db.IngestSession(parser.SessionFile{Path: "/tmp/d2.jsonl", SessionID: "sess-d2"}, msgs2))
	require.NoError(t, db.RebuildDailyStats(time.UTC))

	var count int
	err = db.conn.QueryRow("SELECT COUNT(*) FROM daily_stats").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Check models_used is populated
	var modelsUsed string
	err = db.conn.QueryRow("SELECT models_used FROM daily_stats WHERE date_key = '2026-03-02'").Scan(&modelsUsed)
	require.NoError(t, err)
	assert.Contains(t, modelsUsed, "claude-opus-4-6-20250918")
}

func TestExecuteQuery_WithData(t *testing.T) {
	db, err := OpenMemory()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	ts := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	msgs := []parser.ParsedMessage{
		{SessionID: "sess-q", UUID: "msg-q1", Timestamp: ts, Role: "user", ContentPreview: "test"},
		{SessionID: "sess-q", UUID: "msg-q2", Timestamp: ts.Add(time.Second), Role: "assistant", Model: "claude-sonnet-4-6-20250925"},
	}
	require.NoError(t, db.IngestSession(parser.SessionFile{Path: "/tmp/q.jsonl", SessionID: "sess-q"}, msgs))

	result, err := db.ExecuteQuery("SELECT role, COUNT(*) as cnt FROM messages GROUP BY role ORDER BY role", 20)
	require.NoError(t, err)
	assert.Equal(t, []string{"role", "cnt"}, result.Columns)
	assert.Len(t, result.Rows, 2)
}

func TestExecuteQuery_Limit(t *testing.T) {
	db, err := OpenMemory()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	ts := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	var msgs []parser.ParsedMessage
	for i := 0; i < 10; i++ {
		msgs = append(msgs, parser.ParsedMessage{
			SessionID: "sess-lim", UUID: fmt.Sprintf("msg-lim-%d", i),
			Timestamp: ts.Add(time.Duration(i) * time.Second), Role: "user",
		})
	}
	require.NoError(t, db.IngestSession(parser.SessionFile{Path: "/tmp/lim.jsonl", SessionID: "sess-lim"}, msgs))

	result, err := db.ExecuteQuery("SELECT uuid FROM messages", 3)
	require.NoError(t, err)
	assert.Len(t, result.Rows, 3)
}

func TestExecuteQuery_InvalidSQL(t *testing.T) {
	db, err := OpenMemory()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	_, err = db.ExecuteQuery("SELECT * FROM nonexistent_table", 20)
	assert.Error(t, err)
}

func TestExecuteQuery_DefaultLimit(t *testing.T) {
	db, err := OpenMemory()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	result, err := db.ExecuteQuery("SELECT 1 as val", 0)
	require.NoError(t, err)
	assert.Len(t, result.Rows, 1)
}

func TestFormatValue(t *testing.T) {
	assert.Equal(t, "NULL", formatValue(nil))
	assert.Equal(t, "hello", formatValue([]byte("hello")))
	assert.Equal(t, "42", formatValue(42))
	assert.Equal(t, "3.14", formatValue(3.14))
}

func TestGetTotalCost_Empty(t *testing.T) {
	db, err := OpenMemory()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	cost, err := db.GetTotalCost()
	require.NoError(t, err)
	assert.Equal(t, 0.0, cost)
}

func TestCheckFileState_ChangedMtime(t *testing.T) {
	db, err := OpenMemory()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	now := time.Now()
	require.NoError(t, db.UpdateIngestMeta("/tmp/test.jsonl", 1024, now, 50))

	// Same size, different mtime
	needs, err := db.CheckFileState("/tmp/test.jsonl", 1024, now.Add(time.Hour))
	require.NoError(t, err)
	assert.True(t, needs)
}

func TestOpen_FileDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "subdir", "test.db")

	db, err := Open(dbPath)
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	// Verify it created the directory and DB
	_, err = os.Stat(dbPath)
	require.NoError(t, err)

	// Verify Conn() works
	assert.NotNil(t, db.Conn())

	// Verify schema is applied
	var version int
	err = db.Conn().QueryRow("SELECT MAX(version) FROM schema_version").Scan(&version)
	require.NoError(t, err)
	assert.Equal(t, 1, version)
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
