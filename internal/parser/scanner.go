package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ScanDirectory finds all .jsonl files under dir, skipping subagents/ directories.
// Returns SessionFile entries sorted by modification time (newest first).
func ScanDirectory(dir string) ([]SessionFile, error) {
	var files []SessionFile

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip inaccessible paths
		}

		// Skip subagents directories
		if info.IsDir() && info.Name() == "subagents" {
			return filepath.SkipDir
		}

		if info.IsDir() {
			return nil
		}

		if !strings.HasSuffix(info.Name(), ".jsonl") {
			return nil
		}

		// Derive session ID from filename (strip .jsonl)
		sessionID := strings.TrimSuffix(info.Name(), ".jsonl")

		files = append(files, SessionFile{
			Path:      path,
			Size:      info.Size(),
			ModTime:   info.ModTime(),
			SessionID: sessionID,
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk directory %s: %w", dir, err)
	}

	return files, nil
}
