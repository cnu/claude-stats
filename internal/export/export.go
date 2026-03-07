package export

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/cnu/claude-stats/internal/db"
)

// Sessions exports all sessions as CSV or JSON.
func Sessions(database *db.DB, w io.Writer, format string) error {
	sessions, err := database.GetSessionList("date", 0)
	if err != nil {
		return fmt.Errorf("get sessions: %w", err)
	}

	switch format {
	case "json":
		return sessionsJSON(w, sessions)
	default:
		return sessionsCSV(w, sessions)
	}
}

func sessionsCSV(w io.Writer, sessions []db.SessionListEntry) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"session_id", "project", "started_at", "messages", "cost_usd", "duration_s"}); err != nil {
		return err
	}
	for _, s := range sessions {
		started := ""
		if s.FirstMsgAt > 0 {
			started = time.UnixMilli(s.FirstMsgAt).UTC().Format(time.RFC3339)
		}
		if err := cw.Write([]string{
			s.SessionID,
			s.ProjectName,
			started,
			fmt.Sprintf("%d", s.MessageCount),
			fmt.Sprintf("%.4f", s.CostUSD),
			fmt.Sprintf("%.0f", float64(s.DurationMs)/1000),
		}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func sessionsJSON(w io.Writer, sessions []db.SessionListEntry) error {
	type sessionRow struct {
		SessionID  string  `json:"session_id"`
		Project    string  `json:"project"`
		StartedAt  string  `json:"started_at"`
		Messages   int     `json:"messages"`
		CostUSD    float64 `json:"cost_usd"`
		DurationS  float64 `json:"duration_s"`
	}

	rows := make([]sessionRow, 0, len(sessions))
	for _, s := range sessions {
		started := ""
		if s.FirstMsgAt > 0 {
			started = time.UnixMilli(s.FirstMsgAt).UTC().Format(time.RFC3339)
		}
		rows = append(rows, sessionRow{
			SessionID: s.SessionID,
			Project:   s.ProjectName,
			StartedAt: started,
			Messages:  s.MessageCount,
			CostUSD:   s.CostUSD,
			DurationS: float64(s.DurationMs) / 1000,
		})
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}

// CostSummary exports a cost summary report as Markdown or JSON.
func CostSummary(database *db.DB, w io.Writer, format string) error {
	summary, err := database.GetDashboardSummary()
	if err != nil {
		return fmt.Errorf("get summary: %w", err)
	}

	monthly, err := database.GetMonthlyCosts(12)
	if err != nil {
		return fmt.Errorf("get monthly costs: %w", err)
	}

	models, err := database.GetModelCostBreakdown()
	if err != nil {
		return fmt.Errorf("get model breakdown: %w", err)
	}

	projects, err := database.GetCostByProject()
	if err != nil {
		return fmt.Errorf("get project costs: %w", err)
	}

	switch format {
	case "json":
		return costSummaryJSON(w, summary, monthly, models, projects)
	default:
		return costSummaryMarkdown(w, summary, monthly, models, projects)
	}
}

func costSummaryMarkdown(w io.Writer, summary *db.DashboardSummary, monthly []db.MonthlyCostEntry, models []db.ModelCostBreakdown, projects []db.ProjectCostEntry) error { //nolint:cyclop
	p := func(format string, args ...any) {
		_, _ = fmt.Fprintf(w, format, args...)
	}

	p("# Claude Usage Cost Summary\n\n")
	p("Generated: %s\n\n", time.Now().UTC().Format("2006-01-02 15:04 UTC"))

	// Overview
	p("## Overview\n\n")
	p("| Metric | Value |\n")
	p("|--------|-------|\n")
	p("| Total Sessions | %d |\n", summary.TotalSessions)
	p("| Total Messages | %d |\n", summary.TotalMessages)
	p("| Total Cost | $%.2f |\n", summary.TotalCost)
	p("| Avg Daily Cost | $%.2f |\n", summary.AvgDailyCost)
	p("| Input Tokens | %d |\n", summary.TotalInputTokens)
	p("| Output Tokens | %d |\n", summary.TotalOutputTokens)
	p("| Most Active Project | %s |\n", summary.MostActiveProject)
	p("| Primary Model | %s |\n\n", summary.PrimaryModel)

	// Monthly Trends
	if len(monthly) > 0 {
		p("## Monthly Costs\n\n")
		p("| Month | Cost | Sessions | Messages |\n")
		p("|-------|------|----------|----------|\n")
		for _, m := range monthly {
			p("| %s | $%.2f | %d | %d |\n", m.Month, m.Cost, m.Sessions, m.Messages)
		}
		p("\n")
	}

	// Cost by Model
	if len(models) > 0 {
		p("## Cost by Model\n\n")
		p("| Model | Cost | Messages |\n")
		p("|-------|------|----------|\n")
		for _, m := range models {
			p("| %s | $%.2f | %d |\n", m.Model, m.Cost, m.MessageCount)
		}
		p("\n")
	}

	// Cost by Project
	if len(projects) > 0 {
		p("## Cost by Project\n\n")
		p("| Project | Cost | Sessions |\n")
		p("|---------|------|----------|\n")
		for _, pr := range projects {
			p("| %s | $%.2f | %d |\n", pr.ProjectName, pr.Cost, pr.SessionCount)
		}
		p("\n")
	}

	return nil
}

func costSummaryJSON(w io.Writer, summary *db.DashboardSummary, monthly []db.MonthlyCostEntry, models []db.ModelCostBreakdown, projects []db.ProjectCostEntry) error {
	report := struct {
		GeneratedAt string                 `json:"generated_at"`
		Summary     *db.DashboardSummary   `json:"summary"`
		Monthly     []db.MonthlyCostEntry  `json:"monthly_costs"`
		Models      []db.ModelCostBreakdown `json:"cost_by_model"`
		Projects    []db.ProjectCostEntry  `json:"cost_by_project"`
	}{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Summary:     summary,
		Monthly:     monthly,
		Models:      models,
		Projects:    projects,
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// Dump copies the SQLite database file to the specified output path.
func Dump(srcDBPath, outputPath string) error {
	src, err := os.Open(srcDBPath)
	if err != nil {
		return fmt.Errorf("open source db: %w", err)
	}
	defer src.Close() //nolint:errcheck

	dst, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer dst.Close() //nolint:errcheck

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copy database: %w", err)
	}

	return dst.Close()
}
