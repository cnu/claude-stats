package cli

import (
	"encoding/json"
	"io"
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

func TestSearchCmd_Args(t *testing.T) {
	err := searchCmd.Args(searchCmd, []string{})
	require.Error(t, err)

	err = searchCmd.Args(searchCmd, []string{"keyword"})
	require.NoError(t, err)
}

func TestRunSearch_TableOutput(t *testing.T) {
	dbFile, sessionID := setupSearchData(t)

	oldDBPath := dbPath
	dbPath = dbFile
	defer func() { dbPath = oldDBPath }()

	out := captureStdout(t, func() {
		require.NoError(t, runSearch(nil, []string{"flamingo"}))
	})

	assert.Contains(t, out, sessionID)
	assert.Contains(t, out, "flamingo")
}

func TestRunSearch_JSONOutput(t *testing.T) {
	dbFile, sessionID := setupSearchData(t)

	oldDBPath := dbPath
	oldFormat := searchFormat
	dbPath = dbFile
	searchFormat = "json"
	defer func() {
		dbPath = oldDBPath
		searchFormat = oldFormat
	}()

	out := captureStdout(t, func() {
		require.NoError(t, runSearch(nil, []string{"flamingo"}))
	})

	var rows []map[string]string
	require.NoError(t, json.Unmarshal([]byte(out), &rows))
	require.Len(t, rows, 1)
	assert.Equal(t, sessionID, rows[0]["session_id"])
}

func TestRunSearch_NoMatch(t *testing.T) {
	dbFile, _ := setupSearchData(t)

	oldDBPath := dbPath
	dbPath = dbFile
	defer func() { dbPath = oldDBPath }()

	out := captureStdout(t, func() {
		require.NoError(t, runSearch(nil, []string{"nonexistent-keyword"}))
	})

	assert.Contains(t, out, `No sessions matched "nonexistent-keyword".`)
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	fn()

	require.NoError(t, w.Close())
	os.Stdout = oldStdout
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(out)
}

func setupSearchData(t *testing.T) (string, string) {
	t.Helper()

	tmpDir := t.TempDir()
	dbFile := filepath.Join(tmpDir, "stats.db")
	sourceFile := filepath.Join(tmpDir, "sess-search.jsonl")
	sessionID := "sess-search"

	lines := []string{
		`{"sessionId":"sess-search","uuid":"s1","timestamp":"2026-07-01T10:00:00.000Z","type":"user","cwd":"/home/u/Projects/webapp","message":{"role":"user","content":[{"type":"text","text":"tell me about the flamingo migration"}]}}`,
		`{"sessionId":"sess-search","uuid":"s2","timestamp":"2026-07-01T10:00:01.000Z","type":"assistant","message":{"role":"assistant","model":"claude-sonnet-4-6-20250925","content":[{"type":"text","text":"sure"}]}}`,
	}
	require.NoError(t, os.WriteFile(sourceFile, []byte(strings.Join(lines, "\n")+"\n"), 0644))

	database, err := db.Open(dbFile)
	require.NoError(t, err)
	defer database.Close() //nolint:errcheck

	msgs, err := parser.ParseFile(sourceFile)
	require.NoError(t, err)

	require.NoError(t, database.IngestSession(
		parser.SessionFile{
			Path:      sourceFile,
			SessionID: sessionID,
			Size:      int64(len(lines)),
			ModTime:   time.Now(),
		},
		msgs,
	))

	return dbFile, sessionID
}
