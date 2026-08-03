// Package diagnostics derives read-only health and provenance findings from
// model observations. It does not inspect or modify the machine.
package diagnostics

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/pranvgarg/toolsniff/model"
)

// IssueKind identifies a diagnostic finding without requiring callers to
// parse its message.
type IssueKind string

const (
	IssueShadowedCommand       IssueKind = "shadowed-command"
	IssueBrokenLocation        IssueKind = "broken-location"
	IssueArchitectureMismatch  IssueKind = "architecture-mismatch"
	IssueUnsignedApplication   IssueKind = "unsigned-application"
	IssueUnknownSigningStatus  IssueKind = "unknown-signing-status"
	IssueMissingExecutableLink IssueKind = "missing-executable-link"
	IssueUnknownVersion        IssueKind = "unknown-version"
)

// Issue is a typed, read-only finding associated with one observation. Fields
// not relevant to Kind are left empty. Slices in a returned Issue are owned by
// the result and may be modified by the caller.
type Issue struct {
	Kind                  IssueKind
	ObservationID         string
	Name                  string
	CommandName           string
	PackageName           string
	ExecutableName        string
	LocationPath          string
	ShadowedPaths         []string
	ExpectedArchitectures []string
	ObservedArchitectures []string
	VersionState          model.VersionState
	AvailabilityState     model.AvailabilityState
}

// ProvenanceEdgeKind identifies a relationship in the derived provenance
// graph.
type ProvenanceEdgeKind string

const ProvenancePackageToLocation ProvenanceEdgeKind = "package-to-location"

// ProvenanceEdge connects explicit package provenance to one observed
// location. No edge is inferred from matching display or command names.
type ProvenanceEdge struct {
	Kind          ProvenanceEdgeKind
	ObservationID string
	PackageName   string
	LocationPath  string
	LocationType  model.LocationType
}

// Report contains all diagnostics and provenance relationships derived from a
// set of observations. The slices are deterministically ordered.
type Report struct {
	Issues     []Issue
	Provenance []ProvenanceEdge
}

// Analyze derives health issues and package-to-location provenance without
// changing observations or reading from the filesystem.
func Analyze(observations []model.Observation) Report {
	ordered := append([]model.Observation(nil), observations...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return observationSortKey(ordered[i]) < observationSortKey(ordered[j])
	})

	issues := make([]Issue, 0)
	edges := make([]ProvenanceEdge, 0)
	for _, observation := range ordered {
		issues = append(issues, issuesFor(observation)...)
		edges = append(edges, provenanceFor(observation)...)
	}

	sort.SliceStable(issues, func(i, j int) bool {
		return issueSortKey(issues[i]) < issueSortKey(issues[j])
	})
	sort.SliceStable(edges, func(i, j int) bool {
		return edgeSortKey(edges[i]) < edgeSortKey(edges[j])
	})
	return Report{Issues: issues, Provenance: edges}
}

func issuesFor(observation model.Observation) []Issue {
	issues := make([]Issue, 0)
	base := Issue{
		ObservationID:     observation.ID,
		Name:              observation.DisplayName,
		CommandName:       observation.CommandName,
		AvailabilityState: observation.Availability.State,
	}

	if len(observation.Availability.ShadowedBy) > 0 {
		shadowed := sortedUniqueNonEmpty(observation.Availability.ShadowedBy)
		if len(shadowed) > 0 {
			issues = append(issues, Issue{
				Kind:              IssueShadowedCommand,
				ObservationID:     base.ObservationID,
				Name:              base.Name,
				CommandName:       base.CommandName,
				ShadowedPaths:     shadowed,
				AvailabilityState: base.AvailabilityState,
			})
		}
	}

	if observation.Availability.State == model.AvailabilityUnavailable {
		issues = append(issues, Issue{
			Kind:              IssueBrokenLocation,
			ObservationID:     base.ObservationID,
			Name:              base.Name,
			CommandName:       base.CommandName,
			LocationPath:      unavailablePath(observation),
			AvailabilityState: model.AvailabilityUnavailable,
		})
	}

	if observation.Version.State == model.VersionUnknown {
		issues = append(issues, Issue{
			Kind:          IssueUnknownVersion,
			ObservationID: base.ObservationID,
			Name:          base.Name,
			CommandName:   base.CommandName,
			VersionState:  model.VersionUnknown,
		})
	}

	issues = append(issues, architectureIssues(observation)...)
	issues = append(issues, signingIssues(observation)...)
	issues = append(issues, executableLinkIssues(observation)...)
	return issues
}

func architectureIssues(observation model.Observation) []Issue {
	if observation.Application == nil || len(observation.Application.Architectures) == 0 {
		return nil
	}
	expected := canonicalArchitectures(observation.Application.Architectures)
	if len(expected) == 0 {
		return nil
	}

	issues := make([]Issue, 0)
	locations := sortedLocations(observation.Locations)
	for _, location := range locations {
		if len(location.Architectures) == 0 {
			continue
		}
		observed := canonicalArchitectures(location.Architectures)
		if len(observed) == 0 || architectureIntersection(expected, observed) {
			continue
		}
		issues = append(issues, Issue{
			Kind:                  IssueArchitectureMismatch,
			ObservationID:         observation.ID,
			Name:                  observation.DisplayName,
			CommandName:           observation.CommandName,
			LocationPath:          location.Path,
			ExpectedArchitectures: sortedUniqueNonEmpty(observation.Application.Architectures),
			ObservedArchitectures: sortedUniqueNonEmpty(location.Architectures),
		})
	}
	return issues
}

func signingIssues(observation model.Observation) []Issue {
	if observation.Application == nil {
		return nil
	}
	kind := IssueUnknownSigningStatus
	if observation.Application.Signed != nil && !*observation.Application.Signed {
		kind = IssueUnsignedApplication
	}
	if observation.Application.Signed != nil && *observation.Application.Signed {
		return nil
	}
	return []Issue{{
		Kind:          kind,
		ObservationID: observation.ID,
		Name:          observation.DisplayName,
		CommandName:   observation.CommandName,
		LocationPath:  applicationPath(observation),
	}}
}

func executableLinkIssues(observation model.Observation) []Issue {
	if observation.Package == nil || len(observation.Package.Executables) == 0 {
		return nil
	}

	locations := make([]model.Location, 0, len(observation.Locations))
	for _, location := range observation.Locations {
		if location.Type == model.LocationExecutable && location.Path != "" {
			locations = append(locations, location)
		}
	}
	declared := sortedUniqueNonEmpty(observation.Package.Executables)
	issues := make([]Issue, 0)
	for _, executable := range declared {
		if hasExecutableLink(locations, executable) {
			continue
		}
		issues = append(issues, Issue{
			Kind:           IssueMissingExecutableLink,
			ObservationID:  observation.ID,
			Name:           observation.DisplayName,
			CommandName:    observation.CommandName,
			PackageName:    packageName(observation),
			ExecutableName: executable,
		})
	}
	return issues
}

func provenanceFor(observation model.Observation) []ProvenanceEdge {
	name := packageName(observation)
	if name == "" {
		return nil
	}

	edges := make([]ProvenanceEdge, 0, len(observation.Locations))
	for _, location := range observation.Locations {
		if location.Path == "" {
			continue
		}
		edges = append(edges, ProvenanceEdge{
			Kind:          ProvenancePackageToLocation,
			ObservationID: observation.ID,
			PackageName:   name,
			LocationPath:  location.Path,
			LocationType:  location.Type,
		})
	}
	return edges
}

func packageName(observation model.Observation) string {
	if observation.Package != nil && observation.Package.Name != "" {
		return observation.Package.Name
	}
	return observation.Origin.Package
}

func hasExecutableLink(locations []model.Location, executable string) bool {
	for _, location := range locations {
		if location.Path == executable || filepath.Base(location.Path) == filepath.Base(executable) {
			return true
		}
	}
	return false
}

func unavailablePath(observation model.Observation) string {
	if observation.Availability.ResolvedPath != "" {
		return observation.Availability.ResolvedPath
	}
	locations := sortedLocations(observation.Locations)
	for _, location := range locations {
		if location.Path != "" && location.Type == model.LocationExecutable {
			return location.Path
		}
	}
	if len(locations) > 0 {
		return locations[0].Path
	}
	return ""
}

func applicationPath(observation model.Observation) string {
	for _, location := range sortedLocations(observation.Locations) {
		if location.Type == model.LocationApplication {
			return location.Path
		}
	}
	return ""
}

func sortedLocations(locations []model.Location) []model.Location {
	result := append([]model.Location(nil), locations...)
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Path != result[j].Path {
			return result[i].Path < result[j].Path
		}
		return result[i].Type < result[j].Type
	})
	return result
}

func canonicalArchitectures(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		switch value {
		case "aarch64", "arm64e":
			value = "arm64"
		case "x86-64", "x86_64":
			value = "amd64"
		}
		if value != "" {
			result[value] = struct{}{}
		}
	}
	return result
}

func architectureIntersection(left, right map[string]struct{}) bool {
	for architecture := range left {
		if _, ok := right[architecture]; ok {
			return true
		}
	}
	return false
}

func sortedUniqueNonEmpty(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func observationSortKey(observation model.Observation) string {
	return strings.Join([]string{
		observation.ID,
		observation.DisplayName,
		observation.CommandName,
		string(observation.Kind),
		packageName(observation),
		observation.Origin.Provider,
		observation.Origin.Manager,
	}, "\x00")
}

func issueSortKey(issue Issue) string {
	return strings.Join([]string{
		string(issue.Kind),
		issue.ObservationID,
		issue.Name,
		issue.CommandName,
		issue.PackageName,
		issue.ExecutableName,
		issue.LocationPath,
		strings.Join(issue.ShadowedPaths, "\x00"),
		strings.Join(issue.ExpectedArchitectures, "\x00"),
		strings.Join(issue.ObservedArchitectures, "\x00"),
		string(issue.VersionState),
		string(issue.AvailabilityState),
	}, "\x00")
}

func edgeSortKey(edge ProvenanceEdge) string {
	return strings.Join([]string{
		string(edge.Kind),
		edge.ObservationID,
		edge.PackageName,
		edge.LocationPath,
		string(edge.LocationType),
	}, "\x00")
}
