package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cnu/claude-stats/internal/db"
)

// CostsDataMsg carries all cost tab data.
type CostsDataMsg struct {
	Daily30     []db.DailyCostEntry
	Weekly      []db.WeeklyCostEntry
	Monthly     []db.MonthlyCostEntry
	TopSessions []db.SessionListEntry
	ByProject   []db.ProjectCostEntry
	Cache       *db.CacheEfficiency
	Err         error
}

type costsAggView int

const (
	costsDaily costsAggView = iota
	costsWeekly
	costsMonthly
)

// CostsModel renders the costs tab.
type CostsModel struct {
	database    *db.DB
	width       int
	height      int
	loading     bool
	loaded      bool

	daily30     []db.DailyCostEntry
	weekly      []db.WeeklyCostEntry
	monthly     []db.MonthlyCostEntry
	topSessions []db.SessionListEntry
	byProject   []db.ProjectCostEntry
	cache       *db.CacheEfficiency

	aggView   costsAggView
	scrollTop int
}

// NewCosts creates a new costs model.
func NewCosts(database *db.DB) CostsModel {
	return CostsModel{database: database}
}

// Init returns nil.
func (m CostsModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m CostsModel) Update(msg tea.Msg) (CostsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case CostsDataMsg:
		m.loading = false
		m.loaded = true
		if msg.Err == nil {
			m.daily30 = msg.Daily30
			m.weekly = msg.Weekly
			m.monthly = msg.Monthly
			m.topSessions = msg.TopSessions
			m.byProject = msg.ByProject
			m.cache = msg.Cache
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "d":
			m.aggView = costsDaily
		case "w":
			m.aggView = costsWeekly
		case "m":
			m.aggView = costsMonthly
		case "j", "down":
			m.scrollTop++
		case "k", "up":
			if m.scrollTop > 0 {
				m.scrollTop--
			}
		}
	}
	return m, nil
}

// View renders the costs tab.
func (m CostsModel) View() string {
	if m.loading {
		return LabelStyle.Render("Loading costs...")
	}
	if !m.loaded {
		return PlaceholderStyle.Render("Switching to costs...")
	}

	cardWidth := m.width - 2
	if cardWidth < 40 {
		cardWidth = 40
	}

	var sections []string

	// Cost chart
	sections = append(sections, m.renderCostChart(cardWidth))

	// Top expensive sessions
	if len(m.topSessions) > 0 {
		sections = append(sections, m.renderTopSessions(cardWidth))
	}

	// Cost by project
	if len(m.byProject) > 0 {
		sections = append(sections, m.renderProjectCosts(cardWidth))
	}

	// Cache efficiency
	if m.cache != nil && (m.cache.TotalCacheCreate > 0 || m.cache.TotalCacheRead > 0) {
		sections = append(sections, m.renderCacheEfficiency(cardWidth))
	}

	// Apply vertical scroll
	all := strings.Join(sections, "\n\n")
	lines := strings.Split(all, "\n")
	if m.scrollTop > 0 && m.scrollTop < len(lines) {
		lines = lines[m.scrollTop:]
	}
	visibleLines := m.height - 6
	if visibleLines > 0 && len(lines) > visibleLines {
		lines = lines[:visibleLines]
	}

	return strings.Join(lines, "\n")
}

func (m CostsModel) renderCostChart(width int) string {
	var viewLabel string
	switch m.aggView {
	case costsDaily:
		viewLabel = "[d]aily"
	case costsWeekly:
		viewLabel = "[w]eekly"
	case costsMonthly:
		viewLabel = "[m]onthly"
	}
	title := CardTitleStyle.Render(fmt.Sprintf("Cost Trend — %s", viewLabel))
	hint := LabelStyle.Render("  d/w/m to switch view")

	type chartEntry struct {
		label string
		cost  float64
	}

	var entries []chartEntry
	switch m.aggView {
	case costsDaily:
		for _, e := range m.daily30 {
			label := e.Date
			if len(e.Date) >= 10 {
				label = e.Date[5:]
			}
			entries = append(entries, chartEntry{label, e.Cost})
		}
	case costsWeekly:
		for _, e := range m.weekly {
			label := e.WeekStart
			if len(e.WeekStart) >= 10 {
				label = e.WeekStart[5:]
			}
			entries = append(entries, chartEntry{label, e.Cost})
		}
	case costsMonthly:
		for _, e := range m.monthly {
			entries = append(entries, chartEntry{e.Month, e.Cost})
		}
	}

	if len(entries) == 0 {
		return CardStyle.Width(width).Render(title + "\n" + PlaceholderStyle.Render("  No cost data available"))
	}

	maxCost := 0.0
	for _, e := range entries {
		if e.cost > maxCost {
			maxCost = e.cost
		}
	}

	barWidth := width - 7 - 10 - 6
	if barWidth < 10 {
		barWidth = 10
	}

	var lines []string
	for _, e := range entries {
		filled := 0
		if maxCost > 0 {
			filled = int(float64(barWidth) * e.cost / maxCost)
		}
		if filled < 0 {
			filled = 0
		}
		empty := barWidth - filled

		bar := BarFullStyle.Render(strings.Repeat("█", filled)) +
			BarEmptyStyle.Render(strings.Repeat("░", empty))

		lines = append(lines, fmt.Sprintf("  %s %s  %s", LabelStyle.Render(e.label), bar, formatCost(e.cost)))
	}

	body := strings.Join(lines, "\n")
	return CardStyle.Width(width).Render(fmt.Sprintf("%s\n%s\n%s", title, hint, body))
}

func (m CostsModel) renderTopSessions(width int) string {
	title := CardTitleStyle.Render("Top Expensive Sessions")

	header := fmt.Sprintf("  %-25s  %-12s  %6s  %8s",
		ColumnHeaderStyle.Render("Project"),
		ColumnHeaderStyle.Render("Date"),
		ColumnHeaderStyle.Render("Msgs"),
		ColumnHeaderStyle.Render("Cost"),
	)

	var rows []string
	for _, s := range m.topSessions {
		project := truncate(s.ProjectName, 25)
		if project == "" {
			project = "(unknown)"
		}
		date := formatTimestamp(s.FirstMsgAt)
		line := fmt.Sprintf("  %-25s  %-12s  %6d  %8s",
			ValueStyle.Render(project),
			LabelStyle.Render(date),
			s.MessageCount,
			formatCost(s.CostUSD),
		)
		rows = append(rows, line)
	}

	body := header + "\n" + strings.Join(rows, "\n")
	return CardStyle.Width(width).Render(fmt.Sprintf("%s\n%s", title, body))
}

func (m CostsModel) renderProjectCosts(width int) string {
	title := CardTitleStyle.Render("Cost by Project")

	totalCost := 0.0
	for _, p := range m.byProject {
		totalCost += p.Cost
	}

	var lines []string
	for _, p := range m.byProject {
		pct := 0.0
		if totalCost > 0 {
			pct = p.Cost / totalCost * 100
		}
		name := truncate(p.ProjectName, 30)
		pctStr := LabelStyle.Render(fmt.Sprintf("(%5.1f%%)", pct))
		lines = append(lines, fmt.Sprintf("  %-30s  %8s  %s  %s",
			ValueStyle.Render(name), formatCost(p.Cost), pctStr,
			LabelStyle.Render(fmt.Sprintf("%d sessions", p.SessionCount)),
		))
	}

	body := strings.Join(lines, "\n")
	return CardStyle.Width(width).Render(fmt.Sprintf("%s\n%s", title, body))
}

func (m CostsModel) renderCacheEfficiency(width int) string {
	title := CardTitleStyle.Render("Cache Efficiency")

	c := m.cache
	hitPct := fmt.Sprintf("%.1f%%", c.HitRatio*100)

	line1 := fmt.Sprintf("  %s %s    %s %s",
		LabelStyle.Render("Hit Ratio:"), ValueStyle.Render(hitPct),
		LabelStyle.Render("Cache Reads:"), ValueStyle.Render(formatTokens(c.TotalCacheRead)),
	)
	line2 := fmt.Sprintf("  %s %s",
		LabelStyle.Render("Cache Writes:"), ValueStyle.Render(formatTokens(c.TotalCacheCreate)),
	)

	body := fmt.Sprintf("%s\n%s", line1, line2)
	return CardStyle.Width(width).Render(fmt.Sprintf("%s\n%s", title, body))
}

// LoadCmd returns a command to load all costs data.
func (m CostsModel) LoadCmd() tea.Cmd {
	return func() tea.Msg {
		daily30, err := m.database.GetRecentDailyCosts(30)
		if err != nil {
			return CostsDataMsg{Err: err}
		}
		weekly, err := m.database.GetWeeklyCosts(12)
		if err != nil {
			return CostsDataMsg{Err: err}
		}
		monthly, err := m.database.GetMonthlyCosts(6)
		if err != nil {
			return CostsDataMsg{Err: err}
		}
		topSessions, err := m.database.GetTopExpensiveSessions(10)
		if err != nil {
			return CostsDataMsg{Err: err}
		}
		byProject, err := m.database.GetCostByProject()
		if err != nil {
			return CostsDataMsg{Err: err}
		}
		cache, err := m.database.GetCacheEfficiency()
		if err != nil {
			return CostsDataMsg{Err: err}
		}
		return CostsDataMsg{
			Daily30:     daily30,
			Weekly:      weekly,
			Monthly:     monthly,
			TopSessions: topSessions,
			ByProject:   byProject,
			Cache:       cache,
		}
	}
}
