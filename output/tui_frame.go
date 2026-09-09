package output

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/pranvgarg/toolsniff/model"
)

// compactWidthThreshold is the terminal width below which the vertical
// sidebar collapses into a single-line compact tab strip.
const compactWidthThreshold = 60

// frameChromeRows is the number of fixed rows the bordered frame spends on
// chrome around the sidebar/content area, excluding the footer: top
// border, separator, bottom border. The footer's own row count varies (1
// normally, more when the full key-binding help is expanded via "?") and
// is accounted for separately by callers of contentPaneHeight.
const frameChromeRows = 3

// sidebarCountGap is the gutter between a sidebar label and its right-aligned
// count, in cells. DESIGN.md > Layout > Spacing (`md`).
const sidebarCountGap = 2

// itoa is a local shorthand; sidebar and badge widths are computed from the
// decimal form of counts often enough to be worth naming.
func itoa(v int) string { return strconv.Itoa(v) }

// frameHorizontalChrome is the number of columns the frame spends on borders
// and padding around the sidebar and content columns. It is measured from the
// styles that draw them rather than hard-coded, so changing a padding in
// tui_styles.go can never desynchronize the layout budget from the render.
func frameHorizontalChrome(styles ThemeStyles) int {
	return styles.FramePane.GetHorizontalFrameSize() +
		styles.SidebarPane.GetHorizontalFrameSize() +
		styles.ContentPane.GetHorizontalFrameSize()
}

// tabDisplayLabel is the human name for a tab id. The report tab ids are
// slugs ("path-executables"); the sidebar shows what they mean instead, so the
// navigation reads as English and never shows a bare "available". Anything not
// a report view (a legacy per-source tab) is shown as-is.
func tabDisplayLabel(tab string) string {
	switch ViewCategory(tab) {
	case ViewOverview:
		return "overview"
	case ViewCLI:
		return "CLI tools"
	case ViewPackages:
		return "packages"
	case ViewApplications:
		return "applications"
	case ViewPathExecutables:
		return "on your PATH"
	case ViewNpxHistory:
		return "npx history"
	case ViewChanges:
		return "changes"
	case ViewIssues:
		return "issues"
	}
	return tab
}

// sidebarLabel returns the display label for a tab, appending a warning
// glyph to alerting tabs so they stand out even when not active.
func sidebarLabel(tab string, alert bool, styles ThemeStyles) string {
	label := tabDisplayLabel(tab)
	if alert {
		return label + " " + styles.Glyph.Warning
	}
	return label
}

// tabAlerts marks the tabs that carry a problem the user should notice: the
// old per-source "new since last scan" tab, and the legacy "issues" tab when
// it is not empty. An alerting tab with nothing in it is noise, so zero never
// alerts.
func tabAlerts(tab string, count int) bool {
	if tab == newTabID {
		return true
	}
	return tab == string(ViewIssues) && count > 0
}

// sidebarDims returns the label column width and count column width needed
// to fit every tab's row without truncation. Widths are display cells
// (lipgloss.Width), never rune counts: the warning glyph is a wide rune on
// some terminals and padding it with fmt's %-*s is what misaligned the count
// column. See DESIGN.md > Do's and Don'ts.
func sidebarDims(tabs []string, toolsBySrc map[string][]model.Tool, styles ThemeStyles) (labelWidth, countWidth int) {
	countWidth = 2
	for _, t := range tabs {
		count := len(toolsBySrc[t])
		if w := lipgloss.Width(sidebarLabel(t, tabAlerts(t, count), styles)); w > labelWidth {
			labelWidth = w
		}
		if w := lipgloss.Width(itoa(count)); w > countWidth {
			countWidth = w
		}
	}
	return labelWidth, countWidth
}

// sidebarWidth returns the rendered width of the sidebar column's content,
// excluding the pane's own padding and divider.
func (m tuiModel) sidebarWidth() int {
	labelWidth, countWidth := sidebarDims(m.tabs, m.toolsBySrc, m.styles)
	// "▎N " + label + gutter + count. The selection bar and jump number are
	// reserved on every row so selection never shifts the layout.
	return 3 + labelWidth + sidebarCountGap + countWidth
}

// contentWidth returns the width available to the content pane.
func (m tuiModel) contentWidth() int {
	w := m.frameWidth() - m.sidebarWidth() - frameHorizontalChrome(m.styles)
	if w < 10 {
		w = 10
	}
	return w
}

func (m tuiModel) frameWidth() int {
	if m.width <= 0 {
		return 80
	}
	return m.width
}

func (m tuiModel) frameHeight() int {
	if m.height <= 0 {
		return 24
	}
	return m.height
}

// contentPaneHeight returns the number of content/sidebar rows available
// given the total frame height and the number of rows the footer needs
// (normally 1, more when the full key-binding help is expanded).
func contentPaneHeight(height, footerRows int) int {
	h := height - frameChromeRows - footerRows
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
// cells. If s is already at or beyond width, it is returned unchanged and
// left to the caller's own truncation.
func rightAlign(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return strings.Repeat(" ", width-w) + s
}

// truncateTail truncates s to at most width display cells by dropping
// characters from the *front* and prefixing an ellipsis, so the tail of s
// survives. Used for the Version column when it is showing a filesystem path
// fallback, where the suffix (e.g. the binary name) is far more identifying
// than the shared prefix every path starts with.
func truncateTail(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width("…"+string(r)) > width {
		r = r[1:]
	}
	return "…" + string(r)
}

// renderHeaderLine builds the top border line, with the wordmark, tagline,
// and stats stamp embedded directly into the border, e.g.:
//
//	╭─ ◆ toolsniff ─ dev & AI CLI inventory ── ... ── 47 installed · 6 sources ─╮
//
// lipgloss has no notion of a titled border, so this one line is composed by
// hand; every other join in the frame is declarative.
func renderHeaderLine(width int, title, tagline, stats string, styles ThemeStyles) string {
	border := lipgloss.RoundedBorder()
	inner := width - 2
	if inner < 0 {
		inner = 0
	}

	build := func(tagline, stats string) (left, right string) {
		left = styles.HeaderBorder.Render(border.Top+" ") + styles.HeaderTitle.Render(title)
		if tagline != "" {
			left += styles.HeaderBorder.Render(" "+border.Top+" ") + styles.HeaderTagline.Render(tagline)
		}
		left += styles.HeaderBorder.Render(" ")
		right = styles.HeaderBorder.Render(" ")
		if stats != "" {
			right += styles.HeaderStats.Render(stats) + styles.HeaderBorder.Render(" ")
		}
		right += styles.HeaderBorder.Render(border.Top)
		return left, right
	}

	// At narrow widths the fixed-width left/right components don't fit
	// alongside the corners. Rather than let the line overflow and get
	// clipped, progressively drop the tagline, then the stats stamp.
	left, right := build(tagline, stats)
	fillLen := inner - lipgloss.Width(left) - lipgloss.Width(right)
	if fillLen < 1 {
		left, right = build("", stats)
		fillLen = inner - lipgloss.Width(left) - lipgloss.Width(right)
	}
	if fillLen < 1 {
		left, right = build("", "")
		fillLen = inner - lipgloss.Width(left) - lipgloss.Width(right)
	}
	if fillLen < 1 {
		fillLen = 1
	}
	fill := styles.HeaderBorder.Render(strings.Repeat(border.Top, fillLen))

	return styles.HeaderBorder.Render(border.TopLeft) + left + fill + right +
		styles.HeaderBorder.Render(border.TopRight)
}

// renderSidebarLines renders one row per tab: selection bar, jump number,
// left-aligned label, and a count right-aligned to a shared column so the
// numbers scan vertically. Padded to rowCount rows so it lines up with the
// content pane.
//
// focused is whether the keyboard is on the sidebar layer. The active tab keeps
// its active styling either way -- it is still the open view -- but only a
// focused sidebar draws the selection bar, so the bar always marks what the
// movement keys are about to move. The column is reserved on every row
// regardless, so gaining or losing focus never shifts the labels.
func renderSidebarLines(tabs []string, active int, focused bool, toolsBySrc map[string][]model.Tool, rowCount int, styles ThemeStyles) []string {
	labelWidth, countWidth := sidebarDims(tabs, toolsBySrc, styles)
	width := 3 + labelWidth + sidebarCountGap + countWidth

	lines := make([]string, 0, rowCount)
	for i, t := range tabs {
		if i >= rowCount {
			break
		}
		count := len(toolsBySrc[t])
		alert := tabAlerts(t, count)

		labelStyle, countStyle := styles.Tab, styles.Count
		switch {
		case i == active && alert:
			labelStyle, countStyle = styles.ActiveNewTab, styles.ActiveNewTab
		case i == active:
			labelStyle, countStyle = styles.ActiveTab, styles.ActiveCount
		case alert:
			labelStyle, countStyle = styles.NewTab, styles.NewTab
		}

		bar := " "
		if i == active && focused {
			bar = styles.SelectionBar.Render(styles.Glyph.Selection)
		}

		label := fitWidth(sidebarLabel(t, alert, styles), labelWidth)
		countCell := rightAlign(itoa(count), countWidth)

		lines = append(lines, lipgloss.JoinHorizontal(
			lipgloss.Top,
			bar,
			labelStyle.Render(itoa(i+1)+" "+label),
			strings.Repeat(" ", sidebarCountGap),
			countStyle.Render(countCell),
		))
	}
	for len(lines) < rowCount {
		lines = append(lines, fitWidth("", width))
	}
	return lines
}

// frameStats builds the header's right-hand stamp.
func (m tuiModel) frameStats() string {
	var installedTools, availableCommands, sourceCount int
	if m.report != nil {
		installedTools = len(m.report.report.Installed)
		availableCommands = len(m.report.report.Available)
		seenSources := make(map[string]struct{})
		for _, observation := range m.report.report.AllObservations() {
			if observation.Role != model.RoleHistory {
				seenSources[ObservationSource(observation)] = struct{}{}
			}
		}
		sourceCount = len(seenSources)
	} else {
		currentTools := append(append([]model.Tool{}, m.realTools...), m.available...)
		installedTools, availableCommands = countToolRoles(currentTools)
		sourceCount = countSources(currentTools)
		if installedTools == 0 && availableCommands == 0 {
			// On a genuinely empty scan, tabs falls back to a ["npm"]
			// placeholder so there's something to render, but that's not a
			// real source: report 0, matching --list's "0 tools across 0
			// sources" convention for an empty machine.
			sourceCount = 0
		}
	}
	// "available" is a scanner word: to a user that set is "commands on your
	// PATH that no manager installed". The stamp says so.
	return fmt.Sprintf("%d installed · %d on your PATH · %d sources", installedTools, availableCommands, sourceCount)
}

// renderFrame draws the full bordered frame: header, vertical sidebar,
// content pane, separator, and footer. Everything below the header line is
// composed with lipgloss borders and joins; no column budget is spelled out
// as a literal.
func (m tuiModel) renderFrame() string {
	width, height := m.frameWidth(), m.frameHeight()
	border := lipgloss.RoundedBorder()

	footerLines := m.footerLines()
	cWidth := m.contentWidth()
	rowCount := contentPaneHeight(height, len(footerLines))

	sidebar := m.styles.SidebarPane.Render(lipgloss.JoinVertical(
		lipgloss.Left,
		renderSidebarLines(m.tabs, m.activeTab, m.sidebarFocused(), m.toolsBySrc, rowCount, m.styles)...,
	))
	content := m.styles.ContentPane.Render(lipgloss.JoinVertical(
		lipgloss.Left,
		m.contentLines(cWidth, rowCount)...,
	))
	body := m.styles.FramePane.Render(lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content))

	// The T-junction has to land on the measured sidebar boundary; lipgloss
	// has no junction primitive, so this one line is composed by hand from the
	// same border set the panes use.
	sidebarSpan := lipgloss.Width(sidebar) - m.styles.SidebarPane.GetBorderRightSize()
	contentSpan := width - 2 - sidebarSpan - 1
	if contentSpan < 0 {
		contentSpan = 0
	}
	separator := m.styles.HeaderBorder.Render(
		border.MiddleLeft +
			strings.Repeat(border.Top, sidebarSpan) +
			border.MiddleBottom +
			strings.Repeat(border.Top, contentSpan) +
			border.MiddleRight,
	)

	footerBlock := make([]string, len(footerLines))
	for i, line := range footerLines {
		footerBlock[i] = fitWidth(line, width-frameFooterChrome(m.styles))
	}
	footer := m.styles.FramePane.Render(m.styles.ContentPane.Render(
		lipgloss.JoinVertical(lipgloss.Left, footerBlock...),
	))

	bottom := m.styles.HeaderBorder.Render(
		border.BottomLeft + strings.Repeat(border.Bottom, width-2) + border.BottomRight,
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		renderHeaderLine(width, m.styles.Glyph.Brand+" toolsniff", "dev & AI CLI inventory", m.frameStats(), m.styles),
		body,
		separator,
		footer,
		bottom,
	)
}

// frameFooterChrome is the horizontal budget the footer row spends on the
// frame border plus its own padding.
func frameFooterChrome(styles ThemeStyles) int {
	return styles.FramePane.GetHorizontalFrameSize() + styles.ContentPane.GetHorizontalFrameSize()
}

// renderCompact draws the <60-col fallback: a single-line tab strip in
// place of the sidebar, with no surrounding border.
func (m tuiModel) renderCompact() string {
	parts := make([]string, len(m.tabs))
	for i, t := range m.tabs {
		count := len(m.toolsBySrc[t])
		alert := tabAlerts(t, count)
		if i == m.activeTab {
			label := fmt.Sprintf("%d %s·%d", i+1, tabDisplayLabel(t), count)
			if alert {
				parts[i] = m.styles.ActiveNewTab.Render(label)
			} else {
				parts[i] = m.styles.ActiveTab.Render(label)
			}
			continue
		}
		label := itoa(i + 1)
		if alert {
			parts[i] = m.styles.NewTab.Render(label + m.styles.Glyph.Warning)
		} else {
			parts[i] = m.styles.Tab.Render(label)
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, joinWithSpaces(parts)...)
}

func joinWithSpaces(parts []string) []string {
	out := make([]string, 0, len(parts)*2)
	for i, p := range parts {
		if i > 0 {
			out = append(out, " ")
		}
		out = append(out, p)
	}
	return out
}
