package output

import (
	"os"
	"path/filepath"
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

func TestBuildDetailViewModelShowsSizeForALocationWithKnownPath(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "tool"), make([]byte, 2048), 0o755); err != nil {
		t.Fatalf("write fixture binary: %v", err)
	}
	observation := model.Observation{
		ID:          "sized-tool",
		DisplayName: "sized-tool",
		Kind:        model.KindExecutable,
		Locations:   []model.Location{{Path: filepath.Join(root, "tool"), Type: model.LocationExecutable, Executable: true}},
	}

	detail := BuildDetailViewModel(observation)

	var overview *DetailSection
	for i := range detail.Sections {
		if detail.Sections[i].Title == "Overview" {
			overview = &detail.Sections[i]
		}
	}
	if overview == nil {
		t.Fatal("missing Overview section")
	}
	var sizeField *DetailField
	for i := range overview.Fields {
		if overview.Fields[i].Label == "Size" {
			sizeField = &overview.Fields[i]
		}
	}
	if sizeField == nil {
		t.Fatal("Overview section missing a Size field")
	}
	if !strings.Contains(sizeField.Value, "2.0 KB") && !strings.Contains(sizeField.Value, "2 KB") {
		t.Fatalf("Size field = %q, want a formatted ~2KB value", sizeField.Value)
	}
}

func TestBuildDetailViewModelSizeUnavailableForMissingPath(t *testing.T) {
	observation := model.Observation{
		ID:        "gone-tool",
		Kind:      model.KindExecutable,
		Locations: []model.Location{{Path: "/no/such/path/ever", Type: model.LocationExecutable}},
	}
	detail := BuildDetailViewModel(observation)
	var overview *DetailSection
	for i := range detail.Sections {
		if detail.Sections[i].Title == "Overview" {
			overview = &detail.Sections[i]
		}
	}
	for _, field := range overview.Fields {
		if field.Label == "Size" && field.Value != "size unavailable" {
			t.Fatalf("Size field = %q, want %q", field.Value, "size unavailable")
		}
	}
}
