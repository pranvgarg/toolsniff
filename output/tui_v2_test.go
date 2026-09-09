package output

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/pranvgarg/toolsniff/model"
	"github.com/pranvgarg/toolsniff/registry"
)

func TestFilterReportEmptyResultsExplainActiveView(t *testing.T) {
	tool := observation("npm-tool", model.RoleInstalled, model.KindPackage, model.VersionInfo{Value: "1.0.0", State: model.VersionKnown, Confidence: model.ConfidenceHigh})
	state, err := ParseFilter("source:brew-formula missing")
	if err != nil {
		t.Fatal(err)
	}
	report := NewObservationReport([]model.Observation{tool}, nil, nil, emptyObservationDiff(), nil)
	rows := FilterReport(report, state)
	if len(rows) != 0 || EmptyResultMessage(state, len(rows)) == "" {
		t.Fatalf("expected explained empty result, rows=%d", len(rows))
	}
}

func TestLargeInventoryRowsAndFilteringRemainBounded(t *testing.T) {
	observations := make([]model.Observation, 1000)
	for i := range observations {
		name := fmt.Sprintf("path-tool-%04d", i)
		observations[i] = observationWithPath(name, model.RoleAvailable, model.KindExecutable, model.VersionInfo{State: model.VersionUnknown, Confidence: model.ConfidenceLow}, "/bin/"+name)
	}
	rows := InventoryRows(observations)
	if len(rows) != 1000 || rows[999].Version != "unknown" {
		t.Fatalf("large inventory conversion failed: len=%d last=%+v", len(rows), rows[999])
	}
	state, err := ParseFilter("source:path tool-0999")
	if err != nil {
		t.Fatal(err)
	}
	if got := len(FilterRows(rows, state)); got != 1 {
		t.Fatalf("large inventory filter returned %d rows, want 1", got)
	}
	if got := ResponsiveRowData(rows[0], 40); len(got) != 2 || got[1] == "/bin/"+rows[0].Name {
		t.Fatalf("narrow row data leaked location: %#v", got)
	}
}

func TestReportTUISupportsSearchDetailsAndEsc(t *testing.T) {
	tool := observation("npm-tool", model.RoleInstalled, model.KindPackage, model.VersionInfo{Value: "1.0.0", State: model.VersionKnown, Confidence: model.ConfidenceHigh})
	m := newReportTUIModel(NewObservationReport([]model.Observation{tool}, nil, nil, emptyObservationDiff(), nil), "")
	if len(m.rows) != 1 {
		t.Fatalf("initial rows = %d", len(m.rows))
	}
	m.filterInput = "npm-tool"
	if _, err := ParseFilter(m.filterInput); err != nil {
		t.Fatal(err)
	}
	m.state.Text = m.filterInput
	m.rebuildRows()
	if len(m.rows) != 1 {
		t.Fatalf("search rows = %d", len(m.rows))
	}
	if _, ok := m.selectedObservation(); !ok {
		t.Fatal("selected observation missing")
	}
}

func TestObservationTUILandsOnKindFirstOverview(t *testing.T) {
	tool := observation("npm-tool", model.RoleInstalled, model.KindPackage, model.VersionInfo{Value: "1.0.0", State: model.VersionKnown, Confidence: model.ConfidenceHigh})
	m := newObservationTUIModel(NewObservationReport([]model.Observation{tool}, nil, nil, emptyObservationDiff(), []string{"scanner: warning"}), TUIOptions{Version: "1.2.3"})
	m.splashPhase = splashDone
	m.width, m.height = 100, 30
	m.resizeContent()

	view := m.View()
	// Landing is the overview dashboard, and the navigation is kind-first: what
	// each thing *is*, in words, never a bare "available".
	for _, want := range []string{
		m.styles.Glyph.Brand + " toolsniff", "what's on this machine",
		"CLI tools", "packages", "applications", "on your PATH", "npx history",
		"Node.js packages installed globally",
		"warning: scanner: warning",
	} {
		if !strings.Contains(view.Content, want) {
			t.Errorf("legacy overview view missing %q: %s", want, view.Content)
		}
	}
	if strings.Contains(view.Content, "available") {
		t.Errorf("the overview showed the bare word \"available\": %s", view.Content)
	}
	if !view.AltScreen {
		t.Fatal("legacy shell did not enable the alternate screen")
	}
}

func TestObservationTUIPackagesTabGroupsByManager(t *testing.T) {
	npm := observation("npm-tool", model.RoleInstalled, model.KindPackage, model.VersionInfo{Value: "1.0.0", State: model.VersionKnown, Confidence: model.ConfidenceHigh})
	brew := observation("wget", model.RoleInstalled, model.KindPackage, model.VersionInfo{Value: "1.24.5", State: model.VersionKnown, Confidence: model.ConfidenceHigh})
	brew.Origin = model.Origin{Provider: "homebrew", Manager: "formula", Package: "wget"}
	brew.ID = model.ObservationIdentity(brew)

	m := newObservationTUIModel(NewObservationReport([]model.Observation{npm, brew}, nil, nil, emptyObservationDiff(), nil), TUIOptions{})
	m.splashPhase = splashDone
	m.width, m.height = 100, 30
	m.resizeContent()

	// "3" is the packages tab in the kind-first order.
	updated, _ := m.Update(testKey("3"))
	shell := updated.(tuiModel)
	if shell.report.state.View != ViewPackages {
		t.Fatalf("jump to tab 3 selected %q, want %q", shell.report.state.View, ViewPackages)
	}

	view := shell.View().Content
	// The KIND column is part of the wide form, and each manager gets a counted
	// sub-heading so the pane's total is made of facts a user can act on.
	for _, want := range []string{
		"Packages", "· 2 · installed for you by a package manager",
		"npm packages", "Homebrew formulae",
		"npm-tool", "wget", "KIND", shell.styles.kindMarker(string(model.KindPackage)),
	} {
		if !strings.Contains(view, want) {
			t.Errorf("packages pane missing %q: %s", want, view)
		}
	}
}

func TestPathExecutableRowsNeverSayJustAvailable(t *testing.T) {
	available := observationWithPath("gh", model.RoleAvailable, model.KindExecutable,
		model.VersionInfo{State: model.VersionUnknown, Confidence: model.ConfidenceLow}, "/usr/local/bin/gh")
	report := NewObservationReport(nil, []model.Observation{available}, nil, emptyObservationDiff(), nil)
	m := newObservationTUIModel(report, TUIOptions{})
	m.splashPhase = splashDone
	m.width, m.height = 100, 24
	m.resizeContent()

	// "5" is the on-your-PATH tab.
	updated, _ := m.Update(testKey("5"))
	shell := updated.(tuiModel)
	view := shell.View().Content
	for _, want := range []string{"On your PATH", "not installed by a manager", "on PATH", "gh"} {
		if !strings.Contains(view, want) {
			t.Errorf("PATH pane missing %q: %s", want, view)
		}
	}
	if strings.Contains(view, "available") {
		t.Errorf("PATH pane showed the bare word \"available\": %s", view)
	}

	// The raw vocabulary survives underneath: the row's status, and therefore
	// the `status:` filter facet, is untouched.
	if got := shell.report.rows[0].Status; got != string(StatusAvailable) {
		t.Fatalf("row status = %q, want %q", got, StatusAvailable)
	}
	state, err := ParseFilter("status:available")
	if err != nil {
		t.Fatal(err)
	}
	if got := len(FilterReport(report, state)); got != 1 {
		t.Fatalf("status:available matched %d rows, want 1", got)
	}
}

func TestReportTabsAreKindFirstAndKeepEveryStatusLensReachable(t *testing.T) {
	want := []string{"overview", "cli-tools", "packages", "applications", "path-executables", "npx-history", "changes", "issues"}
	tabs := reportTabsForMode("legacy")
	if len(tabs) != len(want) {
		t.Fatalf("reportTabsForMode(\"legacy\") = %v, want %v", tabs, want)
	}
	for index, tab := range want {
		if tabs[index] != tab {
			t.Fatalf("reportTabsForMode(\"legacy\")[%d] = %q, want %q", index, tabs[index], tab)
		}
		if reportViewForTab("legacy", index) != ViewCategory(tab) || reportTabIndex("legacy", ViewCategory(tab)) != index {
			t.Errorf("tab %q does not round-trip through index %d", tab, index)
		}
	}

	// The status lenses the tabs replaced are still reachable as filters, which
	// is what makes the reorganisation lossless.
	installed := observation("npm-tool", model.RoleInstalled, model.KindPackage, model.VersionInfo{Value: "1.0.0", State: model.VersionKnown, Confidence: model.ConfidenceHigh})
	available := observationWithPath("gh", model.RoleAvailable, model.KindExecutable, model.VersionInfo{State: model.VersionUnknown, Confidence: model.ConfidenceLow}, "/usr/local/bin/gh")
	history := observation("create-vite", model.RoleHistory, model.KindHistory, model.VersionInfo{State: model.VersionNotApplicable, Confidence: model.ConfidenceLow})
	report := NewObservationReport([]model.Observation{installed}, []model.Observation{available}, []model.Observation{history}, emptyObservationDiff(), nil)

	for filter, wantRows := range map[string]int{
		"view:all": 3, "view:installed": 1, "view:available": 1, "view:history": 1,
		"view:packages": 1, "view:path-executables": 1, "view:npx-history": 1,
		"view:cli-tools": 0, "view:applications": 0,
	} {
		state, err := ParseFilter(filter)
		if err != nil {
			t.Fatalf("ParseFilter(%q) failed: %v", filter, err)
		}
		if got := len(FilterReport(report, state)); got != wantRows {
			t.Errorf("%s returned %d rows, want %d", filter, got, wantRows)
		}
	}
}

func TestReportTabsSwitchOnUIMode(t *testing.T) {
	legacy := reportTabsForMode("legacy")
	intent := reportTabsForMode("intent")
	if len(legacy) != 8 {
		t.Fatalf("legacy tabs = %d, want 8", len(legacy))
	}
	if len(intent) != 4 || intent[0] != "manage" || intent[1] != "discover" || intent[2] != "review" || intent[3] != "health" {
		t.Fatalf("intent tabs = %v, want [manage discover review health]", intent)
	}
}

func TestObservationTUIKeepsLegacyInteractionsInsideShell(t *testing.T) {
	tool := observation("npm-tool", model.RoleInstalled, model.KindPackage, model.VersionInfo{Value: "1.0.0", State: model.VersionKnown, Confidence: model.ConfidenceHigh})
	m := newObservationTUIModel(NewObservationReport([]model.Observation{tool}, nil, nil, emptyObservationDiff(), nil), TUIOptions{})
	m.splashPhase = splashDone
	m.width, m.height = 100, 30
	m.resizeContent()

	m.Update(testKey("/"))
	m.Update(testKey("npm"))
	m.Update(testKeyCode(tea.KeyEnter))
	if m.report.filtering || m.report.state.Text != "npm" {
		t.Fatalf("legacy filter state was not retained: filtering=%v state=%+v", m.report.filtering, m.report.state)
	}
	m.Update(testKey("esc"))
	if m.report.filtering {
		t.Fatal("escape did not leave legacy filtering")
	}
	// m.report is a pointer into the shell, so the report-side state these
	// assertions read is updated in place by Update.
	m.Update(testKeyCode(tea.KeyEnter))
	if m.report.detail == nil {
		t.Fatal("enter did not open legacy detail view")
	}
	// The detail pane has to carry the per-kind actions, not just metadata:
	// this is the difference between "here is what we found" and "here is what
	// you can do about it".
	detailView := m.View().Content
	for _, want := range []string{"npm global package", "ACTIONS", "npm update -g npm-tool", "npm uninstall -g npm-tool"} {
		if !strings.Contains(detailView, want) {
			t.Errorf("legacy detail pane missing %q: %s", want, detailView)
		}
	}

	// y yanks the primary per-kind command into the status line. Nothing runs.
	m.Update(testKey("y"))
	if want := "command ready to copy: npm update -g npm-tool"; m.report.status != want {
		t.Fatalf("y status = %q, want %q", m.report.status, want)
	}

	m.Update(testKey("esc"))
	if m.report.detail != nil {
		t.Fatal("escape did not close legacy detail view")
	}

	_, quit := m.Update(testKey("q"))
	if quit == nil {
		t.Fatal("q did not return a quit command")
	}
}

// captureClipboard swaps the OSC 52 write for a recorder, so the action-key
// tests can assert on what a binding yanked with no terminal attached. The
// returned function reports the most recent copy.
func captureClipboard(t *testing.T) func() string {
	t.Helper()
	original := copyToClipboard
	var copied string
	copyToClipboard = func(text string) tea.Cmd {
		copied = text
		return nil
	}
	t.Cleanup(func() { copyToClipboard = original })
	return func() string { return copied }
}

// intentShell builds an intent-mode shell over the given observations, sized
// and past the splash so Update behaves as it does in a running program.
func intentShell(t *testing.T, installed ...model.Observation) tuiModel {
	t.Helper()
	m := newObservationTUIModel(
		NewObservationReport(installed, nil, nil, emptyObservationDiff(), nil),
		TUIOptions{UIMode: uiModeIntent},
	)
	m.splashPhase = splashDone
	m.width, m.height = 100, 30
	m.resizeContent()
	return m
}

func brewObservation(name string) model.Observation {
	observation := observation(name, model.RoleInstalled, model.KindPackage,
		model.VersionInfo{Value: "1.0.0", State: model.VersionKnown, Confidence: model.ConfidenceHigh})
	observation.Origin = model.Origin{Provider: "homebrew", Manager: "formula", Package: name}
	observation.ID = model.ObservationIdentity(observation)
	return observation
}

func TestIntentDigitKeysJumpToTabs(t *testing.T) {
	m := intentShell(t, observation("npm-tool", model.RoleInstalled, model.KindPackage,
		model.VersionInfo{Value: "1.0.0", State: model.VersionKnown, Confidence: model.ConfidenceHigh}))

	for digit, want := range map[string]ViewCategory{
		"1": ViewManage, "2": ViewDiscover, "3": ViewReview, "4": ViewHealth,
	} {
		updated, _ := m.Update(testKey(digit))
		shell := updated.(tuiModel)
		if shell.report.state.View != want {
			t.Errorf("intent %q opened %q, want %q", digit, shell.report.state.View, want)
		}
		if got := shell.activeTab; got != reportTabIndex(uiModeIntent, want) {
			t.Errorf("intent %q left the tab highlight on %d", digit, got)
		}
	}

	// intent has four tabs, so the digits past them are inert rather than
	// wrapping onto a legacy view that this mode does not show.
	m.Update(testKey("1"))
	updated, _ := m.Update(testKey("5"))
	if view := updated.(tuiModel).report.state.View; view != ViewManage {
		t.Errorf("intent \"5\" moved off Manage to %q", view)
	}
}

func TestIntentJumpManageKeyReturnsToManage(t *testing.T) {
	m := intentShell(t, observation("npm-tool", model.RoleInstalled, model.KindPackage,
		model.VersionInfo{Value: "1.0.0", State: model.VersionKnown, Confidence: model.ConfidenceHigh}))

	m.Update(testKey("4"))
	if m.report.state.View != ViewHealth {
		t.Fatalf("setup failed: view = %q", m.report.state.View)
	}
	updated, _ := m.Update(testKey("m"))
	if view := updated.(tuiModel).report.state.View; view != ViewManage {
		t.Fatalf("m opened %q, want %q", view, ViewManage)
	}
}

func TestUpdateCopyUsesPrimaryActionCommand(t *testing.T) {
	clipboard := captureClipboard(t)
	npm := observation("npm-tool", model.RoleInstalled, model.KindPackage,
		model.VersionInfo{Value: "1.0.0", State: model.VersionKnown, Confidence: model.ConfidenceHigh})
	m := intentShell(t, npm)

	observation, ok := m.report.selectedObservation()
	if !ok {
		t.Fatal("no row selected on the Manage tab")
	}
	argv, ok := PrimaryActionCommand(observation)
	if !ok {
		t.Fatal("the npm row has no runnable primary action")
	}
	want := strings.Join(argv, " ")

	m.Update(testKey("u"))
	if got := clipboard(); got != want {
		t.Fatalf("u copied %q, want PrimaryActionCommand's %q", got, want)
	}
	if m.report.status != "copied: "+want {
		t.Fatalf("u status = %q", m.report.status)
	}
}

func TestRemoveCopyUsesPrimaryRemoveCommand(t *testing.T) {
	clipboard := captureClipboard(t)
	npm := observation("npm-tool", model.RoleInstalled, model.KindPackage,
		model.VersionInfo{Value: "1.0.0", State: model.VersionKnown, Confidence: model.ConfidenceHigh})
	m := intentShell(t, npm)

	observation, _ := m.report.selectedObservation()
	argv, ok := PrimaryRemoveCommand(observation)
	if !ok {
		t.Fatal("the npm row has no uninstall command")
	}
	want := strings.Join(argv, " ")
	// The point of a separate accessor: remove must not resolve to the update
	// that PrimaryActionCommand hands back for this same row.
	if primary, _ := PrimaryActionCommand(observation); strings.Join(primary, " ") == want {
		t.Fatal("remove and primary resolved to the same command; the test proves nothing")
	}

	m.Update(testKey("x"))
	if got := clipboard(); got != want {
		t.Fatalf("x copied %q, want PrimaryRemoveCommand's %q", got, want)
	}
}

// A macOS application has an Open action but nothing that uninstalls it, so the
// remove key must say so rather than fall through to whatever else is offered.
func TestRemoveCopyIsInertWithoutAnUninstall(t *testing.T) {
	clipboard := captureClipboard(t)
	app := observationWithPath("Xcode", model.RoleInstalled, model.KindApplication,
		model.VersionInfo{Value: "16.0", State: model.VersionKnown, Confidence: model.ConfidenceHigh},
		"/Applications/Xcode.app")
	app.Origin = model.Origin{Provider: "applications"}
	app.ID = model.ObservationIdentity(app)
	m := intentShell(t, app)

	m.Update(testKey("x"))
	if got := clipboard(); got != "" {
		t.Fatalf("x copied %q for an entry with no uninstall", got)
	}
	if m.report.status == "" || strings.HasPrefix(m.report.status, "copied") {
		t.Fatalf("x status = %q, want an explanation", m.report.status)
	}
}

func TestUpdateCopyAllYanksEveryUpdatableRow(t *testing.T) {
	clipboard := captureClipboard(t)
	npm := observation("npm-tool", model.RoleInstalled, model.KindPackage,
		model.VersionInfo{Value: "1.0.0", State: model.VersionKnown, Confidence: model.ConfidenceHigh})
	brew := brewObservation("wget")
	// pipx offers an uninstall but no in-place upgrade, so it must not appear in
	// the bulk yank even though it is a managed row on the same tab.
	pipx := observation("black", model.RoleInstalled, model.KindPackage,
		model.VersionInfo{Value: "24.1.0", State: model.VersionKnown, Confidence: model.ConfidenceHigh})
	pipx.Origin = model.Origin{Provider: "pipx", Package: "black"}
	pipx.ID = model.ObservationIdentity(pipx)

	m := intentShell(t, npm, brew, pipx)
	m.Update(testKey("U"))

	lines := strings.Split(clipboard(), "\n")
	if len(lines) != 2 {
		t.Fatalf("U copied %d lines, want 2 (the updatable rows): %q", len(lines), clipboard())
	}
	for _, observation := range []model.Observation{npm, brew} {
		argv, ok := PrimaryUpdateCommand(observation)
		if !ok {
			t.Fatalf("%s is not updatable; the fixture is wrong", observation.DisplayName)
		}
		if !strings.Contains(clipboard(), strings.Join(argv, " ")) {
			t.Errorf("U omitted %q", strings.Join(argv, " "))
		}
	}
	if _, ok := PrimaryUpdateCommand(pipx); ok {
		t.Fatal("pipx became updatable; the fixture no longer proves the filter")
	}
	if want := "copied 2 update commands"; m.report.status != want {
		t.Fatalf("U status = %q, want %q", m.report.status, want)
	}
}

// --- multi-select ------------------------------------------------------------

// markedShell is three managed rows on the Manage tab: one npm package, one
// Homebrew formula, and one pipx application. Every one of them has a runnable
// primary action, but only the first two can be upgraded in place -- which is
// what tells the two bulk paths apart.
func markedShell(t *testing.T) tuiModel {
	t.Helper()
	npm := observation("npm-tool", model.RoleInstalled, model.KindPackage,
		model.VersionInfo{Value: "1.0.0", State: model.VersionKnown, Confidence: model.ConfidenceHigh})
	pipx := observation("black", model.RoleInstalled, model.KindPackage,
		model.VersionInfo{Value: "24.1.0", State: model.VersionKnown, Confidence: model.ConfidenceHigh})
	pipx.Origin = model.Origin{Provider: "pipx", Package: "black"}
	pipx.ID = model.ObservationIdentity(pipx)
	return intentShell(t, npm, brewObservation("wget"), pipx)
}

func TestMultiSelectToggle(t *testing.T) {
	m := markedShell(t)
	if len(m.report.rows) < 3 {
		t.Fatalf("setup: Manage lists %d rows, want at least 3", len(m.report.rows))
	}

	// The cursor is its own state: marking row 2 must not move it, and the mark
	// key must not disturb what the single-select keys act on.
	cursor := m.report.selected
	m.report.toggleMark(2)
	if !m.report.selectedSet[2] || len(m.report.selectedSet) != 1 {
		t.Fatalf("after one toggle selectedSet = %v, want exactly {2}", m.report.selectedSet)
	}
	if m.report.selected != cursor {
		t.Fatalf("marking moved the cursor to %d, want %d", m.report.selected, cursor)
	}

	m.report.toggleMark(2)
	if len(m.report.selectedSet) != 0 {
		t.Fatalf("after two toggles selectedSet = %v, want empty", m.report.selectedSet)
	}

	// The space key is the same operation, and the set is a set: pressing it
	// twice on the same row leaves nothing behind.
	m.Update(testKey(" "))
	if !m.report.selectedSet[m.report.selected] {
		t.Fatalf("space did not mark the cursor row; selectedSet = %v", m.report.selectedSet)
	}
	m.Update(testKey(" "))
	if len(m.report.selectedSet) != 0 {
		t.Fatalf("space twice left selectedSet = %v, want empty", m.report.selectedSet)
	}

	// ctrl+a is all-or-nothing over the open view.
	m.Update(testCtrlKey('a'))
	if len(m.report.selectedSet) != len(m.report.rows) {
		t.Fatalf("ctrl+a marked %d of %d rows", len(m.report.selectedSet), len(m.report.rows))
	}
	m.Update(testCtrlKey('a'))
	if len(m.report.selectedSet) != 0 {
		t.Fatalf("ctrl+a again left %d rows marked, want 0", len(m.report.selectedSet))
	}
}

func TestMultiSelectMarksRender(t *testing.T) {
	styles := overviewStyles()
	rows := sortRowsByGroup(InventoryRows(observationsForView(manageReport(), ViewManage)))
	if len(rows) < 3 {
		t.Fatalf("fixture has %d managed rows, want at least 3", len(rows))
	}

	// Cursor on row 0, mark on row 1: the marked row must carry the selection
	// glyph the cursor row carries, and row 2 -- neither -- must not.
	rendered := renderManageRows(rows, 0, map[int]bool{1: true}, 100, 20, styles)
	marked := lineContaining(t, rendered, rows[1].Name)
	if !strings.Contains(marked, styles.Glyph.Selection) {
		t.Errorf("marked row %q lacks the selection glyph %q:\n%s",
			rows[1].Name, styles.Glyph.Selection, marked)
	}
	if plain := lineContaining(t, rendered, rows[2].Name); strings.Contains(plain, styles.Glyph.Selection) {
		t.Errorf("unmarked row %q carries the selection glyph %q:\n%s",
			rows[2].Name, styles.Glyph.Selection, plain)
	}

	// Single-select is unchanged: with no marks the cursor row, and only it,
	// carries the glyph.
	bare := renderManageRows(rows, 0, nil, 100, 20, styles)
	if cursor := lineContaining(t, bare, rows[0].Name); !strings.Contains(cursor, styles.Glyph.Selection) {
		t.Errorf("cursor row lost its selection glyph without marks:\n%s", cursor)
	}
	if plain := lineContaining(t, bare, rows[1].Name); strings.Contains(plain, styles.Glyph.Selection) {
		t.Errorf("row 1 is marked with an empty set:\n%s", plain)
	}
}

func TestBulkCopyUsesMarkedRowsWhenSet(t *testing.T) {
	clipboard := captureClipboard(t)
	m := markedShell(t)

	m.Update(testCtrlKey('a'))
	if len(m.report.selectedSet) != 3 {
		t.Fatalf("ctrl+a marked %d rows, want 3", len(m.report.selectedSet))
	}
	m.Update(testKey("U"))

	lines := strings.Split(clipboard(), "\n")
	if len(lines) != 3 {
		t.Fatalf("U copied %d lines for 3 marked rows: %q", len(lines), clipboard())
	}
	// Every line is the row's own primary action, not a re-derived update: pipx
	// has no upgrade at all, so its uninstall is what proves the marked path
	// reads PrimaryActionCommand.
	for _, observation := range m.report.markedObservations() {
		argv, ok := PrimaryActionCommand(observation)
		if !ok {
			t.Fatalf("%s has no runnable action; the fixture is wrong", observation.DisplayName)
		}
		if !strings.Contains(clipboard(), strings.Join(argv, " ")) {
			t.Errorf("U omitted %q", strings.Join(argv, " "))
		}
	}
	if !strings.Contains(m.report.status, "marked") {
		t.Errorf("U status = %q, want it to name the marked rows", m.report.status)
	}

	// With nothing marked the key keeps its Task 10 meaning: every in-place
	// update in the open view, which excludes the pipx row.
	m.Update(testKey("esc"))
	if len(m.report.selectedSet) != 0 {
		t.Fatalf("esc left %d rows marked", len(m.report.selectedSet))
	}
	m.Update(testKey("U"))
	if got := len(strings.Split(clipboard(), "\n")); got != 2 {
		t.Fatalf("unmarked U copied %d lines, want the 2 updatable rows: %q", got, clipboard())
	}
	if want := "copied 2 update commands"; m.report.status != want {
		t.Fatalf("unmarked U status = %q, want %q", m.report.status, want)
	}
}

func TestEscClearsMarksOnlyWhenPresent(t *testing.T) {
	m := markedShell(t)
	view := m.report.state.View

	m.Update(testKey(" "))
	if len(m.report.selectedSet) != 1 {
		t.Fatalf("space marked %d rows, want 1", len(m.report.selectedSet))
	}
	cursor := m.report.selected

	updated, _ := m.Update(testKey("esc"))
	shell := updated.(tuiModel)
	if len(shell.report.selectedSet) != 0 {
		t.Fatalf("esc left %d rows marked", len(shell.report.selectedSet))
	}
	// Clearing marks is all esc did: the view, the cursor, and the rows are
	// where they were.
	if shell.report.state.View != view || shell.report.selected != cursor {
		t.Fatalf("esc moved to view %q row %d, want %q row %d",
			shell.report.state.View, shell.report.selected, view, cursor)
	}

	// A second esc has no marks to clear and no filter to drop, so it changes
	// nothing -- the ordinary behaviour it had before multi-select existed.
	updated, _ = shell.Update(testKey("esc"))
	shell = updated.(tuiModel)
	if shell.report.state.View != view || shell.report.selected != cursor || len(shell.report.rows) != 3 {
		t.Fatalf("esc on an empty set changed the view: %q row %d, %d rows",
			shell.report.state.View, shell.report.selected, len(shell.report.rows))
	}
}

// lineContaining is the one rendered line holding needle, so a row assertion
// reads the row it names rather than the whole pane.
func lineContaining(t *testing.T, lines []string, needle string) string {
	t.Helper()
	for _, line := range lines {
		if strings.Contains(line, needle) {
			return line
		}
	}
	t.Fatalf("no rendered line contains %q:\n%s", needle, strings.Join(lines, "\n"))
	return ""
}

func TestLegacyDigitKeysUnchanged(t *testing.T) {
	clipboard := captureClipboard(t)
	npm := observation("npm-tool", model.RoleInstalled, model.KindPackage,
		model.VersionInfo{Value: "1.0.0", State: model.VersionKnown, Confidence: model.ConfidenceHigh})
	m := newObservationTUIModel(NewObservationReport([]model.Observation{npm}, nil, nil, emptyObservationDiff(), nil), TUIOptions{})
	m.splashPhase = splashDone
	m.width, m.height = 100, 30
	m.resizeContent()

	// The legacy digit ladder is untouched: 1 is still the overview dashboard,
	// not the intent-mode Manage pane, and the tabs past intent's four still work.
	for digit, want := range map[string]ViewCategory{
		"1": ViewOverview, "3": ViewPackages, "5": ViewPathExecutables, "8": ViewIssues,
	} {
		updated, _ := m.Update(testKey(digit))
		if view := updated.(tuiModel).report.state.View; view != want {
			t.Errorf("legacy %q opened %q, want %q", digit, view, want)
		}
	}

	// m has no intent-first tab to jump to in legacy, so it must leave the view alone.
	m.Update(testKey("3"))
	updated, _ := m.Update(testKey("m"))
	if view := updated.(tuiModel).report.state.View; view != ViewPackages {
		t.Fatalf("legacy \"m\" moved off Packages to %q", view)
	}

	// The action keys are not intent-only: they act on the selected row in legacy too.
	m.Update(testKey("u"))
	argv, _ := PrimaryActionCommand(npm)
	if got, want := clipboard(), strings.Join(argv, " "); got != want {
		t.Fatalf("legacy u copied %q, want %q", got, want)
	}
}

// navShell is a two-row intent-mode shell with the focus layer set explicitly, so every
// navigation test names the layer it is about rather than leaning on the
// default. Two rows is the minimum that can tell "the cursor moved" from "the
// cursor could not move".
func navShell(t *testing.T, layer focusLayer) tuiModel {
	t.Helper()
	m := intentShell(t, brewObservation("ripgrep"), brewObservation("fd"))
	m.focus = layer
	return m
}

func TestTabKeysMoveSidebarWhenSidebarFocused(t *testing.T) {
	m := navShell(t, focusSidebar)
	before := m.activeTab

	updated, _ := m.Update(testKey("right"))
	shell := updated.(tuiModel)
	if shell.activeTab != before+1 {
		t.Fatalf("→ left the sidebar on tab %d, want %d", shell.activeTab, before+1)
	}
	if want := reportViewForTab(uiModeIntent, before+1); shell.report.state.View != want {
		t.Errorf("→ opened %q, want %q", shell.report.state.View, want)
	}
	// Moving the selection is not entering it: the next arrow has to keep
	// walking tabs, not rows.
	if shell.focus != focusSidebar {
		t.Errorf("→ moved focus off the sidebar")
	}

	updated, _ = shell.Update(testKey("left"))
	if back := updated.(tuiModel); back.activeTab != before || back.focus != focusSidebar {
		t.Errorf("← left tab %d focus %v, want tab %d on the sidebar", back.activeTab, back.focus, before)
	}
}

func TestUpDownMovesSidebarWhenSidebarFocused(t *testing.T) {
	m := navShell(t, focusSidebar)
	before, cursor := m.activeTab, m.report.selected

	updated, _ := m.Update(testKey("down"))
	shell := updated.(tuiModel)
	if shell.activeTab != before+1 {
		t.Fatalf("↓ on the sidebar left tab %d, want %d", shell.activeTab, before+1)
	}
	// The row cursor is the pane's, and the pane does not have the keyboard, so
	// ↓ must not walk it forward. It can still be pulled back by the tab it
	// landed on holding fewer rows -- that is rebuildRows, not this key.
	if shell.report.selected > cursor {
		t.Errorf("↓ on the sidebar walked the row cursor forward to %d, want no further than %d",
			shell.report.selected, cursor)
	}
	if shell.focus != focusSidebar {
		t.Errorf("↓ moved focus off the sidebar")
	}
}

func TestEnterOnSidebarOpensPane(t *testing.T) {
	m := navShell(t, focusSidebar)

	updated, _ := m.Update(testKey("enter"))
	shell := updated.(tuiModel)
	if shell.focus != focusPane {
		t.Fatalf("enter on the sidebar left focus on the sidebar")
	}
	if want := reportViewForTab(uiModeIntent, shell.activeTab); shell.report.state.View != want {
		t.Errorf("enter opened %q, want the active tab's view %q", shell.report.state.View, want)
	}
	// One level per enter: the pane is open, but no row's detail is.
	if shell.report.detail != nil {
		t.Errorf("enter on the sidebar opened a row detail as well as the pane")
	}

	// And now enter means what it means in a pane.
	updated, _ = shell.Update(testKey("enter"))
	if shell = updated.(tuiModel); shell.report.detail == nil {
		t.Errorf("enter in the pane did not open the selected row's detail")
	}
}

func TestTabKeyTogglesFocus(t *testing.T) {
	m := navShell(t, focusPane)

	updated, _ := m.Update(testKey("tab"))
	shell := updated.(tuiModel)
	if shell.focus != focusSidebar {
		t.Fatalf("tab from the pane did not focus the sidebar")
	}
	updated, _ = shell.Update(testKey("tab"))
	if shell = updated.(tuiModel); shell.focus != focusPane {
		t.Fatalf("tab from the sidebar did not focus the pane")
	}
}

func TestEscFromPaneReturnsToSidebar(t *testing.T) {
	m := navShell(t, focusPane)
	view := m.report.state.View
	if m.report.detail != nil || len(m.report.selectedSet) > 0 || m.report.drawer.Open {
		t.Fatalf("fixture has something open for esc to close first")
	}

	updated, _ := m.Update(testKey("esc"))
	shell := updated.(tuiModel)
	if shell.focus != focusSidebar {
		t.Fatalf("esc in an empty pane did not return focus to the sidebar")
	}
	// Walking out is all it did: the view is still open behind the sidebar.
	if shell.report.state.View != view {
		t.Errorf("esc moved off %q to %q", view, shell.report.state.View)
	}
}

func TestEscFromSidebarNoOp(t *testing.T) {
	m := navShell(t, focusSidebar)
	view, tab := m.report.state.View, m.activeTab

	updated, _ := m.Update(testKey("esc"))
	shell := updated.(tuiModel)
	if shell.focus != focusSidebar {
		t.Fatalf("esc on the sidebar moved focus to %v", shell.focus)
	}
	if shell.report.state.View != view || shell.activeTab != tab {
		t.Errorf("esc on the sidebar moved to %q (tab %d), want %q (tab %d)",
			shell.report.state.View, shell.activeTab, view, tab)
	}
}

func TestSidebarShowsFocusMarkerOnlyWhenFocused(t *testing.T) {
	styles := overviewStyles()
	tabs := reportTabsForMode(uiModeLegacy)
	active := 1

	focused := renderSidebarLines(tabs, active, true, nil, len(tabs), styles)
	if !strings.Contains(focused[active], styles.Glyph.Selection) {
		t.Errorf("focused sidebar's active tab lacks the selection glyph %q:\n%s",
			styles.Glyph.Selection, focused[active])
	}

	// Unfocused, the active tab is still the open view -- it keeps its label and
	// its count -- but the bar that says "the movement keys are here" is gone.
	unfocused := renderSidebarLines(tabs, active, false, nil, len(tabs), styles)
	if strings.Contains(unfocused[active], styles.Glyph.Selection) {
		t.Errorf("unfocused sidebar still carries the selection glyph:\n%s", unfocused[active])
	}
	if lipgloss.Width(focused[active]) != lipgloss.Width(unfocused[active]) {
		t.Errorf("focus changed the sidebar's width: %d focused, %d unfocused",
			lipgloss.Width(focused[active]), lipgloss.Width(unfocused[active]))
	}
}

func testKey(text string) tea.KeyPressMsg {
	switch text {
	case "enter":
		return testKeyCode(tea.KeyEnter)
	case "esc":
		return testKeyCode(tea.KeyEscape)
	// The named keys have no text of their own: a terminal reports them by code,
	// which is what key.Matches sees, so spelling them as runes would produce a
	// key nothing is bound to.
	case "tab":
		return testKeyCode(tea.KeyTab)
	case "up":
		return testKeyCode(tea.KeyUp)
	case "down":
		return testKeyCode(tea.KeyDown)
	case "left":
		return testKeyCode(tea.KeyLeft)
	case "right":
		return testKeyCode(tea.KeyRight)
	}
	return tea.KeyPressMsg(tea.Key{Text: text, Code: []rune(text)[0]})
}

func testKeyCode(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code})
}

// testCtrlKey is a chord: Bubble Tea stringifies it as "ctrl+<code>", which is
// what the binding matches on.
func testCtrlKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code, Mod: tea.ModCtrl})
}

func emptyObservationDiff() (diff registry.ObservationDiff) {
	return registry.ObservationDiff{}
}
