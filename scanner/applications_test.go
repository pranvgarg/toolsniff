package scanner

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/pranvgarg/toolsniff/model"
)

func applicationFixture(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join("testdata", "applications", name, "Example.app")
}

func TestReadApplicationInfoFixtures(t *testing.T) {
	tests := []struct {
		name string
		want model.ApplicationInfo
	}{
		{
			name: "valid",
			want: model.ApplicationInfo{
				BundleID:       "com.example.fixture",
				DisplayVersion: "Fixture 4.2",
				ShortVersion:   "4.2.1",
				MinimumOS:      "13.0",
				Architectures:  []string{"arm64", "x86_64"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := readApplicationInfo(applicationFixture(t, test.name))
			if err != nil {
				t.Fatalf("readApplicationInfo: %v", err)
			}
			if got.BundleID != test.want.BundleID || got.DisplayVersion != test.want.DisplayVersion || got.ShortVersion != test.want.ShortVersion || got.MinimumOS != test.want.MinimumOS {
				t.Fatalf("unexpected application metadata: %+v", got)
			}
			if len(got.Architectures) != len(test.want.Architectures) {
				t.Fatalf("unexpected architectures: %+v", got.Architectures)
			}
			for i := range test.want.Architectures {
				if got.Architectures[i] != test.want.Architectures[i] {
					t.Errorf("architecture %d = %q, want %q", i, got.Architectures[i], test.want.Architectures[i])
				}
			}
		})
	}
}

func TestReadApplicationInfoMissingMetadata(t *testing.T) {
	_, err := readApplicationInfo(applicationFixture(t, "missing"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected missing Info.plist error, got %v", err)
	}
}

func TestReadApplicationInfoMalformedMetadata(t *testing.T) {
	_, err := readApplicationInfo(applicationFixture(t, "malformed"))
	if err == nil {
		t.Fatal("expected malformed Info.plist error")
	}
	if errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected parse error, got missing file error: %v", err)
	}
}

func TestReadApplicationInfoDoesNotInspectNestedBundles(t *testing.T) {
	bundle := applicationFixture(t, "nested")
	got, err := readApplicationInfo(bundle)
	if err != nil {
		t.Fatalf("readApplicationInfo: %v", err)
	}
	if got.BundleID != "com.example.outer" {
		t.Fatalf("expected outer bundle metadata, got %+v", got)
	}

	tools, err := NewApplicationsScanner([]string{filepath.Dir(bundle)}, nil).Scan()
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(tools) != 1 || tools[0].Path != bundle {
		t.Fatalf("expected only outer bundle, got %+v", tools)
	}
}

func TestApplicationsScannerDeduplicatesFixtureBundles(t *testing.T) {
	root := filepath.Dir(applicationFixture(t, "duplicate"))
	tools, err := NewApplicationsScanner([]string{root, root}, nil).Scan()
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "Example.app" {
		t.Fatalf("expected one duplicate bundle, got %+v", tools)
	}
}

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
