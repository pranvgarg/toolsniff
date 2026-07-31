package output

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/timer"
	tea "charm.land/bubbletea/v2"
	"github.com/pranvgarg/toolsniff/model"
	"github.com/pranvgarg/toolsniff/registry"
	"github.com/pranvgarg/toolsniff/scanner"
)

// resizeDebounce is how long the TUI waits for terminal resizing to settle
// before re-running the (relatively expensive) layout recalculation in
// resizeContent. Rapid resize events during a drag are coalesced into a
// single re-layout once the user stops resizing.
const resizeDebounce = 120 * time.Millisecond

// resizeSettledMsg is sent after resizeDebounce has elapsed since a
// WindowSizeMsg was received. It carries a snapshot of the resize
// generation tag at the time it was scheduled, so a stale message from a
// superseded resize can be detected and dropped.
type resizeSettledMsg struct{ tag int }

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

	keys keyMap
	help help.Model

	width, height int
	resizeTag     int

	splashPhase splashPhase
	splashLines []string
	splashTimer timer.Model
}

// keyMap defines every key binding the TUI recognizes, satisfying
// help.KeyMap so it can be rendered directly via help.Model.View. Up/Down
// are included for display purposes only: table.Model handles ↑/↓/j/k
// movement internally and these bindings are never dispatched against.
type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	PrevTab key.Binding
	NextTab key.Binding
	JumpTab key.Binding
	Filter  key.Binding
	Diff    key.Binding
	Save    key.Binding
	Help    key.Binding
	Quit    key.Binding
}

// defaultKeyMap is the TUI's fixed keybinding set.
var defaultKeyMap = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "move up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "move down"),
	),
	PrevTab: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "prev tab"),
	),
	NextTab: key.NewBinding(
		key.WithKeys("right", "l", "tab"),
		key.WithHelp("→/l/tab", "next tab"),
	),
	JumpTab: key.NewBinding(
		key.WithKeys("1", "2", "3", "4", "5", "6", "7", "8", "9"),
		key.WithHelp("1-9", "jump to tab"),
	),
	Filter: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "filter"),
	),
	Diff: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "diff"),
	),
	Save: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "save"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "toggle help"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
}

// ShortHelp returns the handful of bindings shown in the collapsed footer.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("up", "down", "k", "j"), key.WithHelp("↑/↓", "move")),
		key.NewBinding(key.WithKeys("left", "right", "tab"), key.WithHelp("←/→/tab", "switch tab")),
		k.Filter,
		k.Help,
		k.Quit,
	}
}

// FullHelp returns every binding, grouped into columns, for the expanded
// help view toggled by "?".
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.PrevTab, k.NextTab, k.JumpTab},
		{k.Filter, k.Diff, k.Save, k.Help, k.Quit},
	}
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
		keys:        defaultKeyMap,
		help:        help.New(),
		splashTimer: newSplashTimer(),
	}
}

// versionOrPath returns a tool's version, falling back to its path when no
// version was detected, so every row always has something informative in
// its Version column even for tools where scanning couldn't determine a
// version string.
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
		val := versionOrPath(t)
		if t.Version == "" {
			// Path fallback: right-truncate (keep the tail) so the more
			// identifying suffix (e.g. the binary name) survives, instead
			// of showing an undifferentiated "/opt/homebrew/…" prefix.
			val = truncateTail(val, contentVersionColWidth)
		}
		rows = append(rows, table.Row{t.Name, rightAlign(val, contentVersionColWidth)})
	}
	return rows
}

// rebuildContent recomputes the content table's rows for the active tab
// under the current filter query, resetting the cursor to the top.
func (m *tuiModel) rebuildContent() {
	m.content.SetRows(rowsFor(m.toolsBySrc[m.tabs[m.activeTab]], m.filterQuery))
	m.content.SetCursor(0)
}

// resizeContent recalculates the content table's columns, width, and
// height for the current terminal size and footer state. The footer's row
// count varies (1 normally, more when the full key-binding help is
// expanded via "?"), so this must be called both on every WindowSizeMsg
// and whenever something else could change the footer's line count.
func (m *tuiModel) resizeContent() {
	footerRows := len(m.footerLines())
	if m.width > 0 && m.width < compactWidthThreshold {
		m.help.SetWidth(m.width)
		h := m.height - 1 - footerRows // compact strip + footer
		if h < 1 {
			h = 1
		}
		m.content.SetColumns(columnsFor(m.width))
		m.content.SetWidth(m.width)
		m.content.SetHeight(h)
		return
	}
	m.help.SetWidth(m.width - 4)
	sbWidth := sidebarWidth(m.tabs, m.toolsBySrc)
	cWidth := contentPaneWidth(m.width, sbWidth)
	m.content.SetColumns(columnsFor(cWidth))
	m.content.SetWidth(cWidth)
	m.content.SetHeight(contentPaneHeight(m.height, footerRows))
}

func (m tuiModel) Init() tea.Cmd { return m.splashTimer.Init() }

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if wsMsg, ok := msg.(tea.WindowSizeMsg); ok {
		// Update dimensions immediately so View()'s border sizing always
		// reflects the current terminal size, but debounce the (more
		// expensive) column/mode re-layout in resizeContent: bump the
		// generation tag and schedule a settle check. If another resize
		// arrives before the tick fires, its tag will no longer match
		// m.resizeTag and the stale tick is dropped.
		firstResize := m.width == 0 && m.height == 0
		m.width, m.height = wsMsg.Width, wsMsg.Height
		if firstResize {
			// Run the (usually debounced) re-layout synchronously for the
			// very first resize, so a keypress that dismisses the splash
			// before the debounce would otherwise fire never sees an
			// unconfigured/collapsed table.
			m.resizeContent()
			return m, nil
		}
		m.resizeTag++
		tag := m.resizeTag
		return m, tea.Tick(resizeDebounce, func(_ time.Time) tea.Msg {
			return resizeSettledMsg{tag: tag}
		})
	}

	if settleMsg, ok := msg.(resizeSettledMsg); ok {
		if settleMsg.tag == m.resizeTag {
			m.resizeContent()
		}
		return m, nil
	}

	if m.splashPhase != splashDone {
		return m.updateSplash(msg)
	}

	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		m.statusMsg = ""

		// ctrl+c must always quit, even while filtering: the filtering
		// branch below returns early for every key, which would otherwise
		// swallow it and leave esc as the only way out.
		if keyMsg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		if m.filtering {
			switch keyMsg.String() {
			case "esc":
				m.filtering = false
				m.filterQuery = ""
				m.rebuildContent()
				m.resizeContent()
				return m, nil
			case "enter":
				m.filtering = false
				m.resizeContent()
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

		switch {
		case key.Matches(keyMsg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(keyMsg, m.keys.Filter):
			m.filtering = true
			m.resizeContent()
			return m, nil
		case key.Matches(keyMsg, m.keys.NextTab):
			m.activeTab = (m.activeTab + 1) % len(m.tabs)
			m.filtering = false
			m.filterQuery = ""
			m.rebuildContent()
			return m, nil
		case key.Matches(keyMsg, m.keys.PrevTab):
			m.activeTab = (m.activeTab - 1 + len(m.tabs)) % len(m.tabs)
			m.filtering = false
			m.filterQuery = ""
			m.rebuildContent()
			return m, nil
		case key.Matches(keyMsg, m.keys.JumpTab):
			if idx, err := strconv.Atoi(keyMsg.String()); err == nil && idx >= 1 && idx <= len(m.tabs) {
				m.activeTab = idx - 1
				m.filtering = false
				m.filterQuery = ""
				m.rebuildContent()
			}
			return m, nil
		case key.Matches(keyMsg, m.keys.Save):
			if err := registry.Save(m.regPath, m.realTools); err != nil {
				m.statusMsg = "save failed: " + err.Error()
			} else {
				m.statusMsg = fmt.Sprintf("saved baseline: %d tools", len(m.realTools))
			}
			return m, nil
		case key.Matches(keyMsg, m.keys.Diff):
			for i, t := range m.tabs {
				if t == "new" {
					m.activeTab = i
					m.filtering = false
					m.filterQuery = ""
					m.rebuildContent()
					break
				}
			}
			return m, nil
		case key.Matches(keyMsg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
			m.resizeContent()
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
	return m.help.View(m.keys)
}

// footerLines returns every line the footer needs to render, in order:
// scanner warnings, the save-status message (if set), then the
// keybinding/filter hint. This is the single source of truth for "how many
// lines does the footer need" (used to budget content-pane height) and
// "what does the footer contain" (used by both renderFrame and the compact
// layout) — so nothing gets appended after the frame where it could fall
// outside the fixed height budget and be silently clipped by the alt-screen
// renderer.
func (m tuiModel) footerLines() []string {
	var lines []string
	for _, w := range m.warnings {
		lines = append(lines, footerStyle.Render(fmt.Sprintf("warning: %s: %v", w.Source, w.Err)))
	}
	if m.statusMsg != "" {
		lines = append(lines, statusStyle.Render(m.statusMsg))
	}
	lines = append(lines, strings.Split(m.footerHint(), "\n")...)
	return lines
}

func (m tuiModel) View() tea.View {
	if m.splashPhase != splashDone {
		view := tea.NewView(renderSplash(m.splashLines, m.width, m.height))
		view.AltScreen = true
		return view
	}

	var body string
	if m.width > 0 && m.width < compactWidthThreshold {
		footer := strings.Join(m.footerLines(), "\n")
		body = m.renderCompact() + "\n" + m.content.View() + "\n" + footer
	} else {
		body = m.renderFrame()
	}

	view := tea.NewView(body)
	view.AltScreen = true
	return view
}

// RunTUI launches the interactive Bubbletea program.
func RunTUI(realTools, npxHistory []model.Tool, diff registry.Diff, warnings []scanner.Warning, regPath string) error {
	p := tea.NewProgram(newTUIModel(realTools, npxHistory, diff, warnings, regPath))
	_, err := p.Run()
	return err
}
