package output

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pranvgarg/toolsniff/model"
	"github.com/pranvgarg/toolsniff/registry"
	"github.com/pranvgarg/toolsniff/scanner"
)

// ReportSchemaVersion is the version of the additive observation report.
const ReportSchemaVersion = 2

// ObservationKind and VersionState aliases keep presentation APIs concise
// without creating a second set of domain values.
type ObservationKind = model.ObservationKind
type SourceRole = model.SourceRole
type VersionState = model.VersionState

// ChangeReport keeps each typed event category available to machine and human
// renderers. Events are intentionally not flattened into ordinary rows.
type ChangeReport struct {
	Added     []registry.ChangeEvent `json:"added"`
	Removed   []registry.ChangeEvent `json:"removed"`
	Updated   []registry.ChangeEvent `json:"updated"`
	Relocated []registry.ChangeEvent `json:"relocated"`
	Broken    []registry.ChangeEvent `json:"broken"`
	Repaired  []registry.ChangeEvent `json:"repaired"`
	Shadowed  []registry.ChangeEvent `json:"shadowed"`
}

// ObservationReport is the shared v2 contract consumed by table, JSON, and
// TUI presentation. Filtering operates on this value and never mutates it.
type ObservationReport struct {
	SchemaVersion int                 `json:"schema_version"`
	Installed     []model.Observation `json:"installed"`
	Available     []model.Observation `json:"available"`
	History       []model.Observation `json:"history"`
	Changes       ChangeReport        `json:"changes"`
	Warnings      []string            `json:"warnings"`
}

// Report is a concise compatibility name for the v2 report type.
type Report = ObservationReport

// NewObservationReport creates a v2 report and copies all input slices.
func NewObservationReport(installed, available, history []model.Observation, changes registry.ObservationDiff, warnings []string) ObservationReport {
	return ObservationReport{
		SchemaVersion: ReportSchemaVersion,
		Installed:     copyObservations(installed),
		Available:     copyObservations(available),
		History:       copyObservations(history),
		Changes:       changeReportFromDiff(changes),
		Warnings:      append([]string{}, warnings...),
	}
}

// NewObservationReportFromTools adapts legacy scanner and registry values to
// the v2 report without changing any legacy public renderer signature.
func NewObservationReportFromTools(installed, available, history []model.Tool, diff, availabilityDiff registry.Diff, warnings []scanner.Warning) ObservationReport {
	installedObservations := observationsFromTools(installed)
	availableObservations := observationsFromTools(available)
	historyObservations := observationsFromTools(history)
	changes := mergeChangeReports(legacyChangeReport(diff), legacyChangeReport(availabilityDiff))
	warningStrings := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		warningStrings = append(warningStrings, fmt.Sprintf("%s: %v", warning.Source, warning.Err))
	}
	return ObservationReport{
		SchemaVersion: ReportSchemaVersion,
		Installed:     installedObservations,
		Available:     availableObservations,
		History:       historyObservations,
		Changes:       changes,
		Warnings:      warningStrings,
	}
}

// BuildObservationReport is a short alias for callers that already use the
// report vocabulary.
func BuildObservationReport(installed, available, history []model.Tool, diff, availabilityDiff registry.Diff, warnings []scanner.Warning) ObservationReport {
	return NewObservationReportFromTools(installed, available, history, diff, availabilityDiff, warnings)
}

// NewReport is the concise constructor alias for new v2 integrations.
func NewReport(installed, available, history []model.Observation, changes registry.ObservationDiff, warnings []string) ObservationReport {
	return NewObservationReport(installed, available, history, changes, warnings)
}

func copyObservations(observations []model.Observation) []model.Observation {
	if observations == nil {
		return []model.Observation{}
	}
	return append([]model.Observation(nil), observations...)
}

func observationsFromTools(tools []model.Tool) []model.Observation {
	observations := make([]model.Observation, 0, len(tools))
	for _, tool := range tools {
		observations = append(observations, tool.ToObservation())
	}
	return observations
}

func changeReportFromDiff(diff registry.ObservationDiff) ChangeReport {
	return ChangeReport{
		Added:     copyChangeEvents(diff.Added),
		Removed:   copyChangeEvents(diff.Removed),
		Updated:   copyChangeEvents(diff.Updated),
		Relocated: copyChangeEvents(diff.Relocated),
		Broken:    copyChangeEvents(diff.Broken),
		Repaired:  copyChangeEvents(diff.Repaired),
		Shadowed:  copyChangeEvents(diff.Shadowed),
	}
}

func copyChangeEvents(events []registry.ChangeEvent) []registry.ChangeEvent {
	if events == nil {
		return []registry.ChangeEvent{}
	}
	return append([]registry.ChangeEvent{}, events...)
}

func mergeChangeReports(left, right ChangeReport) ChangeReport {
	return ChangeReport{
		Added:     append(append([]registry.ChangeEvent{}, left.Added...), right.Added...),
		Removed:   append(append([]registry.ChangeEvent{}, left.Removed...), right.Removed...),
		Updated:   append(append([]registry.ChangeEvent{}, left.Updated...), right.Updated...),
		Relocated: append(append([]registry.ChangeEvent{}, left.Relocated...), right.Relocated...),
		Broken:    append(append([]registry.ChangeEvent{}, left.Broken...), right.Broken...),
		Repaired:  append(append([]registry.ChangeEvent{}, left.Repaired...), right.Repaired...),
		Shadowed:  append(append([]registry.ChangeEvent{}, left.Shadowed...), right.Shadowed...),
	}
}

func legacyChangeReport(diff registry.Diff) ChangeReport {
	result := ChangeReport{
		Added:     []registry.ChangeEvent{},
		Removed:   []registry.ChangeEvent{},
		Updated:   []registry.ChangeEvent{},
		Relocated: []registry.ChangeEvent{},
		Broken:    []registry.ChangeEvent{},
		Repaired:  []registry.ChangeEvent{},
		Shadowed:  []registry.ChangeEvent{},
	}
	for _, tool := range diff.Added {
		observation := tool.ToObservation()
		result.Added = append(result.Added, registry.ChangeEvent{Kind: registry.ChangeAdded, Identity: observation.ID, After: &observation})
	}
	for _, tool := range diff.Removed {
		observation := tool.ToObservation()
		result.Removed = append(result.Removed, registry.ChangeEvent{Kind: registry.ChangeRemoved, Identity: observation.ID, Before: &observation})
	}
	for _, change := range diff.Updated {
		before := change.Before.ToObservation()
		after := change.After.ToObservation()
		result.Updated = append(result.Updated, registry.ChangeEvent{Kind: registry.ChangeUpdated, Identity: after.ID, Before: &before, After: &after})
	}
	return result
}

// AllObservations returns a new slice in report order.
func (r ObservationReport) AllObservations() []model.Observation {
	all := make([]model.Observation, 0, len(r.Installed)+len(r.Available)+len(r.History))
	all = append(all, r.Installed...)
	all = append(all, r.Available...)
	all = append(all, r.History...)
	return all
}

// InventoryRow is the compact, explicit representation used by list views.
// Version is never populated from a location.
type InventoryRow struct {
	ObservationID string
	Name          string
	Version       string
	VersionState  string
	Status        string
	Source        string
	Kind          string
}

// InventoryRows converts observations to responsive presentation data in one
// pass, which keeps large PATH inventories cheap while filtering.
func InventoryRows(observations []model.Observation) []InventoryRow {
	rows := make([]InventoryRow, 0, len(observations))
	for _, observation := range observations {
		rows = append(rows, InventoryRowFromObservation(observation))
	}
	return rows
}

// RowsForReport returns rows for the selected primary view.
func RowsForReport(report ObservationReport, view ViewCategory) []InventoryRow {
	var observations []model.Observation
	switch view {
	case ViewInstalled:
		observations = report.Installed
	case ViewAvailable:
		observations = report.Available
	case ViewHistory:
		observations = report.History
	case ViewChanges, ViewIssues:
		return rowsForEvents(report.Changes, view)
	default:
		observations = report.AllObservations()
	}
	return InventoryRows(observations)
}

// InventoryRowFromObservation makes version and status states explicit.
func InventoryRowFromObservation(observation model.Observation) InventoryRow {
	return InventoryRow{
		ObservationID: observation.ID,
		Name:          observation.DisplayName,
		Version:       DisplayVersion(observation.Version),
		VersionState:  string(observation.Version.State),
		Status:        ObservationStatus(observation),
		Source:        ObservationSource(observation),
		Kind:          string(observation.Kind),
	}
}

// DisplayVersion is the only version formatting used by v2 list views.
func DisplayVersion(version model.VersionInfo) string {
	switch version.State {
	case model.VersionKnown:
		return version.Value
	case model.VersionUnknown:
		return "unknown"
	case model.VersionNotApplicable:
		return "n/a"
	case model.VersionNotReported:
		return "not reported"
	default:
		return "unknown"
	}
}

// Status describes an observation or a typed change event in a list view.
type Status string

const (
	StatusInstalled Status = "installed"
	StatusAvailable Status = "available"
	StatusHistory   Status = "history"
	StatusUpdated   Status = "updated"
	StatusRelocated Status = "relocated"
	StatusBroken    Status = "broken"
	StatusRepaired  Status = "repaired"
	StatusShadowed  Status = "shadowed"
)

func ObservationStatus(observation model.Observation) string {
	if observation.Role == model.RoleHistory || observation.Kind == model.KindHistory {
		return string(StatusHistory)
	}
	if len(observation.Availability.ShadowedBy) > 0 {
		return string(StatusShadowed)
	}
	if observation.Availability.State == model.AvailabilityUnavailable {
		return string(StatusBroken)
	}
	if observation.Role == model.RoleAvailable {
		return string(StatusAvailable)
	}
	return string(StatusInstalled)
}

// ObservationSource produces stable source labels from evidence-backed origin
// fields. It also recognizes the legacy adapter's source-shaped providers.
func ObservationSource(observation model.Observation) string {
	if observation.Kind == model.KindHistory || observation.Role == model.RoleHistory {
		return model.SourceNPXHistory
	}
	provider, manager := observation.Origin.Provider, observation.Origin.Manager
	switch {
	case provider == "homebrew" && manager == "formula":
		return model.SourceBrewFormula
	case provider == "homebrew" && manager == "cask":
		return model.SourceBrewCask
	case provider == "npm" && manager == "npx":
		return model.SourceNPXHistory
	case provider == "npm":
		return model.SourceNPM
	case provider == "pipx":
		return model.SourcePipx
	case provider == "applications":
		return model.SourceApplications
	case provider == "unknown" && manager == "manual-or-unknown":
		return model.SourcePath
	case provider != "" && manager != "":
		return provider + "-" + manager
	default:
		return provider
	}
}

func rowsForEvents(changes ChangeReport, view ViewCategory) []InventoryRow {
	rows := make([]InventoryRow, 0)
	appendEvents := func(events []registry.ChangeEvent, status Status) {
		for _, event := range events {
			observation := event.After
			if observation == nil {
				observation = event.Before
			}
			if observation == nil {
				continue
			}
			row := InventoryRowFromObservation(*observation)
			row.ObservationID = event.Identity
			row.Status = string(status)
			rows = append(rows, row)
		}
	}
	if view == ViewIssues {
		appendEvents(changes.Broken, StatusBroken)
		appendEvents(changes.Shadowed, StatusShadowed)
		return rows
	}
	appendEvents(changes.Added, Status("added"))
	appendEvents(changes.Removed, Status("removed"))
	appendEvents(changes.Updated, StatusUpdated)
	appendEvents(changes.Relocated, StatusRelocated)
	appendEvents(changes.Broken, StatusBroken)
	appendEvents(changes.Repaired, StatusRepaired)
	appendEvents(changes.Shadowed, StatusShadowed)
	return rows
}

// SortRows provides deterministic presentation ordering without changing the
// report's source order.
func SortRows(rows []InventoryRow) []InventoryRow {
	result := append([]InventoryRow(nil), rows...)
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Name != result[j].Name {
			return result[i].Name < result[j].Name
		}
		return result[i].ObservationID < result[j].ObservationID
	})
	return result
}

func observationText(observation model.Observation) string {
	parts := []string{
		observation.DisplayName,
		observation.CommandName,
		ObservationSource(observation),
		string(observation.Role),
		string(observation.Kind),
		DisplayVersion(observation.Version),
		string(observation.Version.State),
		ObservationStatus(observation),
	}
	if observation.Package != nil {
		parts = append(parts, observation.Package.Name, observation.Package.Version)
	}
	for _, location := range observation.Locations {
		parts = append(parts, location.Path)
	}
	return strings.ToLower(strings.Join(parts, "\x00"))
}
