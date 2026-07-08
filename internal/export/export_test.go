package export

import (
	"bytes"
	"os"
	"path/filepath"
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

func TestSessionTranscript_Success_MainFileOnly(t *testing.T) {
	database, sessionID, _, cleanup := setupTranscriptDB(t, false)
	defer cleanup()

	var buf bytes.Buffer
	err := SessionTranscript(database, &buf, sessionID)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "# Session Transcript: "+sessionID)
	assert.Contains(t, out, "## Metadata")
	assert.Contains(t, out, "### 1. USER - 2026-03-02T14:00:00Z")
	assert.Contains(t, out, "### 2. ASSISTANT - 2026-03-02T14:00:03Z")
	assert.Contains(t, out, "[thinking]")
	assert.Contains(t, out, "**Tool Use:** `Read` (`toolu_01`)")
	assert.Contains(t, out, "```json")
	assert.Contains(t, out, "{\"file_path\":\"/tmp/a.txt\"}")
}

func TestSessionTranscript_IncludesSubagentMessages(t *testing.T) {
	database, sessionID, _, cleanup := setupTranscriptDB(t, true)
	defer cleanup()

	var buf bytes.Buffer
	err := SessionTranscript(database, &buf, sessionID)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "subagent says hello")
}

func TestSessionTranscript_SortsByTimestampAcrossFiles(t *testing.T) {
	database, sessionID, _, cleanup := setupTranscriptDB(t, true)
	defer cleanup()

	var buf bytes.Buffer
	err := SessionTranscript(database, &buf, sessionID)
	require.NoError(t, err)

	out := buf.String()
	first := strings.Index(out, "subagent says hello")
	second := strings.Index(out, "main user message")
	require.NotEqual(t, -1, first)
	require.NotEqual(t, -1, second)
	assert.Less(t, first, second)
}

func TestSessionTranscript_SessionNotFound(t *testing.T) {
	database, err := db.OpenMemory()
	require.NoError(t, err)
	defer database.Close() //nolint:errcheck

	var buf bytes.Buffer
	err = SessionTranscript(database, &buf, "missing-session")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")
}

func TestSessionTranscript_MissingSourceFile(t *testing.T) {
	database, sessionID, mainPath, cleanup := setupTranscriptDB(t, false)
	defer cleanup()

	require.NoError(t, os.Remove(mainPath))

	var buf bytes.Buffer
	err := SessionTranscript(database, &buf, sessionID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session source file not found")
	assert.Contains(t, err.Error(), mainPath)
}

func TestSessionTranscript_EmptySession(t *testing.T) {
	database, err := db.OpenMemory()
	require.NoError(t, err)
	defer database.Close() //nolint:errcheck

	tmpDir := t.TempDir()
	emptyPath := filepath.Join(tmpDir, "empty.jsonl")
	require.NoError(t, os.WriteFile(emptyPath, []byte(`{"type":"summary","sessionId":"sess-empty"}`+"\n"), 0644))

	_, err = database.Conn().Exec(`INSERT INTO sessions (
		session_id, file_path, project_name, git_branch, claude_version, first_message_at, last_message_at, message_count
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"sess-empty", emptyPath, "demo", "main", "1.0.0", 0, 0, 0)
	require.NoError(t, err)

	var buf bytes.Buffer
	err = SessionTranscript(database, &buf, "sess-empty")
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No messages found.")
}

func setupTranscriptDB(t *testing.T, withSubagent bool) (*db.DB, string, string, func()) {
	t.Helper()

	database, err := db.OpenMemory()
	require.NoError(t, err)

	tmpDir := t.TempDir()
	sessionID := "sess-transcript-1"
	mainPath := filepath.Join(tmpDir, sessionID+".jsonl")

	mainJSONL := strings.Join([]string{
		`{"sessionId":"sess-transcript-1","uuid":"m1","timestamp":"2026-03-02T14:00:00.000Z","type":"user","message":{"role":"user","content":[{"type":"text","text":"main user message"}]}}`,
		`{"sessionId":"sess-transcript-1","uuid":"m2","timestamp":"2026-03-02T14:00:03.000Z","type":"assistant","message":{"role":"assistant","model":"claude-haiku-4-5-20251001","content":[{"type":"thinking","thinking":"reasoning details"},{"type":"tool_use","id":"toolu_01","name":"Read","input":{"file_path":"/tmp/a.txt"}},{"type":"text","text":"assistant answer"}]}}`,
		"",
	}, "\n")
	require.NoError(t, os.WriteFile(mainPath, []byte(mainJSONL), 0644))

	mainMessages, err := parser.ParseFile(mainPath)
	require.NoError(t, err)
	require.NoError(t, database.IngestSession(
		parser.SessionFile{Path: mainPath, SessionID: sessionID},
		mainMessages,
	))

	if withSubagent {
		subDir := filepath.Join(tmpDir, sessionID, "subagents")
		require.NoError(t, os.MkdirAll(subDir, 0755))
		subPath := filepath.Join(subDir, "agent-1.jsonl")
		subJSONL := strings.Join([]string{
			`{"sessionId":"sess-transcript-1","uuid":"sa1","timestamp":"2026-03-02T13:59:59.000Z","type":"assistant","message":{"role":"assistant","model":"claude-sonnet-4-6-20250925","content":[{"type":"text","text":"subagent says hello"}]}}`,
			"",
		}, "\n")
		require.NoError(t, os.WriteFile(subPath, []byte(subJSONL), 0644))

		subMessages, err := parser.ParseFile(subPath)
		require.NoError(t, err)
		require.NoError(t, database.IngestSubagent(
			parser.SessionFile{Path: subPath, SessionID: sessionID, IsSubagent: true},
			subMessages,
		))
	}

	cleanup := func() {
		_ = database.Close()
	}
	return database, sessionID, mainPath, cleanup
}
