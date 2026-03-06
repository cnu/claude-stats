package cli

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/cnu/claude-stats/internal/db"
	"github.com/cnu/claude-stats/internal/parser"
	"github.com/spf13/cobra"
)

var (
	ingestFull    bool
	ingestProject string
	ingestSince   string
	ingestDryRun  bool
)

var ingestCmd = &cobra.Command{
	Use:   "ingest",
	Short: "Parse JSONL files and populate SQLite",
	Long:  "Scan ~/.claude/projects/ and ingest new/modified JSONL files into the SQLite database.",
	RunE:  runIngest,
}

func init() {
	ingestCmd.Flags().BoolVar(&ingestFull, "full", false, "Force full re-ingest (ignore cache)")
	ingestCmd.Flags().StringVar(&ingestProject, "project", "", "Only ingest files for a specific project")
	ingestCmd.Flags().StringVar(&ingestSince, "since", "", "Only ingest files modified after this date (YYYY-MM-DD)")
	ingestCmd.Flags().BoolVar(&ingestDryRun, "dry-run", false, "Show what would be ingested without writing")

	rootCmd.AddCommand(ingestCmd)
}

func runIngest(cmd *cobra.Command, args []string) error {
	start := time.Now()

	// Open database
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close() //nolint:errcheck

	// Scan for JSONL files
	files, err := parser.ScanDirectory(claudeDir)
	if err != nil {
		return fmt.Errorf("scan directory: %w", err)
	}

	slog.Debug("found JSONL files", "count", len(files))

	// Parse since date if provided
	var sinceTime time.Time
	if ingestSince != "" {
		sinceTime, err = time.Parse("2006-01-02", ingestSince)
		if err != nil {
			return fmt.Errorf("parse --since date: %w", err)
		}
	}

	var (
		ingestedSessions int
		ingestedMessages int
		skippedFiles     int
	)

	for _, sf := range files {
		// Filter by project if specified
		if ingestProject != "" {
			messages, err := parser.ParseFile(sf.Path)
			if err != nil {
				slog.Warn("failed to parse file for project check", "path", sf.Path, "error", err)
				continue
			}
			if len(messages) == 0 {
				continue
			}
			// Check if any message CWD contains the project name
			found := false
			for _, m := range messages {
				if m.CWD != "" && contains(m.CWD, ingestProject) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Filter by since date
		if !sinceTime.IsZero() && sf.ModTime.Before(sinceTime) {
			continue
		}

		// Check if file needs ingestion
		if !ingestFull {
			needs, err := database.CheckFileState(sf.Path, sf.Size, sf.ModTime)
			if err != nil {
				slog.Warn("failed to check file state", "path", sf.Path, "error", err)
				continue
			}
			if !needs {
				skippedFiles++
				continue
			}
		}

		if ingestDryRun {
			fmt.Printf("Would ingest: %s (%d bytes)\n", sf.Path, sf.Size)
			ingestedSessions++
			continue
		}

		// Parse file
		messages, err := parser.ParseFile(sf.Path)
		if err != nil {
			slog.Warn("failed to parse file", "path", sf.Path, "error", err)
			continue
		}

		if len(messages) == 0 {
			continue
		}

		// Ingest into database
		if err := database.IngestSession(sf, messages); err != nil {
			slog.Warn("failed to ingest session", "path", sf.Path, "error", err)
			continue
		}

		// Update ingest metadata
		if err := database.UpdateIngestMeta(sf.Path, sf.Size, sf.ModTime, len(messages)); err != nil {
			slog.Warn("failed to update ingest meta", "path", sf.Path, "error", err)
		}

		ingestedSessions++
		ingestedMessages += len(messages)
	}

	// Rebuild daily stats
	if !ingestDryRun && ingestedSessions > 0 {
		tz := getTimezone()
		if err := database.RebuildDailyStats(tz); err != nil {
			slog.Warn("failed to rebuild daily stats", "error", err)
		}
	}

	elapsed := time.Since(start)

	if ingestDryRun {
		fmt.Printf("Dry run: would ingest %d sessions\n", ingestedSessions)
	} else {
		fmt.Printf("Ingested %d sessions (%d messages) in %.1fs",
			ingestedSessions, ingestedMessages, elapsed.Seconds())
		if skippedFiles > 0 {
			fmt.Printf(" [%d unchanged files skipped]", skippedFiles)
		}
		fmt.Println()
	}

	return nil
}

func getTimezone() *time.Location {
	if timezone == "Local" || timezone == "" {
		return time.Local
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		slog.Warn("invalid timezone, using local", "timezone", timezone, "error", err)
		return time.Local
	}
	return loc
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
