package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/cnu/claude-stats/internal/db"
	"github.com/cnu/claude-stats/internal/export"
)

var nonFilenameChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func exportSessionToCWD(database *db.DB, sessionID string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve cwd: %w", err)
	}

	safeID := nonFilenameChars.ReplaceAllString(sessionID, "_")
	if safeID == "" {
		safeID = "session"
	}
	outputPath := filepath.Join(cwd, fmt.Sprintf("session-%s.md", safeID))

	f, err := os.Create(outputPath)
	if err != nil {
		return "", fmt.Errorf("create export file: %w", err)
	}

	if err := export.SessionTranscript(database, f, sessionID); err != nil {
		_ = f.Close()
		_ = os.Remove(outputPath)
		return "", err
	}

	if err := f.Close(); err != nil {
		return "", fmt.Errorf("close export file: %w", err)
	}

	return outputPath, nil
}
