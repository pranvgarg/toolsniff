package output

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/pranvgarg/toolsniff/model"
)

// The Discover view (ui.mode "v3") is Manage's complement: everything on this
// machine that no package manager put here. That is two things -- programs the
// user placed on their PATH themselves, and the npx cache -- and they are one
// list because they answer one question: what is running here that nothing is
// keeping up to date?
//
// Manage groups by manager, because "who installed this" is the fact that
// decides what to do about it. Discover cannot: by definition none of its rows
// has a manager, so grouping by one would produce a single undifferentiated
// block. It groups by the directory the binary sits in instead, which is the
// next most actionable fact -- /usr/local/bin and ~/bin were populated by
// different habits and are cleaned up differently. The npx cache is not a
// directory the user chose, so it trails as its own named block.
//
// Every label here comes from output/kinds.go; the only strings this file
// composes are directory paths read off the machine.

// renderDiscover renders the whole Discover view for a report as exactly height
// lines of exactly width cells. It is the unfiltered entry point -- used by
// tests and by any caller holding only a report -- and derives its rows from
// the same predicate the tab count reads, so the caption and the list can never
// disagree.
func renderDiscover(report ObservationReport, selected int, styles ThemeStyles, width, height int) []string {
	rows := InventoryRows(observationsForView(report, ViewDiscover))
	// No marks, for the reason renderManage passes none: a report carries no
	// multi-select state.
	return renderDiscoverRows(rows, discoverSuggestion(report, rows), selected, nil, width, height, styles)
}

// renderDiscoverRows renders an already-narrowed Discover row set. The TUI
// shell goes through this one rather than through renderDiscover because its
// rows have had the user's search and facets applied; re-deriving them from the
// report would quietly ignore the filter the user is looking at.
func renderDiscoverRows(rows []InventoryRow, suggestion string, selected int, marks rowMarks, width, height int, styles ThemeStyles) []string {
	if height < 1 {
		height = 1
	}
	// Sorting is idempotent: FilterReport already sorts Discover's rows, so this
	// only matters for callers that hand over raw rows. Grouping is coherent
	// only when each directory's rows are contiguous, and paying for a sort
	// twice is cheaper than a pane whose headings repeat.
	rows = sortRowsByDirectory(rows)

	// The footer is the first thing to go. It needs the column header and at
	// least one row above it to be worth a line at all, and it must never push
	// the pane past the caller's height budget.
	body := height
	if suggestion != "" && height >= 3 {
		body = height - 1
	} else {
		suggestion = ""
	}

	lines := renderInventoryGroups(discoverGroups(rows), selected, marks, width, body, styles)
	if suggestion != "" {
		lines = append(lines, fitWidth(styles.Footer.Render(suggestion), width))
	}
	return lines
}

// --- directory groups -------------------------------------------------------

// discoverGroups splits already-directory-sorted rows into blocks: one per
// directory, then the npx cache. It mirrors inventoryGroups, including the flat
// Start index that lets a heading stay unselectable, and differs only in what
// it keys on.
func discoverGroups(rows []InventoryRow) []inventoryGroup {
	groups := make([]inventoryGroup, 0, 8)
	for index, row := range rows {
		key := discoverGroupKey(row)
		if len(groups) > 0 && groups[len(groups)-1].Source == key {
			last := &groups[len(groups)-1]
			last.Rows = append(last.Rows, row)
			continue
		}
		label, meaning := discoverGroupLabel(row)
		groups = append(groups, inventoryGroup{
			Label:   label,
			Meaning: meaning,
			Source:  key,
			Start:   index,
			Rows:    []InventoryRow{row},
		})
	}
	return groups
}

// discoverGroupKey is the directory a row's binary sits in, or the npx cache's
// own source for a history entry. A row whose location the scanner never
// recorded has no directory to group under and falls back to the PATH source,
// so it lands in one named block rather than in a block headed by "".
func discoverGroupKey(row InventoryRow) string {
	if sourceGroupKey(row.Source) == model.SourceNPXHistory {
		return model.SourceNPXHistory
	}
	if strings.TrimSpace(row.Path) == "" {
		return model.SourcePath
	}
	return filepath.Dir(row.Path)
}

// discoverGroupLabel is the heading for a row's block. The two non-directory
// keys read their wording out of kinds.go; a directory is its own label, and
// borrows the PATH gloss because that is exactly what its rows are.
func discoverGroupLabel(row InventoryRow) (string, string) {
	key := discoverGroupKey(row)
	if label, ok := sourceGroupLabels[key]; ok {
		return label, sourceGroupMeanings[key]
	}
	return key, sourceGroupMeanings[model.SourcePath]
}

// sortRowsByDirectory puts the directory blocks first in path order and the npx
// cache last, with rows sorted by name inside a block. FilterReport calls it so
// the flat selection index and the rendered blocks describe the same order --
// the same contract sortRowsByGroup holds for the manager-grouped views.
func sortRowsByDirectory(rows []InventoryRow) []InventoryRow {
	sorted := append([]InventoryRow(nil), rows...)
	sort.SliceStable(sorted, func(i, j int) bool {
		left, right := discoverGroupKey(sorted[i]), discoverGroupKey(sorted[j])
		if left != right {
			leftRank, rightRank := discoverGroupRank(left), discoverGroupRank(right)
			if leftRank != rightRank {
				return leftRank < rightRank
			}
			// Same rank means both are directories: sort them by path, so the
			// blocks read in an order the user can predict.
			return left < right
		}
		return strings.ToLower(sorted[i].Name) < strings.ToLower(sorted[j].Name)
	})
	return sorted
}

// discoverGroupRank ranks one block. A directory is not a source, so it ranks
// as the PATH entries it holds -- which is what puts every directory ahead of
// the npx cache without giving any directory precedence over another.
func discoverGroupRank(key string) int {
	if rank, ok := discoverGroupOrder[key]; ok {
		return rank
	}
	return discoverGroupOrder[model.SourcePath]
}

// --- the suggestion footer --------------------------------------------------

// discoverSuggestion builds the pane's closing tip from what the machine
// already shows: the manager that owns the most of it, and one entry it does
// not own. Both halves are data -- the manager is counted, the command comes
// from that manager's own install template in output/actions.go, and the
// sentence is worded by kinds.go -- so the tip can never offer a manager that
// is not installed here or a command that manager would not accept.
func discoverSuggestion(report ObservationReport, rows []InventoryRow) string {
	name := unmanagedSuggestionName(rows)
	if name == "" {
		return ""
	}
	return DiscoverSuggestionLine(name, installCommandForOrigin(dominantManagerOrigin(report), name))
}

// unmanagedSuggestionName picks the entry the tip talks about: the first PATH
// executable in display order, so the sentence names something the user can see
// without scrolling. npx cache entries are skipped -- nothing installs them.
func unmanagedSuggestionName(rows []InventoryRow) string {
	for _, row := range sortRowsByDirectory(rows) {
		if sourceGroupKey(row.Source) != model.SourceNPXHistory {
			return row.Name
		}
	}
	return ""
}

// dominantManagerOrigin is the origin key that installed the most of this
// machine, among the entries Manage covers. Ties break on sourceGroupOrder, so
// the answer is the established manager rather than whichever the scanner
// happened to report first. Returns "" when no manager installed anything.
func dominantManagerOrigin(report ObservationReport) string {
	counts := make(map[string]int)
	for _, observation := range observationsForView(report, ViewManage) {
		counts[originKey(observation)]++
	}
	best, bestCount := "", 0
	for origin, count := range counts {
		switch {
		case count > bestCount:
			best, bestCount = origin, count
		case count == bestCount && originGroupRank(origin) < originGroupRank(best):
			best = origin
		}
	}
	return best
}
