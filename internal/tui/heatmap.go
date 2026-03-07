package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cnu/claude-stats/internal/db"
)

// HeatmapDataMsg carries the heatmap data.
type HeatmapDataMsg struct {
	Cells []db.HeatmapCell
	Err   error
}

// HeatmapModel renders the heatmap tab.
type HeatmapModel struct {
	database *db.DB
	width    int
	height   int
	loading  bool
	loaded   bool
	cells    []db.HeatmapCell
	grid     [7][24]float64 // normalized 0-1 per cell
	maxVal   float64
	showCost bool // false=messages, true=cost
}

// NewHeatmap creates a new heatmap model.
func NewHeatmap(database *db.DB) HeatmapModel {
	return HeatmapModel{database: database}
}

// Init returns nil.
func (m HeatmapModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m HeatmapModel) Update(msg tea.Msg) (HeatmapModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case HeatmapDataMsg:
		m.loading = false
		m.loaded = true
		if msg.Err == nil {
			m.cells = msg.Cells
			m.buildGrid()
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "t":
			m.showCost = !m.showCost
			m.buildGrid()
		}
	}
	return m, nil
}

func (m *HeatmapModel) buildGrid() {
	// Reset grid
	m.grid = [7][24]float64{}
	m.maxVal = 0

	for _, c := range m.cells {
		var val float64
		if m.showCost {
			val = c.Cost
		} else {
			val = float64(c.MessageCount)
		}
		if c.DayOfWeek >= 0 && c.DayOfWeek < 7 && c.Hour >= 0 && c.Hour < 24 {
			m.grid[c.DayOfWeek][c.Hour] = val
			if val > m.maxVal {
				m.maxVal = val
			}
		}
	}
}

// View renders the heatmap tab.
func (m HeatmapModel) View() string {
	if m.loading {
		return LabelStyle.Render("Loading heatmap...")
	}
	if !m.loaded {
		return PlaceholderStyle.Render("Switching to heatmap...")
	}
	if len(m.cells) == 0 {
		return PlaceholderStyle.Render("No activity data — run `claude-stats ingest` first")
	}

	cardWidth := m.width - 2
	if cardWidth < 40 {
		cardWidth = 40
	}

	modeLabel := "Messages"
	if m.showCost {
		modeLabel = "Cost"
	}
	title := CardTitleStyle.Render(fmt.Sprintf("Activity Heatmap — %s", modeLabel))
	hint := LabelStyle.Render("  t:toggle messages/cost")

	// Hour labels
	var hourLabels strings.Builder
	hourLabels.WriteString("      ") // day label width
	for h := 0; h < 24; h++ {
		if h%3 == 0 {
			fmt.Fprintf(&hourLabels, "%-3s", fmt.Sprintf("%02d", h))
		} else {
			hourLabels.WriteString("   ")
		}
	}

	// Day rows — reorder from SQLite's 0=Sun to Mon-Sun display
	// SQLite: 0=Sun, 1=Mon, 2=Tue, 3=Wed, 4=Thu, 5=Fri, 6=Sat
	// Display order: Mon(1), Tue(2), Wed(3), Thu(4), Fri(5), Sat(6), Sun(0)
	dayOrder := []int{1, 2, 3, 4, 5, 6, 0}
	dayNames := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}

	var rows []string
	for i, dow := range dayOrder {
		var row strings.Builder
		fmt.Fprintf(&row, "  %s ", LabelStyle.Render(dayNames[i]))
		for h := 0; h < 24; h++ {
			val := m.grid[dow][h]
			style := m.intensityStyle(val)
			row.WriteString(style.Render("██ "))
		}
		rows = append(rows, row.String())
	}

	// Legend
	legend := m.renderLegend()

	// Max value info
	maxInfo := ""
	if m.showCost {
		maxInfo = LabelStyle.Render(fmt.Sprintf("  Peak: $%.2f", m.maxVal))
	} else {
		maxInfo = LabelStyle.Render(fmt.Sprintf("  Peak: %d messages", int(m.maxVal)))
	}

	body := fmt.Sprintf("%s\n%s\n\n  %s\n%s\n\n%s\n%s",
		title, hint,
		LabelStyle.Render(hourLabels.String()),
		strings.Join(rows, "\n"),
		legend, maxInfo)

	return CardStyle.Width(cardWidth).Render(body)
}

func (m HeatmapModel) intensityStyle(val float64) lipgloss.Style {
	if m.maxVal == 0 || val == 0 {
		return lipgloss.NewStyle().Foreground(ColorBarEmpty)
	}
	ratio := val / m.maxVal
	switch {
	case ratio > 0.75:
		return lipgloss.NewStyle().Foreground(ColorGreen)
	case ratio > 0.50:
		return lipgloss.NewStyle().Foreground(ColorPrimary)
	case ratio > 0.25:
		return lipgloss.NewStyle().Foreground(ColorMuted)
	default:
		return lipgloss.NewStyle().Foreground(ColorDim)
	}
}

func (m HeatmapModel) renderLegend() string {
	empty := lipgloss.NewStyle().Foreground(ColorBarEmpty).Render("██")
	low := lipgloss.NewStyle().Foreground(ColorDim).Render("██")
	med := lipgloss.NewStyle().Foreground(ColorMuted).Render("██")
	high := lipgloss.NewStyle().Foreground(ColorPrimary).Render("██")
	peak := lipgloss.NewStyle().Foreground(ColorGreen).Render("██")

	return fmt.Sprintf("  %s %s %s %s %s %s %s %s %s %s",
		LabelStyle.Render("Less"), empty, low, med, high, peak, LabelStyle.Render("More"),
		LabelStyle.Render("  "), LabelStyle.Render(""), LabelStyle.Render(""))
}

// LoadCmd returns a command to load heatmap data.
func (m HeatmapModel) LoadCmd() tea.Cmd {
	return func() tea.Msg {
		cells, err := m.database.GetHeatmapData()
		return HeatmapDataMsg{Cells: cells, Err: err}
	}
}
