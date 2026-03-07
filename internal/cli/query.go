package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"

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
		return outputJSON(result)
	case "csv":
		return outputCSV(result)
	default:
		return outputTable(result)
	}
}

func outputTable(result *db.QueryResult) error {
	if len(result.Rows) == 0 {
		fmt.Println("No results.")
		return nil
	}

	// Calculate column widths
	widths := make([]int, len(result.Columns))
	for i, col := range result.Columns {
		widths[i] = len(col)
	}
	for _, row := range result.Rows {
		for i, val := range row {
			if len(val) > widths[i] {
				widths[i] = len(val)
			}
		}
	}

	// Cap column widths at 50
	for i := range widths {
		if widths[i] > 50 {
			widths[i] = 50
		}
	}

	// Print header
	var header strings.Builder
	var separator strings.Builder
	for i, col := range result.Columns {
		if i > 0 {
			header.WriteString(" | ")
			separator.WriteString("-+-")
		}
		header.WriteString(padRight(col, widths[i]))
		separator.WriteString(strings.Repeat("-", widths[i]))
	}
	fmt.Println(header.String())
	fmt.Println(separator.String())

	// Print rows
	for _, row := range result.Rows {
		var line strings.Builder
		for i, val := range row {
			if i > 0 {
				line.WriteString(" | ")
			}
			if len(val) > widths[i] {
				val = val[:widths[i]-3] + "..."
			}
			line.WriteString(padRight(val, widths[i]))
		}
		fmt.Println(line.String())
	}

	fmt.Printf("\n(%d rows)\n", len(result.Rows))
	return nil
}

func outputJSON(result *db.QueryResult) error {
	var rows []map[string]string
	for _, row := range result.Rows {
		m := make(map[string]string)
		for i, col := range result.Columns {
			m[col] = row[i]
		}
		rows = append(rows, m)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}

func outputCSV(result *db.QueryResult) error {
	w := csv.NewWriter(os.Stdout)
	if err := w.Write(result.Columns); err != nil {
		return err
	}
	for _, row := range result.Rows {
		if err := w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
