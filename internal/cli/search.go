package cli

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/cnu/claude-stats/internal/db"
	"github.com/cnu/claude-stats/internal/search"
	"github.com/spf13/cobra"
)

var (
	searchProject       string
	searchLimit         int
	searchFormat        string
	searchCaseSensitive bool
)

var searchCmd = &cobra.Command{
	Use:   "search <keyword>",
	Short: "Search session transcripts for a keyword",
	Long: "Search full Claude Code session transcripts (including subagent transcripts) " +
		"and print matching sessions so you can resume one with `claude --resume <session-id>`.",
	Args: cobra.ExactArgs(1),
	RunE: runSearch,
}

func init() {
	searchCmd.Flags().StringVar(&searchProject, "project", "", "Filter by project name")
	searchCmd.Flags().IntVar(&searchLimit, "limit", 20, "Max sessions to return")
	searchCmd.Flags().StringVar(&searchFormat, "format", "table", "Output format: table, json, csv")
	searchCmd.Flags().BoolVar(&searchCaseSensitive, "case-sensitive", false, "Match case exactly")

	rootCmd.AddCommand(searchCmd)
}

func runSearch(cmd *cobra.Command, args []string) error {
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close() //nolint:errcheck

	keyword := args[0]

	results, err := search.Run(database, search.Options{
		Keyword:       keyword,
		Project:       searchProject,
		CaseSensitive: searchCaseSensitive,
		Limit:         searchLimit,
	})
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}

	if len(results) == 0 {
		fmt.Printf("No sessions matched %q.\n", keyword)
		return nil
	}

	loc := getTimezone()
	result := &db.QueryResult{
		Columns: []string{"session_id", "project", "last_active", "role", "matches", "snippet"},
	}
	for _, r := range results {
		result.Rows = append(result.Rows, []string{
			r.SessionID,
			r.ProjectName,
			time.UnixMilli(r.LastMsgAt).In(loc).Format("2006-01-02"),
			r.SnippetRole,
			strconv.Itoa(r.MatchCount),
			r.Snippet,
		})
	}

	switch searchFormat {
	case "json":
		return formatJSON(os.Stdout, result)
	case "csv":
		return formatCSV(os.Stdout, result)
	default:
		if err := formatTable(os.Stdout, result); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "Resume a session with: claude --resume <session-id>")
		return nil
	}
}
