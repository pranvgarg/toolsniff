package registry

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pranvgarg/toolsniff/model"
)

func TestSaveThenLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "registry.json")
	tools := []model.Tool{
		{Name: "gh", Source: "brew-formula"},
		{Name: "ollama", Source: "applications", Path: "/Applications/Ollama.app"},
	}

	if err := Save(path, tools); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, warning := Load(path)
	if warning != "" {
		t.Errorf("expected no warning, got %q", warning)
	}
	if len(loaded) != 2 || loaded[0].Name != "gh" || loaded[1].Path != "/Applications/Ollama.app" {
		t.Errorf("unexpected loaded tools: %+v", loaded)
	}
}

func TestLoadMissingFileReturnsEmptyNoWarning(t *testing.T) {
	tools, warning := Load(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if len(tools) != 0 {
		t.Errorf("expected no tools, got %+v", tools)
	}
	if warning != "" {
		t.Errorf("expected no warning for a first run, got %q", warning)
	}
}

func TestLoadCorruptFileReturnsEmptyWithWarning(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")
	if err := os.WriteFile(path, []byte("{not valid json"), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	tools, warning := Load(path)
	if len(tools) != 0 {
		t.Errorf("expected no tools for corrupt file, got %+v", tools)
	}
	if warning == "" {
		t.Error("expected a warning for a corrupt registry file")
	}
}

func TestAvailabilityPathUsesSiblingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "registry.json")
	want := filepath.Join(filepath.Dir(path), "availability.json")
	if got := AvailabilityPath(path); got != want {
		t.Fatalf("availability path = %q, want %q", got, want)
	}
}

func TestAvailabilityRegistryIsIndependent(t *testing.T) {
	dir := t.TempDir()
	installedPath := filepath.Join(dir, "registry.json")
	availablePath := AvailabilityPath(installedPath)
	installed := []model.Tool{{Name: "gh", Source: model.SourceBrewFormula}}
	available := []model.Tool{{Name: "gh", Source: model.SourcePath, Path: "/opt/homebrew/bin/gh"}}

	if err := Save(installedPath, installed); err != nil {
		t.Fatalf("saving installed registry: %v", err)
	}
	if err := Save(availablePath, available); err != nil {
		t.Fatalf("saving availability registry: %v", err)
	}

	gotInstalled, installedWarning := Load(installedPath)
	gotAvailable, availableWarning := Load(availablePath)
	if installedWarning != "" || availableWarning != "" {
		t.Fatalf("unexpected warnings: installed=%q available=%q", installedWarning, availableWarning)
	}
	if len(gotInstalled) != 1 || gotInstalled[0].Source != model.SourceBrewFormula {
		t.Errorf("unexpected installed registry: %+v", gotInstalled)
	}
	if len(gotAvailable) != 1 || gotAvailable[0].Source != model.SourcePath {
		t.Errorf("unexpected availability registry: %+v", gotAvailable)
	}
}
