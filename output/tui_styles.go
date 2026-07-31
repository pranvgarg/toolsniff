package output

import "github.com/charmbracelet/lipgloss"

var (
	colorAmber = lipgloss.Color("#ffb454")
	colorCyan  = lipgloss.Color("#7fd8c4")
	colorMuted = lipgloss.Color("#5c6577")

	activeTabStyle = lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Underline(true)
	tabStyle       = lipgloss.NewStyle().Foreground(colorMuted)
	footerStyle    = lipgloss.NewStyle().Foreground(colorMuted).MarginTop(1)
	statusStyle    = lipgloss.NewStyle().Foreground(colorAmber)
)
