package output

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/pranvgarg/toolsniff/model"
)

// npmCLIObservation is an installed npm-global CLI: the case where a manager
// has a real verb to offer, so the inline column has something to show.
func npmCLIObservation() model.Observation {
	observation := model.Observation{
		DisplayName: "gh",
		CommandName: "gh",
		Kind:        model.KindCLI,
		Role:        model.RoleInstalled,
		Origin:      model.Origin{Provider: "npm", Manager: "global", Package: "gh"},
		Version:     model.VersionInfo{Value: "2.40.0", State: model.VersionKnown, Confidence: model.ConfidenceHigh},
		Package:     &model.PackageInfo{Name: "gh", Prefix: "/opt/homebrew/lib/node_modules/gh"},
		Locations:   []model.Location{{Path: "/opt/homebrew/bin/gh", Type: model.LocationExecutable, Executable: true}},
	}
	observation.ID = model.ObservationIdentity(observation)
	return observation
}

// The inline label must be the action row's own wording, not a second copy of
// the same knowledge: asserting against KindActions is what keeps the column
// honest when a verb is reworded in output/actions.go.
func TestInlineActionColumnShowsUpdate(t *testing.T) {
	observation := npmCLIObservation()
	row := InventoryRowFromObservation(observation)

	want := ""
	for _, action := range KindActions(observation) {
		if action.Runnable() {
			want = action.Label
			break
		}
	}
	if want == "" {
		t.Fatal("fixture has no runnable action; the column would have nothing to prove")
	}
	if want != "Update" {
		t.Fatalf("primary npm action = %q, want %q", want, "Update")
	}

	got := inventoryValue(row, columnAction, overviewStyles())
	if got == "" {
		t.Fatal("ACTION cell is empty for an installed npm package")
	}
	if got != want {
		t.Fatalf("inventoryValue(columnAction) = %q, want %q", got, want)
	}
}

// An entry no manager can act on gets a blank cell rather than an invented
// verb, so the column never promises something the action row would not offer.
func TestInlineActionColumnIsBlankWithoutARunnableAction(t *testing.T) {
	observation := model.Observation{
		DisplayName: "mystery",
		CommandName: "mystery",
		Kind:        model.KindExecutable,
		Role:        model.RoleAvailable,
		Origin:      model.Origin{Provider: "unknown", Manager: "manual-or-unknown"},
		Version:     model.VersionInfo{State: model.VersionUnknown, Confidence: model.ConfidenceLow},
	}
	observation.ID = model.ObservationIdentity(observation)

	if _, ok := PrimaryAction(observation); ok {
		t.Skip("fixture gained a runnable action; nothing to assert about the blank case")
	}
	if got := inventoryValue(InventoryRowFromObservation(observation), columnAction, overviewStyles()); got != "" {
		t.Fatalf("ACTION cell = %q, want empty for an entry with no runnable action", got)
	}
}

// The column is gated at the top of the ladder, so every narrower terminal
// renders exactly the columns it did before.
func TestInlineActionColumnAppearsOnlyAtTheWidestWidths(t *testing.T) {
	wide := inventoryColumnsFor(inventoryActionBreakpoint)
	if len(wide) < 6 {
		t.Fatalf("inventoryColumnsFor(%d) returned %d columns, want at least 6",
			inventoryActionBreakpoint, len(wide))
	}
	if last := wide[len(wide)-1].title; last != columnAction {
		t.Fatalf("rightmost column at the breakpoint = %q, want %q", last, columnAction)
	}

	for _, width := range []int{40, 60, 72, 80, inventoryActionBreakpoint - 1} {
		columns := inventoryColumnsFor(width)
		if len(columns) >= 6 {
			t.Errorf("inventoryColumnsFor(%d) returned %d columns, want fewer than 6", width, len(columns))
		}
		for _, column := range columns {
			if column.title == columnAction {
				t.Errorf("inventoryColumnsFor(%d) includes %q below the action breakpoint", width, columnAction)
			}
		}
	}
}

// The column is only worth having if it does not eat the column users scan.
// This is the constraint that put the breakpoint where it is.
func TestActionColumnLeavesNameRoomForAGroupHeading(t *testing.T) {
	columns := inventoryColumnsFor(inventoryActionBreakpoint)
	name := columns[0]
	if name.title != columnName {
		t.Fatalf("first column = %q, want %q", name.title, columnName)
	}
	for source, label := range sourceGroupLabels {
		if lipgloss.Width(label) > name.width {
			t.Errorf("NAME is %d cells at the action breakpoint, too narrow for %s heading %q (%d cells)",
				name.width, source, label, lipgloss.Width(label))
		}
	}
}

// Adding a column must not change the frame contract: every rendered line is
// still exactly width cells, and there are still exactly height of them.
func TestInventoryTableStillFitsItsBudgetWithTheActionColumn(t *testing.T) {
	rows := []InventoryRow{InventoryRowFromObservation(npmCLIObservation())}
	lines := renderInventoryTable(rows, 0, inventoryActionBreakpoint, 5, overviewStyles())
	if len(lines) != 5 {
		t.Fatalf("renderInventoryTable returned %d lines, want 5", len(lines))
	}
	if !strings.Contains(strings.Join(lines, "\n"), columnAction) {
		t.Errorf("rendered table has no %s header:\n%s", columnAction, strings.Join(lines, "\n"))
	}
}
