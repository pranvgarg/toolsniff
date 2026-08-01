package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathScannerDiscoversExecutableFiles(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	if err := os.WriteFile(filepath.Join(first, "claude"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write claude: %v", err)
	}
	if err := os.WriteFile(filepath.Join(first, "not-executable"), []byte("text\n"), 0o644); err != nil {
		t.Fatalf("write non-executable: %v", err)
	}
	if err := os.WriteFile(filepath.Join(second, "claude"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write second claude: %v", err)
	}

	s := NewPathScanner([]string{first, second, first}, nil, nil)
	if s.Name() != "path" {
		t.Errorf("expected Name() == \"path\", got %q", s.Name())
	}
	tools, err := s.Scan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected two distinct executable paths, got %d: %+v", len(tools), tools)
	}
	if tools[0].Name != "claude" || tools[1].Name != "claude" || tools[0].Path == tools[1].Path {
		t.Errorf("expected same name with separate paths, got %+v", tools)
	}
}

func TestPathScannerExcludesSystemDirectories(t *testing.T) {
	systemDir := t.TempDir()
	userDir := t.TempDir()
	for _, dir := range []string{systemDir, userDir} {
		if err := os.WriteFile(filepath.Join(dir, "tool"), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatalf("write tool: %v", err)
		}
	}

	tools, err := NewPathScanner([]string{systemDir, userDir}, []string{systemDir}, nil).Scan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 1 || tools[0].Path != filepath.Join(userDir, "tool") {
		t.Fatalf("unexpected tools: %+v", tools)
	}
}

func TestPathScannerMissingDirectoryIsNotAnError(t *testing.T) {
	tools, err := NewPathScanner([]string{filepath.Join(t.TempDir(), "does-not-exist")}, nil, nil).Scan()
	if err != nil {
		t.Fatalf("expected no error for missing directory, got %v", err)
	}
	if len(tools) != 0 {
		t.Errorf("expected no tools, got %+v", tools)
	}
}
