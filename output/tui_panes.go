package output

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/pranvgarg/toolsniff/registry"
)

// Styled counterparts to the plain-text renderers in tui_detail.go,
// filter_drawer.go, and tui_changes.go. The plain-text forms stay exactly as
// they are: they are the machine/adapter/test surface, and they must not grow
// escape sequences. These are the in-frame views.

const detailLabelWidth = 18

// renderDetailLines renders a DetailViewModel with section headings in the
// accent, labels muted, and values in body text.
func renderDetailLines(detail DetailViewModel, width int, styles ThemeStyles) []string {
	lines := []string{
		fitWidth(styles.DetailHeading.Render(detail.Title), width),
	}
	// The type sentence sits in the heading block, in the accent, carrying the
	// same kind marker the list row shows -- so "the ◇ pkg row" and "npm
	// global package" are visibly the same claim.
	if detail.TypeLabel != "" {
		marker := styles.kindGlyph(detail.Kind)
		lines = append(lines, fitWidth(lipgloss.JoinHorizontal(
			lipgloss.Top,
			styles.Badge.Render(marker+" "+detail.TypeLabel),
		), width))
	}
	// "What this is" sits directly under the type badge, in body text on its own
	// labelled line: the badge names the type, this sentence explains it to
	// someone who has never heard of a cask or a global npm install.
	if detail.WhatThisIs != "" {
		lines = append(lines, whatThisIsLines(detail.WhatThisIs, width, styles)...)
	}
	lines = append(lines, fitWidth(styles.Meta.Render(detail.ObservationID), width))

	// Actions come before the metadata sections. The detail pane does not
	// scroll, and a tool with locations, availability, package, and evidence
	// blocks is taller than the pane -- anything below them is unreachable.
	// What you can *do* outranks what we happen to know.
	lines = append(lines, renderActionLines(detail.Actions, width, styles)...)

	for _, section := range detail.Sections {
		lines = append(lines, "", fitWidth(styles.ColumnHeader.Render(strings.ToUpper(section.Title)), width))
		for _, field := range section.Fields {
			label := fitWidth(field.Label+":", detailLabelWidth)
			value := truncateTail(field.Value, width-detailLabelWidth-3)
			lines = append(lines, fitWidth(lipgloss.JoinHorizontal(
				lipgloss.Top,
				"  ",
				styles.DetailLabel.Render(label),
				" ",
				styles.DetailValue.Render(value),
			), width))
		}
	}
	lines = append(lines, "", fitWidth(styles.Footer.Render("esc back · y copy command · p copy path · c copy JSON · o reveal"), width))
	return lines
}

// whatThisIsLines renders the plain-English sentence as a labelled, wrapped
// block. It is the one place in the TUI that wraps rather than truncates: a
// half-sentence explains nothing, so the text gets a second line instead of an
// ellipsis. Continuation lines are indented to the value column.
func whatThisIsLines(sentence string, width int, styles ThemeStyles) []string {
	valueWidth := width - detailLabelWidth - 2
	if valueWidth < 8 {
		valueWidth = 8
	}
	wrapped := strings.Split(lipgloss.NewStyle().Width(valueWidth).Render(sentence), "\n")
	lines := make([]string, 0, len(wrapped))
	for index, text := range wrapped {
		label := fitWidth("", detailLabelWidth)
		if index == 0 {
			label = styles.DetailLabel.Render(fitWidth("What this is", detailLabelWidth))
		}
		lines = append(lines, fitWidth(lipgloss.JoinHorizontal(
			lipgloss.Top,
			label,
			" ",
			styles.DetailValue.Render(strings.TrimRight(text, " ")),
		), width))
	}
	return lines
}

// renderActionLines renders the Actions section: the label in the accent, the
// command muted behind a copy glyph. An action with no command is an
// informational row, so its label drops to muted and its note takes the place
// of the command -- nothing pretends to be runnable that isn't.
func renderActionLines(actions []Action, width int, styles ThemeStyles) []string {
	if len(actions) == 0 {
		return nil
	}
	lines := []string{"", fitWidth(styles.ColumnHeader.Render("ACTIONS"), width)}
	for _, action := range actions {
		label := fitWidth(action.Label, detailLabelWidth)
		labelStyle, prefix := styles.Badge, styles.Glyph.Copy+" "
		if !action.Runnable() {
			labelStyle, prefix = styles.DetailLabel, "  "
		}
		lines = append(lines, fitWidth(lipgloss.JoinHorizontal(
			lipgloss.Top,
			"  ",
			labelStyle.Render(label),
			" ",
			styles.Meta.Render(prefix+ActionCommandLine(action)),
		), width))
	}
	return lines
}

// renderDrawerLines renders the facet drawer with muted labels and body
// values, so the drawer reads as a form rather than as a wall of text.
func renderDrawerLines(drawer FilterDrawer, styles ThemeStyles) []string {
	field := func(label, value string) string {
		return lipgloss.JoinHorizontal(
			lipgloss.Top,
			styles.DetailLabel.Render(fitWidth(label, 12)),
			styles.DetailValue.Render(value),
		)
	}
	lines := []string{
		styles.DetailHeading.Render("Filter inventory"),
		"",
		field("Search", drawer.Draft),
		field("View", string(drawer.State.View)),
		field("Source", facetValues(drawer.State.Sources)),
		field("Role", roleValues(drawer.State.Roles)),
		field("Kind", kindValues(drawer.State.Kinds)),
		field("Version", versionValues(drawer.State.VersionState)),
		field("Status", statusValues(drawer.State.Statuses)),
		"",
		styles.Footer.Render("enter apply · c clear · esc cancel"),
	}
	if drawer.Error != "" {
		lines = append(lines, styles.Warning.Render(styles.Glyph.Warning+" "+drawer.Error))
	}
	return lines
}

// renderChangeLines renders the changes/issues views with each event category
// carrying its own semantic color, so a BROKEN block is distinguishable from
// an ADDED block without reading the heading.
func renderChangeLines(changes ChangeReport, styles ThemeStyles) []string {
	var lines []string
	section := func(title string, status Status, events []registry.ChangeEvent) {
		if len(events) == 0 {
			return
		}
		style := styles.rowStyle(string(status))
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, styles.ColumnHeader.Render(title)+" "+styles.Meta.Render("("+itoa(len(events))+")"))
		for _, event := range events {
			body := strings.Split(strings.TrimRight(renderChangeEvent(event), "\n"), "\n")
			for i, line := range body {
				if i == 0 {
					lines = append(lines, style.Render(styles.rowGlyph(string(status))+" ")+styles.Body.Render(strings.TrimSpace(line)))
					continue
				}
				lines = append(lines, styles.Meta.Render("    "+strings.TrimSpace(line)))
			}
		}
	}
	section("ADDED", Status("added"), changes.Added)
	section("REMOVED", Status("removed"), changes.Removed)
	section("UPDATED", StatusUpdated, changes.Updated)
	section("RELOCATED", StatusRelocated, changes.Relocated)
	section("BROKEN", StatusBroken, changes.Broken)
	section("REPAIRED", StatusRepaired, changes.Repaired)
	section("SHADOWED", StatusShadowed, changes.Shadowed)
	if len(lines) == 0 {
		lines = append(lines, styles.EmptyState.Render("no changes since the last saved baseline"))
	}
	return lines
}
