package scanner

import (
	"errors"
	"testing"
)

func TestPathScannerChecksCandidates(t *testing.T) {
	lookup := func(name string) (string, error) {
		switch name {
		case "claude":
			return "/opt/homebrew/bin/claude", nil
		case "ollama":
			return "/usr/local/bin/ollama", nil
		default:
			return "", errors.New("not found")
		}
	}

	s := NewPathScanner(lookup, []string{"claude", "ollama", "nonexistent-tool"})
	if s.Name() != "path" {
		t.Errorf("expected Name() == \"path\", got %q", s.Name())
	}
	tools, err := s.Scan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 found tools, got %d: %+v", len(tools), tools)
	}
	if tools[0].Name != "claude" || tools[0].Path != "/opt/homebrew/bin/claude" || tools[0].Source != "path" {
		t.Errorf("unexpected first tool: %+v", tools[0])
	}
}

func TestPathScannerEmptyCandidates(t *testing.T) {
	s := NewPathScanner(func(string) (string, error) { return "", errors.New("nope") }, nil)
	tools, err := s.Scan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 0 {
		t.Errorf("expected no tools, got %+v", tools)
	}
}
