package output

import (
	"fmt"
	"testing"

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

func emptyObservationDiff() (diff registry.ObservationDiff) {
	return registry.ObservationDiff{}
}
