package scanner

import (
	"errors"
	"testing"
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
