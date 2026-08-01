package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplicationsScannerDiscoversBundlesWithoutKeywords(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"Ollama.app", "Preview.app", "Claude.app", "Calculator.app"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", name, err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "Developer", "Tool.app", "Contents", "Helpers", "Nested.app"), 0o755); err != nil {
		t.Fatalf("mkdir nested app fixture: %v", err)
	}

	s := NewApplicationsScanner([]string{root}, nil)
	if s.Name() != "applications" {
		t.Errorf("expected Name() == \"applications\", got %q", s.Name())
	}
	tools, err := s.Scan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 5 {
		t.Fatalf("expected 5 application bundles, got %d: %+v", len(tools), tools)
	}
	for _, tool := range tools {
		if tool.Source != "applications" {
			t.Errorf("expected applications source, got %+v", tool)
		}
	}
	for _, tool := range tools {
		if tool.Name == "Nested.app" {
			t.Error("did not expect helper app inside an application bundle")
		}
	}
}

func TestApplicationsScannerDiscoversMultipleRoots(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	if err := os.Mkdir(filepath.Join(first, "First.app"), 0o755); err != nil {
		t.Fatalf("mkdir first app: %v", err)
	}
	if err := os.Mkdir(filepath.Join(second, "Second.app"), 0o755); err != nil {
		t.Fatalf("mkdir second app: %v", err)
	}

	tools, err := NewApplicationsScanner([]string{first, second, first}, nil).Scan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 2 || tools[0].Name != "First.app" || tools[1].Name != "Second.app" {
		t.Fatalf("unexpected tools: %+v", tools)
	}
}

func TestApplicationsScannerMissingRootIsNotAnError(t *testing.T) {
	tools, err := NewApplicationsScanner([]string{filepath.Join(t.TempDir(), "nope")}, nil).Scan()
	if err != nil {
		t.Fatalf("expected no error for missing root, got %v", err)
	}
	if len(tools) != 0 {
		t.Errorf("expected no tools, got %+v", tools)
	}
}
