package cli

import (
	"fmt"

	"github.com/cnu/claude-stats/internal/db"
	"github.com/spf13/cobra"
)

var recalcCmd = &cobra.Command{
	Use:   "recalc",
	Short: "Recalculate costs with the latest pricing",
	Long: "Reprice all messages from the token counts already stored in the database, " +
		"using the latest model pricing. The JSONL files are not needed.",
	Args: cobra.NoArgs,
	RunE: runRecalc,
}

func init() {
	rootCmd.AddCommand(recalcCmd)
}

func runRecalc(cmd *cobra.Command, args []string) error {
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close() //nolint:errcheck

	loadPricing(cmd.Context())

	res, err := database.RecalculateCosts()
	if err != nil {
		return fmt.Errorf("recalculate costs: %w", err)
	}
	if err := database.RebuildDailyStats(getTimezone()); err != nil {
		return fmt.Errorf("rebuild daily stats: %w", err)
	}

	fmt.Printf("Repriced %d messages (%d changed). Total cost: $%.2f -> $%.2f\n",
		res.Messages, res.Changed, res.OldTotal, res.NewTotal)
	return nil
}
