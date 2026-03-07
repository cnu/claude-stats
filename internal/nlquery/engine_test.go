package nlquery

import (
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

	day1 := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)

	require.NoError(t, database.IngestSession(
		parser.SessionFile{Path: "/tmp/s1.jsonl", SessionID: "sess-1"},
		[]parser.ParsedMessage{
			{SessionID: "sess-1", UUID: "s1-m1", Timestamp: day1, Role: "user",
				CWD: "/home/user/Projects/webapp", ContentPreview: "help me"},
			{SessionID: "sess-1", UUID: "s1-m2", Timestamp: day1.Add(30 * time.Second),
				Role: "assistant", Model: "claude-sonnet-4-6-20250925",
				Usage: parser.UsageStats{InputTokens: 1000, OutputTokens: 200},
				ToolUses: []parser.ToolUse{
					{ID: "t1", Name: "Read", InputPreview: "test"},
				}},
		},
	))

	require.NoError(t, database.RebuildDailyStats(time.UTC))
	return database
}

func TestPatternMatching(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close() //nolint:errcheck
	engine := New(database)

	tests := []struct {
		name    string
		input   string
		wantSQL bool // true = should match a pattern
	}{
		{"total cost", "total cost", true},
		{"total cost variant", "what's the total cost", true},
		{"total cost variant2", "what is my total cost", true},
		{"cost today", "cost today", true},
		{"cost yesterday", "cost yesterday", true},
		{"cost this week", "cost this week", true},
		{"cost current week", "cost current week", true},
		{"cost this month", "cost this month", true},
		{"cost last 7 days", "cost last 7 days", true},
		{"cost past 30 days", "cost past 30 days", true},
		{"most expensive session", "most expensive session", true},
		{"how many sessions", "how many sessions", true},
		{"how many messages", "how many messages", true},
		{"top 5 models", "top 5 models", true},
		{"top models", "top models", true},
		{"cost by project", "cost by project", true},
		{"cost per project", "cost per project", true},
		{"sessions today", "sessions today", true},
		{"sessions this week", "sessions this week", true},
		{"average cost", "average session cost", true},
		{"cost per session", "cost per session", true},
		{"longest session", "longest session", true},
		{"top tools", "top tools", true},
		{"most used tools", "most used tools", true},
		{"cost for project", "cost for webapp", true},
		{"cost of project", "cost of webapp", true},
		{"busiest day", "busiest day", true},
		{"daily cost", "daily cost", true},
		{"unknown query", "what color is the sky", false},
		{"gibberish", "asdfghjkl", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, sql, err := engine.Query(tt.input)
			if tt.wantSQL {
				require.NoError(t, err, "input: %q", tt.input)
				assert.NotEmpty(t, sql, "input: %q", tt.input)
				assert.NotNil(t, result, "input: %q", tt.input)
			} else {
				assert.Error(t, err, "input: %q should not match", tt.input)
				assert.Empty(t, sql)
				assert.Nil(t, result)
			}
		})
	}
}

func TestParameterExtraction(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close() //nolint:errcheck
	engine := New(database)

	// "top 5 models" should extract limit=5
	_, sql, err := engine.Query("top 5 models")
	require.NoError(t, err)
	assert.Contains(t, sql, "LIMIT 5")

	// "top 3 models" should extract limit=3
	_, sql, err = engine.Query("top 3 models")
	require.NoError(t, err)
	assert.Contains(t, sql, "LIMIT 3")

	// "cost last 7 days" should extract 7
	_, sql, err = engine.Query("cost last 7 days")
	require.NoError(t, err)
	assert.Contains(t, sql, "-7 days")
}

func TestEndToEnd(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close() //nolint:errcheck
	engine := New(database)

	// "how many sessions" should return 1
	result, _, err := engine.Query("how many sessions")
	require.NoError(t, err)
	require.Len(t, result.Rows, 1)
	assert.Equal(t, "1", result.Rows[0][0])

	// "how many messages" should return 2
	result, _, err = engine.Query("how many messages")
	require.NoError(t, err)
	require.Len(t, result.Rows, 1)
	assert.Equal(t, "2", result.Rows[0][0])

	// "top tools" should include Read
	result, _, err = engine.Query("top tools")
	require.NoError(t, err)
	require.NotEmpty(t, result.Rows)
	assert.Equal(t, "Read", result.Rows[0][0])
}

func TestNoMatchError(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close() //nolint:errcheck
	engine := New(database)

	_, _, err := engine.Query("what is the meaning of life")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown query")
	assert.Contains(t, err.Error(), "total cost")
}

func TestExamples(t *testing.T) {
	examples := Examples()
	assert.NotEmpty(t, examples)
	assert.Greater(t, len(examples), 5)
}
