package output

import (
	"strings"
	"testing"

	"github.com/pranvgarg/toolsniff/model"
)

func TestBuildDetailViewModelIncludesCapabilitiesSection(t *testing.T) {
	observation := model.Observation{
		ID:          "npx-tool",
		DisplayName: "npx-tool",
		Kind:        model.KindExecutable,
		Role:        model.RoleAvailable,
		Availability: model.AvailabilityInfo{
			State:        model.AvailabilityAvailable,
			ResolvedPath: "/usr/local/bin/npx-tool",
		},
		Locations: []model.Location{
			{Path: "/usr/local/bin/npx-tool", Type: model.LocationExecutable, Executable: true},
		},
		Evidence: []model.Evidence{
			{Type: "filesystem", Source: "PATH", Description: "found on PATH"},
		},
	}

	detail := BuildDetailViewModel(observation)

	var capabilities *DetailSection
	for i := range detail.Sections {
		if detail.Sections[i].Title == "Capabilities" {
			capabilities = &detail.Sections[i]
		}
	}
	if capabilities == nil {
		t.Fatalf("detail view missing a Capabilities section: %+v", detail.Sections)
	}
}

func TestBuildDetailViewModelCapabilitiesSectionSaysNoneDetectedWhenEmpty(t *testing.T) {
	observation := model.Observation{ID: "plain-tool", DisplayName: "plain-tool", Kind: model.KindPackage}
	detail := BuildDetailViewModel(observation)

	var capabilities *DetailSection
	for i := range detail.Sections {
		if detail.Sections[i].Title == "Capabilities" {
			capabilities = &detail.Sections[i]
		}
	}
	if capabilities == nil {
		t.Fatal("detail view missing a Capabilities section")
	}
	if len(capabilities.Fields) != 1 || !strings.Contains(capabilities.Fields[0].Value, "none detected") {
		t.Fatalf("expected a single 'none detected' field, got %+v", capabilities.Fields)
	}
}
