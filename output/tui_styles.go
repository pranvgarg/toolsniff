package output

import (
	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
)

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

// contentTableStyles returns the bubbles/table styling used by the content
// pane: a muted header and a cyan-highlighted, bold selected row.
func contentTableStyles() table.Styles {
	s := table.DefaultStyles()
	s.Header = lipgloss.NewStyle().Foreground(colorMuted).Bold(true).Padding(0, 1)
	s.Cell = lipgloss.NewStyle().Padding(0, 1)
	s.Selected = lipgloss.NewStyle().Foreground(colorCyan).Bold(true)
	return s
}
