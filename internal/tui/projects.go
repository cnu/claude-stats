package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cnu/claude-stats/internal/db"
)

// ProjectsDataMsg carries the project list data.
type ProjectsDataMsg struct {
	Projects []db.ProjectListEntry
	Err      error
}

// ProjectDetailMsg carries sessions for a specific project.
type ProjectDetailMsg struct {
	Sessions []db.SessionListEntry
	Err      error
}

type projectsView int

const (
	projectsListView projectsView = iota
	projectsDetailView
)

// ProjectsModel renders the projects tab.
type ProjectsModel struct {
	database *db.DB
	width    int
	height   int
	loading  bool
	loaded   bool

	// List view
	projects  []db.ProjectListEntry
	cursor    int
	scrollTop int
	sortBy    string

	// Detail view
	view         projectsView
	selectedName string
	sessions     []db.SessionListEntry
	sessCursor   int
	sessScroll   int
}

// NewProjects creates a new projects model.
func NewProjects(database *db.DB) ProjectsModel {
	return ProjectsModel{
		database: database,
		sortBy:   "cost",
	}
}

// Init returns nil.
func (m ProjectsModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m ProjectsModel) Update(msg tea.Msg) (ProjectsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case ProjectsDataMsg:
		m.loading = false
		m.loaded = true
		if msg.Err == nil {
			m.projects = msg.Projects
		}

	case ProjectDetailMsg:
		m.loading = false
		if msg.Err == nil {
			m.sessions = msg.Sessions
			m.view = projectsDetailView
			m.sessCursor = 0
			m.sessScroll = 0
		}

	case tea.KeyMsg:
		if m.view == projectsDetailView {
			return m.updateDetail(msg)
		}
		return m.updateList(msg)
	}
	return m, nil
}

func (m ProjectsModel) updateList(msg tea.KeyMsg) (ProjectsModel, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.cursor < len(m.projects)-1 {
			m.cursor++
			m.adjustListScroll()
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
			m.adjustListScroll()
		}
	case "enter":
		if len(m.projects) > 0 && m.cursor < len(m.projects) {
			m.loading = true
			m.selectedName = m.projects[m.cursor].ProjectName
			return m, m.loadProjectSessions(m.selectedName)
		}
	case "s":
		switch m.sortBy {
		case "cost":
			m.sortBy = "sessions"
		case "sessions":
			m.sortBy = "name"
		case "name":
			m.sortBy = "recent"
		default:
			m.sortBy = "cost"
		}
		m.cursor = 0
		m.scrollTop = 0
		m.loading = true
		return m, m.loadProjects()
	case "G":
		if len(m.projects) > 0 {
			m.cursor = len(m.projects) - 1
			m.adjustListScroll()
		}
	case "g":
		m.cursor = 0
		m.scrollTop = 0
	}
	return m, nil
}

func (m ProjectsModel) updateDetail(msg tea.KeyMsg) (ProjectsModel, tea.Cmd) {
	switch msg.String() {
	case "esc", "backspace":
		m.view = projectsListView
		m.sessions = nil
	case "j", "down":
		if m.sessCursor < len(m.sessions)-1 {
			m.sessCursor++
			m.adjustSessScroll()
		}
	case "k", "up":
		if m.sessCursor > 0 {
			m.sessCursor--
			m.adjustSessScroll()
		}
	}
	return m, nil
}

func (m *ProjectsModel) adjustListScroll() {
	visible := m.visibleListRows()
	if m.cursor < m.scrollTop {
		m.scrollTop = m.cursor
	}
	if m.cursor >= m.scrollTop+visible {
		m.scrollTop = m.cursor - visible + 1
	}
}

func (m *ProjectsModel) adjustSessScroll() {
	visible := m.visibleSessRows()
	if m.sessCursor < m.sessScroll {
		m.sessScroll = m.sessCursor
	}
	if m.sessCursor >= m.sessScroll+visible {
		m.sessScroll = m.sessCursor - visible + 1
	}
}

func (m ProjectsModel) visibleListRows() int {
	rows := m.height - 9
	if rows < 5 {
		rows = 5
	}
	return rows
}

func (m ProjectsModel) visibleSessRows() int {
	rows := m.height - 10
	if rows < 5 {
		rows = 5
	}
	return rows
}

// View renders the projects tab.
func (m ProjectsModel) View() string {
	if m.loading {
		return LabelStyle.Render("Loading projects...")
	}
	if !m.loaded {
		return PlaceholderStyle.Render("Switching to projects...")
	}
	if len(m.projects) == 0 {
		return PlaceholderStyle.Render("No projects found — run `claude-stats ingest` first")
	}

	if m.view == projectsDetailView {
		return m.renderDetail()
	}
	return m.renderList()
}

func (m ProjectsModel) renderList() string {
	sortLabel := LabelStyle.Render(fmt.Sprintf("Sort: %s (s to change)", m.sortBy))
	countLabel := LabelStyle.Render(fmt.Sprintf("%d projects", len(m.projects)))
	header := fmt.Sprintf("  %s    %s", sortLabel, countLabel)

	colHeader := fmt.Sprintf("  %-30s  %8s  %8s  %10s  %-12s  %-25s",
		ColumnHeaderStyle.Render("Project"),
		ColumnHeaderStyle.Render("Sessions"),
		ColumnHeaderStyle.Render("Messages"),
		ColumnHeaderStyle.Render("Cost"),
		ColumnHeaderStyle.Render("Last Active"),
		ColumnHeaderStyle.Render("Top Model"),
	)

	visible := m.visibleListRows()
	end := m.scrollTop + visible
	if end > len(m.projects) {
		end = len(m.projects)
	}

	var rows []string
	for i := m.scrollTop; i < end; i++ {
		p := m.projects[i]
		name := truncate(p.ProjectName, 30)
		lastActive := formatTimestamp(p.LastActiveAt)
		model := truncate(p.TopModel, 25)

		line := fmt.Sprintf("  %-30s  %8d  %8d  %10s  %-12s  %-25s",
			name, p.SessionCount, p.TotalMessages,
			fmt.Sprintf("$%.2f", p.TotalCost), lastActive, model)

		if i == m.cursor {
			line = RowSelectedStyle.Render(line)
		} else {
			line = RowNormalStyle.Render(line)
		}
		rows = append(rows, line)
	}

	scrollInfo := ""
	if len(m.projects) > visible {
		scrollInfo = LabelStyle.Render(fmt.Sprintf("  [%d-%d of %d]", m.scrollTop+1, end, len(m.projects)))
	}

	body := header + "\n\n" + colHeader + "\n" + strings.Join(rows, "\n")
	if scrollInfo != "" {
		body += "\n" + scrollInfo
	}

	return body
}

func (m ProjectsModel) renderDetail() string {
	title := CardTitleStyle.Render(fmt.Sprintf("  Project: %s", m.selectedName))
	countInfo := LabelStyle.Render(fmt.Sprintf("  %d sessions", len(m.sessions)))

	colHeader := fmt.Sprintf("  %-12s  %6s  %8s  %8s",
		ColumnHeaderStyle.Render("Date"),
		ColumnHeaderStyle.Render("Msgs"),
		ColumnHeaderStyle.Render("Cost"),
		ColumnHeaderStyle.Render("Duration"),
	)

	visible := m.visibleSessRows()
	end := m.sessScroll + visible
	if end > len(m.sessions) {
		end = len(m.sessions)
	}

	var rows []string
	for i := m.sessScroll; i < end; i++ {
		s := m.sessions[i]
		date := formatTimestamp(s.FirstMsgAt)
		cost := fmt.Sprintf("$%.2f", s.CostUSD)
		dur := formatDuration(s.DurationMs)

		line := fmt.Sprintf("  %-12s  %6d  %8s  %8s",
			date, s.MessageCount, cost, dur)

		if i == m.sessCursor {
			line = RowSelectedStyle.Render(line)
		} else {
			line = RowNormalStyle.Render(line)
		}
		rows = append(rows, line)
	}

	backHint := LabelStyle.Render("  esc:back  j/k:scroll")

	cardWidth := m.width - 2
	if cardWidth < 40 {
		cardWidth = 40
	}
	headerCard := CardStyle.Width(cardWidth).Render(fmt.Sprintf("%s\n%s", title, countInfo))

	body := headerCard + "\n\n" + colHeader + "\n" + strings.Join(rows, "\n") + "\n\n" + backHint
	return body
}

func (m ProjectsModel) loadProjects() tea.Cmd {
	return func() tea.Msg {
		projects, err := m.database.GetProjectList(m.sortBy)
		return ProjectsDataMsg{Projects: projects, Err: err}
	}
}

func (m ProjectsModel) loadProjectSessions(name string) tea.Cmd {
	return func() tea.Msg {
		sessions, err := m.database.GetProjectSessions(name, 100)
		return ProjectDetailMsg{Sessions: sessions, Err: err}
	}
}

// LoadCmd returns a command to load the project list.
func (m ProjectsModel) LoadCmd() tea.Cmd {
	return m.loadProjects()
}
