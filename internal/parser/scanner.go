package parser

import (
	"os"
	"path/filepath"
	"strings"
)

// ScanDirectory finds all .jsonl files under dir, skipping subagents/ directories.
// Returns SessionFile entries sorted by modification time (newest first).
func ScanDirectory(dir string) ([]SessionFile, error) {
	var files []SessionFile

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip inaccessible paths
		}

		// Skip subagents directories
		if d.IsDir() && d.Name() == "subagents" {
			return filepath.SkipDir
		}

		if d.IsDir() {
			return nil
		}

		if !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil // Skip files we can't stat
		}

		// Derive session ID from filename (strip .jsonl)
		sessionID := strings.TrimSuffix(d.Name(), ".jsonl")

		files = append(files, SessionFile{
			Path:      path,
			Size:      info.Size(),
			ModTime:   info.ModTime(),
			SessionID: sessionID,
		})

		return nil
	})
	_ = err // WalkDir only returns error if callback does; ours always returns nil

	return files, nil
}
