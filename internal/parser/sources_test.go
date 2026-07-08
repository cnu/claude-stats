package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestCollectSessionSourcePaths_PrimaryOnly(t *testing.T) {
	tmpDir := t.TempDir()
	primary := filepath.Join(tmpDir, "session.jsonl")
	writeFile(t, primary, "{}\n")

	paths, err := CollectSessionSourcePaths(primary)
	require.NoError(t, err)
	assert.Equal(t, []string{primary}, paths)
}

func TestCollectSessionSourcePaths_WithSubagents(t *testing.T) {
	tmpDir := t.TempDir()
	primary := filepath.Join(tmpDir, "session.jsonl")
	writeFile(t, primary, "{}\n")

	// Subagents live in a directory named after the session:
	// <dir>/session/subagents/*.jsonl for <dir>/session.jsonl.
	subDir := filepath.Join(tmpDir, "session", "subagents")
	writeFile(t, filepath.Join(subDir, "agent-b.jsonl"), "{}\n")
	writeFile(t, filepath.Join(subDir, "agent-a.jsonl"), "{}\n")
	writeFile(t, filepath.Join(subDir, "notes.txt"), "ignore me")
	require.NoError(t, os.MkdirAll(filepath.Join(subDir, "nested.jsonl"), 0o755))

	paths, err := CollectSessionSourcePaths(primary)
	require.NoError(t, err)
	assert.Equal(t, []string{
		primary,
		filepath.Join(subDir, "agent-a.jsonl"),
		filepath.Join(subDir, "agent-b.jsonl"),
	}, paths)
}

func TestCollectSessionSourcePaths_MissingPrimary(t *testing.T) {
	_, err := CollectSessionSourcePaths(filepath.Join(t.TempDir(), "missing.jsonl"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session source file not found")
}
