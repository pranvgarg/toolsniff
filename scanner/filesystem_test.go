package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pranvgarg/toolsniff/model"
)

func TestScanExecutableDirOnlyReturnsExecutableFiles(t *testing.T) {
	dir := t.TempDir()
	executable := filepath.Join(dir, "tool")
	nonExecutable := filepath.Join(dir, "not-a-tool")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}
	if err := os.WriteFile(nonExecutable, []byte("text\n"), 0o644); err != nil {
		t.Fatalf("write non-executable: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	tools, err := ScanExecutableDir(dir, model.SourcePath, defaultDirReader, defaultFileStat)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "tool" || tools[0].Path != executable {
		t.Fatalf("unexpected tools: %+v", tools)
	}
}

func TestScanExecutableDirMissingDirectoryReturnsError(t *testing.T) {
	_, err := ScanExecutableDir(filepath.Join(t.TempDir(), "missing"), model.SourceCargo, defaultDirReader, defaultFileStat)
	if err == nil {
		t.Fatal("expected missing directory error")
	}
}
