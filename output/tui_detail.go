package output

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pranvgarg/toolsniff/capabilities"
	"github.com/pranvgarg/toolsniff/model"
)

// DetailField is one labeled, read-only value in a detail section.
type DetailField struct {
	Label string
	Value string
}

// DetailSection groups related metadata for progressive disclosure.
type DetailSection struct {
	Title  string
	Fields []DetailField
}

// DetailViewModel is independent of the domain object's JSON shape and can be
// rendered as a modal, side pane, or plain text without changing model types.
type DetailViewModel struct {
	Title string
	// TypeLabel is the one-line "what is this" answer -- "Homebrew cask",
	// "npm global package", "PATH executable". It leads the view because it is
	// the question a user has before any of the metadata below matters.
	TypeLabel string
	// WhatThisIs is the same claim spelled out for someone who does not know
	// what a cask is: "Homebrew cask — a macOS app installed via Homebrew
	// Cask." It comes from the plain-language table in output/kinds.go.
	WhatThisIs    string
	ObservationID string
	Kind          string
	Sections      []DetailSection
	// Actions are the per-kind operations offered for this observation,
	// derived by KindActions. Nothing here has been executed.
	Actions []Action
}

// BuildDetailViewModel includes every optional v2 metadata group when present.
func BuildDetailViewModel(observation model.Observation) DetailViewModel {
	detail := DetailViewModel{
		Title:         observation.DisplayName + " Details",
		TypeLabel:     ObservationTypeLabel(observation),
		WhatThisIs:    WhatThisIsLine(observation),
		ObservationID: observation.ID,
		Kind:          string(observation.Kind),
		Actions:       KindActions(observation),
		Sections: []DetailSection{
			{Title: "Overview", Fields: []DetailField{
				{Label: "Name", Value: observation.DisplayName},
				{Label: "Command", Value: emptyValue(observation.CommandName)},
				{Label: "Type", Value: ObservationTypeLabel(observation)},
				{Label: "Kind", Value: string(observation.Kind)},
				{Label: "Status", Value: StatusDisplayLabel(ObservationStatus(observation))},
				{Label: "Source", Value: ObservationSource(observation)},
				{Label: "Size", Value: observationSizeLabel(observation)},
			}},
			{Title: "Version", Fields: []DetailField{
				{Label: "Version", Value: DisplayVersion(observation.Version)},
				{Label: "Version state", Value: string(observation.Version.State)},
				{Label: "Scheme", Value: emptyValue(string(observation.Version.Scheme))},
				{Label: "Confidence", Value: emptyValue(string(observation.Version.Confidence))},
				{Label: "Retrieved by", Value: emptyValue(observation.Version.RetrievedBy)},
			}},
			{Title: "Origin", Fields: []DetailField{
				{Label: "Provider", Value: observation.Origin.Provider},
				{Label: "Manager", Value: emptyValue(observation.Origin.Manager)},
				{Label: "Package", Value: emptyValue(observation.Origin.Package)},
				{Label: "Manifest", Value: emptyValue(observation.Origin.Manifest)},
			}},
		},
	}

	detail.Sections = append(detail.Sections, DetailSection{
		Title:  "Capabilities",
		Fields: capabilityFields(observation),
	})

	if len(observation.Locations) > 0 {
		fields := make([]DetailField, 0, len(observation.Locations))
		for _, location := range observation.Locations {
			fields = append(fields, DetailField{Label: string(location.Type), Value: location.Path})
		}
		detail.Sections = append(detail.Sections, DetailSection{Title: "Locations", Fields: fields})
	}

	if observation.Availability.State != "" {
		fields := []DetailField{
			{Label: "State", Value: string(observation.Availability.State)},
			{Label: "Active command", Value: yesNo(observation.Availability.ResolvedPath != "")},
		}
		if observation.Availability.ResolvedPath != "" {
			fields = append(fields, DetailField{Label: "Resolved path", Value: observation.Availability.ResolvedPath})
		}
		if observation.Availability.PATHIndex != 0 {
			fields = append(fields, DetailField{Label: "PATH index", Value: fmt.Sprintf("%d", observation.Availability.PATHIndex)})
		}
		if len(observation.Availability.ShadowedBy) > 0 {
			fields = append(fields, DetailField{Label: "Shadowed by", Value: strings.Join(observation.Availability.ShadowedBy, ", ")})
		}
		detail.Sections = append(detail.Sections, DetailSection{Title: "Availability", Fields: fields})
	}

	if observation.Package != nil {
		detail.Sections = append(detail.Sections, DetailSection{Title: "Package", Fields: []DetailField{
			{Label: "Name", Value: emptyValue(observation.Package.Name)},
			{Label: "Version", Value: emptyValue(observation.Package.Version)},
			{Label: "Executables", Value: strings.Join(observation.Package.Executables, ", ")},
			{Label: "Prefix", Value: emptyValue(observation.Package.Prefix)},
			{Label: "Virtual env", Value: emptyValue(observation.Package.VirtualEnv)},
		}})
	}
	if observation.Application != nil {
		detail.Sections = append(detail.Sections, DetailSection{Title: "Application", Fields: []DetailField{
			{Label: "Bundle ID", Value: emptyValue(observation.Application.BundleID)},
			{Label: "Display version", Value: emptyValue(observation.Application.DisplayVersion)},
			{Label: "Short version", Value: emptyValue(observation.Application.ShortVersion)},
			{Label: "Minimum OS", Value: emptyValue(observation.Application.MinimumOS)},
			{Label: "Architectures", Value: strings.Join(observation.Application.Architectures, ", ")},
			{Label: "Signed", Value: optionalBool(observation.Application.Signed)},
			{Label: "Signing team", Value: emptyValue(observation.Application.SigningTeamID)},
		}})
	}
	if observation.History != nil {
		lastUsed := "n/a"
		if observation.History.LastUsed != nil {
			lastUsed = observation.History.LastUsed.Format("2006-01-02")
		}
		detail.Sections = append(detail.Sections, DetailSection{Title: "History", Fields: []DetailField{
			{Label: "Last used", Value: lastUsed},
			{Label: "Cache path", Value: emptyValue(observation.History.CachePath)},
		}})
	}
	if len(observation.Evidence) > 0 {
		fields := make([]DetailField, 0, len(observation.Evidence))
		for _, evidence := range observation.Evidence {
			value := evidence.Source
			if evidence.Description != "" {
				value += ": " + evidence.Description
			}
			fields = append(fields, DetailField{Label: evidence.Type, Value: value})
		}
		detail.Sections = append(detail.Sections, DetailSection{Title: "Evidence", Fields: fields})
	}
	return detail
}

// DetailForObservation is the short constructor name used by view code.
func DetailForObservation(observation model.Observation) DetailViewModel {
	return BuildDetailViewModel(observation)
}

// RenderDetailView renders the detail model without putting a path under a
// Version heading.
func RenderDetailView(detail DetailViewModel) string {
	var b strings.Builder
	fmt.Fprintln(&b, detail.Title)
	if detail.TypeLabel != "" {
		fmt.Fprintln(&b, detail.TypeLabel)
	}
	// The plain-English sentence comes before anything else a reader has to
	// already know the vocabulary to understand.
	if detail.WhatThisIs != "" {
		fmt.Fprintf(&b, "What this is: %s\n", detail.WhatThisIs)
	}
	// Actions lead, matching the styled pane in tui_panes.go: what you can do
	// with the thing is more useful than the metadata that identifies it.
	if len(detail.Actions) > 0 {
		fmt.Fprintf(&b, "\n%s\n", "Actions")
		for _, action := range detail.Actions {
			fmt.Fprintf(&b, "  %-18s %s\n", action.Label+":", ActionCommandLine(action))
		}
	}
	for _, section := range detail.Sections {
		fmt.Fprintf(&b, "\n%s\n", section.Title)
		for _, field := range section.Fields {
			fmt.Fprintf(&b, "  %-18s %s\n", field.Label+":", field.Value)
		}
	}
	return b.String()
}

// ActionCommandLine is the single formatting of an action's right-hand side:
// its shell-ready command when it has one, its note otherwise. It quotes
// nothing and escapes nothing -- it is display text, not a shell string.
func ActionCommandLine(action Action) string {
	if action.Runnable() {
		return strings.Join(action.Command, " ")
	}
	return action.Note
}

// CopySelectedPath returns a path for a caller-owned clipboard integration.
// It does not access the clipboard or execute a command.
func CopySelectedPath(observation model.Observation) string {
	if len(observation.Locations) == 0 {
		return ""
	}
	return observation.Locations[0].Path
}

// CopySelectedObservationJSON returns the selected observation's JSON bytes.
func CopySelectedObservationJSON(observation model.Observation) ([]byte, error) {
	return json.MarshalIndent(observation, "", "  ")
}

// RevealLocationCommand returns a safe, argument-separated Finder command.
// The caller decides whether to present or execute it; no shell is involved.
func RevealLocationCommand(path string) ([]string, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("location path is empty")
	}
	return []string{"open", "-R", filepath.Clean(path)}, nil
}

// capabilityFields runs the same capability detection --capabilities
// exposes as JSON, scoped to this one observation, so the detail pane and
// the --capabilities flag can never disagree about what a tool can do.
func capabilityFields(observation model.Observation) []DetailField {
	results := capabilities.DefaultRegistry().Detect([]model.Observation{observation})
	if len(results) == 0 {
		return []DetailField{{Label: "Capabilities", Value: "none detected"}}
	}
	fields := make([]DetailField, 0, len(results))
	for _, result := range results {
		fields = append(fields, DetailField{
			Label: string(result.Capability.Kind),
			Value: capabilityDetail(result),
		})
	}
	return fields
}

func capabilityDetail(result capabilities.Result) string {
	for _, evidence := range result.Capability.Evidence {
		if evidence.Type == "probe" && evidence.Detail != "" {
			return evidence.Detail
		}
	}
	if len(result.Capability.Evidence) > 0 {
		return result.Capability.Evidence[0].Detail
	}
	return "detected"
}

func emptyValue(value string) string {
	if value == "" {
		return "n/a"
	}
	return value
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func optionalBool(value *bool) string {
	if value == nil {
		return "n/a"
	}
	return yesNo(*value)
}

// observationSizeLabel measures the first location's on-disk footprint.
// Measurement happens here, at detail-view build time, not in InventoryRows
// (output/report.go) -- InventoryRows runs on every repaint, and a
// filesystem walk does not belong on that path. Opening one item's detail
// pane is a deliberate, infrequent action, which is exactly the boundary
// DirectorySize's doc comment already draws.
func observationSizeLabel(observation model.Observation) string {
	if len(observation.Locations) == 0 {
		return "size unavailable"
	}
	size, err := DirectorySize(observation.Locations[0].Path)
	if err != nil {
		return "size unavailable"
	}
	return FormatBytes(size)
}
