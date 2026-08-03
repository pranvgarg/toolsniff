package scanner

import (
	"errors"
	"testing"

	"github.com/pranvgarg/toolsniff/model"
)

func TestNPMScannerParsesGlobalPackages(t *testing.T) {
	fixture := []byte(`{
  "name": "lib",
  "dependencies": {
    "npm": { "version": "10.9.2" },
    "opencode-ai": { "version": "1.18.4" }
  }
}`)
	runner := func(name string, args ...string) ([]byte, error) {
		if name != "npm" {
			t.Fatalf("expected command 'npm', got %q", name)
		}
		return fixture, nil
	}

	s := NewNPMScanner(runner)
	if s.Name() != "npm" {
		t.Errorf("expected Name() == \"npm\", got %q", s.Name())
	}

	tools, err := s.Scan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d: %+v", len(tools), tools)
	}
	if tools[0].Name != "npm" || tools[0].Version != "10.9.2" || tools[0].Source != "npm" {
		t.Errorf("unexpected first tool: %+v", tools[0])
	}
	if tools[1].Name != "opencode-ai" || tools[1].Version != "1.18.4" {
		t.Errorf("unexpected second tool: %+v", tools[1])
	}
}

func TestNPMScannerNotInstalled(t *testing.T) {
	runner := func(name string, args ...string) ([]byte, error) {
		return nil, errors.New("exec: \"npm\": executable file not found in $PATH")
	}
	s := NewNPMScanner(runner)
	tools, err := s.Scan()
	if err == nil {
		t.Fatal("expected an error when npm is not found")
	}
	if len(tools) != 0 {
		t.Errorf("expected no tools on error, got %+v", tools)
	}
}

func TestNPMObservationsFromJSONIncludesPackageAndBinMetadata(t *testing.T) {
	fixture := []byte(`{
  "prefix": "/usr/local",
  "dependencies": {
    "opencode-ai": {
      "version": "1.18.4",
      "path": "/usr/local/lib/node_modules/opencode-ai",
      "bin": { "opencode": "./bin/opencode.js" }
    },
    "plain-package": {}
  }
}`)

	observations, err := NPMObservationsFromJSON(fixture)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(observations) != 2 {
		t.Fatalf("expected 2 observations, got %d", len(observations))
	}
	opencode := observations[0]
	if opencode.DisplayName != "opencode-ai" {
		t.Fatalf("expected sorted opencode observation, got %+v", opencode)
	}
	if opencode.Kind != model.KindCLI || opencode.Origin.Provider != "npm" || opencode.Origin.Manager != "global" {
		t.Errorf("unexpected npm identity: %+v", opencode)
	}
	if opencode.Version.State != model.VersionKnown || opencode.Version.Value != "1.18.4" || !opencode.Version.Comparable {
		t.Errorf("unexpected npm version: %+v", opencode.Version)
	}
	if opencode.Package == nil || opencode.Package.Prefix != "/usr/local/lib/node_modules/opencode-ai" || len(opencode.Package.Executables) != 1 || opencode.Package.Executables[0] != "opencode" {
		t.Errorf("unexpected npm package metadata: %+v", opencode.Package)
	}
	if len(opencode.Locations) != 2 || opencode.Locations[1].Path != "/usr/local/lib/node_modules/opencode-ai/bin/opencode.js" {
		t.Errorf("unexpected npm locations: %+v", opencode.Locations)
	}
	if err := opencode.Validate(); err != nil {
		t.Fatalf("npm observation should validate: %v", err)
	}
}

func TestNPMObservationsMissingMetadataIsStateAware(t *testing.T) {
	observations, err := NPMObservationsFromJSON([]byte(`{"dependencies":{"unknown-package":{}}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(observations) != 1 {
		t.Fatalf("expected one observation, got %d", len(observations))
	}
	observation := observations[0]
	if observation.Version.State != model.VersionNotReported || observation.Version.Value != "" {
		t.Errorf("expected not-reported version, got %+v", observation.Version)
	}
	if len(observation.Locations) != 0 {
		t.Errorf("expected no fabricated locations, got %+v", observation.Locations)
	}
	if err := observation.Validate(); err != nil {
		t.Fatalf("missing metadata observation should validate: %v", err)
	}
}

func TestNPMObservationsRejectMalformedMetadata(t *testing.T) {
	_, err := NPMObservationsFromJSON([]byte(`{"dependencies":{"broken":{"bin":42}}}`))
	if err == nil {
		t.Fatal("expected malformed npm bin metadata to return an error")
	}
}
