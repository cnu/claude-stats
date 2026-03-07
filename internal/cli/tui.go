package cli

import (
	"fmt"
	"log/slog"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cnu/claude-stats/internal/db"
	"github.com/cnu/claude-stats/internal/parser"
	"github.com/cnu/claude-stats/internal/tui"
	"github.com/spf13/cobra"
)

var skipIngest bool

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch interactive TUI dashboard",
	RunE:  runTUI,
}

func init() {
	tuiCmd.Flags().BoolVar(&skipIngest, "skip-ingest", false, "Skip auto-ingest on launch")
	rootCmd.AddCommand(tuiCmd)
}

func runTUI(cmd *cobra.Command, args []string) error {
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close() //nolint:errcheck

	if !skipIngest {
		quickIngest(database)
	}

	app := tui.NewApp(database)
	p := tea.NewProgram(app, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

// quickIngest runs a fast incremental ingest before launching TUI.
func quickIngest(database *db.DB) {
	files, err := parser.ScanDirectory(claudeDir)
	if err != nil {
		slog.Warn("failed to scan directory", "error", err)
		return
	}

	ingested := 0
	for _, sf := range files {
		needs, err := database.CheckFileState(sf.Path, sf.Size, sf.ModTime)
		if err != nil || !needs {
			continue
		}

		messages, err := parser.ParseFile(sf.Path)
		if err != nil || len(messages) == 0 {
			continue
		}

		if err := database.IngestSession(sf, messages); err != nil {
			continue
		}

		_ = database.UpdateIngestMeta(sf.Path, sf.Size, sf.ModTime, len(messages))
		ingested++
	}

	if ingested > 0 {
		tz := getTimezone()
		_ = database.RebuildDailyStats(tz)
		fmt.Printf("Ingested %d new sessions\n", ingested)
	}
}
