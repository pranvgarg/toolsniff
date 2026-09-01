package output

import (
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/pranvgarg/toolsniff/model"
)

// The overview is the landing pane: "what's on this machine", grouped by what
// each thing *is*, with every group's count next to a plain-English gloss. It
// replaces landing on a flat "all" list, which told the user how many rows a
// scanner produced rather than what it found. See DESIGN.md > Components >
// "Overview group row".

// overviewLabelWidth is the label column, in cells. Wide enough for
// "  Homebrew" indented under "Packages" without truncating either.
const overviewLabelWidth = 18

// overviewCountWidth is the count column. Four digits covers a machine with
// thousands of PATH entries.
const overviewCountWidth = 5

// overviewRow is one line of the dashboard.
type overviewRow struct {
	label string
	count int
	// countText replaces the number when set. It exists for the one case a
	// count cannot answer: the Health dashboard's Reclaimable card, where
	// nothing has measured the disk yet and rendering count's zero would claim
	// a scan found nothing. See healthUnknownValue in output/kinds.go.
	countText string
	meaning   string
	// indent marks a manager breakdown nested under its kind. These are the
	// first thing dropped when the pane is too short.
	indent bool
	// alert draws the row in the warning/error tone when its count is non-zero
	// (the Issues row), so a real problem is visible without reading the label.
	alert bool
}

// renderOverview renders the dashboard as at most height lines of exactly width
// cells. Groups are ordered most-useful first; when the pane cannot hold
// everything, the manager breakdown is dropped before any top-level group is.
func renderOverview(report ObservationReport, styles ThemeStyles, width, height int) []string {
	if height < 1 {
		height = 1
	}
	title := fitWidth(styles.DetailHeading.Render(styles.Glyph.Brand+" toolsniff — what's on this machine"), width)
	hint := fitWidth(styles.Footer.Render("→ or 1-8 open a view · ↑/↓ then enter for details"), width)

	inventory, changes := overviewRows(report)
	lines := overviewLines(title, hint, inventory, changes, width, styles)
	if len(lines) <= height {
		return lines
	}

	// Too short for the breakdown: keep every top-level group and drop the
	// per-manager rows, which the Packages tab itself shows anyway.
	collapsed := make([]overviewRow, 0, len(inventory))
	for _, row := range inventory {
		if !row.indent {
			collapsed = append(collapsed, row)
		}
	}
	lines = overviewLines(title, hint, collapsed, changes, width, styles)
	if len(lines) <= height {
		return lines
	}
	// Still too short: keep the top of the list and the hint, so the pane never
	// silently loses its last line to a clip.
	if height == 1 {
		return []string{hint}
	}
	trimmed := append([]string(nil), lines[:height-1]...)
	return append(trimmed, hint)
}

// overviewRows builds the two blocks of the dashboard: the kind groups, and the
// baseline block (changes and issues) below the rule.
func overviewRows(report ObservationReport) (inventory, changes []overviewRow) {
	inventory = []overviewRow{{
		label:   ViewLabel(ViewCLI),
		count:   CountForView(report, ViewCLI),
		meaning: "commands installed by a package manager",
	}, {
		label:   ViewLabel(ViewPackages),
		count:   CountForView(report, ViewPackages),
		meaning: "installed for you by a package manager",
	}}
	inventory = append(inventory, overviewPackageRows(report)...)
	inventory = append(inventory,
		overviewRow{
			label:   ViewLabel(ViewApplications),
			count:   CountForView(report, ViewApplications),
			meaning: "macOS .app bundles you open from Finder",
		},
		overviewRow{
			label:   ViewLabel(ViewPathExecutables),
			count:   CountForView(report, ViewPathExecutables),
			meaning: "not installed by a manager",
		},
		overviewRow{
			label:   ViewLabel(ViewNpxHistory),
			count:   CountForView(report, ViewNpxHistory),
			meaning: "run once via npx, not installed",
		},
	)

	changes = []overviewRow{{
		label:   ViewLabel(ViewChanges),
		count:   CountForView(report, ViewChanges),
		meaning: ViewMeaning(ViewChanges),
	}, {
		label:   ViewLabel(ViewIssues),
		count:   CountForView(report, ViewIssues),
		meaning: "broken or shadowed",
		alert:   true,
	}}
	return inventory, changes
}

// overviewPackageRows breaks the Packages count down by manager, in the same
// order the Packages pane groups them. Homebrew's formulae and casks are
// reported as one number here because "Homebrew" is how a user thinks of them;
// the pane itself keeps them apart.
func overviewPackageRows(report ObservationReport) []overviewRow {
	type bucket struct {
		label   string
		meaning string
		sources []string
	}
	buckets := []bucket{
		{"npm", "Node.js packages installed globally", []string{model.SourceNPM}},
		{"Homebrew", "formulae + casks", []string{model.SourceBrewFormula, model.SourceBrewCask}},
		{"Cargo", "Rust binaries", []string{model.SourceCargo}},
		{"pipx", "Python apps", []string{model.SourcePipx}},
		{"Bun", "Bun packages", []string{model.SourceBun}},
	}

	counts := map[string]int{}
	for _, observation := range report.AllObservations() {
		if !observationMatchesView(observation, ViewPackages) {
			continue
		}
		counts[sourceGroupKey(ObservationSource(observation))]++
	}

	rows := make([]overviewRow, 0, len(buckets))
	for _, b := range buckets {
		total := 0
		for _, source := range b.sources {
			total += counts[source]
			delete(counts, source)
		}
		if total == 0 {
			continue
		}
		rows = append(rows, overviewRow{label: b.label, count: total, meaning: b.meaning, indent: true})
	}
	// Anything from a manager the vocabulary has not been taught is still
	// counted, under its own name, rather than silently vanishing from the sum.
	other := 0
	for _, count := range counts {
		other += count
	}
	if other > 0 {
		rows = append(rows, overviewRow{label: "Other", count: other, meaning: "another package manager", indent: true})
	}
	return rows
}

// overviewLines composes the title, the two blocks, the rule between them, and
// the hint into finished pane lines.
func overviewLines(title, hint string, inventory, changes []overviewRow, width int, styles ThemeStyles) []string {
	lines := []string{title, fitWidth("", width)}
	lines = append(lines, renderOverviewBlock(inventory, width, styles)...)
	lines = append(lines, overviewRule(width, styles))
	lines = append(lines, renderOverviewBlock(changes, width, styles)...)
	lines = append(lines, fitWidth("", width), hint)
	return lines
}

// overviewRule is the divider between the inventory groups and the baseline
// block. It is drawn from the frame's own border glyph in the border tone.
func overviewRule(width int, styles ThemeStyles) string {
	span := overviewLabelWidth + overviewCountWidth + 4
	if span > width {
		span = width
	}
	if span < 1 {
		span = 1
	}
	return fitWidth(styles.HeaderBorder.Render(strings.Repeat(lipgloss.RoundedBorder().Top, span)), width)
}

// renderOverviewBlock renders one block of group rows through lipgloss/table.
// Both blocks are given identical column widths, so the two tables read as one
// list with a rule through it.
func renderOverviewBlock(rows []overviewRow, width int, styles ThemeStyles) []string {
	if len(rows) == 0 {
		return nil
	}
	meaningWidth := width - overviewLabelWidth - overviewCountWidth - 6
	if meaningWidth < 0 {
		meaningWidth = 0
	}

	matrix := make([][]string, 0, len(rows))
	for _, row := range rows {
		label := row.label
		if row.indent {
			label = "  " + label
		}
		meaning := ""
		if meaningWidth > 0 && row.meaning != "" {
			meaning = "(" + row.meaning + ")"
		}
		count := itoa(row.count)
		if row.countText != "" {
			count = row.countText
		}
		matrix = append(matrix, []string{
			fitWidth(label, overviewLabelWidth),
			rightAlign(count, overviewCountWidth),
			fitWidth(meaning, meaningWidth),
		})
	}

	rendered := table.New().
		Border(lipgloss.HiddenBorder()).
		BorderTop(false).BorderBottom(false).BorderLeft(false).
		BorderRight(false).BorderRow(false).BorderColumn(false).BorderHeader(false).
		Rows(matrix...).
		StyleFunc(overviewStyleFunc(rows, styles)).
		Render()

	lines := strings.Split(rendered, "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, fitWidth(line, width))
	}
	return out
}

// overviewStyleFunc keeps the accent for the selection alone: a group label is
// body text, an indented manager is muted, a gloss is meta, and only a non-zero
// Issues count borrows a semantic tone.
func overviewStyleFunc(rows []overviewRow, styles ThemeStyles) table.StyleFunc {
	return func(row, column int) lipgloss.Style {
		style := styles.Body
		if row >= 0 && row < len(rows) {
			data := rows[row]
			switch {
			case data.alert && data.count > 0:
				style = styles.rowStyle(string(StatusBroken))
			case data.indent:
				style = styles.Meta
			}
			if column == 2 && !(data.alert && data.count > 0) {
				style = styles.Meta
			}
		}
		return style.Padding(0, 1)
	}
}
