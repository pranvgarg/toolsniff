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

// reportTUIModel contains only report interaction state. It is hosted by
// tuiModel for the application entry point so the established shell remains
// the owner of lifecycle, layout, and chrome. uiMode carries the resolved
// config ui.mode ("legacy"/"intent"); it lives here because the shell re-reads it on
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
	// selectedSet is multi-select: the rows the user has marked for a bulk
	// action, keyed by their flat index in rows -- the same index selected uses.
	// It lives beside the cursor rather than replacing it, so every single-select
	// key keeps working unchanged and marking is purely additional. Positional
	// keys only stay meaningful while rows does, so rebuildRows drops them.
	selectedSet map[int]bool
	rows        []InventoryRow
	width       int
	height      int
	uiMode      string
}

// focusLayer is which of the two navigation layers the keyboard is driving.
// The model is a file manager's: the sidebar chooses *which pane* is open, the
// pane chooses *which row* is selected, and the movement keys mean whichever of
// those two the focus is on. Without it the tab strip and the row cursor both
// answered to the same keys, and nothing walked back out to the sidebar.
type focusLayer int

const (
	// focusSidebar is home: ←/→ and ↑/↓ walk the tab list, enter opens the tab.
	focusSidebar focusLayer = iota
	// focusPane is inside the open view: ↑/↓ walk the rows, enter opens the
	// selected row's detail, esc returns to the sidebar.
	focusPane
)

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
	// The landing view is the mode's first tab, which in legacy is the overview
	// dashboard rather than a flat list: the first question is "what's on this
	// machine", not "here are 200 rows".
	state.View = ViewCategory(reportTabsForMode(mode)[0])
	model := reportTUIModel{
		report:      report,
		state:       state,
		drawer:      NewFilterDrawer(state),
		selectedSet: map[int]bool{},
		uiMode:      mode,
	}
	model.rebuildRows()
	return model
}

// UI mode ids, matching config's ui.mode values.
const (
	uiModeLegacy = "legacy"
	uiModeIntent = "intent"
)

// UIModeLegacy is the TUIOptions.UIMode value for the original eight-tab
// layout. It is exported so callers that override the configured ui.mode --
// the --legacy-tabs rollback flag -- name the same value reportTabsForMode
// reads instead of repeating the string literal.
const UIModeLegacy = uiModeLegacy

// legacyReportTabs is the kind-first navigation, ordered most useful first. It
// leads with *what each thing is* rather than with what state a scanner filed
// it under; the status lenses (all/installed/available/history) live on in the
// filter drawer as `view:` values. See output/kinds.go for each tab's meaning.
var legacyReportTabs = []string{
	string(ViewOverview),
	string(ViewCLI),
	string(ViewPackages),
	string(ViewApplications),
	string(ViewPathExecutables),
	string(ViewNpxHistory),
	string(ViewChanges),
	string(ViewIssues),
}

// intentReportTabs is the intent-first navigation: four tabs named for what the
// user came to do, not for what kind of thing a row is.
var intentReportTabs = []string{
	string(ViewManage),
	string(ViewDiscover),
	string(ViewReview),
	string(ViewHealth),
}

// reportTabsForMode is the tab set for a config ui.mode value. The returned
// slice is shared and must not be mutated -- callers that keep it (the shell's
// own tabs) copy it first. An unset or unrecognised mode is legacy: navigation
// is chrome, and a typo in the config file should not leave the user without it.
func reportTabsForMode(mode string) []string {
	if mode == uiModeIntent {
		return intentReportTabs
	}
	return legacyReportTabs
}

// newObservationTUIModel puts the report state inside the established TUI shell.
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

// RunObservationTUI launches the report TUI without changing RunTUI.
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
// fallback view need not itself be a tab: in intent mode, "on your PATH" and "npx
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
	if mode == uiModeIntent {
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

		// Quit remains a shell concern so it works from every report overlay.
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
			case key.Matches(keyMsg, m.keys.Focus):
				m.toggleFocus()
				return m, nil
			case key.Matches(keyMsg, m.keys.NextTab):
				// ←/→ are the sidebar's own keys now. From inside a pane they do
				// nothing: the way out is esc, and a stray arrow should not move
				// the ground out from under the row the user is reading.
				if m.focus == focusSidebar {
					m.moveSidebar(1)
				}
				return m, nil
			case key.Matches(keyMsg, m.keys.PrevTab):
				if m.focus == focusSidebar {
					m.moveSidebar(-1)
				}
				return m, nil
			case m.focus == focusSidebar && key.Matches(keyMsg, m.keys.Down):
				// ↑/↓ are contextual: on the sidebar they walk the tab list, in the
				// pane they walk the rows (reportTUIModel.Update owns that half).
				m.moveSidebar(1)
				return m, nil
			case m.focus == focusSidebar && key.Matches(keyMsg, m.keys.Up):
				m.moveSidebar(-1)
				return m, nil
			case m.focus == focusSidebar && key.Matches(keyMsg, m.keys.Open):
				// enter is "go one level in". From the sidebar that means the tab
				// under the selection, which is already the open view -- so the only
				// thing left to move is the focus.
				m.openReportView(reportViewForTab(m.report.uiMode, m.activeTab))
				m.focus = focusPane
				return m, nil
			case key.Matches(keyMsg, m.keys.Back):
				// esc is "go one level out", and the report's own chain (detail →
				// marks → drawer → filters) is the inner half of it. Only once that
				// has nothing left to close does the focus itself walk out, back to
				// the sidebar -- which is the way home this UI did not have.
				if !m.report.dismissTopLayer() && m.focus == focusPane {
					m.focus = focusSidebar
				}
				m.syncReportShell()
				m.resizeContent()
				return m, nil
			case key.Matches(keyMsg, m.keys.JumpTab):
				// One digit ladder for both modes: legacy answers 1-8, intent answers 1-4
				// (Manage/Discover/Review/Health) and ignores 5-9, because the bound
				// is the active mode's tab count.
				if index := int(keyName[0] - '1'); index >= 0 && index < len(reportTabsForMode(m.report.uiMode)) {
					m.openReportView(reportViewForTab(m.report.uiMode, index))
					// A digit names a destination, not a direction: it lands the user
					// in the pane ready to move, exactly as enter from the sidebar does.
					m.focus = focusPane
				}
				return m, nil
			case key.Matches(keyMsg, m.keys.JumpManage):
				// Intent-first navigation only. legacy's tabs are kind-first and have no
				// single "everything a manager installed" pane to land on, so rather
				// than picking an arbitrary near-miss the key stays inert there.
				if m.report.uiMode == uiModeIntent {
					m.openReportView(ViewManage)
					m.focus = focusPane
				}
				return m, nil
			case key.Matches(keyMsg, m.keys.UpdateCopy):
				return m, m.yankSelected(PrimaryActionCommand, "no runnable action for this entry")
			case key.Matches(keyMsg, m.keys.RemoveCopy):
				return m, m.yankSelected(PrimaryRemoveCommand, "nothing here knows how to uninstall this entry")
			case key.Matches(keyMsg, m.keys.UpdateCopyAll):
				return m, m.yankBulk()
			case key.Matches(keyMsg, m.keys.Mark):
				m.report.toggleMark(m.report.selected)
				m.setReportStatus(markStatus(len(m.report.selectedSet)))
				return m, nil
			case key.Matches(keyMsg, m.keys.MarkAll):
				m.report.toggleMarkAll()
				m.setReportStatus(markStatus(len(m.report.selectedSet)))
				return m, nil
			case key.Matches(keyMsg, m.keys.SortBySize):
				m.report.state.SortBySize = !m.report.state.SortBySize
				if m.report.state.SortBySize {
					// Populate sizes once, when the sort is toggled on, not on every repaint
					populateRowSizes(m.report.rows)
				}
				// Re-sort using the view's sort function with the new SortBySize state
				m.report.rows = sortRowsForViewWithSize(m.report.rows, m.report.state.View, m.report.state.SortBySize)
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

// openReportView switches the open pane, keeping the filter drawer's copy of the
// state and the shell's tab highlight in step. Every navigation key routes
// through here so none of them can forget one of the three.
func (m *tuiModel) openReportView(view ViewCategory) {
	m.report.state.View = view
	m.report.drawer.State = m.report.state
	m.report.rebuildRows()
	m.syncReportShell()
}

// moveSidebar walks the sidebar's selection by delta tabs, wrapping, and opens
// what it lands on. Moving the selection and opening the pane are one act here:
// the pane is the sidebar's preview, so there is nothing to "confirm". What
// enter adds is the focus move, which is why this does not touch m.focus.
func (m *tuiModel) moveSidebar(delta int) {
	tabs := reportTabsForMode(m.report.uiMode)
	index := (reportTabIndex(m.report.uiMode, m.report.state.View) + delta + len(tabs)) % len(tabs)
	m.openReportView(reportViewForTab(m.report.uiMode, index))
}

// toggleFocus flips which layer the keyboard drives. It is the keyboard's
// equivalent of clicking on the sidebar versus clicking on a row.
func (m *tuiModel) toggleFocus() {
	if m.focus == focusSidebar {
		m.focus = focusPane
		return
	}
	m.focus = focusSidebar
}

// sidebarFocused reports whether the keyboard is driving the sidebar, and is
// what the sidebar renders its focus marker from. The legacy per-source TUI has
// no second layer to move into -- ←/→ is the whole of its navigation -- so its
// sidebar always reads as focused rather than as permanently handed off.
func (m tuiModel) sidebarFocused() bool {
	return m.report == nil || m.focus == focusSidebar
}

// setReportStatus is the single way the action keys speak: the report owns the
// message and syncReportShell mirrors it onto the shell, so the two can't show
// different text. The re-layout is needed because the status line is part of the
// footer's height budget.
func (m *tuiModel) setReportStatus(text string) {
	m.report.status = text
	m.syncReportShell()
	m.resizeContent()
}

// yankSelected copies the selected row's command for one action verb and reports
// what happened either way. The verb is passed in as the accessor from
// output/actions.go that defines it, so this function never decides what "update"
// or "remove" means for a given manager -- it only moves the result.
func (m *tuiModel) yankSelected(command func(model.Observation) ([]string, bool), missing string) tea.Cmd {
	observation, ok := m.report.selectedObservation()
	if !ok {
		m.setReportStatus("no row selected")
		return nil
	}
	argv, ok := command(observation)
	if !ok {
		m.setReportStatus(missing)
		return nil
	}
	line := strings.Join(argv, " ")
	m.setReportStatus("copied: " + line)
	return copyToClipboard(line)
}

// markStatus says how many rows are marked, so the mark key is never silent.
// Zero marks is a state worth naming too: it is what the user just went back to.
func markStatus(count int) string {
	if count == 0 {
		return "no rows marked"
	}
	noun := "rows"
	if count == 1 {
		noun = "row"
	}
	return fmt.Sprintf("%d %s marked", count, noun)
}

// yankBulk is what the bulk key does. Marked rows are an explicit answer to
// "which ones", so they win over the view's own contents; with nothing marked
// the key keeps its original meaning and yanks every update in the open view.
func (m *tuiModel) yankBulk() tea.Cmd {
	if len(m.report.selectedSet) > 0 {
		return m.yankMarkedActions()
	}
	return m.yankEveryUpdate()
}

// yankMarkedActions copies the primary action of every marked row, one per line
// and in row order. It yanks the primary action rather than the update because
// marking is a deliberate per-row choice: the user picked a cask, an npm package
// and a .app, and what each one is *for* differs -- KindActions decides that,
// exactly as the single-row copy key does.
func (m *tuiModel) yankMarkedActions() tea.Cmd {
	var lines []string
	for _, observation := range m.report.markedObservations() {
		if argv, ok := PrimaryActionCommand(observation); ok {
			lines = append(lines, strings.Join(argv, " "))
		}
	}
	if len(lines) == 0 {
		m.setReportStatus("none of the marked rows has a runnable action")
		return nil
	}
	noun := "commands"
	if len(lines) == 1 {
		noun = "command"
	}
	m.setReportStatus(fmt.Sprintf("copied %d %s from marked rows", len(lines), noun))
	return copyToClipboard(strings.Join(lines, "\n"))
}

// yankEveryUpdate copies one upgrade command per line for every row in the open
// view that has one, in the order the pane lists them: a paste-ready block for a
// shell, scoped to whatever the user has filtered down to rather than to the
// whole machine.
func (m *tuiModel) yankEveryUpdate() tea.Cmd {
	var lines []string
	for _, observation := range m.report.rowObservations() {
		if argv, ok := PrimaryUpdateCommand(observation); ok {
			lines = append(lines, strings.Join(argv, " "))
		}
	}
	if len(lines) == 0 {
		m.setReportStatus("nothing in this view can be updated in place")
		return nil
	}
	noun := "update commands"
	if len(lines) == 1 {
		noun = "update command"
	}
	m.setReportStatus(fmt.Sprintf("copied %d %s", len(lines), noun))
	return copyToClipboard(strings.Join(lines, "\n"))
}

func (m *tuiModel) syncReportShell() {
	if m.report == nil {
		return
	}
	m.activeTab = reportTabIndex(m.report.uiMode, m.report.state.View)
	m.statusMsg = m.report.status
}

// populateRowSizes measures every row's location once, up front, rather
// than lazily during the sort comparator -- sort.SliceStable calls its less
// function O(n log n) times, and DirectorySize is a filesystem walk; paying
// that cost once per toggle instead of once per comparison keeps a sort
// toggle on a large PATH inventory from stalling the UI.
func populateRowSizes(rows []InventoryRow) {
	for i := range rows {
		if rows[i].Path == "" {
			continue
		}
		if size, err := DirectorySize(rows[i].Path); err == nil {
			rows[i].SizeBytes = size
		}
	}
}

// reportContentLines renders the report content pane as exactly height lines of
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
	if m.report.state.View == ViewHealth {
		// Alongside the overview rather than in the switch below, for the two
		// reasons that make Health a dashboard: it writes its own title, so the
		// generic "label · count · meaning" caption would say the pane's name
		// twice; and it has no rows, so the empty-rows branch would replace four
		// truthful zeroes with "nothing of this sort found on this machine".
		body := height - len(lines)
		if body < 1 {
			body = 1
		}
		return padPane(append(lines,
			renderHealth(m.report.report, m.report.selected, m.styles, width, body)...), width, height)
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
	case m.report.state.View == ViewReview:
		// Ahead of the change-list case: Review is event-driven like Changes and
		// Issues, but it renders those events as grouped inventory rows rather
		// than as the legacy category list, so the intent views can diverge from
		// the legacy panes without disturbing them.
		lines = append(lines, renderReviewRows(m.report.rows, m.report.selected, m.report.selectedSet, width, body, m.styles)...)
	case m.report.state.View == ViewChanges || m.report.state.View == ViewIssues:
		lines = append(lines, renderChangeLines(m.report.report.Changes, m.styles)...)
	case m.report.state.View == ViewManage:
		// Ahead of the general grouped case: Manage is a grouped view, but it
		// owns its own renderer so the intent views can diverge from the
		// kind-first panes without disturbing them.
		lines = append(lines, renderManageRows(m.report.rows, m.report.selected, m.report.selectedSet, width, body, m.styles)...)
	case m.report.state.View == ViewDiscover:
		// Manage's complement, and grouped by directory rather than by manager,
		// so it owns its renderer for the same reason Manage does. The tip is
		// derived from the whole report -- which manager owns the most of this
		// machine is not a fact the filtered rows can answer.
		lines = append(lines, renderDiscoverRows(m.report.rows,
			discoverSuggestion(m.report.report, m.report.rows), m.report.selected, m.report.selectedSet, width, body, m.styles)...)
	case groupedView(m.report.state.View):
		lines = append(lines, m.groupedInventoryPane(m.report.rows, m.report.selected, m.report.selectedSet, width, body)...)
	default:
		lines = append(lines, m.inventoryPane(m.report.rows, m.report.selected, m.report.selectedSet, width, body)...)
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
	// A mark names a position in the row list, so a new row list makes every
	// existing mark a claim about rows that are no longer there. Dropping them is
	// the only answer that cannot silently act on something the user never
	// picked: after a filter or a tab change, index 4 is a different entry.
	m.clearMarks()
	if m.selected >= len(m.rows) {
		m.selected = len(m.rows) - 1
	}
	if m.selected < 0 && len(m.rows) > 0 {
		m.selected = 0
	}
}

// toggleMark adds or removes one row's mark. An index outside the current rows
// is ignored rather than stored, so the set can never name a row the pane does
// not list.
func (m *reportTUIModel) toggleMark(index int) {
	if index < 0 || index >= len(m.rows) {
		return
	}
	if m.selectedSet == nil {
		m.selectedSet = map[int]bool{}
	}
	if m.selectedSet[index] {
		delete(m.selectedSet, index)
		return
	}
	m.selectedSet[index] = true
}

// toggleMarkAll marks every row in the open view, or clears the set when they
// are already all marked -- one key for "all of these" and for taking it back.
func (m *reportTUIModel) toggleMarkAll() {
	if len(m.rows) == 0 {
		return
	}
	if len(m.selectedSet) == len(m.rows) {
		m.clearMarks()
		return
	}
	m.selectedSet = make(map[int]bool, len(m.rows))
	for index := range m.rows {
		m.selectedSet[index] = true
	}
}

// dismissTopLayer closes the topmost thing esc can close and says whether it
// closed anything. Marks are cleared ahead of the filter but behind the detail
// pane: esc has always meant "leave what is open" first, and a drawer the user
// is reading is more in the way than a set they cannot see from inside it.
//
// The report answers "was there anything to leave" so the shell can decide what
// esc means when there wasn't -- there is one chain, not one here and a copy of
// its condition in updateReport.
//
// The view chip is excluded from the filter test for the same reason
// reportContentLines excludes it: a view is always open, so counting it would
// make "there is a filter to clear" permanently true and leave esc with nothing
// left to hand back.
func (m *reportTUIModel) dismissTopLayer() bool {
	switch {
	case m.detail != nil:
		m.detail = nil
	case len(m.selectedSet) > 0:
		m.clearMarks()
	case m.drawer.Open:
		m.drawer.CloseDrawer()
	case m.state.Text != "" || len(FilterChips(reportFilterChipState(m.state))) > 0:
		m.state = ClearFilters(m.state)
		m.drawer.State = m.state
		m.rebuildRows()
	default:
		return false
	}
	return true
}

func (m *reportTUIModel) clearMarks() {
	m.selectedSet = map[int]bool{}
}

// markedObservations resolves the marked rows to their observations in row
// order, which is the order the pane lists them and therefore the order a copied
// block of commands reads in.
func (m *reportTUIModel) markedObservations() []model.Observation {
	if len(m.selectedSet) == 0 {
		return nil
	}
	// Indexed by row rather than by rowObservations' output: a row whose
	// observation cannot be resolved is dropped from that slice, which would
	// shift every later row out from under its mark.
	all := m.report.AllObservations()
	byID := make(map[string]model.Observation, len(all))
	for _, observation := range all {
		byID[observation.ID] = observation
	}
	marked := make([]model.Observation, 0, len(m.selectedSet))
	for index, row := range m.rows {
		if !m.selectedSet[index] {
			continue
		}
		if observation, ok := byID[row.ObservationID]; ok {
			marked = append(marked, observation)
		}
	}
	return marked
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

// rowObservations resolves every row in the open view to its observation, in row
// order. It indexes the report once instead of calling selectedObservation per
// row, whose linear scan would turn a bulk yank over a few thousand entries
// quadratic.
func (m *reportTUIModel) rowObservations() []model.Observation {
	all := m.report.AllObservations()
	byID := make(map[string]model.Observation, len(all))
	for _, observation := range all {
		byID[observation.ID] = observation
	}
	observations := make([]model.Observation, 0, len(m.rows))
	for _, row := range m.rows {
		if observation, ok := byID[row.ObservationID]; ok {
			observations = append(observations, observation)
		}
	}
	return observations
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
		m.dismissTopLayer()
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
