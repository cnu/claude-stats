package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CollectSessionSourcePaths returns the primary transcript path plus any
// subagent transcripts, sorted. Subagent files live in a directory named
// after the session next to its transcript: <session-uuid>/subagents/*.jsonl
// for a primary <session-uuid>.jsonl. It fails if the primary path does not
// exist; a missing subagents directory is not an error.
func CollectSessionSourcePaths(primaryPath string) ([]string, error) {
	if _, err := os.Stat(primaryPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session source file not found: %s", primaryPath)
		}
		return nil, fmt.Errorf("stat source file %s: %w", primaryPath, err)
	}

	paths := []string{primaryPath}
	subagentsDir := filepath.Join(strings.TrimSuffix(primaryPath, ".jsonl"), "subagents")
	entries, err := os.ReadDir(subagentsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return paths, nil
		}
		return nil, fmt.Errorf("read subagents directory %s: %w", subagentsDir, err)
	}

	var subagentPaths []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		subagentPaths = append(subagentPaths, filepath.Join(subagentsDir, entry.Name()))
	}
	sort.Strings(subagentPaths)

	return append(paths, subagentPaths...), nil
}
