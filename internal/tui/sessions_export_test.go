package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cnu/claude-stats/internal/db"
	"github.com/cnu/claude-stats/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExportSessionToCWD(t *testing.T) {
	database, sessionID := setupSessionExportDB(t)
	defer database.Close() //nolint:errcheck

	origWD, err := os.Getwd()
	require.NoError(t, err)

	tmpDir := t.TempDir()
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() {
		_ = os.Chdir(origWD)
	})

	outPath, err := exportSessionToCWD(database, sessionID)
	require.NoError(t, err)
	assert.Equal(t, "session-sess_export_test.md", filepath.Base(outPath))
	_, err = os.Stat(outPath)
	require.NoError(t, err)

	data, err := os.ReadFile(outPath)
	require.NoError(t, err)
	out := string(data)
	assert.Contains(t, out, "# Session Transcript: "+sessionID)
	assert.Contains(t, out, "hello from session export test")
}

func TestExportSessionToCWD_NotFound(t *testing.T) {
	database, err := db.OpenMemory()
	require.NoError(t, err)
	defer database.Close() //nolint:errcheck

	origWD, err := os.Getwd()
	require.NoError(t, err)

	tmpDir := t.TempDir()
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() {
		_ = os.Chdir(origWD)
	})

	_, err = exportSessionToCWD(database, "missing-session")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")
}

func setupSessionExportDB(t *testing.T) (*db.DB, string) {
	t.Helper()

	database, err := db.OpenMemory()
	require.NoError(t, err)

	sessionID := "sess/export/test"
	tmpDir := t.TempDir()
	sourcePath := filepath.Join(tmpDir, "session.jsonl")
	lines := []string{
		`{"sessionId":"sess/export/test","uuid":"u1","timestamp":"2026-03-02T14:00:00.000Z","type":"user","message":{"role":"user","content":[{"type":"text","text":"hello from session export test"}]}}`,
		`{"sessionId":"sess/export/test","uuid":"u2","timestamp":"2026-03-02T14:00:01.000Z","type":"assistant","message":{"role":"assistant","model":"claude-haiku-4-5-20251001","content":[{"type":"text","text":"ack"}]}}`,
	}
	require.NoError(t, os.WriteFile(sourcePath, []byte(strings.Join(lines, "\n")+"\n"), 0644))

	msgs, err := parser.ParseFile(sourcePath)
	require.NoError(t, err)

	require.NoError(t, database.IngestSession(
		parser.SessionFile{Path: sourcePath, SessionID: sessionID},
		msgs,
	))

	return database, sessionID
}
