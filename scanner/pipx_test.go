package scanner

import (
	"testing"

	"github.com/pranvgarg/toolsniff/model"
)

func TestPipxScannerParsesVenvs(t *testing.T) {
	fixture := []byte(`{
  "venvs": {
    "black": {
      "metadata": { "main_package": { "package_version": "24.1.0" } }
    }
  }
}`)
	runner := func(name string, args ...string) ([]byte, error) {
		if name != "pipx" {
			t.Fatalf("expected command 'pipx', got %q", name)
		}
		return fixture, nil
	}

	s := NewPipxScanner(runner)
	if s.Name() != "pipx" {
		t.Errorf("expected Name() == \"pipx\", got %q", s.Name())
	}
	tools, err := s.Scan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "black" || tools[0].Version != "24.1.0" || tools[0].Source != "pipx" {
		t.Errorf("unexpected tools: %+v", tools)
	}
}

func TestPipxScannerEmptyVenvs(t *testing.T) {
	runner := func(name string, args ...string) ([]byte, error) {
		return []byte(`{"venvs": {}}`), nil
	}
	s := NewPipxScanner(runner)
	tools, err := s.Scan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 0 {
		t.Errorf("expected no tools, got %+v", tools)
	}
}

func TestPipxObservationsFromJSONIncludesVenvAndExecutableMetadata(t *testing.T) {
	fixture := []byte(`{
  "venvs": {
    "black": {
      "venv_dir": "/Users/test/.local/share/pipx/venvs/black",
      "metadata": {"main_package": {"package_or_url":"black","package_version":"24.1.0"}},
      "app_paths": ["/Users/test/.local/bin/black"],
      "bin_paths": ["/Users/test/.local/share/pipx/venvs/black/bin/black"]
    }
  }
}`)
	observations, err := PipxObservationsFromJSON(fixture)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(observations) != 1 {
		t.Fatalf("expected one observation, got %d", len(observations))
	}
	observation := observations[0]
	if observation.Kind != model.KindCLI || observation.Origin.Package != "black" || observation.Version.Value != "24.1.0" {
		t.Errorf("unexpected pipx observation: %+v", observation)
	}
	if observation.Package == nil || observation.Package.VirtualEnv != "/Users/test/.local/share/pipx/venvs/black" || len(observation.Package.Executables) != 1 {
		t.Errorf("unexpected pipx package metadata: %+v", observation.Package)
	}
	if len(observation.Locations) != 3 || observation.Locations[0].Type != model.LocationVirtualEnv {
		t.Errorf("unexpected pipx locations: %+v", observation.Locations)
	}
	if err := observation.Validate(); err != nil {
		t.Fatalf("pipx observation should validate: %v", err)
	}
}

func TestPipxObservationsMissingMetadataIsStateAware(t *testing.T) {
	observations, err := PipxObservationsFromJSON([]byte(`{"venvs":{"bare":{}}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(observations) != 1 || observations[0].Version.State != model.VersionNotReported {
		t.Fatalf("unexpected missing pipx metadata: %+v", observations)
	}
	if err := observations[0].Validate(); err != nil {
		t.Fatalf("missing metadata observation should validate: %v", err)
	}
}

func TestPipxObservationsRejectMalformedMetadata(t *testing.T) {
	_, err := PipxObservationsFromJSON([]byte(`{"venvs":{"broken":{"app_paths":"not-a-list"}}}`))
	if err == nil {
		t.Fatal("expected malformed pipx metadata to return an error")
	}
}
