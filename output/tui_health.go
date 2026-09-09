package output

// The Health view (ui.mode "intent") answers "is this machine healthy and up to
// date" before the user has to open anything. It is a dashboard, not a list:
// four cards, each a title, a number, and one plain sentence saying what the
// number is. Every card is a doorway -- Updatable and Manage are views with
// tabs behind them, Issues is the view the Review pane's middle block holds --
// so a card is read as "here is how much of this there is, and where to go".
//
// It is rendered through renderOverviewBlock, the same table the Overview
// dashboard draws its group rows with, so the two dashboards line their columns
// up identically and neither can drift into its own idea of a row.
//
// The fourth card is the honest one. "Reclaimable" has no scanner behind it
// yet, so it shows a dash rather than a number: see healthReclaimableLabel and
// healthUnknownValue in output/kinds.go for why a zero would have been a lie.

// renderHealth renders the whole Health view for a report as exactly height
// lines of exactly width cells. Its counts come from CountForView, the same
// call the tab strip and every other pane's caption read, so a card can never
// disagree with the tab it points at.
//
// It takes the same arguments renderManage and renderDiscover do. selected is
// unused: the pane has no rows to move a cursor through, and taking it anyway
// keeps every intent view reachable through one signature.
func renderHealth(report ObservationReport, selected int, styles ThemeStyles, width, height int) []string {
	_ = selected

	title := fitWidth(styles.DetailHeading.Render(
		styles.Glyph.Brand+" "+ViewLabel(ViewHealth)+" — "+ViewMeaning(ViewHealth)), width)
	footer := fitWidth(styles.Footer.Render(healthFooterHint()), width)

	lines := []string{title, fitWidth("", width)}
	lines = append(lines, renderOverviewBlock(healthCards(report), width, styles)...)
	lines = append(lines, fitWidth("", width), footer)

	if len(lines) > height {
		// The footer and the blank above it go first, the way the Discover pane
		// sheds its tip: a card the user cannot read is worth less than the
		// count it would have displaced.
		lines = lines[:len(lines)-2]
	}
	return padPane(lines, width, height)
}

// healthCards builds the dashboard's four rows, in the order the questions get
// asked: what can I bring up to date, what is broken, how much is here, and how
// much could I get back.
func healthCards(report ObservationReport) []overviewRow {
	card := func(view ViewCategory) overviewRow {
		return overviewRow{label: ViewLabel(view), count: CountForView(report, view), meaning: ViewMeaning(view)}
	}
	issues := card(ViewIssues)
	// Issues is the one card that earns a semantic tone, and only when it is
	// non-zero -- the same rule the Overview's Issues row follows, so a real
	// problem looks the same on both dashboards.
	issues.alert = true

	return []overviewRow{
		card(ViewUpdates),
		issues,
		// Manage rather than ViewInstalled: the number is the union of the kind
		// views, which is what the Manage tab lists, and a card must be counted
		// from the same set it is titled after.
		card(ViewManage),
		{
			label:     healthReclaimableLabel,
			countText: healthUnknownValue,
			meaning:   healthReclaimableMeaning,
		},
	}
}

// healthFooterHint names the one key that acts on what the cards describe. The
// key is read from healthCopyKey rather than written out here, so the footer
// cannot outlive the binding, and the destination is a ViewLabel for the same
// reason every other pane's wording is.
func healthFooterHint() string {
	return healthCopyKey + " copies the selected entry's command · " +
		ViewLabel(ViewManage) + " lists everything a manager installed"
}
