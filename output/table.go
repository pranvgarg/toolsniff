package output

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pranvgarg/toolsniff/model"
	"github.com/pranvgarg/toolsniff/registry"
	"github.com/pranvgarg/toolsniff/scanner"
)

func groupBySource(tools []model.Tool) map[string][]model.Tool {
	grouped := map[string][]model.Tool{}
	for _, t := range tools {
		grouped[t.Source] = append(grouped[t.Source], t)
	}
	return grouped
}

func sortedSourceKeys(grouped map[string][]model.Tool) []string {
	keys := make([]string, 0, len(grouped))
	for k := range grouped {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// RenderTable produces a plain grouped-by-source table for --list.
func RenderTable(tools, available, npxHistory []model.Tool, diff, availabilityDiff registry.Diff, warnings []scanner.Warning) string {
	var b strings.Builder

	currentTools := append(append([]model.Tool{}, tools...), available...)
	grouped := groupBySource(currentTools)
	sources := sortedSourceKeys(grouped)
	for _, src := range sources {
		fmt.Fprintf(&b, "%s (%d)\n", strings.ToUpper(src), len(grouped[src]))
		for _, t := range grouped[src] {
			if t.Version != "" {
				fmt.Fprintf(&b, "  %-30s %s\n", t.Name, t.Version)
			} else {
				fmt.Fprintf(&b, "  %s\n", t.Name)
			}
		}
		b.WriteString("\n")
	}

	if len(npxHistory) > 0 {
		fmt.Fprintf(&b, "NPX HISTORY (%d, informational)\n", len(npxHistory))
		for _, t := range npxHistory {
			fmt.Fprintf(&b, "  %-30s %s\n", t.Name, t.Version)
		}
		b.WriteString("\n")
	}

	if diffHasChanges(diff) {
		b.WriteString("NEW SINCE LAST SCAN\n")
		b.WriteString(RenderDiff(diff))
		b.WriteString("\n")
	}
	if diffHasChanges(availabilityDiff) {
		b.WriteString("AVAILABILITY CHANGES\n")
		b.WriteString(RenderDiff(availabilityDiff))
		b.WriteString("\n")
	}

	for _, w := range warnings {
		fmt.Fprintf(&b, "warning: %s: %v\n", w.Source, w.Err)
	}

	installed, availableCount := countToolRoles(currentTools)
	if availableCount == 0 {
		fmt.Fprintf(&b, "%d installed tools across %d sources\n", installed, len(sources))
	} else {
		label := "available command"
		if availableCount != 1 {
			label = "available commands"
		}
		fmt.Fprintf(&b, "%d installed tools and %d %s across %d sources\n", installed, availableCount, label, len(sources))
	}
	return b.String()
}

func countToolRoles(tools []model.Tool) (installed, available int) {
	for _, tool := range tools {
		if tool.Role == model.RoleAvailable || tool.Source == model.SourcePath {
			available++
		} else {
			installed++
		}
	}
	return installed, available
}

func countSources(tools []model.Tool) int {
	sources := make(map[string]struct{}, len(tools))
	for _, tool := range tools {
		if tool.Role == model.RoleHistory || tool.Source == model.SourceNPXHistory {
			continue
		}
		sources[tool.Source] = struct{}{}
	}
	return len(sources)
}

func diffHasChanges(diff registry.Diff) bool {
	return len(diff.Added) > 0 || len(diff.Removed) > 0 || len(diff.Updated) > 0
}

// RenderDiff renders just the added/removed tools, used by --diff and
// embedded into RenderTable.
func RenderDiff(diff registry.Diff) string {
	if !diffHasChanges(diff) {
		return "no changes since last scan\n"
	}
	var b strings.Builder
	for _, t := range diff.Added {
		fmt.Fprintf(&b, "  + %s (%s)\n", t.Name, t.Source)
	}
	for _, t := range diff.Removed {
		fmt.Fprintf(&b, "  - %s (%s)\n", t.Name, t.Source)
	}
	if len(diff.Updated) > 0 {
		b.WriteString("UPDATED\n")
		for _, change := range diff.Updated {
			fmt.Fprintf(&b, "  ~ %s (%s) %s -> %s\n", change.After.Name, change.After.Source, change.Before.Version, change.After.Version)
		}
	}
	return b.String()
}

// RenderObservationTable renders the v2 report with separate version, status,
// and source columns. RenderTable remains available to direct legacy callers.
func RenderObservationTable(report ObservationReport) string {
	rows := RowsForReport(report, ViewAll)
	var b strings.Builder
	fmt.Fprintln(&b, "NAME                           VERSION        STATUS       SOURCE")
	fmt.Fprintln(&b, "---------------------------------------------------------------------")
	for _, row := range rows {
		fmt.Fprintf(&b, "%-30s %-14s %-12s %s\n", row.Name, row.Version, row.Status, row.Source)
	}
	if len(rows) == 0 {
		b.WriteString("No observations found.\n")
	}
	if len(report.Changes.Events()) > 0 {
		b.WriteString("\nCHANGES\n")
		b.WriteString(RenderChangeReport(report.Changes))
	}
	if len(report.Warnings) > 0 {
		for _, warning := range report.Warnings {
			fmt.Fprintf(&b, "warning: %s\n", warning)
		}
	}
	return b.String()
}

// RenderReportTable is the descriptive alias for the v2 table renderer.
func RenderReportTable(report ObservationReport) string {
	return RenderObservationTable(report)
}

// RenderTableV2 is kept as an obvious migration target for the future CLI.
func RenderTableV2(report ObservationReport) string {
	return RenderObservationTable(report)
}
