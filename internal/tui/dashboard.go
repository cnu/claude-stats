package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cnu/claude-stats/internal/db"
)

// DashboardModel renders the dashboard tab.
type DashboardModel struct {
	summary *db.DashboardSummary
	daily   []db.DailyCostEntry
	models  []db.ModelCostBreakdown
	loading bool
	width   int
	height  int
}

// NewDashboard creates a new dashboard model.
func NewDashboard() DashboardModel {
	return DashboardModel{loading: true}
}

// Init returns nil.
func (m DashboardModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m DashboardModel) Update(msg tea.Msg) (DashboardModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case DataLoadedMsg:
		m.loading = false
		if msg.Err == nil {
			m.summary = msg.Summary
			m.daily = msg.Daily
			m.models = msg.Models
		}
	}
	return m, nil
}

// View renders the dashboard.
func (m DashboardModel) View() string {
	if m.loading {
		return LabelStyle.Render("Loading dashboard...")
	}

	if m.summary == nil || m.summary.TotalSessions == 0 {
		return PlaceholderStyle.Render("No data yet — run `claude-stats ingest` first")
	}

	cardWidth := m.width - 2
	if cardWidth < 40 {
		cardWidth = 40
	}

	var sections []string

	// Summary card
	sections = append(sections, m.renderSummary(cardWidth))

	// Daily cost bar chart
	if len(m.daily) > 0 {
		sections = append(sections, m.renderDailyChart(cardWidth))
	}

	// Model breakdown
	if len(m.models) > 0 {
		sections = append(sections, m.renderModelBreakdown(cardWidth))
	}

	return strings.Join(sections, "\n\n")
}

func (m DashboardModel) renderSummary(width int) string {
	s := m.summary

	line1 := fmt.Sprintf(
		"  %s %s    %s %s    %s %s",
		LabelStyle.Render("Sessions:"), ValueStyle.Render(formatInt(s.TotalSessions)),
		LabelStyle.Render("Messages:"), ValueStyle.Render(formatInt(s.TotalMessages)),
		LabelStyle.Render("Cost:"), formatCost(s.TotalCost),
	)

	line2 := fmt.Sprintf(
		"  %s %s    %s %s",
		LabelStyle.Render("Tokens:"), ValueStyle.Render(fmt.Sprintf("%s in / %s out", formatTokens(s.TotalInputTokens), formatTokens(s.TotalOutputTokens))),
		LabelStyle.Render("Avg/Day:"), formatCost(s.AvgDailyCost),
	)

	line3 := fmt.Sprintf(
		"  %s %s    %s %s",
		LabelStyle.Render("Cache:"), ValueStyle.Render(fmt.Sprintf("%s write / %s read", formatTokens(s.TotalCacheCreateTokens), formatTokens(s.TotalCacheReadTokens))),
		LabelStyle.Render(""), ValueStyle.Render(""),
	)

	line4 := fmt.Sprintf(
		"  %s %s    %s %s",
		LabelStyle.Render("Project:"), ValueStyle.Render(truncate(s.MostActiveProject, 30)),
		LabelStyle.Render("Model:"), ValueStyle.Render(truncate(s.PrimaryModel, 30)),
	)

	title := CardTitleStyle.Render("  Summary")
	body := fmt.Sprintf("%s\n%s\n%s\n%s", line1, line2, line3, line4)

	return CardStyle.Width(width).Render(fmt.Sprintf("%s\n%s", title, body))
}

func (m DashboardModel) renderDailyChart(width int) string {
	title := CardTitleStyle.Render("Last 7 Days")

	// Find max cost for scaling
	maxCost := 0.0
	for _, e := range m.daily {
		if e.Cost > maxCost {
			maxCost = e.Cost
		}
	}

	// Label width = "Mar 01 " = 7, cost width = " $XX.XX" ~= 8, borders/padding ~= 6
	barWidth := width - 7 - 10 - 6
	if barWidth < 10 {
		barWidth = 10
	}

	var lines []string
	for _, e := range m.daily {
		// Parse date for display
		label := e.Date
		if len(e.Date) >= 10 {
			label = e.Date[5:] // "03-01"
		}

		filled := 0
		if maxCost > 0 {
			filled = int(float64(barWidth) * e.Cost / maxCost)
		}
		if filled < 0 {
			filled = 0
		}
		empty := barWidth - filled

		bar := BarFullStyle.Render(strings.Repeat("█", filled)) +
			BarEmptyStyle.Render(strings.Repeat("░", empty))

		lines = append(lines, fmt.Sprintf("  %s %s  %s", LabelStyle.Render(label), bar, formatCost(e.Cost)))
	}

	body := strings.Join(lines, "\n")
	return CardStyle.Width(width).Render(fmt.Sprintf("%s\n%s", title, body))
}

func (m DashboardModel) renderModelBreakdown(width int) string {
	title := CardTitleStyle.Render("Cost by Model")

	totalCost := 0.0
	for _, model := range m.models {
		totalCost += model.Cost
	}

	var lines []string
	for _, model := range m.models {
		pct := 0.0
		if totalCost > 0 {
			pct = model.Cost / totalCost * 100
		}

		name := truncate(model.Model, 35)
		pctStr := LabelStyle.Render(fmt.Sprintf("(%5.1f%%)", pct))
		lines = append(lines, fmt.Sprintf("  %-35s  %8s  %s",
			ValueStyle.Render(name), formatCost(model.Cost), pctStr))
	}

	body := strings.Join(lines, "\n")
	return CardStyle.Width(width).Render(fmt.Sprintf("%s\n%s", title, body))
}

// Formatting helpers.

func formatInt(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

func formatTokens(n int64) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

func formatCost(c float64) string {
	return CostStyle.Render(fmt.Sprintf("$%.2f", c))
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
