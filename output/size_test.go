package output

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirectorySizeSumsFileBytesRecursively(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write a.txt: %v", err)
	}
	nested := filepath.Join(root, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "b.txt"), []byte("world!"), 0o644); err != nil {
		t.Fatalf("write b.txt: %v", err)
	}

	got, err := DirectorySize(root)
	if err != nil {
		t.Fatalf("DirectorySize: %v", err)
	}
	if want := int64(len("hello") + len("world!")); got != want {
		t.Fatalf("DirectorySize = %d, want %d", got, want)
	}
}

func TestDirectorySizeOfPlainFileIsItsOwnSize(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "single-binary")
	if err := os.WriteFile(path, []byte("0123456789"), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := DirectorySize(path)
	if err != nil {
		t.Fatalf("DirectorySize: %v", err)
	}
	if got != 10 {
		t.Fatalf("DirectorySize(file) = %d, want 10", got)
	}
}

func TestDirectorySizeMissingPathReturnsError(t *testing.T) {
	if _, err := DirectorySize(filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
		t.Fatal("expected an error for a missing path")
	}
}
