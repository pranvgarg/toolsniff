package output

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/pranvgarg/toolsniff/model"
)

// compactWidthThreshold is the terminal width below which the vertical
// sidebar collapses into a single-line compact tab strip.
const compactWidthThreshold = 60

// frameNonContentRows is the number of rows the bordered frame spends on
// chrome (top border, separator, footer, bottom border) around the
// sidebar/content area.
const frameNonContentRows = 4

// frameFixedCols is the number of columns the bordered frame spends on
// borders and padding around the sidebar and content columns:
// "│ " + sidebar + " │ " + content + " │".
const frameFixedCols = 7

// sidebarLabel returns the display label for a tab, appending a warning
// glyph to the "new" tab so it stands out even when not the active tab.
func sidebarLabel(tab string) string {
	if tab == "new" {
		return "new ⚠"
	}
	return tab
}

// sidebarDims returns the label column width and count column width needed
// to fit every tab's row without truncation.
func sidebarDims(tabs []string, toolsBySrc map[string][]model.Tool) (labelWidth, countWidth int) {
	countWidth = 2
	for _, t := range tabs {
		if w := lipgloss.Width(sidebarLabel(t)); w > labelWidth {
			labelWidth = w
		}
		if w := len(fmt.Sprintf("%d", len(toolsBySrc[t]))); w > countWidth {
			countWidth = w
		}
	}
	return labelWidth, countWidth
}

// sidebarWidth returns the total rendered width of the sidebar column
// (excluding the leading number+space, which is included).
func sidebarWidth(tabs []string, toolsBySrc map[string][]model.Tool) int {
	labelWidth, countWidth := sidebarDims(tabs, toolsBySrc)
	// "N " + label + " " + count
	return 2 + labelWidth + 1 + countWidth
}

// contentPaneWidth returns the width available to the content pane given
// the total frame width and the sidebar's width.
func contentPaneWidth(width, sbWidth int) int {
	w := width - sbWidth - frameFixedCols
	if w < 10 {
		w = 10
	}
	return w
}

// contentPaneHeight returns the number of content/sidebar rows available
// given the total frame height.
func contentPaneHeight(height int) int {
	h := height - frameNonContentRows
	if h < 1 {
		h = 1
	}
	return h
}

// fitWidth pads or truncates s (ANSI-aware) to exactly width display cells.
func fitWidth(s string, width int) string {
	if width < 0 {
		width = 0
	}
	return lipgloss.NewStyle().Width(width).MaxWidth(width).Render(s)
}

// rightAlign left-pads s with spaces so it occupies exactly width display
// cells (used for the content table's version column, which bubbles/table
// has no built-in alignment option for). If s is already at or beyond
// width, it is returned unchanged and left to the table's own truncation.
func rightAlign(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return strings.Repeat(" ", width-w) + s
}

// renderHeaderLine builds the top border line, with the wordmark, tagline,
// and stats stamp embedded directly into the border, e.g.:
//
//	┌─ ◆ toolsniff ─ dev & AI CLI inventory ── ... ── 47 tools · 6 sources ─┐
func renderHeaderLine(width int, title, tagline, stats string) string {
	inner := width - 2
	if inner < 0 {
		inner = 0
	}

	left := headerBorderStyle.Render("─ ") + headerTitleStyle.Render(title) +
		headerBorderStyle.Render(" ─ ") + headerTaglineStyle.Render(tagline) + headerBorderStyle.Render(" ")
	right := headerBorderStyle.Render(" ") + headerStatsStyle.Render(stats) + headerBorderStyle.Render(" ─")

	fillLen := inner - lipgloss.Width(left) - lipgloss.Width(right)
	if fillLen < 1 {
		fillLen = 1
	}
	fill := headerBorderStyle.Render(strings.Repeat("─", fillLen))

	return headerBorderStyle.Render("┌") + left + fill + right + headerBorderStyle.Render("┐")
}

// renderSidebarLines renders one row per tab (numbered, active-styled),
// padded/truncated to rowCount rows so it lines up with the content pane.
func renderSidebarLines(tabs []string, active int, toolsBySrc map[string][]model.Tool, rowCount int) []string {
	labelWidth, countWidth := sidebarDims(tabs, toolsBySrc)
	width := 2 + labelWidth + 1 + countWidth

	lines := make([]string, 0, rowCount)
	for i, t := range tabs {
		if i >= rowCount {
			break
		}
		row := fmt.Sprintf("%d %-*s %*d", i+1, labelWidth, sidebarLabel(t), countWidth, len(toolsBySrc[t]))
		row = fitWidth(row, width)

		var styled string
		switch {
		case i == active && t == "new":
			styled = activeNewTabStyle.Render(row)
		case i == active:
			styled = activeTabStyle.Render(row)
		case t == "new":
			styled = newTabStyle.Render(row)
		default:
			styled = tabStyle.Render(row)
		}
		lines = append(lines, styled)
	}
	for len(lines) < rowCount {
		lines = append(lines, fitWidth("", width))
	}
	return lines
}

// renderFrame draws the full bordered frame: header, vertical sidebar,
// content pane, and footer.
func (m tuiModel) renderFrame() string {
	width := m.width
	if width <= 0 {
		width = 80
	}
	height := m.height
	if height <= 0 {
		height = 24
	}

	totalTools := 0
	sourceCount := 0
	for _, t := range m.tabs {
		if t == "new" || t == "npx-history" {
			continue
		}
		totalTools += len(m.toolsBySrc[t])
		sourceCount++
	}
	stats := fmt.Sprintf("%d tools · %d sources", totalTools, sourceCount)

	top := renderHeaderLine(width, "◆ toolsniff", "dev & AI CLI inventory", stats)

	sbWidth := sidebarWidth(m.tabs, m.toolsBySrc)
	cWidth := contentPaneWidth(width, sbWidth)
	rowCount := contentPaneHeight(height)

	sidebarLines := renderSidebarLines(m.tabs, m.activeTab, m.toolsBySrc, rowCount)
	contentLines := strings.Split(m.content.View(), "\n")

	rows := make([]string, rowCount)
	for i := 0; i < rowCount; i++ {
		var contentLine string
		if i < len(contentLines) {
			contentLine = contentLines[i]
		}
		rows[i] = "│ " + sidebarLines[i] + " │ " + fitWidth(contentLine, cWidth) + " │"
	}

	sep := "├" + strings.Repeat("─", sbWidth+2) + "┴" + strings.Repeat("─", cWidth+2) + "┤"
	sep = headerBorderStyle.Render(sep)

	footerLine := "│ " + fitWidth(m.footerHint(), width-4) + " │"

	bottom := headerBorderStyle.Render("└" + strings.Repeat("─", width-2) + "┘")

	lines := make([]string, 0, rowCount+4)
	lines = append(lines, top)
	lines = append(lines, rows...)
	lines = append(lines, sep, footerLine, bottom)
	return strings.Join(lines, "\n")
}

// renderCompact draws the <60-col fallback: a single-line tab strip in
// place of the sidebar, with no surrounding border.
func (m tuiModel) renderCompact() string {
	parts := make([]string, len(m.tabs))
	for i, t := range m.tabs {
		count := len(m.toolsBySrc[t])
		if i == m.activeTab {
			label := fmt.Sprintf("[%d %s·%d]", i+1, t, count)
			if t == "new" {
				parts[i] = activeNewTabStyle.Render(label)
			} else {
				parts[i] = activeTabStyle.Render(label)
			}
			continue
		}
		label := fmt.Sprintf("%d", i+1)
		if t == "new" {
			parts[i] = newTabStyle.Render(label + "⚠")
		} else {
			parts[i] = tabStyle.Render(label)
		}
	}
	return strings.Join(parts, " ")
}
