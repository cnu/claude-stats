package tui

import "github.com/charmbracelet/lipgloss"

// Claude brand color palette.
var (
	ColorPrimary   = lipgloss.Color("#DA7756") // terra cotta
	ColorCream     = lipgloss.Color("#F5F5F0") // cream / warm white
	ColorGreen     = lipgloss.Color("#8FBE6A") // sage green
	ColorMuted     = lipgloss.Color("#8C8C8C") // medium gray
	ColorDim       = lipgloss.Color("#5C5C5C") // dim gray
	ColorBorder    = lipgloss.Color("#4A4A4A") // charcoal
	ColorBarEmpty  = lipgloss.Color("#333333") // dark gray
)

// Component styles.
var (
	TabActiveStyle   = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	TabInactiveStyle = lipgloss.NewStyle().Foreground(ColorDim)
	TabSepStyle      = lipgloss.NewStyle().Foreground(ColorBorder)
	CardStyle        = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorBorder).Padding(0, 1)
	CardTitleStyle   = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).MarginBottom(1)
	LabelStyle       = lipgloss.NewStyle().Foreground(ColorMuted)
	ValueStyle       = lipgloss.NewStyle().Bold(true).Foreground(ColorCream)
	CostStyle        = lipgloss.NewStyle().Foreground(ColorGreen)
	BarFullStyle     = lipgloss.NewStyle().Foreground(ColorPrimary)
	BarEmptyStyle    = lipgloss.NewStyle().Foreground(ColorBarEmpty)
	StatusBarStyle   = lipgloss.NewStyle().Foreground(ColorDim)
	PlaceholderStyle = lipgloss.NewStyle().Foreground(ColorMuted).Italic(true)
	DividerStyle     = lipgloss.NewStyle().Foreground(ColorBorder)

	RowSelectedStyle   = lipgloss.NewStyle().Background(ColorBorder).Foreground(ColorCream)
	RowNormalStyle     = lipgloss.NewStyle().Foreground(ColorCream)
	ColumnHeaderStyle  = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Underline(true)
	RoleUserStyle      = lipgloss.NewStyle().Foreground(ColorGreen)
	RoleAssistantStyle = lipgloss.NewStyle().Foreground(ColorPrimary)
)
