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
func RenderTable(tools, npxHistory []model.Tool, diff registry.Diff, warnings []scanner.Warning) string {
	var b strings.Builder

	grouped := groupBySource(tools)
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

	if len(diff.Added) > 0 || len(diff.Removed) > 0 {
		b.WriteString("NEW SINCE LAST SCAN\n")
		b.WriteString(RenderDiff(diff))
		b.WriteString("\n")
	}

	for _, w := range warnings {
		fmt.Fprintf(&b, "warning: %s: %v\n", w.Source, w.Err)
	}

	fmt.Fprintf(&b, "%d tools across %d sources\n", len(tools), len(sources))
	return b.String()
}

// RenderDiff renders just the added/removed tools, used by --diff and
// embedded into RenderTable.
func RenderDiff(diff registry.Diff) string {
	if len(diff.Added) == 0 && len(diff.Removed) == 0 {
		return "no changes since last scan\n"
	}
	var b strings.Builder
	for _, t := range diff.Added {
		fmt.Fprintf(&b, "  + %s (%s)\n", t.Name, t.Source)
	}
	for _, t := range diff.Removed {
		fmt.Fprintf(&b, "  - %s (%s)\n", t.Name, t.Source)
	}
	return b.String()
}
