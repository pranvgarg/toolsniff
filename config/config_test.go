package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadMissingFileReturnsDiscoveryDefaults(t *testing.T) {
	t.Setenv("PATH", filepath.Join(t.TempDir(), "bin"))
	settings, err := Load(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(settings.Applications.Roots) == 0 {
		t.Fatal("expected application roots")
	}
	if len(settings.Path.Directories) != 1 {
		t.Fatalf("expected PATH to be split into one directory, got %+v", settings.Path.Directories)
	}
	if !settings.Bun.Enabled {
		t.Fatal("expected Bun discovery to be enabled by default")
	}
}

func TestLoadTOMLOverridesDiscoverySettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	content := []byte("" +
		"[applications]\n" +
		"roots = [\"~/Custom Apps\"]\n" +
		"ignore_paths = [\"~/Custom Apps/Ignore.app\"]\n" +
		"\n[path]\n" +
		"directories = [\"~/bin\"]\n" +
		"exclude_directories = [\"/custom/system\"]\n" +
		"\n[bun]\n" +
		"enabled = false\n" +
		"\n[registry]\n" +
		"path = \"~/state/registry.json\"\n" +
		"\n[execution]\n" +
		"timeout = \"12s\"\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	settings, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	home, _ := os.UserHomeDir()
	if len(settings.Applications.Roots) != 1 || settings.Applications.Roots[0] != filepath.Join(home, "Custom Apps") {
		t.Fatalf("unexpected application roots: %+v", settings.Applications.Roots)
	}
	if len(settings.Path.Directories) != 1 || settings.Path.Directories[0] != filepath.Join(home, "bin") {
		t.Fatalf("unexpected PATH directories: %+v", settings.Path.Directories)
	}
	if settings.Bun.Enabled {
		t.Fatal("expected Bun to be disabled")
	}
	if settings.RegistryPath != filepath.Join(home, "state", "registry.json") {
		t.Fatalf("unexpected registry path: %s", settings.RegistryPath)
	}
	if settings.ExecTimeout != 12*time.Second {
		t.Fatalf("unexpected timeout: %s", settings.ExecTimeout)
	}
}

func TestEnvironmentOverridesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[execution]\ntimeout = \"2s\"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("TOOLSNIFF_EXEC_TIMEOUT", "15s")

	settings, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if settings.ExecTimeout != 15*time.Second {
		t.Fatalf("expected environment override, got %s", settings.ExecTimeout)
	}
}

func TestInvalidEnvironmentTimeoutReturnsError(t *testing.T) {
	t.Setenv("TOOLSNIFF_EXEC_TIMEOUT", "not-a-duration")
	if _, err := Load(""); err == nil {
		t.Fatal("expected invalid environment timeout error")
	}
}

func TestLoadThemePresetAndColorOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	content := []byte("" +
		"[theme]\n" +
		"preset = \"midnight\"\n" +
		"[theme.colors]\n" +
		"selection_background = \"#ff00aa\"\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	settings, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if settings.Theme.Preset != "midnight" {
		t.Fatalf("expected midnight preset, got %q", settings.Theme.Preset)
	}
	if settings.Theme.Colors.SelectionBackground != "#ff00aa" {
		t.Fatalf("expected custom selection background, got %q", settings.Theme.Colors.SelectionBackground)
	}
	if settings.Theme.Colors.Accent != "#82aaff" {
		t.Fatalf("expected midnight accent to remain, got %q", settings.Theme.Colors.Accent)
	}
}

func TestLoadRejectsUnknownThemePreset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[theme]\npreset = \"unknown\"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected unknown theme preset error")
	}
}

func TestLoadRejectsInvalidThemeColor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[theme.colors]\naccent = \"not-a-color\"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected invalid theme color error")
	}
}

func TestSaveThemePreservesOtherConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[execution]\ntimeout = \"12s\"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	theme, err := ThemeSettingsForPreset("nord")
	if err != nil {
		t.Fatalf("get theme: %v", err)
	}
	if err := SaveTheme(path, theme); err != nil {
		t.Fatalf("save theme: %v", err)
	}
	settings, err := Load(path)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if settings.Theme.Preset != "nord" {
		t.Fatalf("expected nord theme, got %q", settings.Theme.Preset)
	}
	if settings.ExecTimeout != 12*time.Second {
		t.Fatalf("expected execution timeout to be preserved, got %s", settings.ExecTimeout)
	}
}
