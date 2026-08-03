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
	m := newReportTUIModel(NewObservationReport([]model.Observation{tool}, nil, nil, emptyObservationDiff(), nil))
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

func TestObservationTUIUsesExistingShellChromeForV2Report(t *testing.T) {
	tool := observation("npm-tool", model.RoleInstalled, model.KindPackage, model.VersionInfo{Value: "1.0.0", State: model.VersionKnown, Confidence: model.ConfidenceHigh})
	m := newObservationTUIModel(NewObservationReport([]model.Observation{tool}, nil, nil, emptyObservationDiff(), []string{"scanner: warning"}), TUIOptions{Version: "1.2.3"})
	m.splashPhase = splashDone
	m.width, m.height = 100, 30
	m.resizeContent()

	view := m.View()
	for _, want := range []string{"◆ toolsniff", "all", "installed", "npm-tool", "warning: scanner: warning"} {
		if !strings.Contains(view.Content, want) {
			t.Errorf("v2 shell view missing %q: %s", want, view.Content)
		}
	}
	if !view.AltScreen {
		t.Fatal("v2 shell did not enable the alternate screen")
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
	m.Update(testKeyCode(tea.KeyEnter))
	if m.report.detail == nil {
		t.Fatal("enter did not open v2 detail view")
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
