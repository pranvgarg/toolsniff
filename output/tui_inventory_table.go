package output

import (
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
)

// This file owns every inventory list rendered inside the TUI frame. Both the
// v2 report pane and the legacy per-source pane go through it, so "installed
// vs available vs broken" looks the same everywhere. See DESIGN.md >
// Components > "Row -- installed/available/broken".

// Column indices in the rendered matrix. The two leading one-cell columns are
// structural: barColumn carries the selection marker, glyphColumn carries the
// monochrome-safe status marker.
const (
	barColumn = iota
	glyphColumn
	firstDataColumn
)

// inventoryColumn is one rendered column. Widths are display cells and exclude
// the one cell of padding applied to each side by the cell style.
type inventoryColumn struct {
	title string
	width int
	right bool
	tail  bool // truncate from the front, keeping the identifying suffix
}

// Column titles double as the value selector in inventoryValue, so a column
// is added in exactly one place.
const (
	columnName    = "NAME"
	columnVersion = "VERSION"
	columnKind    = "KIND"
	columnStatus  = "STATUS"
	columnSource  = "SOURCE"
	columnAction  = "ACTION"
)

const (
	inventoryVersionWidth = 10
	inventoryKindWidth    = 6 // "↺ hist", the widest marker
	inventoryStatusWidth  = 10
	inventorySourceWidth  = 14
	// Fits the verbs a manager offers ("Uninstall" is the longest); the rare
	// longer label truncates rather than widening the column for every row.
	inventoryActionWidth = 10

	// Column-ladder breakpoints, in content-pane cells. DESIGN.md > Layout >
	// "Responsive ladder". Kind sits above status because the status glyph
	// column already carries status at every width, while nothing else in the
	// row says whether a name is a package, an app, or a bare executable.
	inventoryStatusBreakpoint = 48
	inventoryKindBreakpoint   = 60
	inventorySourceBreakpoint = 72
	// Action is last on the ladder and sits well above source rather than
	// alongside it. Every fixed-width column is paid for out of NAME, so
	// sharing the source breakpoint would collapse NAME to 8 cells and cost
	// the one column a user actually scans. 105 is the first width at which
	// NAME still holds the longest manager-group heading (41 cells) with the
	// action column present -- i.e. the first width where it is free.
	// TestActionColumnLeavesNameRoomForAGroupHeading pins that derivation.
	inventoryActionBreakpoint = 105
)

// inventoryColumnsFor returns the data columns that fit in a content pane of
// the given width, widest form first.
func inventoryColumnsFor(width int) []inventoryColumn {
	if width <= 0 {
		width = 80
	}
	columns := []inventoryColumn{
		{title: columnName},
		{title: columnVersion, width: inventoryVersionWidth, right: true, tail: true},
	}
	// Appended in display order; each breakpoint is higher than the last so
	// the conditions stay monotonic with the order.
	if width >= inventoryKindBreakpoint {
		columns = append(columns, inventoryColumn{title: columnKind, width: inventoryKindWidth})
	}
	if width >= inventoryStatusBreakpoint {
		columns = append(columns, inventoryColumn{title: columnStatus, width: inventoryStatusWidth})
	}
	if width >= inventorySourceBreakpoint {
		columns = append(columns, inventoryColumn{title: columnSource, width: inventorySourceWidth})
	}
	// Rightmost: a verb is only worth a column once the row already says where
	// the entry came from, since "Uninstall" means nothing without its manager.
	if width >= inventoryActionBreakpoint {
		columns = append(columns, inventoryColumn{title: columnAction, width: inventoryActionWidth})
	}

	// Every column costs its own width plus one cell of padding per side; the
	// two structural columns cost one cell each with no padding. Name absorbs
	// whatever is left instead of being stretched across the whole pane and
	// leaving a dead gutter on the right.
	fixed := 2
	for _, column := range columns {
		fixed += column.width + 2
	}
	name := width - fixed
	if name < 6 {
		name = 6
	}
	columns[0].width = name
	return columns
}

// inventoryCells formats one row into the fixed-width cells of columns. Values
// are selected by column title rather than by position, so inserting a column
// into the ladder cannot silently shift every value one cell to the right.
func inventoryCells(row InventoryRow, columns []inventoryColumn, styles ThemeStyles) []string {
	cells := make([]string, 0, len(columns))
	for _, column := range columns {
		cells = append(cells, fitCell(inventoryValue(row, column.title, styles), column))
	}
	return cells
}

func inventoryValue(row InventoryRow, title string, styles ThemeStyles) string {
	switch title {
	case columnName:
		return row.Name
	case columnVersion:
		return row.Version
	case columnKind:
		return styles.kindMarker(row.Kind)
	case columnStatus:
		return StatusDisplayLabel(row.Status)
	case columnSource:
		return row.Source
	case columnAction:
		// Display only. The wording comes from output/actions.go via
		// InventoryRowFromObservation, so the cell can never name an operation
		// the action row would not offer. Blank when nothing is runnable.
		return row.Action
	}
	return ""
}

// actionCellColumn is the ACTION column's index in the rendered matrix, or -1
// when the ladder is too narrow to include it. The style funcs address columns
// by index, so the title-to-index mapping is resolved once, here.
func actionCellColumn(columns []inventoryColumn) int {
	for index, column := range columns {
		if column.title == columnAction {
			return firstDataColumn + index
		}
	}
	return -1
}

func fitCell(value string, column inventoryColumn) string {
	if column.tail {
		value = truncateTail(value, column.width)
	}
	if column.right {
		value = rightAlign(value, column.width)
	}
	return fitWidth(value, column.width)
}

// renderInventoryTable renders rows as exactly height lines, each exactly
// width cells wide: one header line plus a scrolled window of data rows. The
// window is computed here rather than delegated to a viewport so the selected
// row's index stays meaningful to the style function.
func renderInventoryTable(rows []InventoryRow, selected, width, height int, styles ThemeStyles) []string {
	if height < 1 {
		height = 1
	}
	columns := inventoryColumnsFor(width)

	bodyHeight := height - 1 // the header line
	if bodyHeight < 0 {
		bodyHeight = 0
	}
	start := scrollOffset(selected, len(rows), bodyHeight)
	end := start + bodyHeight
	if end > len(rows) {
		end = len(rows)
	}
	window := rows[start:end]
	cursor := selected - start

	headers := make([]string, 0, len(columns)+firstDataColumn)
	headers = append(headers, " ", " ")
	for _, column := range columns {
		headers = append(headers, fitCell(column.title, column))
	}

	matrix := make([][]string, 0, len(window))
	for index, row := range window {
		cells := make([]string, 0, len(columns)+firstDataColumn)
		bar := " "
		if index == cursor {
			bar = styles.Glyph.Selection
		}
		cells = append(cells, bar, styles.rowGlyph(row.Status))
		matrix = append(matrix, append(cells, inventoryCells(row, columns, styles)...))
	}

	rendered := table.New().
		Border(lipgloss.HiddenBorder()).
		BorderTop(false).BorderBottom(false).BorderLeft(false).
		BorderRight(false).BorderRow(false).BorderColumn(false).BorderHeader(false).
		Headers(headers...).
		Rows(matrix...).
		StyleFunc(inventoryStyleFunc(window, cursor, actionCellColumn(columns), styles)).
		Render()

	// A table with no data rows still emits a trailing blank line; fitLines
	// trims it so the caller's height budget is exact.
	return fitLines(strings.Split(rendered, "\n"), width, height)
}

// --- manager sub-groups ----------------------------------------------------

// inventoryGroup is one manager's contiguous block of rows inside a kind-first
// pane. Start is the index of its first row in the flat, selectable row slice,
// so a group heading never has to become a selectable row.
type inventoryGroup struct {
	Label   string
	Meaning string
	Source  string
	Start   int
	Rows    []InventoryRow
}

// inventoryGroups splits already-group-sorted rows into manager blocks. Rows
// that are not sorted by group still render correctly; they just produce one
// block per run, which is a visible symptom rather than a silent mis-grouping.
func inventoryGroups(rows []InventoryRow) []inventoryGroup {
	groups := make([]inventoryGroup, 0, 8)
	for index, row := range rows {
		key := sourceGroupKey(row.Source)
		if len(groups) > 0 && sourceGroupKey(groups[len(groups)-1].Source) == key {
			last := &groups[len(groups)-1]
			last.Rows = append(last.Rows, row)
			continue
		}
		groups = append(groups, inventoryGroup{
			Label:   sourceGroupLabel(row.Source),
			Meaning: sourceGroupMeaning(row.Source),
			Source:  row.Source,
			Start:   index,
			Rows:    []InventoryRow{row},
		})
	}
	return groups
}

// groupedEntry is one rendered line of a grouped pane: either a manager
// heading or a real row. Only rows carry a flat index, so selection always
// maps to an observation.
type groupedEntry struct {
	heading bool
	label   string
	meaning string
	count   int
	row     InventoryRow
	flat    int
}

func groupedEntries(groups []inventoryGroup) []groupedEntry {
	entries := make([]groupedEntry, 0, len(groups)*4)
	for _, group := range groups {
		entries = append(entries, groupedEntry{
			heading: true,
			label:   group.Label,
			meaning: group.Meaning,
			count:   len(group.Rows),
			flat:    -1,
		})
		for offset, row := range group.Rows {
			entries = append(entries, groupedEntry{row: row, flat: group.Start + offset})
		}
	}
	return entries
}

// groupedInventoryLineCount is how many lines a grouped pane needs to show
// everything: the column header, plus one line per row and per heading.
func groupedInventoryLineCount(rows []InventoryRow) int {
	return 1 + len(rows) + len(inventoryGroups(rows))
}

// renderGroupedInventoryTable renders rows as exactly height lines, with a
// counted manager heading above each block. It is the same single-table render
// as renderInventoryTable -- a heading is a row whose NAME cell carries the
// label and whose VERSION cell carries the count -- so every column stays
// aligned across blocks without a colspan primitive lipgloss does not have.
// See DESIGN.md > Components > "Kind sub-group header".
func renderGroupedInventoryTable(rows []InventoryRow, selected, width, height int, styles ThemeStyles) []string {
	if height < 1 {
		height = 1
	}
	columns := inventoryColumnsFor(width)
	entries := groupedEntries(inventoryGroups(rows))

	bodyHeight := height - 1 // the column header line
	if bodyHeight < 0 {
		bodyHeight = 0
	}
	cursorEntry := -1
	for index, entry := range entries {
		if !entry.heading && entry.flat == selected {
			cursorEntry = index
			break
		}
	}
	start := scrollOffset(cursorEntry, len(entries), bodyHeight)
	end := start + bodyHeight
	if end > len(entries) {
		end = len(entries)
	}
	window := entries[start:end]
	cursor := cursorEntry - start

	headers := make([]string, 0, len(columns)+firstDataColumn)
	headers = append(headers, " ", " ")
	for _, column := range columns {
		headers = append(headers, fitCell(column.title, column))
	}

	matrix := make([][]string, 0, len(window))
	for index, entry := range window {
		cells := []string{" ", " "}
		if entry.heading {
			matrix = append(matrix, append(cells, groupHeadingCells(entry, columns)...))
			continue
		}
		if index == cursor {
			cells[barColumn] = styles.Glyph.Selection
		}
		cells[glyphColumn] = styles.rowGlyph(entry.row.Status)
		matrix = append(matrix, append(cells, inventoryCells(entry.row, columns, styles)...))
	}

	rendered := table.New().
		Border(lipgloss.HiddenBorder()).
		BorderTop(false).BorderBottom(false).BorderLeft(false).
		BorderRight(false).BorderRow(false).BorderColumn(false).BorderHeader(false).
		Headers(headers...).
		Rows(matrix...).
		StyleFunc(groupedStyleFunc(window, cursor, actionCellColumn(columns), styles)).
		Render()

	return fitLines(strings.Split(rendered, "\n"), width, height)
}

// groupHeadingCells puts the manager label in the NAME column and its count in
// the VERSION column, where the numeric is already right-aligned. The gloss is
// appended only when it fits, so a narrow pane loses the explanation rather
// than the label.
func groupHeadingCells(entry groupedEntry, columns []inventoryColumn) []string {
	cells := make([]string, 0, len(columns))
	for _, column := range columns {
		switch column.title {
		case columnName:
			label := entry.label
			if entry.meaning != "" {
				withMeaning := label + " — " + entry.meaning
				if lipgloss.Width(withMeaning) <= column.width {
					label = withMeaning
				}
			}
			cells = append(cells, fitCell(label, column))
		case columnVersion:
			cells = append(cells, fitCell(itoa(entry.count), column))
		default:
			cells = append(cells, fitCell("", column))
		}
	}
	return cells
}

// groupedStyleFunc is inventoryStyleFunc plus one case: a heading row takes the
// column-header style, so it reads as structure rather than as an entry with a
// missing version.
// muteActionCell drops the ACTION cell to the muted tone so the verb reads as
// an affordance rather than as data about the entry. The row's semantic color
// still owns every other cell, and the caller applies selection afterwards, so
// a selected row keeps its raised background.
func muteActionCell(style lipgloss.Style, column, action, row int, styles ThemeStyles) lipgloss.Style {
	if action < 0 || column != action || row == table.HeaderRow {
		return style
	}
	return style.Foreground(styles.Palette.Muted.color())
}

func groupedStyleFunc(window []groupedEntry, cursor, action int, styles ThemeStyles) table.StyleFunc {
	return func(row, column int) lipgloss.Style {
		var style lipgloss.Style
		heading := false
		switch {
		case row == table.HeaderRow:
			style = styles.ColumnHeader
		case row >= 0 && row < len(window):
			if window[row].heading {
				style, heading = styles.ColumnHeader, true
			} else {
				style = styles.rowStyle(window[row].row.Status)
			}
		default:
			style = styles.Body
		}

		switch column {
		case barColumn:
			if row == cursor && row != table.HeaderRow {
				style = styles.SelectionBar
			}
		case glyphColumn:
			// no padding; the glyph is already a single cell
		default:
			style = style.Padding(0, 1)
			if !heading {
				style = muteActionCell(style, column, action, row, styles)
			}
		}

		if row == cursor && row != table.HeaderRow && !heading {
			style = style.Background(styles.Palette.SurfaceSelected.color()).Bold(true)
		}
		return style
	}
}

// fitLines trims a rendered table's trailing blank line and pads or truncates
// to exactly height lines of exactly width cells.
func fitLines(lines []string, width, height int) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" && len(lines) > height {
		lines = lines[:len(lines)-1]
	}
	out := make([]string, height)
	for i := range out {
		if i < len(lines) {
			out[i] = fitWidth(lines[i], width)
			continue
		}
		out[i] = fitWidth("", width)
	}
	return out
}

// inventoryStyleFunc maps (row, column) to a style. The row's semantic status
// drives its color; selection adds a raised background and a weight bump on
// top of it, so a selected broken row still reads as broken.
func inventoryStyleFunc(window []InventoryRow, cursor, action int, styles ThemeStyles) table.StyleFunc {
	return func(row, column int) lipgloss.Style {
		var style lipgloss.Style
		switch {
		case row == table.HeaderRow:
			style = styles.ColumnHeader
		case row >= 0 && row < len(window):
			style = styles.rowStyle(window[row].Status)
		default:
			style = styles.Body
		}

		switch column {
		case barColumn:
			if row == cursor && row != table.HeaderRow {
				style = styles.SelectionBar
			}
		case glyphColumn:
			// no padding; the glyph is already a single cell
		default:
			style = style.Padding(0, 1)
			style = muteActionCell(style, column, action, row, styles)
		}

		if row == cursor && row != table.HeaderRow {
			style = style.Background(styles.Palette.SurfaceSelected.color()).Bold(true)
		}
		return style
	}
}

// scrollOffset keeps the cursor inside a window of the given height, centering
// it once the list is longer than the pane.
func scrollOffset(selected, total, height int) int {
	if height <= 0 || total <= height || selected < 0 {
		return 0
	}
	offset := selected - height/2
	if offset < 0 {
		offset = 0
	}
	if offset > total-height {
		offset = total - height
	}
	return offset
}

// renderRowCountBadge returns the right-aligned "13/203 shown" indicator used
// when a view has fewer rows than the pane can hold, so an under-full pane is
// never ambiguous with a broken one.
func renderRowCountBadge(shown, total, width int, styles ThemeStyles) string {
	label := itoa(shown) + "/" + itoa(total) + " shown"
	return fitWidth(rightAlign(styles.Badge.Render(label), width), width)
}
