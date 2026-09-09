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

func TestFormatBytesSmallValues(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{512, "512 B"},
		{1023, "1023 B"},
	}
	for _, tt := range tests {
		got := FormatBytes(tt.bytes)
		if got != tt.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestFormatBytesKilobytes(t *testing.T) {
	got := FormatBytes(1024)
	if got != "1.0 KB" {
		t.Errorf("FormatBytes(1024) = %q, want %q", got, "1.0 KB")
	}

	got = FormatBytes(1536)
	if got != "1.5 KB" {
		t.Errorf("FormatBytes(1536) = %q, want %q", got, "1.5 KB")
	}
}

func TestFormatBytesLargeValues(t *testing.T) {
	tests := []struct {
		bytes int64
		unit  string
	}{
		{1024 * 1024, "MB"},
		{1024 * 1024 * 1024, "GB"},
	}
	for _, tt := range tests {
		got := FormatBytes(tt.bytes)
		if !contains(got, tt.unit) {
			t.Errorf("FormatBytes(%d) = %q, want to contain %q", tt.bytes, got, tt.unit)
		}
	}
}

func TestFormatBytesDoesNotPanicAtTiBBoundary(t *testing.T) {
	// This should not panic, even though 1 TiB would require a 4th unit.
	got := FormatBytes(1024 * 1024 * 1024 * 1024)
	if !contains(got, "GB") {
		t.Errorf("FormatBytes(1 TiB) = %q, want to contain GB", got)
	}
	// Verify it's a large number (clamped to GB, so 1024.0 GB)
	if !contains(got, "1024") {
		t.Errorf("FormatBytes(1 TiB) = %q, want to contain 1024", got)
	}
}

// contains is a simple helper to check if a string contains a substring.
func contains(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
