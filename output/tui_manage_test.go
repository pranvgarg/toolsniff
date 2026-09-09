package output

import (
	"strings"
	"testing"

	"github.com/pranvgarg/toolsniff/model"
	"github.com/pranvgarg/toolsniff/registry"
)

// manageReport is one managed entry per kind the Manage view unions, plus one
// hand-placed PATH executable it must leave out. The managed entries are listed
// out of manager-priority order so a passing ordering assertion proves the view
// sorts rather than that the fixture was already sorted.
func manageReport() ObservationReport {
	managed := func(name string, kind model.ObservationKind, provider, manager string) model.Observation {
		observation := model.Observation{
			DisplayName: name, CommandName: name, Kind: kind, Role: model.RoleInstalled,
			Origin:    model.Origin{Provider: provider, Manager: manager, Package: name},
			Version:   model.VersionInfo{Value: "1.0.0", State: model.VersionKnown, Confidence: model.ConfidenceHigh},
			Locations: []model.Location{{Path: "/opt/" + name, Type: model.LocationExecutable}},
		}
		observation.ID = model.ObservationIdentity(observation)
		return observation
	}
	installed := []model.Observation{
		managed("wget", model.KindPackage, "homebrew", "formula"),
		managed("opencode", model.KindCLI, "npm", "global"),
		managed("Ghostty", model.KindApplication, "applications", ""),
	}
	available := []model.Observation{observationWithPath("mystery", model.RoleAvailable, model.KindExecutable,
		model.VersionInfo{State: model.VersionUnknown, Confidence: model.ConfidenceLow}, "/usr/local/bin/mystery")}
	return NewObservationReport(installed, available, nil, registry.ObservationDiff{}, nil)
}

func TestRenderManageGroupsInstalledByManager(t *testing.T) {
	report := manageReport()
	rendered := strings.Join(renderManage(report, 0, overviewStyles(), 80, 40), "\n")
	if strings.TrimSpace(rendered) == "" {
		t.Fatal("renderManage returned no content")
	}

	npm := strings.Index(rendered, sourceGroupLabels[model.SourceNPM])
	brew := strings.Index(rendered, sourceGroupLabels[model.SourceBrewFormula])
	if npm < 0 {
		t.Fatalf("missing %q heading in:\n%s", sourceGroupLabels[model.SourceNPM], rendered)
	}
	if brew < 0 {
		t.Fatalf("missing %q heading in:\n%s", sourceGroupLabels[model.SourceBrewFormula], rendered)
	}
	// npm ranks above Homebrew in sourceGroupOrder, so it leads regardless of
	// the order the scanner reported the observations in.
	if npm > brew {
		t.Fatalf("expected %q before %q in:\n%s",
			sourceGroupLabels[model.SourceNPM], sourceGroupLabels[model.SourceBrewFormula], rendered)
	}
	// A program the user placed on their PATH is not something a manager can
	// act on, so it is absent from Manage entirely -- and therefore cannot be
	// the leading group.
	if strings.Contains(rendered, sourceGroupLabels[model.SourcePath]) {
		t.Fatalf("unexpected %q group in Manage:\n%s", sourceGroupLabels[model.SourcePath], rendered)
	}

	if got := CountForView(report, ViewManage); got != 3 {
		t.Fatalf("CountForView(ViewManage) = %d, want 3 (npm + Homebrew + application)", got)
	}
}

// The renderer being right is not the same as the shell reaching it, so this
// drives the intent tab strip the way a user would.
func TestObservationTUIManageTabRendersOneGroupedList(t *testing.T) {
	m := newObservationTUIModel(manageReport(), TUIOptions{UIMode: uiModeIntent})
	m.splashPhase = splashDone
	m.width, m.height = 100, 30
	m.resizeContent()

	// "1" is the Manage tab, first in the intent-first order.
	updated, _ := m.Update(testKey("1"))
	shell := updated.(tuiModel)
	if shell.report.state.View != ViewManage {
		t.Fatalf("jump to tab 1 selected %q, want %q", shell.report.state.View, ViewManage)
	}

	view := shell.View().Content
	for _, want := range []string{
		ViewLabel(ViewManage), "· 3 · " + ViewMeaning(ViewManage),
		sourceGroupLabels[model.SourceNPM], sourceGroupLabels[model.SourceBrewFormula],
		sourceGroupLabels[model.SourceApplications],
		"opencode", "wget", "Ghostty",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("manage pane missing %q: %s", want, view)
		}
	}
	if strings.Contains(view, "mystery") {
		t.Errorf("manage pane shows a hand-placed PATH executable: %s", view)
	}
}

func TestRenderManageRendersExactlyHeightLines(t *testing.T) {
	report := manageReport()
	for _, height := range []int{1, 3, 40} {
		lines := renderManage(report, 0, overviewStyles(), 80, height)
		if len(lines) != height {
			t.Fatalf("renderManage(height=%d) returned %d lines", height, len(lines))
		}
	}
}
