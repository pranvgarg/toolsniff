package output

import (
	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
	"github.com/pranvgarg/toolsniff/config"
	"image/color"
)

// ThemeStyles is the single style vocabulary used by every TUI surface.
// Components consume semantic styles instead of choosing colors themselves.
type ThemeStyles struct {
	ActiveTab     lipgloss.Style
	ActiveNewTab  lipgloss.Style
	Tab           lipgloss.Style
	NewTab        lipgloss.Style
	Footer        lipgloss.Style
	Status        lipgloss.Style
	Warning       lipgloss.Style
	HeaderBorder  lipgloss.Style
	HeaderTitle   lipgloss.Style
	HeaderTagline lipgloss.Style
	HeaderStats   lipgloss.Style
	Wordmark      lipgloss.Style
	Version       lipgloss.Style
	SplashBorder  lipgloss.Style
	Panel         lipgloss.Style
	Table         table.Styles
}

// NewThemeStyles translates the config-layer semantic palette into Lip Gloss
// styles. All TUI components should receive styles from this factory.
func NewThemeStyles(theme config.ThemeSettings) ThemeStyles {
	colors := theme.Colors
	accent := lipgloss.Color(colors.Accent)
	secondary := lipgloss.Color(colors.Secondary)
	muted := lipgloss.Color(colors.Muted)
	border := lipgloss.Color(colors.Border)
	text := lipgloss.Color(colors.Text)
	selectionForeground := lipgloss.Color(colors.SelectionForeground)
	selectionBackground := lipgloss.Color(colors.SelectionBackground)
	warning := lipgloss.Color(colors.Warning)
	warningBackground := lipgloss.Color(colors.WarningBackground)

	return ThemeStyles{
		// Background blocks make selection obvious in both panes. The active
		// tab retains an underline as a secondary cue for monochrome terminals.
		ActiveTab:     lipgloss.NewStyle().Foreground(selectionForeground).Background(selectionBackground).Bold(true).Underline(true),
		ActiveNewTab:  lipgloss.NewStyle().Foreground(selectionForeground).Background(warning).Bold(true).Underline(true),
		Tab:           lipgloss.NewStyle().Foreground(muted),
		NewTab:        lipgloss.NewStyle().Foreground(warning).Background(warningBackground),
		Footer:        lipgloss.NewStyle().Foreground(muted),
		Status:        lipgloss.NewStyle().Foreground(warning),
		Warning:       lipgloss.NewStyle().Foreground(warning),
		HeaderBorder:  lipgloss.NewStyle().Foreground(border),
		HeaderTitle:   lipgloss.NewStyle().Foreground(accent).Bold(true),
		HeaderTagline: lipgloss.NewStyle().Foreground(muted),
		HeaderStats:   lipgloss.NewStyle().Foreground(secondary),
		Wordmark:      lipgloss.NewStyle().Foreground(accent).Bold(true),
		Version:       lipgloss.NewStyle().Foreground(muted),
		SplashBorder:  lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(border),
		Panel:         lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).Padding(1, 2),
		Table:         contentTableStyles(text, muted, selectionForeground, selectionBackground),
	}
}

func contentTableStyles(text, muted, selectionForeground, selectionBackground color.Color) table.Styles {
	s := table.DefaultStyles()
	s.Header = lipgloss.NewStyle().Foreground(muted).Bold(true).Padding(0, 1)
	s.Cell = lipgloss.NewStyle().Foreground(text).Padding(0, 1)
	// The selected row gets a full-width contrast block instead of only a
	// foreground color change, which makes selection clear in the right pane.
	s.Selected = lipgloss.NewStyle().Foreground(selectionForeground).Background(selectionBackground).Bold(true)
	return s
}
