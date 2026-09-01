package output

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
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
			t.Errorf("v2 overview view missing %q: %s", want, view.Content)
		}
	}
	if strings.Contains(view.Content, "available") {
		t.Errorf("the overview showed the bare word \"available\": %s", view.Content)
	}
	if !view.AltScreen {
		t.Fatal("v2 shell did not enable the alternate screen")
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
	tabs := reportTabsForMode("v2")
	if len(tabs) != len(want) {
		t.Fatalf("reportTabsForMode(\"v2\") = %v, want %v", tabs, want)
	}
	for index, tab := range want {
		if tabs[index] != tab {
			t.Fatalf("reportTabsForMode(\"v2\")[%d] = %q, want %q", index, tabs[index], tab)
		}
		if reportViewForTab("v2", index) != ViewCategory(tab) || reportTabIndex("v2", ViewCategory(tab)) != index {
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
	v2 := reportTabsForMode("v2")
	v3 := reportTabsForMode("v3")
	if len(v2) != 8 {
		t.Fatalf("v2 tabs = %d, want 8", len(v2))
	}
	if len(v3) != 4 || v3[0] != "manage" || v3[1] != "discover" || v3[2] != "review" || v3[3] != "health" {
		t.Fatalf("v3 tabs = %v, want [manage discover review health]", v3)
	}
}

func TestObservationTUIKeepsV2InteractionsInsideShell(t *testing.T) {
	tool := observation("npm-tool", model.RoleInstalled, model.KindPackage, model.VersionInfo{Value: "1.0.0", State: model.VersionKnown, Confidence: model.ConfidenceHigh})
	m := newObservationTUIModel(NewObservationReport([]model.Observation{tool}, nil, nil, emptyObservationDiff(), nil), TUIOptions{})
	m.splashPhase = splashDone
	m.width, m.height = 100, 30
	m.resizeContent()

	m.Update(testKey("/"))
	m.Update(testKey("npm"))
	m.Update(testKeyCode(tea.KeyEnter))
	if m.report.filtering || m.report.state.Text != "npm" {
		t.Fatalf("v2 filter state was not retained: filtering=%v state=%+v", m.report.filtering, m.report.state)
	}
	m.Update(testKey("esc"))
	if m.report.filtering {
		t.Fatal("escape did not leave v2 filtering")
	}
	// m.report is a pointer into the shell, so the report-side state these
	// assertions read is updated in place by Update.
	m.Update(testKeyCode(tea.KeyEnter))
	if m.report.detail == nil {
		t.Fatal("enter did not open v2 detail view")
	}
	// The detail pane has to carry the per-kind actions, not just metadata:
	// this is the difference between "here is what we found" and "here is what
	// you can do about it".
	detailView := m.View().Content
	for _, want := range []string{"npm global package", "ACTIONS", "npm update -g npm-tool", "npm uninstall -g npm-tool"} {
		if !strings.Contains(detailView, want) {
			t.Errorf("v2 detail pane missing %q: %s", want, detailView)
		}
	}

	// y yanks the primary per-kind command into the status line. Nothing runs.
	m.Update(testKey("y"))
	if want := "command ready to copy: npm update -g npm-tool"; m.report.status != want {
		t.Fatalf("y status = %q, want %q", m.report.status, want)
	}

	m.Update(testKey("esc"))
	if m.report.detail != nil {
		t.Fatal("escape did not close v2 detail view")
	}

	_, quit := m.Update(testKey("q"))
	if quit == nil {
		t.Fatal("q did not return a quit command")
	}
}

func testKey(text string) tea.KeyPressMsg {
	switch text {
	case "enter":
		return testKeyCode(tea.KeyEnter)
	case "esc":
		return testKeyCode(tea.KeyEscape)
	}
	return tea.KeyPressMsg(tea.Key{Text: text, Code: []rune(text)[0]})
}

func testKeyCode(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code})
}

func emptyObservationDiff() (diff registry.ObservationDiff) {
	return registry.ObservationDiff{}
}
