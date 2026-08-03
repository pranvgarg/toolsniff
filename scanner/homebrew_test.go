package scanner

import (
	"errors"
	"testing"

	"github.com/pranvgarg/toolsniff/model"
)

func TestHomebrewFormulaScannerParsesOnePerLine(t *testing.T) {
	runner := func(name string, args ...string) ([]byte, error) {
		if name != "brew" {
			t.Fatalf("expected command 'brew', got %q", name)
		}
		if len(args) < 2 || args[0] != "list" || args[1] != "--formula" {
			t.Fatalf("expected 'brew list --formula ...', got %v", args)
		}
		return []byte("gh\nmole\nwget\n"), nil
	}

	s := NewHomebrewFormulaScanner(runner)
	if s.Name() != "brew-formula" {
		t.Errorf("expected Name() == \"brew-formula\", got %q", s.Name())
	}

	tools, err := s.Scan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d: %+v", len(tools), tools)
	}
	if tools[0].Name != "gh" || tools[0].Source != "brew-formula" {
		t.Errorf("unexpected first tool: %+v", tools[0])
	}
}

func TestHomebrewCaskScannerParsesOnePerLine(t *testing.T) {
	runner := func(name string, args ...string) ([]byte, error) {
		if len(args) < 2 || args[1] != "--cask" {
			t.Fatalf("expected 'brew list --cask ...', got %v", args)
		}
		return []byte("ollama\nlm-studio\n"), nil
	}

	s := NewHomebrewCaskScanner(runner)
	if s.Name() != "brew-cask" {
		t.Errorf("expected Name() == \"brew-cask\", got %q", s.Name())
	}
	tools, err := s.Scan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 2 || tools[0].Source != "brew-cask" {
		t.Errorf("unexpected tools: %+v", tools)
	}
}

func TestHomebrewScannerNotInstalled(t *testing.T) {
	runner := func(name string, args ...string) ([]byte, error) {
		return nil, errors.New("exec: \"brew\": executable file not found in $PATH")
	}
	s := NewHomebrewFormulaScanner(runner)
	tools, err := s.Scan()
	if err == nil {
		t.Fatal("expected an error when brew is not found")
	}
	if len(tools) != 0 {
		t.Errorf("expected no tools on error, got %+v", tools)
	}
}

func TestHomebrewFormulaObservationsFromJSONIncludesVersionAndPrefix(t *testing.T) {
	fixture := []byte(`{
  "formulae": [{
    "name": "gh",
    "versions": {"stable": "2.75.0"},
    "prefix": "/opt/homebrew/opt/gh",
    "installed": [{"version": "2.75.0"}]
  }]
}`)
	observations, err := HomebrewFormulaObservationsFromJSON(fixture)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(observations) != 1 {
		t.Fatalf("expected one observation, got %d", len(observations))
	}
	observation := observations[0]
	if observation.Origin.Provider != "homebrew" || observation.Origin.Manager != "formula" || observation.Version.Value != "2.75.0" {
		t.Errorf("unexpected formula observation: %+v", observation)
	}
	if observation.Package == nil || observation.Package.Prefix != "/opt/homebrew/opt/gh" || len(observation.Locations) != 1 {
		t.Errorf("unexpected formula metadata: %+v locations=%+v", observation.Package, observation.Locations)
	}
	if err := observation.Validate(); err != nil {
		t.Fatalf("formula observation should validate: %v", err)
	}
}

func TestHomebrewCaskObservationsFromJSONIncludesVersion(t *testing.T) {
	observations, err := HomebrewCaskObservationsFromJSON([]byte(`{
  "casks": [{"token":"ollama","name":"Ollama","version":"0.1.2"}]
}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(observations) != 1 || observations[0].Origin.Manager != "cask" || observations[0].Version.Value != "0.1.2" {
		t.Fatalf("unexpected cask observations: %+v", observations)
	}
	if err := observations[0].Validate(); err != nil {
		t.Fatalf("cask observation should validate: %v", err)
	}
}

func TestHomebrewObservationsMissingVersionIsStateAware(t *testing.T) {
	observations, err := HomebrewFormulaObservationsFromJSON([]byte(`{"formulae":[{"name":"local-tool"}]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if observations[0].Version.State != model.VersionNotReported || observations[0].Package.Version != "" {
		t.Errorf("expected missing formula version to be not-reported: %+v", observations[0])
	}
}

func TestHomebrewObservationsRejectMalformedMetadata(t *testing.T) {
	_, err := HomebrewFormulaObservationsFromJSON([]byte(`{"formulae":[{"name":"broken","installed":"not-an-array"}]}`))
	if err == nil {
		t.Fatal("expected malformed Homebrew metadata to return an error")
	}
}
