package output

import (
	"fmt"
	"strings"

	"github.com/pranvgarg/toolsniff/model"
)

// ParseFilter parses optional facet:value tokens and treats everything else as
// normal global search text. It deliberately does not enable regex syntax.
func ParseFilter(input string) (FilterState, error) {
	state := NewFilterState()
	var text []string
	for _, token := range strings.Fields(input) {
		key, value, isFacet := strings.Cut(token, ":")
		if !isFacet {
			text = append(text, token)
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			return FilterState{}, fmt.Errorf("filter %q has an empty value", key)
		}
		values := strings.Split(value, ",")
		switch key {
		case "source":
			for _, item := range values {
				if item == "" {
					return FilterState{}, fmt.Errorf("source filter has an empty value")
				}
				state.Sources[item] = true
			}
		case "role":
			for _, item := range values {
				role := model.SourceRole(item)
				if !role.IsValid() {
					return FilterState{}, fmt.Errorf("invalid role %q", item)
				}
				state.Roles[role] = true
			}
		case "kind":
			for _, item := range values {
				kind := model.ObservationKind(item)
				if !kind.IsValid() {
					return FilterState{}, fmt.Errorf("invalid kind %q", item)
				}
				state.Kinds[kind] = true
			}
		case "version", "version_state":
			for _, item := range values {
				versionState := normalizeVersionState(item)
				if !versionState.IsValid() {
					return FilterState{}, fmt.Errorf("invalid version state %q", item)
				}
				state.VersionState[versionState] = true
			}
		case "status":
			for _, item := range values {
				status := Status(item)
				if !validStatus(status) {
					return FilterState{}, fmt.Errorf("invalid status %q", item)
				}
				state.Statuses[status] = true
			}
		case "view":
			if len(values) != 1 {
				return FilterState{}, fmt.Errorf("view accepts one value")
			}
			view := ViewCategory(values[0])
			if !validView(view) {
				return FilterState{}, fmt.Errorf("invalid view %q", values[0])
			}
			state.View = view
		default:
			return FilterState{}, fmt.Errorf("unknown filter facet %q", key)
		}
	}
	state.Text = strings.Join(text, " ")
	return state, nil
}

// ParseFilterState is an explicit alias for API users that prefer the type
// name in the function name.
func ParseFilterState(input string) (FilterState, error) {
	return ParseFilter(input)
}

func normalizeVersionState(value string) model.VersionState {
	switch value {
	case "n/a", "na", "not_applicable", "notapplicable":
		return model.VersionNotApplicable
	case "not_reported", "notreported":
		return model.VersionNotReported
	default:
		return model.VersionState(value)
	}
}

func validStatus(status Status) bool {
	switch status {
	case StatusInstalled, StatusAvailable, StatusHistory, StatusUpdated, StatusRelocated, StatusBroken, StatusRepaired, StatusShadowed, Status("added"), Status("removed"):
		return true
	default:
		return false
	}
}

// validView accepts both the kind-first tab views and the status lenses. The
// lenses are no longer tabs, so `view:installed` in the filter drawer is now
// the way back to them -- keeping them parseable is what makes the tab
// reorganisation lossless.
func validView(view ViewCategory) bool {
	switch view {
	case ViewOverview, ViewCLI, ViewPackages, ViewApplications, ViewPathExecutables, ViewNpxHistory:
		return true
	case ViewManage, ViewDiscover, ViewReview, ViewHealth:
		return true
	case ViewAll, ViewInstalled, ViewAvailable, ViewHistory, ViewChanges, ViewIssues:
		return true
	default:
		return false
	}
}
