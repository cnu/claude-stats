package cli

import (
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

func TestExportSessionCmd_Args(t *testing.T) {
	err := exportSessionCmd.Args(exportSessionCmd, []string{})
	require.Error(t, err)

	err = exportSessionCmd.Args(exportSessionCmd, []string{"sess-1"})
	require.NoError(t, err)
}

func TestRunExportSession_WritesOutputFile(t *testing.T) {
	dbFile, sessionID := setupExportSessionData(t)
	outFile := filepath.Join(t.TempDir(), "session.md")

	oldDBPath := dbPath
	oldExportOutput := exportOutput
	dbPath = dbFile
	exportOutput = outFile
	defer func() {
		dbPath = oldDBPath
		exportOutput = oldExportOutput
	}()

	err := runExportSession(nil, []string{sessionID})
	require.NoError(t, err)

	data, err := os.ReadFile(outFile)
	require.NoError(t, err)
	assert.Contains(t, string(data), "# Session Transcript: "+sessionID)
}

func TestRunExportSession_WritesStdout(t *testing.T) {
	dbFile, sessionID := setupExportSessionData(t)

	oldDBPath := dbPath
	oldExportOutput := exportOutput
	dbPath = dbFile
	exportOutput = "-"
	defer func() {
		dbPath = oldDBPath
		exportOutput = oldExportOutput
	}()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	err = runExportSession(nil, []string{sessionID})
	require.NoError(t, err)

	require.NoError(t, w.Close())
	os.Stdout = oldStdout

	out, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Contains(t, string(out), "# Session Transcript: "+sessionID)
}

func setupExportSessionData(t *testing.T) (string, string) {
	t.Helper()

	tmpDir := t.TempDir()
	dbFile := filepath.Join(tmpDir, "stats.db")
	sourceFile := filepath.Join(tmpDir, "sess-cli.jsonl")
	sessionID := "sess-cli"

	lines := []string{
		`{"sessionId":"sess-cli","uuid":"c1","timestamp":"2026-03-02T14:00:00.000Z","type":"user","message":{"role":"user","content":[{"type":"text","text":"hello"}]}}`,
		`{"sessionId":"sess-cli","uuid":"c2","timestamp":"2026-03-02T14:00:01.000Z","type":"assistant","message":{"role":"assistant","model":"claude-haiku-4-5-20251001","content":[{"type":"text","text":"hi"}]}}`,
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
