// scanner/npx_test.go
package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pranvgarg/toolsniff/model"
)

// makeNPXFixture builds a fake ~/.npm/_npx directory with one hash dir
// containing a node_modules/.bin symlink pointing at a package, mimicking
// what `npx <pkg>` leaves behind.
func makeNPXFixture(t *testing.T, root string, hash, binName, pkgName string) {
	t.Helper()
	binDir := filepath.Join(root, hash, "node_modules", ".bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	pkgDir := filepath.Join(root, hash, "node_modules", pkgName)
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	target := filepath.Join("..", pkgName, "bin", "cli.js")
	if err := os.Symlink(target, filepath.Join(binDir, binName)); err != nil {
		t.Fatalf("symlink: %v", err)
	}
}

func TestNPXScannerResolvesPackageFromBinSymlink(t *testing.T) {
	root := t.TempDir()
	makeNPXFixture(t, root, "hash1", "create-vite", "create-vite")
	makeNPXFixture(t, root, "hash2", "prisma", "@prisma/cli")

	s := NewNPXScanner(root)
	if s.Name() != model.SourceNPXHistory {
		t.Errorf("expected Name() == \"npx-history\", got %q", s.Name())
	}

	tools, err := s.Scan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d: %+v", len(tools), tools)
	}
	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.Name] = true
		if tool.Source != model.SourceNPXHistory {
			t.Errorf("expected Source == \"npx-history\", got %q", tool.Source)
		}
	}
	if !names["create-vite"] || !names["@prisma/cli"] {
		t.Errorf("unexpected tool names: %+v", tools)
	}
}

func TestNPXScannerMissingDirIsNotAnError(t *testing.T) {
	s := NewNPXScanner(filepath.Join(t.TempDir(), "does-not-exist"))
	tools, err := s.Scan()
	if err != nil {
		t.Fatalf("expected no error for missing directory, got %v", err)
	}
	if len(tools) != 0 {
		t.Errorf("expected no tools, got %+v", tools)
	}
}
