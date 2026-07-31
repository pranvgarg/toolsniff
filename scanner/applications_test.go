package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplicationsScannerFiltersByKeyword(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"Ollama.app", "Preview.app", "Claude.app", "Calculator.app"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}

	s := NewApplicationsScanner(dir, []string{"ollama", "claude"})
	if s.Name() != "applications" {
		t.Errorf("expected Name() == \"applications\", got %q", s.Name())
	}
	tools, err := s.Scan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 matched apps, got %d: %+v", len(tools), tools)
	}
	names := map[string]bool{tools[0].Name: true, tools[1].Name: true}
	if !names["Ollama.app"] || !names["Claude.app"] {
		t.Errorf("unexpected matched apps: %+v", tools)
	}
}

func TestApplicationsScannerMissingDirIsNotAnError(t *testing.T) {
	s := NewApplicationsScanner(filepath.Join(t.TempDir(), "nope"), DefaultApplicationKeywords())
	tools, err := s.Scan()
	if err != nil {
		t.Fatalf("expected no error for missing directory, got %v", err)
	}
	if len(tools) != 0 {
		t.Errorf("expected no tools, got %+v", tools)
	}
}
