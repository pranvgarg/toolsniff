package output

import (
	"encoding/json"
	"fmt"

	"github.com/pranvgarg/toolsniff/model"
	"github.com/pranvgarg/toolsniff/registry"
	"github.com/pranvgarg/toolsniff/scanner"
)

// JSONReport is the full --json output shape.
type JSONReport struct {
	Tools            []model.Tool          `json:"tools"`
	Available        []model.Tool          `json:"available"`
	NPXHistory       []model.Tool          `json:"npx_history"`
	Added            []model.Tool          `json:"added"`
	Removed          []model.Tool          `json:"removed"`
	Updated          []registry.ToolChange `json:"updated"`
	AvailableAdded   []model.Tool          `json:"available_added"`
	AvailableRemoved []model.Tool          `json:"available_removed"`
	AvailableUpdated []registry.ToolChange `json:"available_updated"`
	Warnings         []string              `json:"warnings"`
}

// RenderJSON produces the full scan report as indented JSON.
func RenderJSON(tools, available, npxHistory []model.Tool, diff, availabilityDiff registry.Diff, warnings []scanner.Warning) ([]byte, error) {
	report := JSONReport{
		Tools:            tools,
		Available:        available,
		NPXHistory:       npxHistory,
		Added:            diff.Added,
		Removed:          diff.Removed,
		Updated:          diff.Updated,
		AvailableAdded:   availabilityDiff.Added,
		AvailableRemoved: availabilityDiff.Removed,
		AvailableUpdated: availabilityDiff.Updated,
		Warnings:         make([]string, 0, len(warnings)),
	}
	for _, w := range warnings {
		report.Warnings = append(report.Warnings, fmt.Sprintf("%s: %v", w.Source, w.Err))
	}
	return json.MarshalIndent(report, "", "  ")
}

// RenderObservationJSON renders the v2 report used by the migrated CLI. The
// legacy RenderJSON function remains available to direct callers.
func RenderObservationJSON(report ObservationReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}

// RenderReportJSON is a descriptive alias for callers that do not need to
// distinguish the report's domain type from its JSON representation.
func RenderReportJSON(report ObservationReport) ([]byte, error) {
	return RenderObservationJSON(report)
}

// RenderJSONV2 is kept as an obvious migration target for the future CLI.
func RenderJSONV2(report ObservationReport) ([]byte, error) {
	return RenderObservationJSON(report)
}
