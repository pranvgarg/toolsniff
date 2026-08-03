package output

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/pranvgarg/toolsniff/model"
	"github.com/pranvgarg/toolsniff/registry"
)

func TestObservationReportJSONHasV2BucketsAndTypedChanges(t *testing.T) {
	installed := observation("npm-tool", model.RoleInstalled, model.KindPackage, model.VersionInfo{Value: "1.2.3", State: model.VersionKnown, Confidence: model.ConfidenceHigh})
	available := observation("path-tool", model.RoleAvailable, model.KindExecutable, model.VersionInfo{State: model.VersionUnknown, Confidence: model.ConfidenceLow})
	history := observation("create-vite", model.RoleHistory, model.KindHistory, model.VersionInfo{State: model.VersionNotApplicable, Confidence: model.ConfidenceLow})
	change := registry.ChangeEvent{Kind: registry.ChangeUpdated, Identity: installed.ID, Before: &installed, After: &available}
	report := NewObservationReport([]model.Observation{installed}, []model.Observation{available}, []model.Observation{history}, registry.ObservationDiff{Updated: []registry.ChangeEvent{change}}, []string{"scanner: warning"})

	data, err := RenderObservationJSON(report)
	if err != nil {
		t.Fatalf("RenderObservationJSON failed: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if decoded["schema_version"] != float64(2) {
		t.Fatalf("schema_version = %v, want 2", decoded["schema_version"])
	}
	for _, key := range []string{"installed", "available", "history", "changes", "warnings"} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("missing v2 JSON field %q", key)
		}
	}
	changes, ok := decoded["changes"].(map[string]any)
	if !ok || len(changes["updated"].([]any)) != 1 {
		t.Fatalf("updated change was not preserved: %#v", decoded["changes"])
	}
}

func TestInventoryRowsUseExplicitVersionStatesAndSeparateColumns(t *testing.T) {
	tests := []struct {
		state model.VersionState
		want  string
	}{
		{model.VersionKnown, "1.0.0"},
		{model.VersionUnknown, "unknown"},
		{model.VersionNotApplicable, "n/a"},
		{model.VersionNotReported, "not reported"},
	}
	for _, test := range tests {
		row := InventoryRowFromObservation(observation("tool", model.RoleInstalled, model.KindPackage, model.VersionInfo{Value: valueForState(test.state), State: test.state, Confidence: model.ConfidenceLow}))
		if row.Version != test.want || row.VersionState != string(test.state) {
			t.Errorf("state %q produced row %+v", test.state, row)
		}
	}
	row := InventoryRowFromObservation(observationWithPath("path-tool", model.RoleAvailable, model.KindExecutable, model.VersionInfo{State: model.VersionUnknown, Confidence: model.ConfidenceLow}, "/bin/path-tool"))
	if row.Version == "/bin/path-tool" || row.Source != model.SourcePath || row.Status != string(StatusAvailable) {
		t.Fatalf("path leaked into version or status/source missing: %+v", row)
	}
}

func TestRenderObservationTableKeepsVersionSeparateFromLocation(t *testing.T) {
	tool := observationWithPath("path-tool", model.RoleAvailable, model.KindExecutable, model.VersionInfo{State: model.VersionUnknown, Confidence: model.ConfidenceLow}, "/bin/path-tool")
	output := RenderObservationTable(NewObservationReport(nil, []model.Observation{tool}, nil, registry.ObservationDiff{}, nil))
	if !strings.Contains(output, "unknown") || !strings.Contains(output, "available") || !strings.Contains(output, "path") {
		t.Fatalf("missing explicit row columns: %s", output)
	}
	if strings.Contains(output, "Version       /bin/path-tool") {
		t.Fatalf("location rendered as version: %s", output)
	}
}

func TestRenderTypedChangeEventsKeepsBeforeAfterContext(t *testing.T) {
	before := observationWithPath("tool", model.RoleInstalled, model.KindPackage, model.VersionInfo{Value: "1.0.0", State: model.VersionKnown, Confidence: model.ConfidenceHigh}, "/old/tool")
	after := observationWithPath("tool", model.RoleInstalled, model.KindPackage, model.VersionInfo{Value: "1.1.0", State: model.VersionKnown, Confidence: model.ConfidenceHigh}, "/new/tool")
	changes := ChangeReport{
		Updated:   []registry.ChangeEvent{{Kind: registry.ChangeUpdated, Before: &before, After: &after}},
		Relocated: []registry.ChangeEvent{{Kind: registry.ChangeRelocated, Before: &before, After: &after}},
	}
	output := RenderChangeReport(changes)
	for _, want := range []string{"UPDATED", "1.0.0 -> 1.1.0", "RELOCATED", "/old/tool", "-> /new/tool"} {
		if !strings.Contains(output, want) {
			t.Errorf("change output missing %q: %s", want, output)
		}
	}
}

func valueForState(state model.VersionState) string {
	if state == model.VersionKnown {
		return "1.0.0"
	}
	return ""
}

func observation(name string, role model.SourceRole, kind model.ObservationKind, version model.VersionInfo) model.Observation {
	return observationWithPath(name, role, kind, version, "/opt/"+name)
}

func observationWithPath(name string, role model.SourceRole, kind model.ObservationKind, version model.VersionInfo, path string) model.Observation {
	provider := "npm"
	manager := "global"
	if role == model.RoleAvailable {
		provider = "unknown"
		manager = "manual-or-unknown"
	}
	if role == model.RoleHistory {
		provider = "npm"
		manager = "npx"
	}
	observation := model.Observation{
		DisplayName:  name,
		CommandName:  name,
		Kind:         kind,
		Role:         role,
		Origin:       model.Origin{Provider: provider, Manager: manager, Package: name},
		Version:      version,
		Locations:    []model.Location{{Path: path, Type: model.LocationExecutable}},
		Availability: model.AvailabilityInfo{State: model.AvailabilityAvailable, ResolvedPath: path},
	}
	if kind == model.KindHistory {
		observation.History = &model.HistoryInfo{CachePath: path}
	}
	observation.ID = model.ObservationIdentity(observation)
	return observation
}
