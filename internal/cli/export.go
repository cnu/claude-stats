package cli

import (
	"fmt"
	"os"

	"github.com/cnu/claude-stats/internal/db"
	"github.com/cnu/claude-stats/internal/export"
	"github.com/spf13/cobra"
)

var (
	exportOutput string
	exportFormat string
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export data in various formats",
	Long:  "Export sessions, cost summaries, or database dumps.",
}

var exportSessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "Export all sessions as CSV or JSON",
	RunE:  runExportSessions,
}

var exportCostSummaryCmd = &cobra.Command{
	Use:   "cost-summary",
	Short: "Export cost summary report",
	RunE:  runExportCostSummary,
}

var exportDumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "Copy the SQLite database to a file",
	RunE:  runExportDump,
}

func init() {
	exportSessionsCmd.Flags().StringVar(&exportFormat, "format", "csv", "Output format: csv, json")
	exportSessionsCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Output file (default: stdout)")

	exportCostSummaryCmd.Flags().StringVar(&exportFormat, "format", "markdown", "Output format: markdown, json")
	exportCostSummaryCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Output file (default: stdout)")

	exportDumpCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Output file (required)")
	exportDumpCmd.MarkFlagRequired("output") //nolint:errcheck

	exportCmd.AddCommand(exportSessionsCmd)
	exportCmd.AddCommand(exportCostSummaryCmd)
	exportCmd.AddCommand(exportDumpCmd)
	rootCmd.AddCommand(exportCmd)
}

func runExportSessions(cmd *cobra.Command, args []string) error {
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close() //nolint:errcheck

	w, cleanup, err := getOutputWriter(exportOutput)
	if err != nil {
		return err
	}
	defer cleanup()

	return export.Sessions(database, w, exportFormat)
}

func runExportCostSummary(cmd *cobra.Command, args []string) error {
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close() //nolint:errcheck

	w, cleanup, err := getOutputWriter(exportOutput)
	if err != nil {
		return err
	}
	defer cleanup()

	return export.CostSummary(database, w, exportFormat)
}

func runExportDump(cmd *cobra.Command, args []string) error {
	if err := export.Dump(dbPath, exportOutput); err != nil {
		return err
	}
	fmt.Printf("Database exported to %s\n", exportOutput)
	return nil
}

// getOutputWriter returns an io.Writer for the given path, or os.Stdout if empty.
func getOutputWriter(path string) (*os.File, func(), error) {
	if path == "" || path == "-" {
		return os.Stdout, func() {}, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, fmt.Errorf("create output file: %w", err)
	}
	return f, func() { f.Close() }, nil //nolint:errcheck
}
