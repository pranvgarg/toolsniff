package output

import "charm.land/lipgloss/v2"

var (
	colorAmber = lipgloss.Color("#ffb454")
	colorCyan  = lipgloss.Color("#7fd8c4")
	colorMuted = lipgloss.Color("#5c6577")

	activeTabStyle     = lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Underline(true)
	activeNewTabStyle  = lipgloss.NewStyle().Foreground(colorAmber).Bold(true).Underline(true)
	tabStyle           = lipgloss.NewStyle().Foreground(colorMuted)
	newTabStyle        = lipgloss.NewStyle().Foreground(colorAmber)
	footerStyle        = lipgloss.NewStyle().Foreground(colorMuted).MarginTop(1)
	statusStyle        = lipgloss.NewStyle().Foreground(colorAmber)
	headerBorderStyle  = lipgloss.NewStyle().Foreground(colorMuted)
	headerTitleStyle   = lipgloss.NewStyle().Foreground(colorAmber).Bold(true)
	headerTaglineStyle = lipgloss.NewStyle().Foreground(colorMuted)
	headerStatsStyle   = lipgloss.NewStyle().Foreground(colorMuted)
)
