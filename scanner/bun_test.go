package scanner

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/pranvgarg/toolsniff/model"
)

func TestBunScannerDiscoversGlobalBinaries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bun-tool")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write bun fixture: %v", err)
	}
	runner := func(name string, args ...string) ([]byte, error) {
		if name != "bun" || len(args) != 3 || args[0] != "pm" || args[1] != "bin" || args[2] != "-g" {
			t.Fatalf("unexpected command: %q %v", name, args)
		}
		return []byte(dir + "\n"), nil
	}

	s := NewBunScanner(runner)
	if s.Name() != "bun" {
		t.Errorf("expected Name() == \"bun\", got %q", s.Name())
	}
	tools, err := s.Scan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "bun-tool" || tools[0].Source != "bun" {
		t.Fatalf("unexpected tools: %+v", tools)
	}
}

func TestBunScannerNotInstalled(t *testing.T) {
	s := NewBunScanner(func(string, ...string) ([]byte, error) {
		return nil, errors.New("bun not found")
	})
	tools, err := s.Scan()
	if err == nil {
		t.Fatal("expected an error when bun is not found")
	}
	if len(tools) != 0 {
		t.Errorf("expected no tools on error, got %+v", tools)
	}
}

func TestBunObservationsExposeExecutableLocationsWithoutFabricatingMetadata(t *testing.T) {
	observations := BunObservationsFromTools([]model.Tool{{Name: "bun-tool", Source: model.SourceBun, Path: "/tmp/bun/bin/bun-tool"}})
	if len(observations) != 1 {
		t.Fatalf("expected one observation, got %d", len(observations))
	}
	observation := observations[0]
	if observation.Kind != model.KindExecutable || observation.Origin.Provider != "bun" || observation.Version.State != model.VersionNotReported {
		t.Errorf("unexpected bun observation: %+v", observation)
	}
	if len(observation.Locations) != 1 || observation.Locations[0].Path != "/tmp/bun/bin/bun-tool" || !observation.Locations[0].Executable {
		t.Errorf("unexpected bun locations: %+v", observation.Locations)
	}
	if err := observation.Validate(); err != nil {
		t.Fatalf("bun observation should validate: %v", err)
	}
}
