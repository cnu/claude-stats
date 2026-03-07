package parser

import (
	"os"
	"path/filepath"
	"strings"
)

// ScanDirectory finds all .jsonl files under dir, including subagent files.
// Subagent files get their SessionID from the parent directory name.
func ScanDirectory(dir string) ([]SessionFile, error) {
	var files []SessionFile

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip inaccessible paths
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

		// Check if this is a subagent file (path contains /subagents/)
		isSubagent := false
		sessionID := strings.TrimSuffix(d.Name(), ".jsonl")

		rel, relErr := filepath.Rel(dir, path)
		if relErr == nil {
			parts := strings.Split(rel, string(filepath.Separator))
			for i, part := range parts {
				if part == "subagents" && i >= 1 {
					isSubagent = true
					// Session ID is the directory before "subagents"
					sessionID = parts[i-1]
					break
				}
			}
		}

		files = append(files, SessionFile{
			Path:       path,
			Size:       info.Size(),
			ModTime:    info.ModTime(),
			SessionID:  sessionID,
			IsSubagent: isSubagent,
		})

		return nil
	})
	_ = err // WalkDir only returns error if callback does; ours always returns nil

	return files, nil
}
