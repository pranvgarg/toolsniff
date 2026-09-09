package output

import (
	"strings"
	"testing"

	"github.com/pranvgarg/toolsniff/model"
	"github.com/pranvgarg/toolsniff/registry"
)

// reviewReport carries one event of each sort Review has a heading for: an
// arrival that was not in the last saved baseline, a command that stopped
// resolving, and a command another copy now shadows. The three land in three
// different sections, which is what makes the partition observable.
func reviewReport() ObservationReport {
	added := observationWithPath("ripgrep", model.RoleInstalled, model.KindCLI,
		model.VersionInfo{Value: "14.1.0", State: model.VersionKnown, Confidence: model.ConfidenceHigh},
		"/opt/homebrew/bin/rg")

	broken := observationWithPath("dangling", model.RoleInstalled, model.KindCLI,
		model.VersionInfo{State: model.VersionUnknown, Confidence: model.ConfidenceLow},
		"/opt/homebrew/bin/dangling")
	broken.Availability = model.AvailabilityInfo{State: model.AvailabilityUnavailable}

	shadowed := observationWithPath("python", model.RoleInstalled, model.KindCLI,
		model.VersionInfo{Value: "3.12.0", State: model.VersionKnown, Confidence: model.ConfidenceHigh},
		"/opt/homebrew/bin/python")
	shadowed.Availability = model.AvailabilityInfo{ShadowedBy: []string{"/usr/bin/python"}}

	updated := observationWithPath("wget", model.RoleInstalled, model.KindCLI,
		model.VersionInfo{Value: "1.25.0", State: model.VersionKnown, Confidence: model.ConfidenceHigh},
		"/opt/homebrew/bin/wget")

	return NewObservationReport(
		[]model.Observation{added, broken, shadowed, updated}, nil, nil,
		registry.ObservationDiff{
			Added:    []registry.ChangeEvent{{Kind: registry.ChangeAdded, Identity: added.ID, After: &added}},
			Broken:   []registry.ChangeEvent{{Kind: registry.ChangeBroken, Identity: broken.ID, Before: &broken, After: &broken}},
			Shadowed: []registry.ChangeEvent{{Kind: registry.ChangeShadowed, Identity: shadowed.ID, Before: &shadowed, After: &shadowed}},
			Updated:  []registry.ChangeEvent{{Kind: registry.ChangeUpdated, Identity: updated.ID, Before: &updated, After: &updated}},
		}, nil)
}

func TestRenderReviewShowsChangesAndIssues(t *testing.T) {
	report := reviewReport()
	rendered := strings.Join(renderReview(report, 0, overviewStyles(), 80, 50), "\n")
	if strings.TrimSpace(rendered) == "" {
		t.Fatal("renderReview returned no content")
	}

	// Every event is named, whichever section claims it.
	for _, want := range []string{"ripgrep", "dangling", "python", "wget"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("review pane missing the entry %q:\n%s", want, rendered)
		}
	}

	// Each section heading is present, and each is the existing view's own
	// wording rather than a phrase invented for this pane.
	for _, section := range reviewSections {
		if !strings.Contains(rendered, section.Label) {
			t.Errorf("review pane missing the %q heading:\n%s", section.Label, rendered)
		}
	}

	// The reason a row is worth looking at rides in the STATUS column, in the
	// vocabulary the rest of the app already shows.
	for _, want := range []string{
		StatusDisplayLabel(string(StatusBroken)),
		StatusDisplayLabel(string(StatusShadowed)),
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("review pane missing the reason %q:\n%s", want, rendered)
		}
	}

	// The count the tab shows is the count the pane renders. The sections
	// partition the change categories rather than overlapping them, so the sum
	// of the three is every event exactly once -- an overlapping Changes and
	// Issues would report 6 here for 4 rendered rows.
	changes := report.Changes
	want := len(changes.Updated) + len(changes.Broken) + len(changes.Shadowed) + len(changes.Added)
	if got := CountForView(report, ViewReview); got != want {
		t.Fatalf("CountForView(ViewReview) = %d, want %d (changes + issues + new)", got, want)
	}
	if got := len(RowsForReport(report, ViewReview, false)); got != want {
		t.Fatalf("RowsForReport(ViewReview, false) returned %d rows, want %d", got, want)
	}
}

// The three sections partition the seven change categories, so a broken or
// shadowed event belongs to Issues alone. Listing it under Changes as well --
// which is what Events() does, since Broken and Shadowed are two of its seven
// categories -- would show the same tool twice and count it twice.
func TestRenderReviewSectionsDoNotOverlap(t *testing.T) {
	seen := make(map[Status]string)
	for _, section := range reviewSections {
		for _, status := range section.Statuses {
			if other, ok := seen[status]; ok {
				t.Errorf("status %q is claimed by both %q and %q", status, other, section.Label)
			}
			seen[status] = section.Label
		}
	}

	// Exhaustive as well as disjoint: a category no section claims would vanish
	// from the pane while still counting toward the tab.
	report := reviewReport()
	if len(seen) != len(rowsForEvents(fullChangeReport(), ViewChanges)) {
		t.Errorf("reviewSections claims %d statuses, want one per change category", len(seen))
	}
	if got, want := len(RowsForReport(report, ViewReview, false)), len(report.Changes.Events()); got != want {
		t.Fatalf("review rows = %d, want %d (one per event)", got, want)
	}
}

// "New" is a real section, not a placeholder: registry.ComputeObservationDiff
// files an identity that is in the fresh scan but not in the saved registry as
// an Added event, so the baseline delta the heading names already exists.
func TestRenderReviewShowsNewSinceBaseline(t *testing.T) {
	report := reviewReport()
	rendered := strings.Join(renderReview(report, 0, overviewStyles(), 96, 50), "\n")

	if !strings.Contains(rendered, reviewNewLabel) {
		t.Fatalf("review pane missing the %q heading:\n%s", reviewNewLabel, rendered)
	}
	// The arrival is under New, not under Changes: the added row is the last of
	// the four, because New is the pane's trailing section.
	rows := RowsForReport(report, ViewReview, false)
	last := rows[len(rows)-1]
	if last.Status != string(StatusAdded) || last.Name != "ripgrep" {
		t.Errorf("expected the added entry last, got %q (%s)", last.Name, last.Status)
	}

	// With nothing new since the baseline the heading is absent rather than
	// standing over an empty block.
	empty := NewObservationReport(nil, nil, nil, registry.ObservationDiff{}, nil)
	if bare := strings.Join(renderReview(empty, 0, overviewStyles(), 96, 20), "\n"); strings.Contains(bare, reviewNewLabel) {
		t.Errorf("review pane shows the %q heading with no events:\n%s", reviewNewLabel, bare)
	}
}

// The pane must never overflow the caller's height budget; the frame sizes
// itself on the promise that a renderer returns exactly what it was given.
func TestRenderReviewRendersExactlyHeightLines(t *testing.T) {
	report := reviewReport()
	for _, height := range []int{1, 2, 3, 50} {
		lines := renderReview(report, 0, overviewStyles(), 80, height)
		if len(lines) != height {
			t.Fatalf("renderReview(height=%d) returned %d lines", height, len(lines))
		}
	}
}

// The renderer being right is not the same as the shell reaching it, so this
// drives the intent tab strip the way a user would.
func TestObservationTUIReviewTabRendersChangesAndIssues(t *testing.T) {
	m := newObservationTUIModel(reviewReport(), TUIOptions{UIMode: uiModeIntent})
	m.splashPhase = splashDone
	m.width, m.height = 110, 30
	m.resizeContent()

	// "3" is the Review tab, third in the intent-first order.
	updated, _ := m.Update(testKey("3"))
	shell := updated.(tuiModel)
	if shell.report.state.View != ViewReview {
		t.Fatalf("jump to tab 3 selected %q, want %q", shell.report.state.View, ViewReview)
	}

	view := shell.View().Content
	for _, want := range []string{
		ViewLabel(ViewReview), "· 4 · " + ViewMeaning(ViewReview),
		ViewLabel(ViewIssues), reviewNewLabel, "dangling", "ripgrep",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("review pane missing %q: %s", want, view)
		}
	}
}

// fullChangeReport is one event in every category, used to check that the
// Review sections between them claim all seven.
func fullChangeReport() ChangeReport {
	observation := observationWithPath("probe", model.RoleInstalled, model.KindCLI,
		model.VersionInfo{State: model.VersionUnknown, Confidence: model.ConfidenceLow}, "/opt/probe")
	event := func(kind registry.ChangeKind) []registry.ChangeEvent {
		return []registry.ChangeEvent{{Kind: kind, Identity: observation.ID, Before: &observation, After: &observation}}
	}
	return ChangeReport{
		Added:     event(registry.ChangeAdded),
		Removed:   event(registry.ChangeRemoved),
		Updated:   event(registry.ChangeUpdated),
		Relocated: event(registry.ChangeRelocated),
		Broken:    event(registry.ChangeBroken),
		Repaired:  event(registry.ChangeRepaired),
		Shadowed:  event(registry.ChangeShadowed),
	}
}
