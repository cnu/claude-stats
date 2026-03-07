package cli

import (
	"fmt"
	"os"

	"github.com/cnu/claude-stats/internal/db"
	"github.com/cnu/claude-stats/internal/nlquery"
	"github.com/spf13/cobra"
)

var (
	querySQL    bool
	queryFormat string
	queryLimit  int
)

var queryCmd = &cobra.Command{
	Use:   "query [question]",
	Short: "Run a one-shot query",
	Long:  "Run a natural language or SQL query against the usage database.",
	Args:  cobra.ExactArgs(1),
	RunE:  runQuery,
}

func init() {
	queryCmd.Flags().BoolVar(&querySQL, "sql", false, "Interpret input as raw SQL")
	queryCmd.Flags().StringVar(&queryFormat, "format", "table", "Output format: table, json, csv")
	queryCmd.Flags().IntVar(&queryLimit, "limit", 20, "Max rows to return")

	rootCmd.AddCommand(queryCmd)
}

func runQuery(cmd *cobra.Command, args []string) error {
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close() //nolint:errcheck

	query := args[0]

	var result *db.QueryResult

	if !querySQL {
		engine := nlquery.New(database)
		var sql string
		result, sql, err = engine.Query(query)
		if err != nil {
			return fmt.Errorf("query: %w", err)
		}
		if verbose {
			fmt.Fprintf(os.Stderr, "SQL: %s\n\n", sql)
		}
	} else {
		result, err = database.ExecuteQuery(query, queryLimit)
		if err != nil {
			return fmt.Errorf("execute query: %w", err)
		}
	}

	switch queryFormat {
	case "json":
		return formatJSON(os.Stdout, result)
	case "csv":
		return formatCSV(os.Stdout, result)
	default:
		return formatTable(os.Stdout, result)
	}
}
