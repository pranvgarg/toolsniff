package output

import (
	"sort"
	"strings"

	"github.com/pranvgarg/toolsniff/model"
)

// ViewCategory is the primary intent-oriented inventory navigation.
type ViewCategory string

const (
	// Kind-first views. These are the primary navigation: what a thing *is*
	// comes before what state it is in. See output/kinds.go for the plain
	// meaning of each one.
	ViewOverview        ViewCategory = "overview"
	ViewCLI             ViewCategory = "cli-tools"
	ViewPackages        ViewCategory = "packages"
	ViewApplications    ViewCategory = "applications"
	ViewPathExecutables ViewCategory = "path-executables"
	ViewNpxHistory      ViewCategory = "npx-history"

	// Intent-first views (ui.mode "v3"). These replace the kind-first tabs
	// when enabled. See output/kinds.go for their plain meaning.
	ViewManage   ViewCategory = "manage"
	ViewDiscover ViewCategory = "discover"
	ViewReview   ViewCategory = "review"
	ViewHealth   ViewCategory = "health"

	// Status lenses. These are no longer tabs, but every one of them remains
	// reachable through the filter drawer and the `view:` filter facet, so the
	// kind-first reorganisation removes no way of looking at the data.
	ViewAll       ViewCategory = "all"
	ViewInstalled ViewCategory = "installed"
	ViewAvailable ViewCategory = "available"
	ViewHistory   ViewCategory = "history"

	ViewChanges ViewCategory = "changes"
	ViewIssues  ViewCategory = "issues"
)

// FilterState contains visual facets and the optional plain-text query. A map
// is OR within one facet and all populated facets are ANDed together.
type FilterState struct {
	Text         string
	Sources      map[string]bool
	Roles        map[model.SourceRole]bool
	Kinds        map[model.ObservationKind]bool
	VersionState map[model.VersionState]bool
	Statuses     map[Status]bool
	View         ViewCategory
}

// NewFilterState returns a normalized, empty state.
func NewFilterState() FilterState {
	return FilterState{
		Sources:      make(map[string]bool),
		Roles:        make(map[model.SourceRole]bool),
		Kinds:        make(map[model.ObservationKind]bool),
		VersionState: make(map[model.VersionState]bool),
		Statuses:     make(map[Status]bool),
		View:         ViewAll,
	}
}

func (f FilterState) normalized() FilterState {
	result := f
	if result.Sources == nil {
		result.Sources = make(map[string]bool)
	}
	if result.Roles == nil {
		result.Roles = make(map[model.SourceRole]bool)
	}
	if result.Kinds == nil {
		result.Kinds = make(map[model.ObservationKind]bool)
	}
	if result.VersionState == nil {
		result.VersionState = make(map[model.VersionState]bool)
	}
	if result.Statuses == nil {
		result.Statuses = make(map[Status]bool)
	}
	if result.View == "" {
		result.View = ViewAll
	}
	return result
}

// ClearFilters returns a new state with all query and facet visibility
// constraints removed.
func ClearFilters(state FilterState) FilterState {
	cleared := NewFilterState()
	cleared.View = state.View
	return cleared
}

// MatchObservation applies all populated facets and global text search.
func MatchObservation(state FilterState, observation model.Observation) bool {
	state = state.normalized()
	if len(state.Sources) > 0 && !state.Sources[ObservationSource(observation)] {
		return false
	}
	if len(state.Roles) > 0 && !state.Roles[observation.Role] {
		return false
	}
	if len(state.Kinds) > 0 && !state.Kinds[observation.Kind] {
		return false
	}
	if len(state.VersionState) > 0 && !state.VersionState[observation.Version.State] {
		return false
	}
	if len(state.Statuses) > 0 && !state.Statuses[Status(ObservationStatus(observation))] {
		return false
	}
	if state.Text != "" && !strings.Contains(observationText(observation), strings.ToLower(strings.TrimSpace(state.Text))) {
		return false
	}
	return true
}

// FilterObservations preserves input order and does not mutate the report.
func FilterObservations(observations []model.Observation, state FilterState) []model.Observation {
	filtered := make([]model.Observation, 0, len(observations))
	for _, observation := range observations {
		if MatchObservation(state, observation) {
			filtered = append(filtered, observation)
		}
	}
	return filtered
}

// FilterRows applies the same AND semantics to presentation rows. It is useful
// when a view has already discarded domain metadata.
func FilterRows(rows []InventoryRow, state FilterState) []InventoryRow {
	state = state.normalized()
	filtered := make([]InventoryRow, 0, len(rows))
	for _, row := range rows {
		if len(state.Sources) > 0 && !state.Sources[row.Source] || len(state.Kinds) > 0 && !state.Kinds[model.ObservationKind(row.Kind)] || len(state.VersionState) > 0 && !state.VersionState[model.VersionState(row.VersionState)] || len(state.Statuses) > 0 && !state.Statuses[Status(row.Status)] {
			continue
		}
		text := strings.ToLower(strings.Join([]string{row.Name, row.Version, row.VersionState, row.Status, row.Source, row.Kind}, "\x00"))
		if state.Text != "" && !strings.Contains(text, strings.ToLower(strings.TrimSpace(state.Text))) {
			continue
		}
		filtered = append(filtered, row)
	}
	return filtered
}

// FilterReport returns filtered rows for any primary view. Changes and issues
// are built from typed events before facets are applied.
func FilterReport(report ObservationReport, state FilterState) []InventoryRow {
	state = state.normalized()
	if !eventDrivenView(state.View) {
		rows := FilterRows(InventoryRows(FilterObservations(observationsForView(report, state.View), state)), state)
		return sortRowsForView(rows, state.View)
	}
	return FilterRows(RowsForReport(report, state.View), state)
}

// eventDrivenView reports whether a view's rows are built from typed change
// events rather than from observations. Such a view has no observation set of
// its own: an event is a pair of before/after states keyed on an identity, and
// two events can name the same identity, so narrowing the report's buckets
// could never produce the same list.
func eventDrivenView(view ViewCategory) bool {
	return view == ViewChanges || view == ViewIssues || view == ViewReview
}

// observationsForView narrows the report to one view's observations. The status
// lenses read the report's own buckets; the kind-first views and the intent
// views built from them run the shared predicate over everything, so a tab is
// never limited to one bucket. Views absent from the switch fall through to the
// whole report, which is why an intent view must be listed here: Manage showing
// everything, PATH executables included, would be a lie about what a manager
// can act on.
func observationsForView(report ObservationReport, view ViewCategory) []model.Observation {
	switch view {
	case ViewInstalled:
		return report.Installed
	case ViewAvailable:
		return report.Available
	case ViewHistory:
		return report.History
	case ViewChanges, ViewIssues, ViewReview:
		// Event-driven views hold no observations; their rows come from
		// RowsForReport. Listed explicitly rather than left to the default,
		// which would hand back the whole report -- Review claiming every
		// entry on the machine would be a lie about what needs attention.
		return []model.Observation{}
	case ViewCLI, ViewPackages, ViewApplications, ViewPathExecutables, ViewNpxHistory, ViewManage, ViewDiscover:
		all := report.AllObservations()
		matching := make([]model.Observation, 0, len(all))
		for _, observation := range all {
			if observationMatchesView(observation, view) {
				matching = append(matching, observation)
			}
		}
		return matching
	default:
		return report.AllObservations()
	}
}

// CountForView is the number of observations a view holds. Tab counts, the
// overview, and the pane caption all read it, so no two of them can disagree.
func CountForView(report ObservationReport, view ViewCategory) int {
	switch view {
	case ViewChanges:
		return len(report.Changes.Events())
	case ViewIssues:
		return len(report.Changes.Broken) + len(report.Changes.Shadowed)
	case ViewReview:
		// Review is the sum of its three sections, which is every event exactly
		// once: reviewSections partitions the seven categories rather than
		// overlapping them, so Changes + Issues + New cannot double-count the
		// broken and shadowed events that Events() already carries.
		return len(report.Changes.Events())
	case ViewOverview:
		return len(report.AllObservations())
	default:
		return len(observationsForView(report, view))
	}
}

// sortRowsForView puts a view's rows in the order its pane draws them. Any
// pane that renders sub-headings is only coherent if each block's rows are
// contiguous, and sorting here rather than in the renderer keeps the flat
// selection index and the rendered blocks describing the same order. A view
// with no sub-headings keeps the report's own order.
func sortRowsForView(rows []InventoryRow, view ViewCategory) []InventoryRow {
	switch {
	case groupedView(view):
		return sortRowsByGroup(rows)
	case view == ViewDiscover:
		// Discover's blocks are directories, not managers; see
		// output/tui_discover.go for why it cannot group the way the rest do.
		return sortRowsByDirectory(rows)
	default:
		return rows
	}
}

// sortRowsByGroup orders rows by manager group, then by name inside a group.
func sortRowsByGroup(rows []InventoryRow) []InventoryRow {
	sorted := append([]InventoryRow(nil), rows...)
	sort.SliceStable(sorted, func(i, j int) bool {
		left, right := sourceGroupRank(sorted[i].Source), sourceGroupRank(sorted[j].Source)
		if left != right {
			return left < right
		}
		if sorted[i].Source != sorted[j].Source {
			return sorted[i].Source < sorted[j].Source
		}
		return strings.ToLower(sorted[i].Name) < strings.ToLower(sorted[j].Name)
	})
	return sorted
}

// FilterChip is a removable visual representation of one active constraint.
type FilterChip struct {
	Key   string
	Value string
}

func (c FilterChip) String() string {
	return c.Key + ":" + c.Value
}

// FilterChips returns deterministic chips suitable for a status bar or drawer.
func FilterChips(state FilterState) []FilterChip {
	state = state.normalized()
	chips := make([]FilterChip, 0)
	if strings.TrimSpace(state.Text) != "" {
		chips = append(chips, FilterChip{Key: "search", Value: strings.TrimSpace(state.Text)})
	}
	if state.View != ViewAll {
		chips = append(chips, FilterChip{Key: "view", Value: string(state.View)})
	}
	appendStringMap := func(key string, values map[string]bool) {
		keys := make([]string, 0, len(values))
		for value, enabled := range values {
			if enabled {
				keys = append(keys, value)
			}
		}
		sort.Strings(keys)
		for _, value := range keys {
			chips = append(chips, FilterChip{Key: key, Value: value})
		}
	}
	appendRoleMap := func(key string, values map[model.SourceRole]bool) {
		keys := make([]string, 0, len(values))
		for value, enabled := range values {
			if enabled {
				keys = append(keys, string(value))
			}
		}
		sort.Strings(keys)
		for _, value := range keys {
			chips = append(chips, FilterChip{Key: key, Value: value})
		}
	}
	appendKindMap := func(key string, values map[model.ObservationKind]bool) {
		keys := make([]string, 0, len(values))
		for value, enabled := range values {
			if enabled {
				keys = append(keys, string(value))
			}
		}
		sort.Strings(keys)
		for _, value := range keys {
			chips = append(chips, FilterChip{Key: key, Value: value})
		}
	}
	appendVersionMap := func(key string, values map[model.VersionState]bool) {
		keys := make([]string, 0, len(values))
		for value, enabled := range values {
			if enabled {
				keys = append(keys, string(value))
			}
		}
		sort.Strings(keys)
		for _, value := range keys {
			chips = append(chips, FilterChip{Key: key, Value: value})
		}
	}
	appendStatusMap := func(key string, values map[Status]bool) {
		keys := make([]string, 0, len(values))
		for value, enabled := range values {
			if enabled {
				keys = append(keys, string(value))
			}
		}
		sort.Strings(keys)
		for _, value := range keys {
			chips = append(chips, FilterChip{Key: key, Value: value})
		}
	}
	appendStringMap("source", state.Sources)
	appendRoleMap("role", state.Roles)
	appendKindMap("kind", state.Kinds)
	appendVersionMap("version", state.VersionState)
	appendStatusMap("status", state.Statuses)
	return chips
}

// RemoveFilterChip removes one selected value while preserving all others.
func RemoveFilterChip(state FilterState, chip FilterChip) FilterState {
	state = state.normalized()
	switch chip.Key {
	case "search":
		state.Text = ""
	case "view":
		state.View = ViewAll
	case "source":
		delete(state.Sources, chip.Value)
	case "role":
		delete(state.Roles, model.SourceRole(chip.Value))
	case "kind":
		delete(state.Kinds, model.ObservationKind(chip.Value))
	case "version":
		delete(state.VersionState, model.VersionState(chip.Value))
	case "status":
		delete(state.Statuses, Status(chip.Value))
	}
	return state
}

// EmptyResultMessage explains why a view is empty and gives the user an
// actionable reset hint instead of showing a blank table.
func EmptyResultMessage(state FilterState, matches int) string {
	if matches > 0 {
		return ""
	}
	chips := FilterChips(state)
	if len(chips) == 0 {
		return "No observations found."
	}
	lines := []string{"No matches", "", "Current filters:"}
	for _, chip := range chips {
		lines = append(lines, "  "+chip.String())
	}
	lines = append(lines, "", "Try: clear filters or search all sources")
	return strings.Join(lines, "\n")
}

// MatchCount is a small convenience for filter bars and chips.
func MatchCount(rows []InventoryRow, state FilterState) int {
	return len(FilterRows(rows, state))
}
