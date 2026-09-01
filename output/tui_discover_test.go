package output

import (
	"strings"
	"testing"

	"github.com/pranvgarg/toolsniff/model"
	"github.com/pranvgarg/toolsniff/registry"
)

// discoverReport is one entry of each sort Discover has an opinion about: a
// program the user placed on their PATH themselves, a package npm installed
// (which Discover must leave out), and an npx cache entry.
func discoverReport() ObservationReport {
	manual := observationWithPath("foo", model.RoleAvailable, model.KindExecutable,
		model.VersionInfo{State: model.VersionUnknown, Confidence: model.ConfidenceLow}, "/usr/local/bin/foo")

	managed := model.Observation{
		DisplayName: "opencode", CommandName: "opencode", Kind: model.KindCLI, Role: model.RoleInstalled,
		Origin:    model.Origin{Provider: "npm", Manager: "global", Package: "opencode"},
		Version:   model.VersionInfo{Value: "1.0.0", State: model.VersionKnown, Confidence: model.ConfidenceHigh},
		Locations: []model.Location{{Path: "/opt/homebrew/bin/opencode", Type: model.LocationExecutable}},
	}
	managed.ID = model.ObservationIdentity(managed)

	cached := observationWithPath("cowsay", model.RoleHistory, model.KindHistory,
		model.VersionInfo{State: model.VersionNotApplicable, Confidence: model.ConfidenceLow}, "/Users/x/.npm/_npx/cowsay")
	cached.History = &model.HistoryInfo{CachePath: "/Users/x/.npm/_npx/cowsay"}

	return NewObservationReport([]model.Observation{managed}, []model.Observation{manual},
		[]model.Observation{cached}, registry.ObservationDiff{}, nil)
}

func TestRenderDiscoverShowsUnmanagedAndSuggestions(t *testing.T) {
	report := discoverReport()
	rendered := strings.Join(renderDiscover(report, 0, overviewStyles(), 80, 50), "\n")
	if strings.TrimSpace(rendered) == "" {
		t.Fatal("renderDiscover returned no content")
	}

	// The hand-placed binary is the point of the view, and it is grouped under
	// the directory it actually sits in rather than under a manager.
	if !strings.Contains(rendered, "foo") {
		t.Errorf("discover pane missing the hand-placed PATH executable:\n%s", rendered)
	}
	if !strings.Contains(rendered, "/usr/local/bin") {
		t.Errorf("discover pane missing the directory heading:\n%s", rendered)
	}
	// A package npm installed is something a manager owns, so it belongs to
	// Manage; showing it here would make the view's meaning false.
	if strings.Contains(rendered, "opencode") {
		t.Errorf("discover pane shows a manager-installed package:\n%s", rendered)
	}
	npx := strings.Index(rendered, sourceGroupLabels[model.SourceNPXHistory])
	if npx < 0 {
		t.Fatalf("discover pane missing the %q group:\n%s", sourceGroupLabels[model.SourceNPXHistory], rendered)
	}
	// The npx cache is the residue of this view, not its lead: directories the
	// user actually populated come first. sourceGroupOrder ranks the other way
	// round -- it is written for Manage -- so this pins Discover's own order.
	if directory := strings.Index(rendered, "/usr/local/bin"); directory > npx {
		t.Errorf("expected the directory group before %q in:\n%s",
			sourceGroupLabels[model.SourceNPXHistory], rendered)
	}

	// The suggestion is derived, not written: npm owns the only managed entry,
	// so the tip offers npm's own install template for the unmanaged binary.
	if want := strings.Join(installCommandForOrigin(originNPM, "foo"), " "); !strings.Contains(rendered, want) {
		t.Errorf("discover pane missing the suggested %q command:\n%s", want, rendered)
	}

	// The count the tab shows is the count the pane renders: two entries, with
	// the npm package excluded.
	if got := CountForView(report, ViewDiscover); got != 2 {
		t.Fatalf("CountForView(ViewDiscover) = %d, want 2 (PATH executable + npx history)", got)
	}
}

// The footer is the first thing to go when the pane cannot hold everything;
// overflowing the caller's height budget would push the frame apart.
func TestRenderDiscoverRendersExactlyHeightLines(t *testing.T) {
	report := discoverReport()
	for _, height := range []int{1, 2, 3, 50} {
		lines := renderDiscover(report, 0, overviewStyles(), 80, height)
		if len(lines) != height {
			t.Fatalf("renderDiscover(height=%d) returned %d lines", height, len(lines))
		}
	}
}

// A machine with no managed entries has no manager to suggest, so the tip says
// the smaller true thing rather than naming a manager that is not installed.
func TestRenderDiscoverSuggestsNothingUnmanageableWithoutAManager(t *testing.T) {
	manual := observationWithPath("foo", model.RoleAvailable, model.KindExecutable,
		model.VersionInfo{State: model.VersionUnknown, Confidence: model.ConfidenceLow}, "/usr/local/bin/foo")
	report := NewObservationReport(nil, []model.Observation{manual}, nil, registry.ObservationDiff{}, nil)

	rendered := strings.Join(renderDiscover(report, 0, overviewStyles(), 80, 20), "\n")
	if want := DiscoverSuggestionLine("foo", nil); !strings.Contains(rendered, want) {
		t.Errorf("discover pane missing the manager-less tip %q:\n%s", want, rendered)
	}
}

// The renderer being right is not the same as the shell reaching it, so this
// drives the v3 tab strip the way a user would.
func TestObservationTUIDiscoverTabRendersUnmanagedEntries(t *testing.T) {
	m := newObservationTUIModel(discoverReport(), TUIOptions{UIMode: uiModeV3})
	m.splashPhase = splashDone
	m.width, m.height = 100, 30
	m.resizeContent()

	// "2" is the Discover tab, second in the intent-first order.
	updated, _ := m.Update(testKey("2"))
	shell := updated.(tuiModel)
	if shell.report.state.View != ViewDiscover {
		t.Fatalf("jump to tab 2 selected %q, want %q", shell.report.state.View, ViewDiscover)
	}

	view := shell.View().Content
	for _, want := range []string{
		ViewLabel(ViewDiscover), "· 2 · " + ViewMeaning(ViewDiscover),
		"foo", sourceGroupLabels[model.SourceNPXHistory],
	} {
		if !strings.Contains(view, want) {
			t.Errorf("discover pane missing %q: %s", want, view)
		}
	}
	if strings.Contains(view, "opencode") {
		t.Errorf("discover pane shows a manager-installed package: %s", view)
	}
}
