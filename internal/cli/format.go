package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/cnu/claude-stats/internal/db"
)

// formatTable writes a QueryResult as an ASCII table to w.
func formatTable(w io.Writer, result *db.QueryResult) error {
	if len(result.Rows) == 0 {
		_, _ = fmt.Fprintln(w, "No results.")
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
	_, _ = fmt.Fprintln(w, header.String())
	_, _ = fmt.Fprintln(w, separator.String())

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
		_, _ = fmt.Fprintln(w, line.String())
	}

	_, _ = fmt.Fprintf(w, "\n(%d rows)\n", len(result.Rows))
	return nil
}

// formatJSON writes a QueryResult as a JSON array of objects to w.
func formatJSON(w io.Writer, result *db.QueryResult) error {
	var rows []map[string]string
	for _, row := range result.Rows {
		m := make(map[string]string)
		for i, col := range result.Columns {
			m[col] = row[i]
		}
		rows = append(rows, m)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}

// formatCSV writes a QueryResult as RFC 4180 CSV to w.
func formatCSV(w io.Writer, result *db.QueryResult) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(result.Columns); err != nil {
		return err
	}
	for _, row := range result.Rows {
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
