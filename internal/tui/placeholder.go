package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// PlaceholderModel renders a "Coming Soon" message for unimplemented tabs.
type PlaceholderModel struct {
	tabName string
	width   int
	height  int
}

// NewPlaceholder creates a new placeholder for the given tab name.
func NewPlaceholder(tabName string) PlaceholderModel {
	return PlaceholderModel{tabName: tabName}
}

// Init returns nil.
func (m PlaceholderModel) Init() tea.Cmd {
	return nil
}

// Update handles window size messages.
func (m PlaceholderModel) Update(msg tea.Msg) (PlaceholderModel, tea.Cmd) {
	if msg, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

// View renders the placeholder.
func (m PlaceholderModel) View() string {
	text := fmt.Sprintf("%s — Coming Soon", m.tabName)
	styled := PlaceholderStyle.Render(text)

	if m.width > 0 && m.height > 0 {
		return lipglossPlace(m.width, m.height, styled)
	}
	return styled
}

func lipglossPlace(width, height int, content string) string {
	return fmt.Sprintf("%*s", (width+len(content))/2, content)
}
