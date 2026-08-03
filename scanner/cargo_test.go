// scanner/cargo_test.go
package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pranvgarg/toolsniff/model"
)

func TestCargoScannerListsBinaries(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"ripgrep", "bat"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatalf("write fixture binary: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "not-executable"), []byte("text\n"), 0o644); err != nil {
		t.Fatalf("write non-executable fixture: %v", err)
	}

	s := NewCargoScanner(dir)
	if s.Name() != "cargo" {
		t.Errorf("expected Name() == \"cargo\", got %q", s.Name())
	}
	tools, err := s.Scan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 2 || tools[0].Name != "bat" || tools[0].Source != "cargo" {
		t.Errorf("unexpected tools: %+v", tools)
	}
}

func TestCargoScannerMissingDirIsNotAnError(t *testing.T) {
	s := NewCargoScanner(filepath.Join(t.TempDir(), "does-not-exist"))
	tools, err := s.Scan()
	if err != nil {
		t.Fatalf("expected no error for missing directory, got %v", err)
	}
	if len(tools) != 0 {
		t.Errorf("expected no tools, got %+v", tools)
	}
}

func TestCargoObservationsExposeExecutableLocationsWithoutFabricatingMetadata(t *testing.T) {
	observations := CargoObservationsFromTools([]model.Tool{{Name: "ripgrep", Source: model.SourceCargo, Path: "/Users/test/.cargo/bin/rg"}})
	if len(observations) != 1 {
		t.Fatalf("expected one observation, got %d", len(observations))
	}
	observation := observations[0]
	if observation.Kind != model.KindExecutable || observation.Origin.Provider != "cargo" || observation.Version.State != model.VersionNotReported {
		t.Errorf("unexpected cargo observation: %+v", observation)
	}
	if len(observation.Locations) != 1 || observation.Locations[0].Path != "/Users/test/.cargo/bin/rg" || !observation.Locations[0].Executable {
		t.Errorf("unexpected cargo locations: %+v", observation.Locations)
	}
	if err := observation.Validate(); err != nil {
		t.Fatalf("cargo observation should validate: %v", err)
	}
}
