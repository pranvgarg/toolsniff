package output

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/pranvgarg/toolsniff/model"
	"github.com/pranvgarg/toolsniff/registry"
)

// reportTUIModel contains only v2 report interaction state. It is hosted by
// tuiModel for the application entry point so the established shell remains
// the owner of lifecycle, layout, and chrome. uiMode carries the resolved
// config ui.mode ("v2"/"v3"); it lives here because the shell re-reads it on
// every tab move and every sync.
type reportTUIModel struct {
	report      ObservationReport
	state       FilterState
	drawer      FilterDrawer
	filtering   bool
	filterInput string
	detail      *DetailViewModel
	status      string
	selected    int
	rows        []InventoryRow
	width       int
	height      int
	uiMode      string
}

// NewObservationTUIModel creates an additive report-backed Bubble Tea model.
func NewObservationTUIModel(report ObservationReport) tea.Model {
	model := newObservationTUIModel(report, TUIOptions{})
	return &model
}

// NewReportTUIModel is an alias with a shorter name for application adapters.
func NewReportTUIModel(report ObservationReport) tea.Model {
	return NewObservationTUIModel(report)
}

func newReportTUIModel(report ObservationReport, mode string) reportTUIModel {
	state := NewFilterState()
	// The landing view is the mode's first tab, which in v2 is the overview
	// dashboard rather than a flat list: the first question is "what's on this
	// machine", not "here are 200 rows".
	state.View = ViewCategory(reportTabsForMode(mode)[0])
	model := reportTUIModel{report: report, state: state, drawer: NewFilterDrawer(state), uiMode: mode}
	model.rebuildRows()
	return model
}

// UI mode ids, matching config's ui.mode values.
const (
	uiModeV2 = "v2"
	uiModeV3 = "v3"
)

// v2ReportTabs is the kind-first navigation, ordered most useful first. It leads
// with *what each thing is* rather than with what state a scanner filed it
// under; the status lenses (all/installed/available/history) live on in the
// filter drawer as `view:` values. See output/kinds.go for each tab's meaning.
var v2ReportTabs = []string{
	string(ViewOverview),
	string(ViewCLI),
	string(ViewPackages),
	string(ViewApplications),
	string(ViewPathExecutables),
	string(ViewNpxHistory),
	string(ViewChanges),
	string(ViewIssues),
}

// v3ReportTabs is the intent-first navigation: four tabs named for what the user
// came to do, not for what kind of thing a row is.
var v3ReportTabs = []string{
	string(ViewManage),
	string(ViewDiscover),
	string(ViewReview),
	string(ViewHealth),
}

// reportTabsForMode is the tab set for a config ui.mode value. The returned
// slice is shared and must not be mutated -- callers that keep it (the shell's
// own tabs) copy it first. An unset or unrecognised mode is v2: navigation is
// chrome, and a typo in the config file should not leave the user without it.
func reportTabsForMode(mode string) []string {
	if mode == uiModeV3 {
		return v3ReportTabs
	}
	return v2ReportTabs
}

// newObservationTUIModel puts the v2 state inside the established TUI shell.
// Keeping construction here makes the additive report model usable on its own
// in tests and by adapters while the application gets the full TUI chrome.
func newObservationTUIModel(report ObservationReport, options TUIOptions) tuiModel {
	shell := newTUIModel(nil, nil, nil, registry.Diff{}, nil, options)
	state := newReportTUIModel(report, options.UIMode)
	shell.report = &state
	shell.reportWarnings = append([]string(nil), report.Warnings...)
	shell.tabs = append([]string(nil), reportTabsForMode(options.UIMode)...)
	// keyMap.FullHelp builds the "?" digit list but has no model to ask which
	// tabs exist, so the resolved set is handed to it here.
	shell.keys.reportTabs = shell.tabs
	shell.toolsBySrc = reportSidebarCounts(report, options.UIMode)
	shell.activeTab = reportTabIndex(options.UIMode, state.state.View)
	shell.syncReportShell()
	return shell
}

// RunObservationTUI launches the v2 report TUI without changing RunTUI.
func RunObservationTUI(report ObservationReport, options TUIOptions) error {
	model := newObservationTUIModel(report, options)
	program := tea.NewProgram(&model)
	_, err := program.Run()
	return err
}

// reportSidebarCounts fills the shell's count map for every tab in the active
// mode. Counts come from CountForView, the same predicate the panes filter
// with, so a sidebar number and its pane can never disagree.
func reportSidebarCounts(report ObservationReport, mode string) map[string][]model.Tool {
	tabs := reportTabsForMode(mode)
	result := make(map[string][]model.Tool, len(tabs))
	for _, tab := range tabs {
		result[tab] = make([]model.Tool, CountForView(report, ViewCategory(tab)))
	}
	return result
}

// reportTabIndex maps a view onto its tab in the active mode. A status lens
// reached through the filter drawer has no tab of its own, so it keeps the
// highlight on the tab that holds the same rows rather than snapping back to
// the first one. The lookup is a search rather than a recursive call because a
// fallback view need not itself be a tab: in v3, "on your PATH" and "npx
// history" are both folded into Discover.
func reportTabIndex(mode string, view ViewCategory) int {
	tabs := reportTabsForMode(mode)
	for index, tab := range tabs {
		if tab == string(view) {
			return index
		}
	}

	var fallback ViewCategory
	switch view {
	case ViewAvailable:
		fallback = ViewPathExecutables
	case ViewHistory:
		fallback = ViewNpxHistory
	default:
		return 0
	}
	if mode == uiModeV3 {
		fallback = ViewDiscover
	}
	for index, tab := range tabs {
		if tab == string(fallback) {
			return index
		}
	}
	return 0
}

func reportViewForTab(mode string, index int) ViewCategory {
	tabs := reportTabsForMode(mode)
	if index < 0 || index >= len(tabs) {
		return ViewCategory(tabs[0])
	}
	return ViewCategory(tabs[index])
}

func (m tuiModel) updateReport(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		keyName := keyMsg.String()
		m.statusMsg = ""

		// Quit remains a shell concern so it works from every v2 overlay.
		if keyName == "ctrl+c" || keyName == "q" {
			return m, tea.Quit
		}

		overlayOpen := m.report.filtering || m.report.drawer.Open
		if !overlayOpen {
			switch {
			case key.Matches(keyMsg, m.keys.Theme):
				m.openThemePicker()
				return m, nil
			case key.Matches(keyMsg, m.keys.Help):
				m.help.ShowAll = !m.help.ShowAll
				m.resizeContent()
				return m, nil
			case key.Matches(keyMsg, m.keys.Save):
				if err := registry.SaveObservations(m.regPath, m.report.report.Installed); err != nil {
					m.statusMsg = "save failed: " + err.Error()
				} else if err := registry.SaveObservations(registry.AvailabilityPath(m.regPath), m.report.report.Available); err != nil {
					m.statusMsg = "save failed: " + err.Error()
				} else {
					m.statusMsg = fmt.Sprintf("saved baseline: %d installed, %d available", len(m.report.report.Installed), len(m.report.report.Available))
				}
				m.report.status = m.statusMsg
				m.syncReportShell()
				m.resizeContent()
				return m, nil
			case key.Matches(keyMsg, m.keys.NextTab):
				tabs := reportTabsForMode(m.report.uiMode)
				index := (reportTabIndex(m.report.uiMode, m.report.state.View) + 1) % len(tabs)
				m.report.state.View = reportViewForTab(m.report.uiMode, index)
				m.report.drawer.State = m.report.state
				m.report.rebuildRows()
				m.syncReportShell()
				return m, nil
			case key.Matches(keyMsg, m.keys.PrevTab):
				tabs := reportTabsForMode(m.report.uiMode)
				index := (reportTabIndex(m.report.uiMode, m.report.state.View) - 1 + len(tabs)) % len(tabs)
				m.report.state.View = reportViewForTab(m.report.uiMode, index)
				m.report.drawer.State = m.report.state
				m.report.rebuildRows()
				m.syncReportShell()
				return m, nil
			case key.Matches(keyMsg, m.keys.JumpTab):
				if index := int(keyName[0] - '1'); index >= 0 && index < len(reportTabsForMode(m.report.uiMode)) {
					m.report.state.View = reportViewForTab(m.report.uiMode, index)
					m.report.drawer.State = m.report.state
					m.report.rebuildRows()
					m.syncReportShell()
				}
				return m, nil
			}
		}
	}

	updated, cmd := m.report.Update(msg)
	if reportModel, ok := updated.(*reportTUIModel); ok {
		m.report = reportModel
	}
	m.syncReportShell()
	m.resizeContent()
	return m, cmd
}

func (m *tuiModel) syncReportShell() {
	if m.report == nil {
		return
	}
	m.activeTab = reportTabIndex(m.report.uiMode, m.report.state.View)
	m.statusMsg = m.report.status
}

// reportContentLines renders the v2 content pane as exactly height lines of
// exactly width cells: the detail pane, the filter drawer, the overview
// dashboard, the change list, or a kind-first inventory pane, depending on what
// the user has open.
func (m tuiModel) reportContentLines(width, height int) []string {
	if m.report.detail != nil {
		return padPane(renderDetailLines(*m.report.detail, width, m.styles), width, height)
	}
	if m.report.drawer.Open {
		return padPane(renderDrawerLines(m.report.drawer, m.styles), width, height)
	}

	// The filter line only appears while the user is typing or has constraints
	// applied; otherwise the pane's first line is spent saying what the view is
	// in plain English, which is worth more than an empty chip list.
	var lines []string
	if m.report.filtering {
		lines = append(lines, fitWidth(m.styles.Badge.Render("/")+m.styles.Body.Render(m.report.filterInput)+
			m.styles.Footer.Render(" ▏esc clear · enter apply"), width))
	} else if len(FilterChips(reportFilterChipState(m.report.state))) > 0 {
		lines = append(lines, fitWidth(m.styles.Footer.Render(FilterSummary(m.report.state, len(m.report.rows))), width))
	}

	if m.report.state.View == ViewOverview {
		body := height - len(lines)
		if body < 1 {
			body = 1
		}
		return padPane(append(lines, renderOverview(m.report.report, m.styles, width, body)...), width, height)
	}

	// Every non-overview pane leads with its caption: the view's title, its
	// count, and what it means. This is where "available" is spelled out as
	// "On your PATH · 9 · not installed by a manager".
	lines = append(lines, fitWidth(
		m.styles.Badge.Render(ViewLabel(m.report.state.View))+
			m.styles.Meta.Render(" · "+itoa(len(m.report.rows))+" · "+ViewMeaning(m.report.state.View)), width))

	body := height - len(lines)
	if body < 1 {
		body = 1
	}
	switch {
	case len(m.report.rows) == 0:
		// With no filters applied an empty kind view is a fact about the
		// machine, not a failed search, and it should say so in those words.
		message := EmptyResultMessage(m.report.state, 0)
		if len(FilterChips(reportFilterChipState(m.report.state))) == 0 {
			message = "Nothing of this sort found on this machine."
		}
		lines = append(lines, m.styles.EmptyState.Render(message))
	case m.report.state.View == ViewChanges || m.report.state.View == ViewIssues:
		lines = append(lines, renderChangeLines(m.report.report.Changes, m.styles)...)
	case groupedView(m.report.state.View):
		lines = append(lines, m.groupedInventoryPane(m.report.rows, m.report.selected, width, body)...)
	default:
		lines = append(lines, m.inventoryPane(m.report.rows, m.report.selected, width, body)...)
	}
	return padPane(lines, width, height)
}

// reportFilterChipState hides the view chip when deciding whether the filter
// line is worth a row: the caption already says which view is open, so a bare
// "view:packages" chip is not new information.
func reportFilterChipState(state FilterState) FilterState {
	state.View = ViewAll
	return state
}

func (m *reportTUIModel) Init() tea.Cmd { return nil }

func (m *reportTUIModel) rebuildRows() {
	m.rows = FilterReport(m.report, m.state)
	if m.selected >= len(m.rows) {
		m.selected = len(m.rows) - 1
	}
	if m.selected < 0 && len(m.rows) > 0 {
		m.selected = 0
	}
}

func (m *reportTUIModel) selectedObservation() (model.Observation, bool) {
	if m.selected < 0 || m.selected >= len(m.rows) {
		return model.Observation{}, false
	}
	for _, observation := range m.report.AllObservations() {
		if observation.ID == m.rows[m.selected].ObservationID {
			return observation, true
		}
	}
	return model.Observation{}, false
}

func (m *reportTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.width, m.height = size.Width, size.Height
		return m, nil
	}
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	key := keyMsg.String()
	if m.filtering {
		switch key {
		case "esc":
			m.filtering = false
			m.filterInput = ""
		case "enter":
			state, err := ParseFilter(m.filterInput)
			if err != nil {
				m.status = err.Error()
				return m, nil
			}
			state.View = m.state.View
			m.state = state
			m.drawer.State = state
			m.filtering = false
			m.rebuildRows()
		case "backspace":
			input := []rune(m.filterInput)
			if len(input) > 0 {
				m.filterInput = string(input[:len(input)-1])
			}
		default:
			if text := keyMsg.Key().Text; text != "" {
				m.filterInput += text
			}
		}
		return m, nil
	}
	if m.drawer.Open {
		switch key {
		case "esc":
			m.drawer.CloseDrawer()
		case "enter":
			if err := m.drawer.Apply(); err != nil {
				m.status = err.Error()
				return m, nil
			}
			m.state = m.drawer.State
			m.rebuildRows()
		case "backspace":
			input := []rune(m.drawer.Draft)
			if len(input) > 0 {
				m.drawer.Draft = string(input[:len(input)-1])
			}
		case "c":
			m.drawer.Clear()
		default:
			if text := keyMsg.Key().Text; text != "" {
				m.drawer.Draft += text
			}
		}
		return m, nil
	}

	switch key {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "/":
		m.filtering = true
		m.filterInput = m.state.Text
	case "f":
		m.drawer.State = m.state
		m.drawer.OpenDrawer()
	case "esc":
		if m.detail != nil {
			m.detail = nil
		} else if m.drawer.Open {
			m.drawer.CloseDrawer()
		} else if m.state.Text != "" || len(FilterChips(m.state)) > 0 {
			m.state = ClearFilters(m.state)
			m.drawer.State = m.state
			m.rebuildRows()
		}
	case "enter":
		if observation, ok := m.selectedObservation(); ok {
			detail := BuildDetailViewModel(observation)
			m.detail = &detail
		}
	case "d":
		m.state.View = ViewChanges
		m.rebuildRows()
	case "i":
		m.state.View = ViewIssues
		m.rebuildRows()
	case "a":
		m.state.View = ViewAll
		m.rebuildRows()
	case "up", "k":
		if m.selected > 0 {
			m.selected--
		}
	case "down", "j":
		if m.selected+1 < len(m.rows) {
			m.selected++
		}
	case "p":
		if observation, ok := m.selectedObservation(); ok {
			if path := CopySelectedPath(observation); path != "" {
				m.status = "path ready to copy: " + path
			} else {
				m.status = "selected observation has no location"
			}
		}
	case "c":
		if observation, ok := m.selectedObservation(); ok {
			if _, err := CopySelectedObservationJSON(observation); err != nil {
				m.status = "copy failed: " + err.Error()
			} else {
				m.status = "observation JSON ready to copy"
			}
		}
	case "o":
		if observation, ok := m.selectedObservation(); ok {
			if command, err := RevealLocationCommand(CopySelectedPath(observation)); err != nil {
				m.status = err.Error()
			} else {
				m.status = "reveal command ready: " + strings.Join(command, " ")
			}
		}
	case "y":
		// Yank the primary per-kind command (upgrade/uninstall/open/reveal,
		// whichever KindActions offers first). Nothing is executed; the
		// command is handed to the status line for the user to copy.
		if observation, ok := m.selectedObservation(); ok {
			if command, ok := PrimaryActionCommand(observation); ok {
				m.status = "command ready to copy: " + strings.Join(command, " ")
			} else {
				m.status = "no runnable action for this entry"
			}
		}
	}
	return m, nil
}

func (m *reportTUIModel) View() tea.View {
	width, height := m.width, m.height
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	if m.detail != nil {
		return tea.NewView(lipgloss.NewStyle().Width(width).MaxHeight(height).Render(RenderDetailView(*m.detail)))
	}
	if m.drawer.Open {
		return tea.NewView(lipgloss.NewStyle().Width(width).MaxHeight(height).Render(m.drawer.View()))
	}

	// The plain-text adapter view leads with the same caption the styled pane
	// does -- what the open view is, in words -- then keeps the raw per-bucket
	// counts below it, which is the surface adapters and scripts read.
	lines := []string{
		fmt.Sprintf("toolsniff inventory v%d", m.report.SchemaVersion),
		ViewCaption(m.state.View, len(m.rows)),
		viewCounts(m.report),
	}
	if m.filtering {
		lines = append(lines, "Filter: "+m.filterInput)
	} else {
		lines = append(lines, FilterSummary(m.state, len(m.rows)))
	}
	if len(m.rows) == 0 {
		lines = append(lines, EmptyResultMessage(m.state, 0))
	} else if m.state.View == ViewChanges || m.state.View == ViewIssues {
		lines = append(lines, RenderChangeReport(m.report.Changes))
	} else {
		lines = append(lines, responsiveRows(m.rows, width)...)
	}
	if m.status != "" {
		lines = append(lines, "", m.status)
	}
	for _, warning := range m.report.Warnings {
		lines = append(lines, "warning: "+warning)
	}
	return tea.NewView(lipgloss.NewStyle().Width(width).MaxHeight(height).Render(strings.Join(lines, "\n")))
}

// ResponsiveRowData gives adapters the narrow, medium, and wide row fields
// without requiring the TUI model. It never substitutes a location for a
// version.
func ResponsiveRowData(row InventoryRow, width int) []string {
	kind := kindLabel(row.Kind)
	switch {
	case width < 45:
		return []string{row.Name, row.Version}
	case width < 60:
		return []string{row.Name, row.Version, kind}
	case width < 75:
		return []string{row.Name, row.Version, kind, row.Status}
	default:
		return []string{row.Name, row.Version, kind, row.Status, row.Source}
	}
}

// responsiveHeaders mirrors ResponsiveRowData's ladder. It is spelled out
// rather than derived from a synthetic row, because "Kind" is a column title
// and kindLabel only knows how to name actual kinds.
func responsiveHeaders(width int) []string {
	switch {
	case width < 45:
		return []string{"Name", "Version"}
	case width < 60:
		return []string{"Name", "Version", "Kind"}
	case width < 75:
		return []string{"Name", "Version", "Kind", "Status"}
	default:
		return []string{"Name", "Version", "Kind", "Status", "Source"}
	}
}

func responsiveRows(rows []InventoryRow, width int) []string {
	lines := make([]string, 0, len(rows)+1)
	lines = append(lines, strings.Join(responsiveHeaders(width), " | "))
	for index, row := range rows {
		prefix := "  "
		if index == 0 {
			prefix = "> "
		}
		lines = append(lines, prefix+strings.Join(ResponsiveRowData(row, width), " | "))
	}
	return lines
}

func viewCounts(report ObservationReport) string {
	return fmt.Sprintf("ALL %d  INSTALLED %d  AVAILABLE %d  CHANGES %d  ISSUES %d  HISTORY %d", len(report.AllObservations()), len(report.Installed), len(report.Available), len(report.Changes.Events()), len(report.Changes.Broken)+len(report.Changes.Shadowed), len(report.History))
}
