package scanner

import (
	"errors"
	"testing"
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
