package output

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/pranvgarg/toolsniff/model"
	"github.com/pranvgarg/toolsniff/registry"
)

// reportTUIModel contains only v2 report interaction state. It is hosted by
// tuiModel for the application entry point so the established shell remains
// the owner of lifecycle, layout, and chrome.
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

func newReportTUIModel(report ObservationReport) reportTUIModel {
	state := NewFilterState()
	model := reportTUIModel{report: report, state: state, drawer: NewFilterDrawer(state)}
	model.rebuildRows()
	return model
}

var reportTabs = []string{"all", "installed", "available", "history", "changes", "issues"}

// newObservationTUIModel puts the v2 state inside the established TUI shell.
// Keeping construction here makes the additive report model usable on its own
// in tests and by adapters while the application gets the full TUI chrome.
func newObservationTUIModel(report ObservationReport, options TUIOptions) tuiModel {
	shell := newTUIModel(nil, nil, nil, registry.Diff{}, nil, options)
	state := newReportTUIModel(report)
	shell.report = &state
	shell.reportWarnings = append([]string(nil), report.Warnings...)
	shell.tabs = append([]string(nil), reportTabs...)
	shell.toolsBySrc = reportSidebarCounts(report)
	shell.activeTab = reportTabIndex(state.state.View)
	shell.content.SetRows(reportTableRows(state.rows, 0))
	shell.content.SetColumns(reportColumnsFor(0))
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

func reportSidebarCounts(report ObservationReport) map[string][]model.Tool {
	counts := map[string]int{
		"all":       len(report.AllObservations()),
		"installed": len(report.Installed),
		"available": len(report.Available),
		"history":   len(report.History),
		"changes":   len(report.Changes.Events()),
		"issues":    len(report.Changes.Broken) + len(report.Changes.Shadowed),
	}
	result := make(map[string][]model.Tool, len(counts))
	for tab, count := range counts {
		result[tab] = make([]model.Tool, count)
	}
	return result
}

func reportTabIndex(view ViewCategory) int {
	for index, tab := range reportTabs {
		if tab == string(view) {
			return index
		}
	}
	return 0
}

func reportViewForTab(index int) ViewCategory {
	if index < 0 || index >= len(reportTabs) {
		return ViewAll
	}
	return ViewCategory(reportTabs[index])
}

func reportColumnsFor(width int) []table.Column {
	if width <= 0 {
		width = 80
	}
	nameWidth := width - contentVersionColWidth - 4
	if nameWidth < 4 {
		nameWidth = 4
	}
	switch {
	case width < 45:
		return []table.Column{{Title: "Name", Width: nameWidth}, {Title: "Version", Width: contentVersionColWidth}}
	case width < 75:
		statusWidth := 12
		nameWidth = width - contentVersionColWidth - statusWidth - 8
		if nameWidth < 4 {
			nameWidth = 4
		}
		return []table.Column{
			{Title: "Name", Width: nameWidth},
			{Title: "Version", Width: contentVersionColWidth},
			{Title: "Status", Width: statusWidth},
		}
	default:
		statusWidth, sourceWidth := 12, 16
		nameWidth = width - contentVersionColWidth - statusWidth - sourceWidth - 10
		if nameWidth < 4 {
			nameWidth = 4
		}
		return []table.Column{
			{Title: "Name", Width: nameWidth},
			{Title: "Version", Width: contentVersionColWidth},
			{Title: "Status", Width: statusWidth},
			{Title: "Source", Width: sourceWidth},
		}
	}
}

func reportTableRows(rows []InventoryRow, width int) []table.Row {
	result := make([]table.Row, 0, len(rows))
	for _, row := range rows {
		result = append(result, table.Row(ResponsiveRowData(row, width)))
	}
	return result
}

func (m tuiModel) reportLayoutWidth() int {
	if m.width > 0 && m.width < compactWidthThreshold {
		return m.width
	}
	return contentPaneWidth(m.width, sidebarWidth(m.tabs, m.toolsBySrc))
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
				m.report.state.View = reportViewForTab((reportTabIndex(m.report.state.View) + 1) % len(reportTabs))
				m.report.drawer.State = m.report.state
				m.report.rebuildRows()
				m.syncReportShell()
				return m, nil
			case key.Matches(keyMsg, m.keys.PrevTab):
				index := (reportTabIndex(m.report.state.View) - 1 + len(reportTabs)) % len(reportTabs)
				m.report.state.View = reportViewForTab(index)
				m.report.drawer.State = m.report.state
				m.report.rebuildRows()
				m.syncReportShell()
				return m, nil
			case key.Matches(keyMsg, m.keys.JumpTab):
				if index := int(keyName[0] - '1'); index >= 0 && index < len(reportTabs) {
					m.report.state.View = reportViewForTab(index)
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
	m.activeTab = reportTabIndex(m.report.state.View)
	m.statusMsg = m.report.status
	m.content.SetRows(reportTableRows(m.report.rows, m.reportLayoutWidth()))
	m.content.SetCursor(m.report.selected)
}

func (m tuiModel) reportContentLines() []string {
	if m.report.detail != nil {
		return strings.Split(RenderDetailView(*m.report.detail), "\n")
	}
	if m.report.drawer.Open {
		return strings.Split(m.report.drawer.View(), "\n")
	}
	lines := []string{}
	if m.report.filtering {
		lines = append(lines, "Filter: "+m.report.filterInput)
	} else {
		lines = append(lines, FilterSummary(m.report.state, len(m.report.rows)))
	}
	if len(m.report.rows) == 0 {
		lines = append(lines, EmptyResultMessage(m.report.state, 0))
	} else if m.report.state.View == ViewChanges || m.report.state.View == ViewIssues {
		lines = append(lines, RenderChangeReport(m.report.report.Changes))
	} else {
		lines = append(lines, strings.Split(m.content.View(), "\n")...)
	}
	return lines
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

	lines := []string{fmt.Sprintf("toolsniff inventory v%d", m.report.SchemaVersion), viewCounts(m.report)}
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
	switch {
	case width < 45:
		return []string{row.Name, row.Version}
	case width < 75:
		return []string{row.Name, row.Version, row.Status}
	default:
		return []string{row.Name, row.Version, row.Status, row.Source}
	}
}

func responsiveRows(rows []InventoryRow, width int) []string {
	lines := make([]string, 0, len(rows)+1)
	fields := ResponsiveRowData(InventoryRow{Name: "Name", Version: "Version", Status: "Status", Source: "Source"}, width)
	lines = append(lines, strings.Join(fields, " | "))
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
