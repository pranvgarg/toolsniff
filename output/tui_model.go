package output

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/timer"
	tea "charm.land/bubbletea/v2"
	"github.com/pranvgarg/toolsniff/model"
	"github.com/pranvgarg/toolsniff/registry"
	"github.com/pranvgarg/toolsniff/scanner"
)

// tabOrder is the fixed, meaningful order tabs appear in: real sources
// first (in scan-priority order), then npx-history (informational), then
// "new" (only present when there's an actual diff).
var tabOrder = []string{
	"npm", "brew-formula", "brew-cask", "pipx", "cargo",
	"applications", "path", "npx-history",
}

// contentVersionColWidth is the fixed display width of the content table's
// right-aligned Version column. It does not vary with terminal width; only
// the Name column grows/shrinks to fill the available space.
const contentVersionColWidth = 10

type tuiModel struct {
	tabs        []string
	toolsBySrc  map[string][]model.Tool
	activeTab   int
	content     table.Model
	filtering   bool
	filterQuery string
	realTools   []model.Tool
	regPath     string
	statusMsg   string
	warnings    []scanner.Warning

	width, height int

	splashPhase splashPhase
	splashLines []string
	splashTimer timer.Model
}

func newTUIModel(realTools, npxHistory []model.Tool, diff registry.Diff, warnings []scanner.Warning, regPath string) tuiModel {
	toolsBySrc := map[string][]model.Tool{}
	for _, t := range realTools {
		toolsBySrc[t.Source] = append(toolsBySrc[t.Source], t)
	}
	if len(npxHistory) > 0 {
		toolsBySrc["npx-history"] = npxHistory
	}

	if len(diff.Added) > 0 || len(diff.Removed) > 0 {
		newTab := append([]model.Tool{}, diff.Added...)
		newTab = append(newTab, diff.Removed...)
		toolsBySrc["new"] = newTab
	}

	tabs := make([]string, 0, len(tabOrder)+1)
	for _, src := range tabOrder {
		if _, ok := toolsBySrc[src]; ok {
			tabs = append(tabs, src)
		}
	}
	if _, ok := toolsBySrc["new"]; ok {
		tabs = append(tabs, "new")
	}
	if len(tabs) == 0 {
		tabs = []string{"npm"}
	}

	t := table.New(
		table.WithColumns(columnsFor(0)),
		table.WithRows(rowsFor(toolsBySrc[tabs[0]], "")),
		table.WithFocused(true),
	)
	t.SetStyles(contentTableStyles())

	return tuiModel{
		tabs:        tabs,
		toolsBySrc:  toolsBySrc,
		content:     t,
		realTools:   realTools,
		regPath:     regPath,
		warnings:    warnings,
		splashTimer: newSplashTimer(),
	}
}

// versionOrPath returns a tool's version, falling back to its path when no
// version was detected. This mirrors the old toolItem.Description() logic.
func versionOrPath(t model.Tool) string {
	if t.Version != "" {
		return t.Version
	}
	return t.Path
}

// columnsFor returns the content table's columns sized for the given pane
// width: a fixed-width Version column and a Name column that fills the
// remaining space.
func columnsFor(width int) []table.Column {
	nameWidth := width - contentVersionColWidth - 4
	if nameWidth < 4 {
		nameWidth = 4
	}
	return []table.Column{
		{Title: "Name", Width: nameWidth},
		{Title: "Version", Width: contentVersionColWidth},
	}
}

// rowsFor builds the content table's rows for tools, keeping only those
// whose name case-insensitively contains filter (all tools when filter is
// empty). The version cell is pre-padded to contentVersionColWidth so it
// renders right-aligned, since bubbles/table has no built-in alignment.
func rowsFor(tools []model.Tool, filter string) []table.Row {
	lowerFilter := strings.ToLower(filter)
	rows := make([]table.Row, 0, len(tools))
	for _, t := range tools {
		if filter != "" && !strings.Contains(strings.ToLower(t.Name), lowerFilter) {
			continue
		}
		rows = append(rows, table.Row{t.Name, rightAlign(versionOrPath(t), contentVersionColWidth)})
	}
	return rows
}

// rebuildContent recomputes the content table's rows for the active tab
// under the current filter query, resetting the cursor to the top.
func (m *tuiModel) rebuildContent() {
	m.content.SetRows(rowsFor(m.toolsBySrc[m.tabs[m.activeTab]], m.filterQuery))
	m.content.SetCursor(0)
}

func (m tuiModel) Init() tea.Cmd { return m.splashTimer.Init() }

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if wsMsg, ok := msg.(tea.WindowSizeMsg); ok {
		m.width, m.height = wsMsg.Width, wsMsg.Height
		if m.width > 0 && m.width < compactWidthThreshold {
			h := m.height - 2 // compact strip + footer
			if h < 1 {
				h = 1
			}
			m.content.SetColumns(columnsFor(m.width))
			m.content.SetWidth(m.width)
			m.content.SetHeight(h)
		} else {
			sbWidth := sidebarWidth(m.tabs, m.toolsBySrc)
			cWidth := contentPaneWidth(m.width, sbWidth)
			m.content.SetColumns(columnsFor(cWidth))
			m.content.SetWidth(cWidth)
			m.content.SetHeight(contentPaneHeight(m.height))
		}
		return m, nil
	}

	if m.splashPhase != splashDone {
		return m.updateSplash(msg)
	}

	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		if m.filtering {
			switch keyMsg.String() {
			case "esc":
				m.filtering = false
				m.filterQuery = ""
				m.rebuildContent()
				return m, nil
			case "enter":
				m.filtering = false
				return m, nil
			case "backspace":
				if m.filterQuery != "" {
					r := []rune(m.filterQuery)
					m.filterQuery = string(r[:len(r)-1])
					m.rebuildContent()
				}
				return m, nil
			default:
				if text := keyMsg.Key().Text; text != "" {
					m.filterQuery += text
					m.rebuildContent()
				}
				return m, nil
			}
		}

		switch keyMsg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "/":
			m.filtering = true
			return m, nil
		case "tab":
			m.activeTab = (m.activeTab + 1) % len(m.tabs)
			m.rebuildContent()
			return m, nil
		case "s":
			if err := registry.Save(m.regPath, m.realTools); err != nil {
				m.statusMsg = "save failed: " + err.Error()
			} else {
				m.statusMsg = fmt.Sprintf("saved baseline: %d tools", len(m.realTools))
			}
			return m, nil
		case "d":
			for i, t := range m.tabs {
				if t == "new" {
					m.activeTab = i
					m.rebuildContent()
					break
				}
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.content, cmd = m.content.Update(msg)
	return m, cmd
}

// footerHint returns the footer/status line shown at the bottom of both the
// bordered frame and the compact layout: the live filter query and match
// count while filtering (or once a filter is applied), otherwise the
// default keybinding hint. Keeping this in the footer (rather than an
// appended line) ensures it stays within the frame's fixed height budget
// instead of scrolling off-screen.
func (m tuiModel) footerHint() string {
	if m.filtering || m.filterQuery != "" {
		n := len(m.content.Rows())
		unit := "match"
		if n != 1 {
			unit = "matches"
		}
		return fmt.Sprintf("/%s — %d %s (esc clear · enter apply)", m.filterQuery, n, unit)
	}
	return "↑↓ move · tab switch · / filter · d diff · s save · q quit"
}

func (m tuiModel) View() tea.View {
	if m.splashPhase != splashDone {
		view := tea.NewView(renderSplash(m.splashLines, m.width, m.height))
		view.AltScreen = true
		return view
	}

	var body string
	if m.width > 0 && m.width < compactWidthThreshold {
		body = m.renderCompact() + "\n" + m.content.View()
	} else {
		body = m.renderFrame()
	}

	parts := []string{body}
	for _, w := range m.warnings {
		parts = append(parts, footerStyle.Render(fmt.Sprintf("warning: %s: %v", w.Source, w.Err)))
	}
	if m.statusMsg != "" {
		parts = append(parts, statusStyle.Render(m.statusMsg))
	}
	if m.width > 0 && m.width < compactWidthThreshold {
		parts = append(parts, footerStyle.Render(m.footerHint()))
	}
	view := tea.NewView(strings.Join(parts, "\n"))
	view.AltScreen = true
	return view
}

// RunTUI launches the interactive Bubbletea program.
func RunTUI(realTools, npxHistory []model.Tool, diff registry.Diff, warnings []scanner.Warning, regPath string) error {
	p := tea.NewProgram(newTUIModel(realTools, npxHistory, diff, warnings, regPath))
	_, err := p.Run()
	return err
}
