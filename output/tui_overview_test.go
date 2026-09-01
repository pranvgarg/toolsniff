package output

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/pranvgarg/toolsniff/config"
	"github.com/pranvgarg/toolsniff/model"
	"github.com/pranvgarg/toolsniff/registry"
)

// overviewReport is a small machine with one entry per kind and per manager.
func overviewReport() ObservationReport {
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
		managed("opencode", model.KindCLI, "npm", "global"),
		managed("left-pad", model.KindPackage, "npm", "global"),
		managed("wget", model.KindPackage, "homebrew", "formula"),
		managed("visual-studio-code", model.KindPackage, "homebrew", "cask"),
		managed("ripgrep", model.KindExecutable, "cargo", "install"),
		managed("ruff", model.KindPackage, "pipx", "global"),
		managed("Ghostty", model.KindApplication, "applications", ""),
	}
	available := []model.Observation{observationWithPath("gh", model.RoleAvailable, model.KindExecutable,
		model.VersionInfo{State: model.VersionUnknown, Confidence: model.ConfidenceLow}, "/usr/local/bin/gh")}
	history := []model.Observation{observation("create-vite", model.RoleHistory, model.KindHistory,
		model.VersionInfo{State: model.VersionNotApplicable, Confidence: model.ConfidenceLow})}

	broken := installed[2]
	return NewObservationReport(installed, available, history, registry.ObservationDiff{
		Added:  []registry.ChangeEvent{{Kind: registry.ChangeAdded, Identity: installed[0].ID, After: &installed[0]}},
		Broken: []registry.ChangeEvent{{Kind: registry.ChangeBroken, Identity: broken.ID, Before: &broken}},
	}, nil)
}

func overviewStyles() ThemeStyles {
	theme, err := config.ThemeSettingsForPreset("toolsniff")
	if err != nil {
		return NewThemeStyles(config.ThemeSettings{})
	}
	return NewThemeStyles(theme)
}

func TestRenderOverviewGroupsByKindWithPlainMeanings(t *testing.T) {
	report := overviewReport()
	lines := renderOverview(report, overviewStyles(), 96, 24)
	rendered := strings.Join(lines, "\n")

	for _, want := range []string{
		"what's on this machine",
		"CLI tools", "commands installed by a package manager",
		"Packages", "npm", "Node.js packages installed globally",
		"Homebrew", "formulae + casks", "Cargo", "Rust binaries", "pipx", "Python apps",
		"Applications", "macOS .app bundles you open from Finder",
		"On your PATH", "not installed by a manager",
		"npx history", "run once via npx, not installed",
		"Changes", "since the last saved baseline",
		"Issues", "broken or shadowed",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("overview missing %q:\n%s", want, rendered)
		}
	}
	// The overview describes the machine in plain words; the scanner's own
	// bucket names must not leak into it.
	if strings.Contains(rendered, "available") {
		t.Errorf("overview leaked the bare word \"available\":\n%s", rendered)
	}
}

func TestOverviewCountsMatchTheTabsTheyLinkTo(t *testing.T) {
	report := overviewReport()
	inventory, changes := overviewRows(report)

	byLabel := map[string]int{}
	for _, row := range append(append([]overviewRow(nil), inventory...), changes...) {
		byLabel[strings.TrimSpace(row.label)] = row.count
	}
	for label, view := range map[string]ViewCategory{
		"CLI tools":    ViewCLI,
		"Packages":     ViewPackages,
		"Applications": ViewApplications,
		"On your PATH": ViewPathExecutables,
		"npx history":  ViewNpxHistory,
		"Changes":      ViewChanges,
		"Issues":       ViewIssues,
	} {
		if want := CountForView(report, view); byLabel[label] != want {
			t.Errorf("overview %q count = %d, but view %q holds %d", label, byLabel[label], view, want)
		}
	}

	// The manager breakdown has to add up to the Packages total, or the
	// dashboard is quietly dropping entries.
	breakdown := 0
	for _, row := range inventory {
		if row.indent {
			breakdown += row.count
		}
	}
	if want := CountForView(report, ViewPackages); breakdown != want {
		t.Fatalf("manager breakdown sums to %d, Packages holds %d", breakdown, want)
	}
}

func TestRenderOverviewStaysInsideItsPane(t *testing.T) {
	report := overviewReport()
	styles := overviewStyles()
	for _, height := range []int{1, 4, 8, 12, 24, 40} {
		for _, width := range []int{40, 60, 96} {
			lines := renderOverview(report, styles, width, height)
			if len(lines) > height {
				t.Fatalf("overview at %dx%d produced %d lines", width, height, len(lines))
			}
			for index, line := range lines {
				if got := lipgloss.Width(line); got > width {
					t.Fatalf("overview line %d at width %d is %d cells wide: %q", index, width, got, line)
				}
			}
		}
	}
	// A pane too short for the manager breakdown still names every top-level
	// group rather than truncating the list.
	short := strings.Join(renderOverview(report, styles, 96, 12), "\n")
	for _, want := range []string{"CLI tools", "Packages", "Applications", "On your PATH", "npx history", "Issues"} {
		if !strings.Contains(short, want) {
			t.Errorf("short overview dropped %q:\n%s", want, short)
		}
	}
}
