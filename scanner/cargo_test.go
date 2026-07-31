// scanner/cargo_test.go
package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCargoScannerListsBinaries(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"ripgrep", "bat"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatalf("write fixture binary: %v", err)
		}
	}

	s := NewCargoScanner(dir)
	if s.Name() != "cargo" {
		t.Errorf("expected Name() == \"cargo\", got %q", s.Name())
	}
	tools, err := s.Scan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 2 || tools[0].Name != "bat" || tools[0].Source != "cargo" {
		t.Errorf("unexpected tools: %+v", tools)
	}
}

func TestCargoScannerMissingDirIsNotAnError(t *testing.T) {
	s := NewCargoScanner(filepath.Join(t.TempDir(), "does-not-exist"))
	tools, err := s.Scan()
	if err != nil {
		t.Fatalf("expected no error for missing directory, got %v", err)
	}
	if len(tools) != 0 {
		t.Errorf("expected no tools, got %+v", tools)
	}
}
