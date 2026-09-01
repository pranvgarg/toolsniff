package output

// The Manage view (ui.mode "v3") answers one question: what is on this machine
// that a package manager put here, and could therefore update or remove it. It
// is deliberately a single list rather than three tabs -- to the person doing
// the managing, an npm CLI tool, a Homebrew formula with no command, and a
// Homebrew cask are the same job, and splitting them by kind hides the one fact
// that decides what to do about each: who installed it.
//
// The list is ordered by sourceGroupOrder, so the manager that owns most of the
// machine leads and the residue trails. See output/kinds.go for that order and
// for every label the headings use.

// renderManage renders the whole Manage view for a report as exactly height
// lines of exactly width cells. It is the unfiltered entry point -- used by
// tests and by any caller holding only a report -- and derives its rows from
// the same predicate the tab count reads, so the caption and the list can never
// disagree.
// It takes styles where renderOverview does, and hands off to a row-level
// renderer that takes them where renderGroupedInventoryTable does.
func renderManage(report ObservationReport, selected int, styles ThemeStyles, width, height int) []string {
	return renderManageRows(InventoryRows(observationsForView(report, ViewManage)), selected, width, height, styles)
}

// renderManageRows renders an already-narrowed Manage row set. The TUI shell
// goes through this one rather than through renderManage because its rows have
// had the user's search and facets applied; re-deriving them from the report
// would quietly ignore the filter the user is looking at.
func renderManageRows(rows []InventoryRow, selected, width, height int, styles ThemeStyles) []string {
	// Sorting is idempotent: FilterReport already sorts every grouped view, so
	// this only matters for callers that hand over raw rows. Grouping is only
	// coherent when each manager's rows are contiguous, and paying for a sort
	// twice is cheaper than a pane whose headings repeat.
	return renderGroupedInventoryTable(sortRowsByGroup(rows), selected, width, height, styles)
}
