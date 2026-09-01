package output

// The Review view (ui.mode "v3") answers one question: what on this machine
// needs a decision. Its material is not the inventory but the typed change
// events -- what moved since the last saved baseline, and what is wrong now --
// so it is the one intent view whose rows are built from events rather than
// from observations.
//
// The v2 navigation split that material across two tabs, Changes and Issues,
// which is a split the events themselves do not honour: Broken and Shadowed
// are two of the seven change categories, so the two tabs overlapped and a
// broken tool appeared under both. Review partitions the seven categories
// instead of intersecting them -- see reviewSections in output/kinds.go -- so
// every event is shown exactly once, under the heading that says why it is
// worth looking at:
//
//	Changes   removed, updated, relocated, repaired
//	Issues    broken, shadowed
//	New       added
//
// Each row carries its own reason in the STATUS column, in the same wording
// and the same semantic colour every other pane uses, so "broken" here and
// "broken" in the inventory are visibly the same claim. Every label comes from
// output/kinds.go; this file composes no strings of its own.

// renderReview renders the whole Review view for a report as exactly height
// lines of exactly width cells. It is the unfiltered entry point -- used by
// tests and by any caller holding only a report -- and derives its rows from
// RowsForReport, the same call the tab count is checked against, so the caption
// and the list can never disagree.
// It takes styles where renderManage and renderDiscover do, and hands off to
// the one row-level renderer every grouped pane in the TUI shares.
func renderReview(report ObservationReport, selected int, styles ThemeStyles, width, height int) []string {
	return renderReviewRows(RowsForReport(report, ViewReview), selected, width, height, styles)
}

// renderReviewRows renders an already-narrowed Review row set. The TUI shell
// goes through this one rather than through renderReview because its rows have
// had the user's search and facets applied; re-deriving them from the report
// would quietly ignore the filter the user is looking at.
func renderReviewRows(rows []InventoryRow, selected, width, height int, styles ThemeStyles) []string {
	return renderInventoryGroups(reviewGroups(rows), selected, width, height, styles)
}

// reviewGroups splits already-section-ordered rows into the Review blocks. It
// mirrors inventoryGroups and discoverGroups, including the flat Start index
// that lets a heading stay unselectable, and differs only in what it keys on:
// the section a row's status belongs to rather than the manager that installed
// it, because a change event has no manager to group under.
//
// Rows arrive in section order from rowsForEvents, so each section's rows are
// already contiguous. Rows that are not still render correctly; they just
// produce one block per run, which is a visible symptom rather than a silent
// mis-grouping -- the same contract inventoryGroups holds.
func reviewGroups(rows []InventoryRow) []inventoryGroup {
	groups := make([]inventoryGroup, 0, len(reviewSections))
	for index, row := range rows {
		section := reviewSectionFor(Status(row.Status))
		if len(groups) > 0 && groups[len(groups)-1].Source == section.Key {
			last := &groups[len(groups)-1]
			last.Rows = append(last.Rows, row)
			continue
		}
		groups = append(groups, inventoryGroup{
			Label:   section.Label,
			Meaning: section.Meaning,
			Source:  section.Key,
			Start:   index,
			Rows:    []InventoryRow{row},
		})
	}
	return groups
}
