package output

import (
	"strings"
	"testing"
)

// healthReport is the overview fixture, which already has everything the Health
// cards count: npm and Homebrew entries a manager can upgrade, one broken
// event, and seven manager-installed entries. Sharing it keeps the two
// dashboards describing the same machine.
func healthReport() ObservationReport { return overviewReport() }

// cardLine returns the rendered line that carries a card's title, so a count
// can be asserted against the card it belongs to rather than against the pane
// as a whole -- a bare "1" would match almost anything.
func cardLine(t *testing.T, rendered, title string) string {
	t.Helper()
	for _, line := range strings.Split(rendered, "\n") {
		if strings.Contains(line, title) {
			return line
		}
	}
	t.Fatalf("health pane has no card titled %q:\n%s", title, rendered)
	return ""
}

func TestRenderHealthShowsCards(t *testing.T) {
	report := healthReport()
	rendered := strings.Join(renderHealth(report, 0, overviewStyles(), 96, 50), "\n")
	if strings.TrimSpace(rendered) == "" {
		t.Fatal("renderHealth returned no content")
	}

	// The three counted cards each show the count the rest of the TUI would
	// show for the same view, so a card and its tab can never disagree.
	for _, view := range []ViewCategory{ViewUpdates, ViewIssues, ViewManage} {
		count := CountForView(report, view)
		if count == 0 {
			t.Fatalf("fixture does not exercise %s: count is 0", view)
		}
		line := cardLine(t, rendered, ViewLabel(view))
		if !strings.Contains(line, itoa(count)) {
			t.Errorf("%s card %q missing its count %d", view, strings.TrimSpace(line), count)
		}
	}

	// Reclaimable is present but deliberately unmeasured: the card says so with
	// a dash rather than inventing a number no scanner has produced.
	reclaimable := cardLine(t, rendered, healthReclaimableLabel)
	if !strings.Contains(reclaimable, healthUnknownValue) {
		t.Errorf("reclaimable card %q missing the %q placeholder", strings.TrimSpace(reclaimable), healthUnknownValue)
	}
	if strings.ContainsAny(reclaimable, "0123456789") {
		t.Errorf("reclaimable card %q shows a number; nothing measures it yet", strings.TrimSpace(reclaimable))
	}

	// The footer names a key the shell actually binds.
	if !strings.Contains(rendered, healthCopyKey+" copies") {
		t.Errorf("health pane missing the copy-command hint:\n%s", rendered)
	}
}

// Updatable is derived from the action vocabulary, not from a version state the
// model does not have: an entry counts when KindActions offers an update.
func TestCountForViewUpdatesFollowsTheUpdateAction(t *testing.T) {
	report := healthReport()
	want := 0
	for _, observation := range report.AllObservations() {
		for _, action := range KindActions(observation) {
			if action.Kind == ActionUpdate {
				want++
				break
			}
		}
	}
	if got := CountForView(report, ViewUpdates); got != want {
		t.Fatalf("CountForView(ViewUpdates) = %d, want %d", got, want)
	}
	// Updatable is a subset of what a manager installed; it can never exceed it.
	if managed := CountForView(report, ViewManage); want > managed {
		t.Fatalf("updatable %d exceeds manager-installed %d", want, managed)
	}
}

// The pane must never overflow the caller's height budget; the frame sizes
// itself on the promise that a renderer returns exactly what it was given.
func TestRenderHealthRendersExactlyHeightLines(t *testing.T) {
	report := healthReport()
	for _, height := range []int{1, 2, 3, 50} {
		lines := renderHealth(report, 0, overviewStyles(), 80, height)
		if len(lines) != height {
			t.Fatalf("renderHealth(height=%d) returned %d lines", height, len(lines))
		}
	}
}

// A machine with nothing on it still gets the dashboard: zero updates and zero
// issues is the answer to "is this healthy", not an empty pane.
func TestRenderHealthShowsZerosOnAnEmptyMachine(t *testing.T) {
	rendered := strings.Join(renderHealth(ObservationReport{}, 0, overviewStyles(), 96, 20), "\n")
	for _, title := range []string{ViewLabel(ViewUpdates), ViewLabel(ViewIssues), healthReclaimableLabel} {
		if !strings.Contains(rendered, title) {
			t.Errorf("empty-machine health pane missing the %q card:\n%s", title, rendered)
		}
	}
}

// The renderer being right is not the same as the shell reaching it, so this
// drives the v3 tab strip the way a user would.
func TestObservationTUIHealthTabRendersCards(t *testing.T) {
	m := newObservationTUIModel(healthReport(), TUIOptions{UIMode: uiModeV3})
	m.splashPhase = splashDone
	m.width, m.height = 110, 30
	m.resizeContent()

	// "4" is the Health tab, fourth in the intent-first order.
	updated, _ := m.Update(testKey("4"))
	shell := updated.(tuiModel)
	if shell.report.state.View != ViewHealth {
		t.Fatalf("jump to tab 4 selected %q, want %q", shell.report.state.View, ViewHealth)
	}

	view := shell.View().Content
	for _, want := range []string{
		ViewLabel(ViewHealth), ViewLabel(ViewUpdates), ViewLabel(ViewIssues),
		healthReclaimableLabel, healthUnknownValue,
	} {
		if !strings.Contains(view, want) {
			t.Errorf("health pane missing %q: %s", want, view)
		}
	}
}
