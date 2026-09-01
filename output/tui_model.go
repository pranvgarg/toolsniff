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
	"charm.land/lipgloss/v2"
	"github.com/pranvgarg/toolsniff/config"
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

const newTabID = "new"

// copyToClipboard yanks text into the system clipboard. It goes through the
// terminal's OSC 52 sequence rather than pbcopy/wl-copy, which keeps the promise
// output/actions.go opens with: no process is started and no shell is involved,
// on any platform. It is a variable rather than a direct call so a test can
// capture what a binding copied with no terminal attached.
var copyToClipboard = tea.SetClipboard

type tuiModel struct {
	tabs        []string
	toolsBySrc  map[string][]model.Tool
	sources     map[string]scanner.SourceInfo
	activeTab   int
	content     table.Model
	baseRows    []InventoryRow
	styles      ThemeStyles
	filtering   bool
	filterQuery string
	realTools   []model.Tool
	available   []model.Tool
	regPath     string
	statusMsg   string
	configPath  string
	warnings    []scanner.Warning
	version     string
	theme       config.ThemeSettings
	themeNames  []string
	themePicker bool
	themeIndex  int

	// focus is which of the two navigation layers the keyboard drives: the
	// sidebar (which pane is open) or the pane (which row is selected). See
	// focusLayer in tui_v2.go for what each key means on each side.
	focus focusLayer

	keys keyMap
	help help.Model

	width, height int
	resizeTag     int

	splashPhase splashPhase
	splashLines []string
	splashTimer timer.Model

	// report is set only for the v2 entry point. The surrounding model remains
	// responsible for lifecycle, layout, chrome, and global key handling while
	// the report model owns v2 filtering and selection state.
	report         *reportTUIModel
	reportWarnings []string
}

// TUIOptions contains runtime metadata needed by the TUI. Keeping it in one
// options value avoids growing RunTUI's positional argument list.
type TUIOptions struct {
	Sources      []scanner.SourceInfo
	RegistryPath string
	Version      string
	Theme        config.ThemeSettings
	ConfigPath   string

	// UIMode is the config's ui.mode: "v2" for the eight kind-first tabs, "v3"
	// for the four intent-first ones. Empty means v2, so every existing caller
	// (and every test that passes TUIOptions{}) keeps the tabs it had.
	UIMode string
}

// keyMap defines every key binding the TUI recognizes, satisfying
// help.KeyMap so it can be rendered directly via help.Model.View. Up/Down are
// dispatched against only while the sidebar holds focus; inside a pane the row
// cursor is moved by table.Model (legacy) or reportTUIModel.Update (v2/v3),
// both of which read ↑/↓/j/k themselves.
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
	Theme   key.Binding
	Quit    key.Binding

	// The two-layer navigation keys. Focus swaps which layer the movement keys
	// drive; Open goes one level in (sidebar → pane, row → detail) and Back one
	// level out (detail → pane → sidebar), so every way in has a way out.
	Focus key.Binding
	Open  key.Binding
	Back  key.Binding

	// The action keys yank a command for the selected row onto the clipboard.
	// What each one resolves to is decided entirely by KindActions in
	// output/actions.go -- no command string is spelled anywhere near a binding.
	UpdateCopy    key.Binding
	UpdateCopyAll key.Binding
	RemoveCopy    key.Binding
	JumpManage    key.Binding

	// Multi-select. Marking rows changes nothing about what a row is; it only
	// widens what the bulk key above acts on, from "everything in this view" to
	// "the ones you picked".
	Mark    key.Binding
	MarkAll key.Binding

	// reportTabs is the tab set the "?" digit list describes. FullHelp is a
	// method on keyMap with no model to ask, so newObservationTUIModel hands
	// the mode's resolved tabs here; nil (the legacy RunTUI path) means v2.
	reportTabs []string
}

// defaultKeyMap is the TUI's fixed keybinding set.
var defaultKeyMap = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up (tab or row)"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down (tab or row)"),
	),
	PrevTab: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "prev tab"),
	),
	// "tab" moved off this binding and onto Focus: the tab strip and ←/→ were
	// two affordances for one action, and the key named after the tab strip is
	// worth more as the one that says which layer you are in.
	NextTab: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "next tab"),
	),
	Focus: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "focus sidebar/pane"),
	),
	Open: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "open (tab → pane · row → detail)"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back (detail → pane → sidebar)"),
	),
	JumpTab: key.NewBinding(
		key.WithKeys("1", "2", "3", "4", "5", "6", "7", "8", "9"),
		key.WithHelp("1-8", "jump to view"),
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
	Theme: key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "theme"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	UpdateCopy: key.NewBinding(
		key.WithKeys("u"),
		key.WithHelp("u", "copy this row's action"),
	),
	// Shift-u. A terminal may report it as a bare uppercase rune or as lowercase
	// plus a shift modifier; both stringify to "U", so one key covers both.
	UpdateCopyAll: key.NewBinding(
		key.WithKeys("U"),
		key.WithHelp("U", "copy every update here"),
	),
	RemoveCopy: key.NewBinding(
		key.WithKeys("x"),
		key.WithHelp("x", "copy this row's uninstall"),
	),
	JumpManage: key.NewBinding(
		key.WithKeys("m"),
		key.WithHelp("m", "manage"),
	),
	// Space is the mark key every list UI uses; "v" is the vi-flavoured alias for
	// the same thing. Bubble Tea v2 stringifies the space bar as "space".
	Mark: key.NewBinding(
		key.WithKeys("space", "v"),
		key.WithHelp("space/v", "mark this row"),
	),
	MarkAll: key.NewBinding(
		key.WithKeys("ctrl+a"),
		key.WithHelp("ctrl+a", "mark every row here"),
	),
}

// ShortHelp returns the handful of bindings shown in the collapsed footer.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("up", "down", "k", "j"), key.WithHelp("↑/↓", "move")),
		key.NewBinding(key.WithKeys("left", "right"), key.WithHelp("←/→", "switch tab")),
		// Short forms of Focus and Back: the collapsed strip has room for the key
		// and what it does, not for the whole ladder each one walks. "?" spells
		// them out in full.
		key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "focus")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		k.Filter,
		k.Help,
		k.Quit,
	}
}

// FullHelp returns every binding, grouped into columns, for the expanded
// help view toggled by "?". The last two columns are the view digits with what
// each view *means*, so "?" answers "where do I find my Homebrew apps" and not
// only "which key moves down". The action keys get a column of their own rather
// than lengthening the global one: the footer budgets its height from the
// tallest column, so a fifth entry there would cost every layout a row.
func (k keyMap) FullHelp() [][]key.Binding {
	digits := viewHelpBindings(k.reportTabs)
	split := (len(digits) + 1) / 2
	return [][]key.Binding{
		// The navigation column is the two-layer model read top to bottom: move
		// within a layer, move between layers, then in and out of one.
		{k.Up, k.Down, k.PrevTab, k.NextTab, k.Focus, k.Open, k.Back},
		{k.JumpTab, k.Filter, k.Diff, k.Save, k.Help, k.Theme, k.Quit},
		{k.UpdateCopy, k.UpdateCopyAll, k.RemoveCopy, k.JumpManage, k.Mark, k.MarkAll},
		digits[:split],
		digits[split:],
	}
}

// viewHelpBindings renders one binding per report tab: the digit that jumps to
// it, described by its plain-English label. An empty tab set means the caller
// is the legacy per-source TUI, which still describes the v2 views.
func viewHelpBindings(tabs []string) []key.Binding {
	if len(tabs) == 0 {
		tabs = reportTabsForMode(uiModeV2)
	}
	bindings := make([]key.Binding, 0, len(tabs))
	for index, tab := range tabs {
		digit := itoa(index + 1)
		bindings = append(bindings, key.NewBinding(
			key.WithKeys(digit),
			key.WithHelp(digit, tabDisplayLabel(tab)),
		))
	}
	return bindings
}

func newTUIModel(realTools, available, npxHistory []model.Tool, diff registry.Diff, warnings []scanner.Warning, options TUIOptions) tuiModel {
	styles := NewThemeStyles(options.Theme)
	toolsBySrc := map[string][]model.Tool{}
	for _, t := range realTools {
		toolsBySrc[t.Source] = append(toolsBySrc[t.Source], t)
	}
	for _, t := range available {
		toolsBySrc[t.Source] = append(toolsBySrc[t.Source], t)
	}
	if len(npxHistory) > 0 {
		toolsBySrc[model.SourceNPXHistory] = npxHistory
	}

	if diffHasChanges(diff) {
		newTab := append([]model.Tool{}, diff.Added...)
		newTab = append(newTab, diff.Removed...)
		for _, change := range diff.Updated {
			newTab = append(newTab, change.After)
		}
		toolsBySrc[newTabID] = newTab
	}

	sources := make(map[string]scanner.SourceInfo, len(options.Sources))
	tabs := make([]string, 0, len(options.Sources)+1)
	for _, source := range options.Sources {
		sources[source.ID] = source
		if _, ok := toolsBySrc[source.ID]; ok {
			tabs = append(tabs, source.ID)
		}
	}
	if _, ok := toolsBySrc[newTabID]; ok {
		tabs = append(tabs, newTabID)
	}
	if len(tabs) == 0 {
		tabs = []string{model.SourceNPM}
	}

	baseRows := baseInventoryRows(toolsBySrc[tabs[0]], "")
	// The bubbles table is no longer the renderer -- renderInventoryTable in
	// tui_inventory_table.go draws every list so installed/available/broken
	// can be colored per row, which bubbles/table's three-style API cannot
	// express. It is retained purely as the cursor/scroll state machine for
	// the legacy per-source view, which is why it gets rows but no styles.
	t := table.New(
		table.WithColumns([]table.Column{{Title: "Name", Width: 1}}),
		table.WithRows(cursorRows(len(baseRows))),
		table.WithFocused(true),
	)

	helpModel := help.New()
	helpModel.Styles = helpStyles(styles)

	return tuiModel{
		tabs:       tabs,
		toolsBySrc: toolsBySrc,
		sources:    sources,
		content:    t,
		baseRows:   baseRows,
		styles:     styles,
		realTools:  realTools,
		available:  available,
		regPath:    options.RegistryPath,
		warnings:   warnings,
		version:    options.Version,
		theme:      options.Theme,
		themeNames: config.ThemePresets(),
		configPath: options.ConfigPath,
		// A tab is already open on the first frame, so starting in the pane makes
		// the landing view immediately interactive: ↑/↓ move rows and enter opens
		// a detail without a preliminary "go in" keystroke. esc is the way back
		// out to the sidebar, and ←/→ still switch tabs from there.
		focus:       focusPane,
		keys:        defaultKeyMap,
		help:        helpModel,
		splashTimer: newSplashTimer(),
	}
}

// helpStyles maps the bubbles help component onto the design system so the
// footer hints share the footer's vocabulary instead of shipping their own.
func helpStyles(styles ThemeStyles) help.Styles {
	s := help.DefaultDarkStyles()
	s.ShortKey, s.FullKey = styles.FooterKey, styles.FooterKey
	s.ShortDesc, s.FullDesc = styles.Footer, styles.Footer
	s.ShortSeparator, s.FullSeparator = styles.Footer, styles.Footer
	s.Ellipsis = styles.Footer
	return s
}

// cursorRows returns n placeholder rows. The bubbles table only needs a row
// count to bound cursor movement; the visible cells come from baseRows.
func cursorRows(n int) []table.Row {
	rows := make([]table.Row, n)
	for i := range rows {
		rows[i] = table.Row{""}
	}
	return rows
}

func (m tuiModel) isInformationalTab(tab string) bool {
	info, ok := m.sources[tab]
	return ok && (info.Informational || info.Role == model.RoleHistory)
}

func (m *tuiModel) openThemePicker() {
	m.themeNames = config.ThemePresets()
	m.themeIndex = 0
	for i, name := range m.themeNames {
		if name == m.theme.Preset {
			m.themeIndex = i
			break
		}
	}
	m.themePicker = true
}

func (m *tuiModel) updateThemePicker(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.themePicker = false
		return m, nil
	case "up", "k":
		m.themeIndex = (m.themeIndex - 1 + len(m.themeNames)) % len(m.themeNames)
		return m, nil
	case "down", "j":
		m.themeIndex = (m.themeIndex + 1) % len(m.themeNames)
		return m, nil
	case "enter":
		name := m.themeNames[m.themeIndex]
		theme, err := config.ThemeSettingsForPreset(name)
		if err != nil {
			m.statusMsg = "theme failed: " + err.Error()
			m.themePicker = false
			return m, nil
		}
		m.theme = theme
		m.styles = NewThemeStyles(theme)
		m.help.Styles = helpStyles(m.styles)
		m.themePicker = false
		if err := config.SaveTheme(m.configPath, theme); err != nil {
			m.statusMsg = "theme applied, save failed: " + err.Error()
		} else {
			m.statusMsg = "theme: " + name
		}
		m.resizeContent()
		return m, nil
	}
	return m, nil
}

func (m tuiModel) renderThemePicker() string {
	width, height := m.width, m.height
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}

	lines := []string{m.styles.DetailHeading.Render("Choose a theme"), ""}
	for i, name := range m.themeNames {
		preview, err := config.ThemeSettingsForPreset(name)
		if err != nil {
			continue
		}
		previewStyles := NewThemeStyles(preview)
		// Each row previews its own palette: the accent swatch on the left,
		// then the three semantic status tones the row colors use.
		swatches := lipgloss.JoinHorizontal(
			lipgloss.Top,
			previewStyles.RowInstalled.Render(previewStyles.Glyph.Installed),
			previewStyles.RowAvailable.Render(previewStyles.Glyph.Available),
			previewStyles.RowBroken.Render(previewStyles.Glyph.Broken),
		)
		marker, label := " ", previewStyles.Tab.Render(name)
		if i == m.themeIndex {
			marker = previewStyles.SelectionBar.Render(previewStyles.Glyph.Selection)
			label = previewStyles.ActiveTab.Render(name)
		}
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, marker, " ", swatches, "  ", label))
	}
	lines = append(lines, "", m.styles.Footer.Render("↑/↓ choose · enter apply · esc cancel"))

	panelWidth := 34
	if width-4 < panelWidth {
		panelWidth = width - 4
	}
	if panelWidth < 12 {
		panelWidth = 12
	}
	panel := m.styles.Modal.Width(panelWidth).Render(strings.Join(lines, "\n"))
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, panel)
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

// baseInventoryRows builds the legacy per-source view's rows, keeping only
// those whose name case-insensitively contains filter. Routing them through
// InventoryRow is what lets the legacy pane share the semantic per-row
// coloring and the Status/Source columns with the v2 report pane.
func baseInventoryRows(tools []model.Tool, filter string) []InventoryRow {
	lowerFilter := strings.ToLower(filter)
	rows := make([]InventoryRow, 0, len(tools))
	for _, t := range tools {
		if filter != "" && !strings.Contains(strings.ToLower(t.Name), lowerFilter) {
			continue
		}
		row := InventoryRowFromObservation(t.ToObservation())
		if t.Version == "" {
			// Path fallback: the location is more informative than a bare
			// "unknown" here, and truncateTail keeps its identifying suffix.
			row.Version = versionOrPath(t)
		}
		rows = append(rows, row)
	}
	return rows
}

// rebuildContent recomputes the content rows for the active tab under the
// current filter query, resetting the cursor to the top.
func (m *tuiModel) rebuildContent() {
	m.baseRows = baseInventoryRows(m.toolsBySrc[m.tabs[m.activeTab]], m.filterQuery)
	m.content.SetRows(cursorRows(len(m.baseRows)))
	m.content.SetCursor(0)
}

// resizeContent recalculates the cursor viewport's width and height for the
// current terminal size and footer state. The footer's row count varies (1
// normally, more when the full key-binding help is expanded via "?"), so this
// must be called both on every WindowSizeMsg and whenever something else could
// change the footer's line count.
func (m *tuiModel) resizeContent() {
	footerRows := len(m.footerLines())
	if m.width > 0 && m.width < compactWidthThreshold {
		m.help.SetWidth(m.width)
		h := m.height - 1 - footerRows // compact strip + footer
		if h < 1 {
			h = 1
		}
		m.content.SetWidth(m.width)
		m.content.SetHeight(h)
		return
	}
	m.help.SetWidth(m.width - frameFooterChrome(m.styles))
	m.content.SetWidth(m.contentWidth())
	m.content.SetHeight(contentPaneHeight(m.frameHeight(), footerRows))
}

// contentLines renders the content pane as exactly height lines of exactly
// width cells. It is the single entry point both the bordered frame and the
// compact layout use, so the two never drift apart.
func (m tuiModel) contentLines(width, height int) []string {
	if m.report != nil {
		return m.reportContentLines(width, height)
	}
	// The legacy per-source TUI has no report and so no marks: multi-select is a
	// v2/v3 report-pane capability, and nil is the empty set.
	return padPane(m.inventoryPane(m.baseRows, m.content.Cursor(), nil, width, height), width, height)
}

// inventoryPane renders a flat list plus its under-full row-count badge.
func (m tuiModel) inventoryPane(rows []InventoryRow, selected int, marks rowMarks, width, height int) []string {
	return m.paneForRows(rows, selected, marks, width, height, false)
}

// groupedInventoryPane renders a list broken into counted manager sub-groups,
// used by the kind-first views where a flat count ("79 packages") is not a fact
// anyone can act on.
func (m tuiModel) groupedInventoryPane(rows []InventoryRow, selected int, marks rowMarks, width, height int) []string {
	return m.paneForRows(rows, selected, marks, width, height, true)
}

func (m tuiModel) paneForRows(rows []InventoryRow, selected int, marks rowMarks, width, height int, grouped bool) []string {
	if len(rows) == 0 {
		return []string{fitWidth(m.styles.EmptyState.Render("nothing to show here"), width)}
	}
	var lines []string
	var used int
	if grouped {
		lines, used = renderGroupedInventoryTable(rows, selected, marks, width, height, m.styles), groupedInventoryLineCount(rows)
	} else {
		lines, used = renderInventoryTable(rows, selected, marks, width, height, m.styles), len(rows)+1
	}
	// A view that doesn't fill the pane gets an explicit count on its last
	// line, so blank space below a short list reads as "that's all of them"
	// rather than as a rendering failure.
	if used < height {
		lines[height-1] = renderRowCountBadge(len(rows), len(rows), width, m.styles)
	}
	return lines
}

// padPane trims or pads lines to exactly height rows of exactly width cells.
func padPane(lines []string, width, height int) []string {
	out := make([]string, height)
	for i := range out {
		if i < len(lines) {
			out[i] = fitWidth(lines[i], width)
			continue
		}
		out[i] = fitWidth("", width)
	}
	return out
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

	if m.themePicker {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			return m.updateThemePicker(keyMsg)
		}
		return m, nil
	}

	if m.splashPhase != splashDone {
		return m.updateSplash(msg)
	}

	if m.report != nil {
		return m.updateReport(msg)
	}

	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		m.statusMsg = ""
		m.resizeContent()

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
				if strings.EqualFold(strings.TrimSpace(m.filterQuery), "theme") {
					m.filtering = false
					m.filterQuery = ""
					m.openThemePicker()
					m.resizeContent()
					return m, nil
				}
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
		case key.Matches(keyMsg, m.keys.Theme):
			m.openThemePicker()
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
			m.resizeContent()
			return m, nil
		case key.Matches(keyMsg, m.keys.Diff):
			for i, t := range m.tabs {
				if t == newTabID {
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
	if m.report != nil {
		if m.report.filtering {
			n := len(m.report.rows)
			unit := "match"
			if n != 1 {
				unit = "matches"
			}
			return fmt.Sprintf("/%s — %d %s (esc clear · enter apply)", m.report.filterInput, n, unit)
		}
		return m.help.View(m.keys)
	}
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
	// Severity first, and visually distinct from the hints: a warning row is
	// glyph-prefixed and bold-amber, a hint row is plain muted. The eye has to
	// be able to find a real problem without reading the keybindings.
	warn := func(text string) string {
		return m.styles.Warning.Render(m.styles.Glyph.Warning + " warning: " + text)
	}
	for _, w := range m.warnings {
		lines = append(lines, warn(fmt.Sprintf("%s: %v", w.Source, w.Err)))
	}
	for _, warning := range m.reportWarnings {
		lines = append(lines, warn(warning))
	}
	if m.statusMsg != "" {
		lines = append(lines, m.styles.Status.Render(m.statusMsg))
	}
	lines = append(lines, strings.Split(m.footerHint(), "\n")...)
	return lines
}

func (m tuiModel) View() tea.View {
	if m.splashPhase != splashDone {
		view := tea.NewView(renderSplash(m.splashLines, m.width, m.height, m.version, m.styles))
		view.AltScreen = true
		return view
	}

	var body string
	if m.themePicker {
		body = m.renderThemePicker()
	} else if m.width > 0 && m.width < compactWidthThreshold {
		footerLines := m.footerLines()
		height := m.frameHeight() - 1 - len(footerLines)
		if height < 1 {
			height = 1
		}
		body = lipgloss.JoinVertical(
			lipgloss.Left,
			m.renderCompact(),
			lipgloss.JoinVertical(lipgloss.Left, m.contentLines(m.frameWidth(), height)...),
			lipgloss.JoinVertical(lipgloss.Left, footerLines...),
		)
	} else {
		body = m.renderFrame()
	}

	view := tea.NewView(body)
	view.AltScreen = true
	return view
}

// RunTUI launches the interactive Bubbletea program.
func RunTUI(realTools, available, npxHistory []model.Tool, diff registry.Diff, warnings []scanner.Warning, options TUIOptions) error {
	p := tea.NewProgram(newTUIModel(realTools, available, npxHistory, diff, warnings, options))
	_, err := p.Run()
	return err
}
