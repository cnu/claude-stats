package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cnu/claude-stats/internal/db"
)

// SessionsDataMsg carries the sessions list data.
type SessionsDataMsg struct {
	Sessions []db.SessionListEntry
	Err      error
}

// SessionDetailMsg carries detail data for a single session.
type SessionDetailMsg struct {
	Detail   *db.SessionDetail
	Messages []db.MessageEntry
	Err      error
}

type sessionsView int

const (
	sessionsListView sessionsView = iota
	sessionsDetailView
)

// SessionsModel renders the sessions tab.
type SessionsModel struct {
	database *db.DB
	width    int
	height   int
	loading  bool
	loaded   bool

	// List view
	sessions  []db.SessionListEntry
	cursor    int
	scrollTop int
	sortBy    string

	// Detail view
	view         sessionsView
	detail       *db.SessionDetail
	messages     []db.MessageEntry
	msgCursor    int
	msgScrollTop int
}

// NewSessions creates a new sessions model.
func NewSessions(database *db.DB) SessionsModel {
	return SessionsModel{
		database: database,
		sortBy:   "date",
	}
}

// Init returns nil.
func (m SessionsModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m SessionsModel) Update(msg tea.Msg) (SessionsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case SessionsDataMsg:
		m.loading = false
		m.loaded = true
		if msg.Err == nil {
			m.sessions = msg.Sessions
		}

	case SessionDetailMsg:
		m.loading = false
		if msg.Err == nil {
			m.detail = msg.Detail
			m.messages = msg.Messages
			m.view = sessionsDetailView
			m.msgCursor = 0
			m.msgScrollTop = 0
		}

	case tea.KeyMsg:
		if m.view == sessionsDetailView {
			return m.updateDetail(msg)
		}
		return m.updateList(msg)
	}
	return m, nil
}

func (m SessionsModel) updateList(msg tea.KeyMsg) (SessionsModel, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.cursor < len(m.sessions)-1 {
			m.cursor++
			m.adjustListScroll()
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
			m.adjustListScroll()
		}
	case "enter":
		if len(m.sessions) > 0 && m.cursor < len(m.sessions) {
			m.loading = true
			sid := m.sessions[m.cursor].SessionID
			return m, m.loadDetail(sid)
		}
	case "s":
		switch m.sortBy {
		case "date":
			m.sortBy = "cost"
		case "cost":
			m.sortBy = "messages"
		default:
			m.sortBy = "date"
		}
		m.cursor = 0
		m.scrollTop = 0
		m.loading = true
		return m, m.loadSessions()
	case "G":
		if len(m.sessions) > 0 {
			m.cursor = len(m.sessions) - 1
			m.adjustListScroll()
		}
	case "g":
		m.cursor = 0
		m.scrollTop = 0
	}
	return m, nil
}

func (m SessionsModel) updateDetail(msg tea.KeyMsg) (SessionsModel, tea.Cmd) {
	switch msg.String() {
	case "esc", "backspace":
		m.view = sessionsListView
		m.detail = nil
		m.messages = nil
	case "j", "down":
		if m.msgCursor < len(m.messages)-1 {
			m.msgCursor++
			m.adjustMsgScroll()
		}
	case "k", "up":
		if m.msgCursor > 0 {
			m.msgCursor--
			m.adjustMsgScroll()
		}
	}
	return m, nil
}

func (m *SessionsModel) adjustListScroll() {
	visible := m.visibleListRows()
	if m.cursor < m.scrollTop {
		m.scrollTop = m.cursor
	}
	if m.cursor >= m.scrollTop+visible {
		m.scrollTop = m.cursor - visible + 1
	}
}

func (m *SessionsModel) adjustMsgScroll() {
	visible := m.visibleMsgRows()
	if m.msgCursor < m.msgScrollTop {
		m.msgScrollTop = m.msgCursor
	}
	if m.msgCursor >= m.msgScrollTop+visible {
		m.msgScrollTop = m.msgCursor - visible + 1
	}
}

func (m SessionsModel) visibleListRows() int {
	// tab bar(1) + divider(1) + blank(1) + header(2) + status(2) + footer(2) = ~9
	rows := m.height - 9
	if rows < 5 {
		rows = 5
	}
	return rows
}

func (m SessionsModel) visibleMsgRows() int {
	// tab bar + divider + summary card(~8) + header(2) + status(2) = ~14
	rows := m.height - 14
	if rows < 5 {
		rows = 5
	}
	return rows
}

// View renders the sessions tab.
func (m SessionsModel) View() string {
	if m.loading {
		return LabelStyle.Render("Loading sessions...")
	}
	if !m.loaded {
		return PlaceholderStyle.Render("Switching to sessions...")
	}
	if len(m.sessions) == 0 {
		return PlaceholderStyle.Render("No sessions found — run `claude-stats ingest` first")
	}

	if m.view == sessionsDetailView {
		return m.renderDetail()
	}
	return m.renderList()
}

func (m SessionsModel) renderList() string {
	sortLabel := LabelStyle.Render(fmt.Sprintf("Sort: %s (s to change)", m.sortBy))
	countLabel := LabelStyle.Render(fmt.Sprintf("%d sessions", len(m.sessions)))
	header := fmt.Sprintf("  %s    %s", sortLabel, countLabel)

	// Column headers
	colHeader := fmt.Sprintf("  %-25s  %-12s  %6s  %8s  %8s",
		ColumnHeaderStyle.Render("Project"),
		ColumnHeaderStyle.Render("Date"),
		ColumnHeaderStyle.Render("Msgs"),
		ColumnHeaderStyle.Render("Cost"),
		ColumnHeaderStyle.Render("Duration"),
	)

	visible := m.visibleListRows()
	end := m.scrollTop + visible
	if end > len(m.sessions) {
		end = len(m.sessions)
	}

	var rows []string
	for i := m.scrollTop; i < end; i++ {
		s := m.sessions[i]
		project := truncate(s.ProjectName, 25)
		if project == "" {
			project = "(unknown)"
		}
		date := formatTimestamp(s.FirstMsgAt)
		msgs := fmt.Sprintf("%d", s.MessageCount)
		cost := fmt.Sprintf("$%.2f", s.CostUSD)
		dur := formatDuration(s.DurationMs)

		line := fmt.Sprintf("  %-25s  %-12s  %6s  %8s  %8s",
			project, date, msgs, cost, dur)

		if i == m.cursor {
			line = RowSelectedStyle.Render(line)
		} else {
			line = RowNormalStyle.Render(line)
		}
		rows = append(rows, line)
	}

	// Scroll indicator
	scrollInfo := ""
	if len(m.sessions) > visible {
		scrollInfo = LabelStyle.Render(fmt.Sprintf("  [%d-%d of %d]", m.scrollTop+1, end, len(m.sessions)))
	}

	body := header + "\n\n" + colHeader + "\n" + strings.Join(rows, "\n")
	if scrollInfo != "" {
		body += "\n" + scrollInfo
	}

	return body
}

func (m SessionsModel) renderDetail() string {
	if m.detail == nil {
		return LabelStyle.Render("Loading session detail...")
	}

	cardWidth := m.width - 2
	if cardWidth < 40 {
		cardWidth = 40
	}

	d := m.detail
	var sections []string

	// Summary card
	line1 := fmt.Sprintf("  %s %s    %s %s",
		LabelStyle.Render("Project:"), ValueStyle.Render(truncate(d.ProjectName, 30)),
		LabelStyle.Render("Branch:"), ValueStyle.Render(truncate(d.GitBranch, 20)),
	)
	line2 := fmt.Sprintf("  %s %s    %s %s    %s %s",
		LabelStyle.Render("Messages:"), ValueStyle.Render(fmt.Sprintf("%d (%d user / %d asst)", d.MessageCount, d.UserMsgCount, d.AsstMsgCount)),
		LabelStyle.Render("Cost:"), formatCost(d.CostUSD),
		LabelStyle.Render("Duration:"), ValueStyle.Render(formatDuration(d.DurationMs)),
	)
	line3 := fmt.Sprintf("  %s %s    %s %s",
		LabelStyle.Render("Tokens:"), ValueStyle.Render(fmt.Sprintf("%s in / %s out", formatTokens(d.InputTokens), formatTokens(d.OutputTokens))),
		LabelStyle.Render("Cache:"), ValueStyle.Render(fmt.Sprintf("%s write / %s read", formatTokens(d.CacheCreate), formatTokens(d.CacheRead))),
	)
	line4 := fmt.Sprintf("  %s %s — %s",
		LabelStyle.Render("Time:"),
		ValueStyle.Render(formatTimestampFull(d.FirstMsgAt)),
		ValueStyle.Render(formatTimestampFull(d.LastMsgAt)),
	)

	title := CardTitleStyle.Render("  Session Detail")
	summaryBody := fmt.Sprintf("%s\n%s\n%s\n%s", line1, line2, line3, line4)
	sections = append(sections, CardStyle.Width(cardWidth).Render(fmt.Sprintf("%s\n%s", title, summaryBody)))

	// Messages list
	if len(m.messages) > 0 {
		msgTitle := CardTitleStyle.Render(fmt.Sprintf("Messages (%d)", len(m.messages)))

		msgHeader := fmt.Sprintf("  %-10s  %-30s  %8s  %8s  %s",
			ColumnHeaderStyle.Render("Role"),
			ColumnHeaderStyle.Render("Model"),
			ColumnHeaderStyle.Render("Tokens"),
			ColumnHeaderStyle.Render("Cost"),
			ColumnHeaderStyle.Render("Preview"),
		)

		visible := m.visibleMsgRows()
		end := m.msgScrollTop + visible
		if end > len(m.messages) {
			end = len(m.messages)
		}

		var msgRows []string
		for i := m.msgScrollTop; i < end; i++ {
			msg := m.messages[i]
			roleStyle := RoleUserStyle
			if msg.Role == "assistant" {
				roleStyle = RoleAssistantStyle
			}

			model := truncate(msg.Model, 30)
			tokens := fmt.Sprintf("%d/%d", msg.InputTokens, msg.OutputTokens)
			cost := fmt.Sprintf("$%.4f", msg.CostUSD)
			preview := truncate(strings.ReplaceAll(msg.ContentPreview, "\n", " "), 40)

			line := fmt.Sprintf("  %-10s  %-30s  %8s  %8s  %s",
				roleStyle.Render(msg.Role), LabelStyle.Render(model),
				ValueStyle.Render(tokens), CostStyle.Render(cost),
				LabelStyle.Render(preview),
			)

			if i == m.msgCursor {
				line = RowSelectedStyle.Render(line)
			}
			msgRows = append(msgRows, line)
		}

		msgBody := msgHeader + "\n" + strings.Join(msgRows, "\n")
		sections = append(sections, CardStyle.Width(cardWidth).Render(fmt.Sprintf("%s\n%s", msgTitle, msgBody)))
	}

	backHint := LabelStyle.Render("  esc:back  j/k:scroll")
	sections = append(sections, backHint)

	return strings.Join(sections, "\n\n")
}

// LoadSessions returns a command to load the sessions list.
func (m SessionsModel) loadSessions() tea.Cmd {
	return func() tea.Msg {
		sessions, err := m.database.GetSessionList(m.sortBy, 200)
		return SessionsDataMsg{Sessions: sessions, Err: err}
	}
}

func (m SessionsModel) loadDetail(sessionID string) tea.Cmd {
	return func() tea.Msg {
		detail, err := m.database.GetSessionDetail(sessionID)
		if err != nil {
			return SessionDetailMsg{Err: err}
		}
		messages, err := m.database.GetSessionMessages(sessionID)
		return SessionDetailMsg{Detail: detail, Messages: messages, Err: err}
	}
}

// LoadCmd returns a command to load initial sessions data.
func (m SessionsModel) LoadCmd() tea.Cmd {
	return m.loadSessions()
}

// Formatting helpers.

func formatDuration(ms int64) string {
	d := time.Duration(ms) * time.Millisecond
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}

func formatTimestamp(ms int64) string {
	if ms == 0 {
		return "—"
	}
	t := time.UnixMilli(ms).Local()
	return t.Format("Jan 02 15:04")
}

func formatTimestampFull(ms int64) string {
	if ms == 0 {
		return "—"
	}
	t := time.UnixMilli(ms).Local()
	return t.Format("2006-01-02 15:04:05")
}
