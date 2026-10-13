package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/cnu/claude-stats/internal/pricing"
	"github.com/spf13/cobra"
)

// Version and BuildDate are set via ldflags at build time.
var (
	Version   = "dev"
	BuildDate = "unknown"
)

var (
	dbPath    string
	claudeDir string
	verbose   bool
	noColor   bool
	timezone  string
	offline   bool
)

var rootCmd = &cobra.Command{
	Use:   "claude-stats",
	Short: "Analytics for Claude Code usage",
	Long:  "Parse Claude Code JSONL session files into SQLite and explore usage statistics, costs, and session analytics.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if verbose {
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
		}
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Default to TUI when no subcommand given
		return runTUI(cmd, args)
	},
}

func init() {
	home, _ := os.UserHomeDir()

	defaultDB := filepath.Join(home, ".claude-stats", "claude-stats.db")
	defaultClaudeDir := filepath.Join(home, ".claude", "projects")
	defaultTimezone := "Local"

	// Allow env vars to override defaults
	if v := os.Getenv("CLAUDE_STATS_DB"); v != "" {
		defaultDB = v
	}
	if v := os.Getenv("CLAUDE_STATS_CLAUDE_DIR"); v != "" {
		defaultClaudeDir = v
	}
	if v := os.Getenv("CLAUDE_STATS_TIMEZONE"); v != "" {
		defaultTimezone = v
	}

	rootCmd.PersistentFlags().StringVar(&dbPath, "db", defaultDB, "SQLite database path")
	rootCmd.PersistentFlags().StringVar(&claudeDir, "claude-dir", defaultClaudeDir, "Claude data directory")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", os.Getenv("CLAUDE_STATS_VERBOSE") != "", "Enable debug logging")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable color output")
	rootCmd.PersistentFlags().StringVar(&timezone, "timezone", defaultTimezone, "Timezone for date grouping")
	rootCmd.PersistentFlags().BoolVar(&offline, "offline", os.Getenv("CLAUDE_STATS_OFFLINE") != "", "Do not fetch the latest model pricing; use cached or built-in prices")
}

// loadPricing fetches the latest model pricing (cached for 24h next to the
// database) so that new models are priced correctly during ingest.
func loadPricing(ctx context.Context) {
	src := pricing.Load(ctx, pricing.LoadOptions{
		CachePath: filepath.Join(filepath.Dir(dbPath), "pricing.json"),
		Offline:   offline,
	})
	slog.Debug("pricing loaded", "source", src)
}

// Execute runs the root command.
func Execute() error {
	// Respect NO_COLOR env var
	if os.Getenv("NO_COLOR") != "" {
		noColor = true
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	return nil
}
