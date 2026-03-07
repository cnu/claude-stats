package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cnu/claude-stats/internal/db"
)

// Tab represents a TUI tab.
type Tab int

const (
	TabDashboard Tab = iota
	TabSessions
	TabCosts
	TabProjects
	TabHeatmap
	TabQuery
)

var tabNames = []string{"Dashboard", "Sessions", "Costs", "Projects", "Heatmap", "Query"}

// DataLoadedMsg carries fetched dashboard data.
type DataLoadedMsg struct {
	Summary *db.DashboardSummary
	Daily   []db.DailyCostEntry
	Models  []db.ModelCostBreakdown
	Err     error
}

// RefreshMsg triggers a data reload.
type RefreshMsg struct{}

// App is the root Bubble Tea model.
type App struct {
	db        *db.DB
	activeTab Tab
	width     int
	height    int
	showHelp  bool

	dashboard    DashboardModel
	placeholders [5]PlaceholderModel // Sessions, Costs, Projects, Heatmap, Query
}

// NewApp creates a new TUI app.
func NewApp(database *db.DB) App {
	return App{
		db:        database,
		dashboard: NewDashboard(),
		placeholders: [5]PlaceholderModel{
			NewPlaceholder("Sessions"),
			NewPlaceholder("Costs"),
			NewPlaceholder("Projects"),
			NewPlaceholder("Heatmap"),
			NewPlaceholder("Query"),
		},
	}
}

// Init loads initial data.
func (a App) Init() tea.Cmd {
	return a.loadData
}

// Update handles messages.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return a, tea.Quit
		case "?":
			a.showHelp = !a.showHelp
			return a, nil
		case "r":
			a.dashboard.loading = true
			return a, a.loadData
		case "1":
			a.activeTab = TabDashboard
		case "2":
			a.activeTab = TabSessions
		case "3":
			a.activeTab = TabCosts
		case "4":
			a.activeTab = TabProjects
		case "5":
			a.activeTab = TabHeatmap
		case "6":
			a.activeTab = TabQuery
		}

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		// Forward to all tabs
		a.dashboard, _ = a.dashboard.Update(msg)
		for i := range a.placeholders {
			a.placeholders[i], _ = a.placeholders[i].Update(msg)
		}
		return a, nil

	case DataLoadedMsg:
		a.dashboard, _ = a.dashboard.Update(msg)
		return a, nil
	}

	return a, nil
}

// View renders the TUI.
func (a App) View() string {
	var b strings.Builder

	// Tab bar
	b.WriteString(a.renderTabBar())
	b.WriteString("\n")
	b.WriteString(DividerStyle.Render(strings.Repeat("─", a.width)))
	b.WriteString("\n\n")

	// Active tab content
	switch a.activeTab {
	case TabDashboard:
		b.WriteString(a.dashboard.View())
	default:
		idx := int(a.activeTab) - 1
		if idx >= 0 && idx < len(a.placeholders) {
			b.WriteString(a.placeholders[idx].View())
		}
	}

	// Help overlay
	if a.showHelp {
		b.WriteString("\n\n")
		b.WriteString(a.renderHelp())
	}

	// Status bar
	b.WriteString("\n\n")
	b.WriteString(a.renderStatusBar())

	return b.String()
}

func (a App) renderTabBar() string {
	var tabs []string
	for i, name := range tabNames {
		num := TabSepStyle.Render(fmt.Sprintf("%d:", i+1))
		if Tab(i) == a.activeTab {
			tabs = append(tabs, num+" "+TabActiveStyle.Render(name))
		} else {
			tabs = append(tabs, num+" "+TabInactiveStyle.Render(name))
		}
	}
	sep := TabSepStyle.Render(" │ ")
	return " " + strings.Join(tabs, sep)
}

func (a App) renderStatusBar() string {
	return StatusBarStyle.Render(" 1-6:tabs  r:refresh  q:quit  ?:help")
}

func (a App) renderHelp() string {
	help := `Keyboard Shortcuts:
  1-6    Switch tabs
  r      Refresh data
  q      Quit
  ?      Toggle this help`
	return CardStyle.Render(help)
}

func (a App) loadData() tea.Msg {
	summary, err := a.db.GetDashboardSummary()
	if err != nil {
		return DataLoadedMsg{Err: err}
	}

	daily, err := a.db.GetRecentDailyCosts(7)
	if err != nil {
		return DataLoadedMsg{Err: err}
	}

	models, err := a.db.GetModelCostBreakdown()
	if err != nil {
		return DataLoadedMsg{Err: err}
	}

	return DataLoadedMsg{
		Summary: summary,
		Daily:   daily,
		Models:  models,
	}
}
