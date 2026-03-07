package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cnu/claude-stats/internal/db"
	"github.com/cnu/claude-stats/internal/nlquery"
)

// QueryResultMsg carries query execution results.
type QueryResultMsg struct {
	Result *db.QueryResult
	SQL    string
	Err    error
}

// QueryModel renders the query tab with text input and results.
type QueryModel struct {
	database *db.DB
	nlEngine *nlquery.Engine
	width    int
	height   int

	// Input
	input     []rune
	cursorPos int
	sqlMode   bool // false=NL, true=SQL

	// History
	history    []string
	histIdx    int // -1=current input, 0+=history entry
	histBuffer string

	// Results
	result       *db.QueryResult
	lastSQL      string
	lastErr      string
	resultScroll int
}

// NewQuery creates a new query model.
func NewQuery(database *db.DB, engine *nlquery.Engine) QueryModel {
	return QueryModel{
		database: database,
		nlEngine: engine,
		histIdx:  -1,
	}
}

// HasInput returns true if the input buffer is non-empty.
func (m QueryModel) HasInput() bool {
	return len(m.input) > 0
}

// Init returns nil.
func (m QueryModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m QueryModel) Update(msg tea.Msg) (QueryModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case QueryResultMsg:
		if msg.Err != nil {
			m.lastErr = msg.Err.Error()
			m.result = nil
		} else {
			m.lastErr = ""
			m.result = msg.Result
			m.resultScroll = 0
		}
		m.lastSQL = msg.SQL

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m QueryModel) handleKey(msg tea.KeyMsg) (QueryModel, tea.Cmd) {
	key := msg.String()

	switch key {
	case "enter":
		return m.submit()
	case "tab":
		m.sqlMode = !m.sqlMode
		return m, nil
	case "esc":
		if m.result != nil || m.lastErr != "" {
			m.result = nil
			m.lastErr = ""
			m.lastSQL = ""
			m.resultScroll = 0
		} else if len(m.input) > 0 {
			m.input = nil
			m.cursorPos = 0
		}
		return m, nil
	case "ctrl+u":
		m.input = nil
		m.cursorPos = 0
		return m, nil
	case "backspace":
		if m.cursorPos > 0 {
			m.input = append(m.input[:m.cursorPos-1], m.input[m.cursorPos:]...)
			m.cursorPos--
		}
		return m, nil
	case "delete":
		if m.cursorPos < len(m.input) {
			m.input = append(m.input[:m.cursorPos], m.input[m.cursorPos+1:]...)
		}
		return m, nil
	case "left":
		if m.cursorPos > 0 {
			m.cursorPos--
		}
		return m, nil
	case "right":
		if m.cursorPos < len(m.input) {
			m.cursorPos++
		}
		return m, nil
	case "home", "ctrl+a":
		m.cursorPos = 0
		return m, nil
	case "end", "ctrl+e":
		m.cursorPos = len(m.input)
		return m, nil
	case "up":
		if len(m.input) == 0 && m.result != nil {
			// Scroll results up
			if m.resultScroll > 0 {
				m.resultScroll--
			}
		} else if len(m.input) == 0 && len(m.history) > 0 {
			// Navigate history
			if m.histIdx == -1 {
				m.histBuffer = string(m.input)
				m.histIdx = 0
			} else if m.histIdx < len(m.history)-1 {
				m.histIdx++
			}
			m.input = []rune(m.history[m.histIdx])
			m.cursorPos = len(m.input)
		}
		return m, nil
	case "down":
		if m.result != nil && len(m.input) == 0 {
			// Scroll results down
			m.resultScroll++
		} else if m.histIdx >= 0 {
			m.histIdx--
			if m.histIdx < 0 {
				m.input = []rune(m.histBuffer)
			} else {
				m.input = []rune(m.history[m.histIdx])
			}
			m.cursorPos = len(m.input)
		}
		return m, nil
	case "j":
		if m.result != nil && len(m.input) == 0 {
			m.resultScroll++
			return m, nil
		}
	case "k":
		if m.result != nil && len(m.input) == 0 {
			if m.resultScroll > 0 {
				m.resultScroll--
			}
			return m, nil
		}
	}

	// Default: insert character
	if len(key) == 1 || msg.Type == tea.KeyRunes {
		for _, r := range msg.Runes {
			m.input = append(m.input[:m.cursorPos], append([]rune{r}, m.input[m.cursorPos:]...)...)
			m.cursorPos++
		}
	}
	return m, nil
}

func (m QueryModel) submit() (QueryModel, tea.Cmd) {
	text := strings.TrimSpace(string(m.input))
	if text == "" {
		return m, nil
	}

	// Add to history
	if len(m.history) == 0 || m.history[0] != text {
		m.history = append([]string{text}, m.history...)
		if len(m.history) > 50 {
			m.history = m.history[:50]
		}
	}
	m.histIdx = -1

	// Clear input
	m.input = nil
	m.cursorPos = 0

	// Execute query
	if m.sqlMode {
		return m, m.executeSQL(text)
	}
	return m, m.executeNL(text)
}

func (m QueryModel) executeSQL(sql string) tea.Cmd {
	return func() tea.Msg {
		result, err := m.database.ExecuteQuery(sql, 50)
		return QueryResultMsg{Result: result, SQL: sql, Err: err}
	}
}

func (m QueryModel) executeNL(input string) tea.Cmd {
	return func() tea.Msg {
		result, sql, err := m.nlEngine.Query(input)
		return QueryResultMsg{Result: result, SQL: sql, Err: err}
	}
}

// View renders the query tab.
func (m QueryModel) View() string {
	var sections []string

	cardWidth := m.width - 2
	if cardWidth < 40 {
		cardWidth = 40
	}

	// Mode indicator + input
	mode := "NL"
	if m.sqlMode {
		mode = "SQL"
	}
	modeStyle := TabActiveStyle.Render(fmt.Sprintf("[%s]", mode))

	inputStr := string(m.input)
	cursor := "█"
	if m.cursorPos <= len(m.input) {
		before := string(m.input[:m.cursorPos])
		after := ""
		if m.cursorPos < len(m.input) {
			after = string(m.input[m.cursorPos:])
		}
		inputStr = before + cursor + after
	}

	prompt := fmt.Sprintf("  %s %s %s",
		modeStyle,
		ValueStyle.Render(">"),
		ValueStyle.Render(inputStr))

	modeHint := LabelStyle.Render("  tab:toggle NL/SQL  enter:run  esc:clear  up/down:history")
	sections = append(sections, prompt+"\n"+modeHint)

	// SQL used (if NL mode)
	if m.lastSQL != "" && !m.sqlMode {
		sqlPreview := truncate(strings.ReplaceAll(m.lastSQL, "\n", " "), cardWidth-10)
		sqlPreview = strings.Join(strings.Fields(sqlPreview), " ") // collapse whitespace
		sections = append(sections, LabelStyle.Render(fmt.Sprintf("  SQL: %s", sqlPreview)))
	}

	// Error
	if m.lastErr != "" {
		sections = append(sections, ErrorStyle.Render(fmt.Sprintf("  Error: %s", m.lastErr)))
	}

	// Results table
	if m.result != nil && len(m.result.Rows) > 0 {
		sections = append(sections, m.renderResults(cardWidth))
	} else if m.result != nil {
		sections = append(sections, LabelStyle.Render("  No results."))
	}

	// Empty state: show examples
	if m.result == nil && m.lastErr == "" && len(m.input) == 0 {
		sections = append(sections, m.renderExamples())
	}

	return strings.Join(sections, "\n\n")
}

func (m QueryModel) renderResults(width int) string {
	r := m.result

	// Calculate column widths
	widths := make([]int, len(r.Columns))
	for i, col := range r.Columns {
		widths[i] = len(col)
	}
	for _, row := range r.Rows {
		for i, val := range row {
			if i < len(widths) && len(val) > widths[i] {
				widths[i] = len(val)
			}
		}
	}
	// Cap widths
	for i := range widths {
		if widths[i] > 40 {
			widths[i] = 40
		}
	}

	// Header
	var header strings.Builder
	header.WriteString("  ")
	for i, col := range r.Columns {
		if i > 0 {
			header.WriteString("  ")
		}
		padded := col
		if len(padded) < widths[i] {
			padded += strings.Repeat(" ", widths[i]-len(padded))
		}
		header.WriteString(ColumnHeaderStyle.Render(padded))
	}

	// Rows with scroll
	visibleRows := m.height - 14
	if visibleRows < 5 {
		visibleRows = 5
	}

	start := m.resultScroll
	if start > len(r.Rows) {
		start = len(r.Rows)
	}
	end := start + visibleRows
	if end > len(r.Rows) {
		end = len(r.Rows)
	}

	var lines []string
	for _, row := range r.Rows[start:end] {
		var line strings.Builder
		line.WriteString("  ")
		for i, val := range row {
			if i >= len(widths) {
				break
			}
			if i > 0 {
				line.WriteString("  ")
			}
			display := val
			if len(display) > widths[i] {
				display = display[:widths[i]-3] + "..."
			}
			if len(display) < widths[i] {
				display += strings.Repeat(" ", widths[i]-len(display))
			}
			line.WriteString(ValueStyle.Render(display))
		}
		lines = append(lines, line.String())
	}

	countInfo := LabelStyle.Render(fmt.Sprintf("  (%d rows)", len(r.Rows)))
	if len(r.Rows) > visibleRows {
		countInfo = LabelStyle.Render(fmt.Sprintf("  [%d-%d of %d rows]  j/k:scroll", start+1, end, len(r.Rows)))
	}

	return header.String() + "\n" + strings.Join(lines, "\n") + "\n" + countInfo
}

func (m QueryModel) renderExamples() string {
	examples := nlquery.Examples()

	title := CardTitleStyle.Render("  Example Queries")
	var lines []string
	for _, ex := range examples {
		lines = append(lines, fmt.Sprintf("    %s", LabelStyle.Render(ex)))
	}

	cardWidth := m.width - 2
	if cardWidth < 40 {
		cardWidth = 40
	}
	body := strings.Join(lines, "\n")
	return CardStyle.Width(cardWidth).Render(fmt.Sprintf("%s\n%s", title, body))
}
