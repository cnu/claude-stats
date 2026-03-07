package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cnu/claude-stats/internal/db"
	"github.com/cnu/claude-stats/internal/nlquery"
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

	dashboard DashboardModel
	sessions  SessionsModel
	costs     CostsModel
	projects  ProjectsModel
	heatmap   HeatmapModel
	query     QueryModel
}

// NewApp creates a new TUI app.
func NewApp(database *db.DB) App {
	nlEng := nlquery.New(database)
	return App{
		db:        database,
		dashboard: NewDashboard(),
		sessions:  NewSessions(database),
		costs:     NewCosts(database),
		projects:  NewProjects(database),
		heatmap:   NewHeatmap(database),
		query:     NewQuery(database, nlEng),
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
		// Query tab gets ALL keys when it has input (except ctrl+c)
		if a.activeTab == TabQuery && a.query.HasInput() {
			if msg.String() == "ctrl+c" {
				return a, tea.Quit
			}
			var cmd tea.Cmd
			a.query, cmd = a.query.Update(msg)
			return a, cmd
		}

		// Global keys handled first
		switch msg.String() {
		case "q", "ctrl+c":
			return a, tea.Quit
		case "?":
			a.showHelp = !a.showHelp
			return a, nil
		case "r":
			return a, a.refreshActiveTab()
		case "1":
			a.activeTab = TabDashboard
			return a, nil
		case "2":
			return a, a.switchToTab(TabSessions)
		case "3":
			return a, a.switchToTab(TabCosts)
		case "4":
			return a, a.switchToTab(TabProjects)
		case "5":
			return a, a.switchToTab(TabHeatmap)
		case "6":
			a.activeTab = TabQuery
			return a, nil
		}

		// Delegate remaining keys to active tab
		var cmd tea.Cmd
		switch a.activeTab {
		case TabSessions:
			a.sessions, cmd = a.sessions.Update(msg)
		case TabCosts:
			a.costs, cmd = a.costs.Update(msg)
		case TabProjects:
			a.projects, cmd = a.projects.Update(msg)
		case TabHeatmap:
			a.heatmap, cmd = a.heatmap.Update(msg)
		case TabQuery:
			a.query, cmd = a.query.Update(msg)
		}
		return a, cmd

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		// Forward to all tabs
		a.dashboard, _ = a.dashboard.Update(msg)
		a.sessions, _ = a.sessions.Update(msg)
		a.costs, _ = a.costs.Update(msg)
		a.projects, _ = a.projects.Update(msg)
		a.heatmap, _ = a.heatmap.Update(msg)
		a.query, _ = a.query.Update(msg)
		return a, nil

	case DataLoadedMsg:
		a.dashboard, _ = a.dashboard.Update(msg)
		return a, nil

	case SessionsDataMsg:
		a.sessions, _ = a.sessions.Update(msg)
		return a, nil

	case SessionDetailMsg:
		a.sessions, _ = a.sessions.Update(msg)
		return a, nil

	case CostsDataMsg:
		a.costs, _ = a.costs.Update(msg)
		return a, nil

	case ProjectsDataMsg:
		a.projects, _ = a.projects.Update(msg)
		return a, nil

	case ProjectDetailMsg:
		a.projects, _ = a.projects.Update(msg)
		return a, nil

	case HeatmapDataMsg:
		a.heatmap, _ = a.heatmap.Update(msg)
		return a, nil

	case QueryResultMsg:
		a.query, _ = a.query.Update(msg)
		return a, nil
	}

	return a, nil
}

func (a *App) switchToTab(tab Tab) tea.Cmd {
	a.activeTab = tab
	switch tab {
	case TabSessions:
		if !a.sessions.loaded {
			a.sessions.loading = true
			return a.sessions.LoadCmd()
		}
	case TabCosts:
		if !a.costs.loaded {
			a.costs.loading = true
			return a.costs.LoadCmd()
		}
	case TabProjects:
		if !a.projects.loaded {
			a.projects.loading = true
			return a.projects.LoadCmd()
		}
	case TabHeatmap:
		if !a.heatmap.loaded {
			a.heatmap.loading = true
			return a.heatmap.LoadCmd()
		}
	}
	return nil
}

func (a *App) refreshActiveTab() tea.Cmd {
	switch a.activeTab {
	case TabDashboard:
		a.dashboard.loading = true
		return a.loadData
	case TabSessions:
		a.sessions.loading = true
		a.sessions.loaded = false
		return a.sessions.LoadCmd()
	case TabCosts:
		a.costs.loading = true
		a.costs.loaded = false
		return a.costs.LoadCmd()
	case TabProjects:
		a.projects.loading = true
		a.projects.loaded = false
		return a.projects.LoadCmd()
	case TabHeatmap:
		a.heatmap.loading = true
		a.heatmap.loaded = false
		return a.heatmap.LoadCmd()
	}
	return nil
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
	case TabSessions:
		b.WriteString(a.sessions.View())
	case TabCosts:
		b.WriteString(a.costs.View())
	case TabProjects:
		b.WriteString(a.projects.View())
	case TabHeatmap:
		b.WriteString(a.heatmap.View())
	case TabQuery:
		b.WriteString(a.query.View())
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
	base := " 1-6:tabs  r:refresh  q:quit  ?:help"
	switch a.activeTab {
	case TabSessions:
		base += "  │  j/k:navigate  enter:detail  s:sort  esc:back"
	case TabCosts:
		base += "  │  d/w/m:daily/weekly/monthly  j/k:scroll"
	case TabProjects:
		base += "  │  j/k:navigate  enter:detail  s:sort  esc:back"
	case TabHeatmap:
		base += "  │  t:toggle messages/cost"
	case TabQuery:
		base += "  │  enter:run  tab:NL/SQL  esc:clear  up/down:history"
	}
	return StatusBarStyle.Render(base)
}

func (a App) renderHelp() string {
	help := `Keyboard Shortcuts:
  1-6    Switch tabs
  r      Refresh data
  q      Quit
  ?      Toggle this help

  Sessions tab:
    j/k    Navigate sessions
    enter  View session detail
    s      Cycle sort (date/cost/messages)
    g/G    Jump to top/bottom
    esc    Back to list

  Costs tab:
    d/w/m  Switch daily/weekly/monthly
    j/k    Scroll content

  Projects tab:
    j/k    Navigate projects
    enter  View project sessions
    s      Cycle sort (cost/sessions/name/recent)
    esc    Back to list

  Heatmap tab:
    t      Toggle messages/cost view

  Query tab:
    enter  Run query
    tab    Toggle NL/SQL mode
    esc    Clear results/input
    up/dn  Query history (when input empty)`
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
