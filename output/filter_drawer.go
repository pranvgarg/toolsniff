package output

import (
	"fmt"
	"sort"
	"strings"
)

// FilterDrawer is a small view-model for the visual facet drawer. The TUI can
// bind controls to State without knowing parser or chip implementation details.
type FilterDrawer struct {
	State FilterState
	Open  bool
	Draft string
	Error string
}

func NewFilterDrawer(state FilterState) FilterDrawer {
	return FilterDrawer{State: state}
}

func (d *FilterDrawer) OpenDrawer() {
	d.Open = true
	d.Error = ""
	d.Draft = filterInput(d.State)
}

func (d *FilterDrawer) CloseDrawer() {
	d.Open = false
	d.Error = ""
}

// Apply parses the draft and replaces State only when all facets are valid.
func (d *FilterDrawer) Apply() error {
	state, err := ParseFilter(d.Draft)
	if err != nil {
		d.Error = err.Error()
		return err
	}
	if state.View == ViewAll && d.State.View != ViewAll {
		state.View = d.State.View
	}
	d.State = state
	d.Open = false
	d.Error = ""
	return nil
}

func (d *FilterDrawer) Clear() {
	d.State = ClearFilters(d.State)
	d.Draft = ""
	d.Error = ""
}

// View renders the drawer as plain text so it works in tests and in narrow
// terminals where a full form would not fit.
func (d FilterDrawer) View() string {
	lines := []string{"Filter Inventory", "", "Search       " + d.State.Text, "View         " + string(d.State.View)}
	lines = append(lines, "Source       "+facetValues(d.State.Sources))
	lines = append(lines, "Role         "+roleValues(d.State.Roles))
	lines = append(lines, "Kind         "+kindValues(d.State.Kinds))
	lines = append(lines, "Version      "+versionValues(d.State.VersionState))
	lines = append(lines, "Status       "+statusValues(d.State.Statuses))
	lines = append(lines, "", "Apply     Clear     Cancel")
	if d.Error != "" {
		lines = append(lines, "Error: "+d.Error)
	}
	return strings.Join(lines, "\n")
}

func filterInput(state FilterState) string {
	parts := []string{}
	if state.Text != "" {
		parts = append(parts, state.Text)
	}
	for _, chip := range FilterChips(state) {
		if chip.Key != "search" && chip.Key != "view" {
			parts = append(parts, chip.String())
		}
	}
	return strings.Join(parts, " ")
}

func facetValues(values map[string]bool) string {
	if len(values) == 0 {
		return "All"
	}
	parts := []string{}
	for value, enabled := range values {
		if enabled {
			parts = append(parts, value)
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

func roleValues(values map[SourceRole]bool) string {
	parts := []string{}
	for value, enabled := range values {
		if enabled {
			parts = append(parts, string(value))
		}
	}
	sort.Strings(parts)
	if len(parts) == 0 {
		return "Any"
	}
	return strings.Join(parts, ", ")
}

func kindValues(values map[ObservationKind]bool) string {
	parts := []string{}
	for value, enabled := range values {
		if enabled {
			parts = append(parts, string(value))
		}
	}
	sort.Strings(parts)
	if len(parts) == 0 {
		return "Any"
	}
	return strings.Join(parts, ", ")
}

func versionValues(values map[VersionState]bool) string {
	parts := []string{}
	for value, enabled := range values {
		if enabled {
			parts = append(parts, string(value))
		}
	}
	sort.Strings(parts)
	if len(parts) == 0 {
		return "Any"
	}
	return strings.Join(parts, ", ")
}

func statusValues(values map[Status]bool) string {
	parts := []string{}
	for value, enabled := range values {
		if enabled {
			parts = append(parts, string(value))
		}
	}
	sort.Strings(parts)
	if len(parts) == 0 {
		return "Any"
	}
	return strings.Join(parts, ", ")
}

// FilterSummary returns the compact chip/count line used above a table.
func FilterSummary(state FilterState, matches int) string {
	chips := FilterChips(state)
	parts := make([]string, 0, len(chips))
	for _, chip := range chips {
		parts = append(parts, "["+chip.String()+" x]")
	}
	return fmt.Sprintf("Filter: %s    %d %s", strings.Join(parts, " "), matches, plural(matches, "match", "matches"))
}

func plural(count int, singular, pluralValue string) string {
	if count == 1 {
		return singular
	}
	return pluralValue
}
