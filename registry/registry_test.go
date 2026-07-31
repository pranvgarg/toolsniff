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
