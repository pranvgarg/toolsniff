package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// Settings is the fully resolved runtime configuration used to construct
// scanners and persistence services.
type Settings struct {
	Applications ApplicationSettings
	Path         PathSettings
	Bun          BunSettings
	Theme        ThemeSettings
	NPXDir       string
	CargoBinDir  string
	RegistryPath string
	ExecTimeout  time.Duration
	ConfigPath   string
}

type ApplicationSettings struct {
	Roots      []string
	IgnorePath []string
}

type PathSettings struct {
	Directories []string
	Excluded    []string
	IgnoreNames []string
}

type BunSettings struct {
	Enabled bool
}

// ThemeSettings contains the selected preset and any user color overrides.
// Output packages translate these semantic colors into Lip Gloss styles.
type ThemeSettings struct {
	Preset string
	Colors ThemeColors
}

type ThemeColors struct {
	Accent              string `toml:"accent"`
	Secondary           string `toml:"secondary"`
	Muted               string `toml:"muted"`
	Border              string `toml:"border"`
	Text                string `toml:"text"`
	SelectionForeground string `toml:"selection_foreground"`
	SelectionBackground string `toml:"selection_background"`
	Warning             string `toml:"warning"`
	WarningBackground   string `toml:"warning_background"`
	Error               string `toml:"error"`
	Success             string `toml:"success"`
}

// fileConfig is intentionally separate from Settings so omitted TOML values
// do not erase platform-aware defaults.
type fileConfig struct {
	Applications struct {
		Roots      []string `toml:"roots"`
		IgnorePath []string `toml:"ignore_paths"`
	} `toml:"applications"`
	Path struct {
		Directories []string `toml:"directories"`
		Excluded    []string `toml:"exclude_directories"`
		IgnoreNames []string `toml:"ignore_names"`
	} `toml:"path"`
	Bun struct {
		Enabled *bool `toml:"enabled"`
	} `toml:"bun"`
	Theme struct {
		Preset string      `toml:"preset"`
		Colors ThemeColors `toml:"colors"`
	} `toml:"theme"`
	NPX struct {
		Dir string `toml:"dir"`
	} `toml:"npx"`
	Cargo struct {
		BinDir string `toml:"bin_dir"`
	} `toml:"cargo"`
	Registry struct {
		Path string `toml:"path"`
	} `toml:"registry"`
	Execution struct {
		Timeout string `toml:"timeout"`
	} `toml:"execution"`
}

// DefaultConfigPath returns the user-editable configuration path. The
// TOOLSNIFF_CONFIG environment variable is useful for profiles and tests.
func DefaultConfigPath() string {
	if path := os.Getenv("TOOLSNIFF_CONFIG"); path != "" {
		return expandPath(path)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "toolsniff", "config.toml")
}

// DefaultSettings returns platform-aware defaults without requiring a config
// file. Defaults are discovery policy, not a list of known tool names.
func DefaultSettings() Settings {
	home, _ := os.UserHomeDir()
	applications := []string{"/Applications"}
	if home != "" {
		applications = append(applications, filepath.Join(home, "Applications"))
	}

	pathDirs := filepath.SplitList(os.Getenv("PATH"))
	pathExcluded := []string{"/bin", "/sbin", "/usr/bin", "/usr/sbin"}
	if runtime.GOOS == "darwin" {
		pathExcluded = append(pathExcluded, "/System/Cryptexes/App/usr/bin")
	}

	return Settings{
		Applications: ApplicationSettings{Roots: uniquePaths(applications)},
		Path: PathSettings{
			Directories: uniquePaths(pathDirs),
			Excluded:    uniquePaths(pathExcluded),
		},
		Bun:          BunSettings{Enabled: true},
		Theme:        DefaultThemeSettings(),
		NPXDir:       defaultNPXDir(),
		CargoBinDir:  defaultCargoBinDir(),
		RegistryPath: defaultRegistryPath(),
		ExecTimeout:  8 * time.Second,
	}
}

// Load resolves a configuration file over the platform-aware defaults. A
// missing file is expected and returns defaults; malformed values are errors.
func Load(path string) (Settings, error) {
	settings := DefaultSettings()
	settings.ConfigPath = path
	if path == "" {
		return applyEnvironment(settings)
	}

	data, err := os.ReadFile(expandPath(path))
	if err != nil {
		if os.IsNotExist(err) {
			return applyEnvironment(settings)
		}
		return Settings{}, fmt.Errorf("config: reading %s: %w", path, err)
	}

	var file fileConfig
	if err := toml.Unmarshal(data, &file); err != nil {
		return Settings{}, fmt.Errorf("config: parsing %s: %w", path, err)
	}
	if err := applyFileConfig(&settings, file); err != nil {
		return Settings{}, err
	}
	return applyEnvironment(settings)
}

func applyFileConfig(settings *Settings, file fileConfig) error {
	if len(file.Applications.Roots) > 0 {
		settings.Applications.Roots = uniquePaths(file.Applications.Roots)
	}
	settings.Applications.IgnorePath = uniquePaths(file.Applications.IgnorePath)
	if len(file.Path.Directories) > 0 {
		settings.Path.Directories = uniquePaths(file.Path.Directories)
	}
	if len(file.Path.Excluded) > 0 {
		settings.Path.Excluded = uniquePaths(append(settings.Path.Excluded, file.Path.Excluded...))
	}
	settings.Path.IgnoreNames = uniqueNames(file.Path.IgnoreNames)
	if file.Bun.Enabled != nil {
		settings.Bun.Enabled = *file.Bun.Enabled
	}
	if err := applyThemeConfig(&settings.Theme, file.Theme.Preset, file.Theme.Colors); err != nil {
		return err
	}
	if file.NPX.Dir != "" {
		settings.NPXDir = expandPath(file.NPX.Dir)
	}
	if file.Cargo.BinDir != "" {
		settings.CargoBinDir = expandPath(file.Cargo.BinDir)
	}
	if file.Registry.Path != "" {
		settings.RegistryPath = expandPath(file.Registry.Path)
	}
	if file.Execution.Timeout != "" {
		timeout, err := time.ParseDuration(file.Execution.Timeout)
		if err != nil || timeout <= 0 {
			return fmt.Errorf("config: invalid execution.timeout %q", file.Execution.Timeout)
		}
		settings.ExecTimeout = timeout
	}
	return nil
}

func applyEnvironment(settings Settings) (Settings, error) {
	if value := os.Getenv("TOOLSNIFF_THEME"); value != "" {
		if err := applyThemeConfig(&settings.Theme, value, ThemeColors{}); err != nil {
			return Settings{}, err
		}
	}
	if value := os.Getenv("TOOLSNIFF_APPLICATION_ROOTS"); value != "" {
		settings.Applications.Roots = uniquePaths(filepath.SplitList(value))
	}
	if value := os.Getenv("TOOLSNIFF_PATH_DIRECTORIES"); value != "" {
		settings.Path.Directories = uniquePaths(filepath.SplitList(value))
	}
	if value := os.Getenv("TOOLSNIFF_PATH_EXCLUDE"); value != "" {
		settings.Path.Excluded = uniquePaths(append(settings.Path.Excluded, filepath.SplitList(value)...))
	}
	if value := os.Getenv("TOOLSNIFF_REGISTRY"); value != "" {
		settings.RegistryPath = expandPath(value)
	}
	if value := os.Getenv("TOOLSNIFF_NPX_DIR"); value != "" {
		settings.NPXDir = expandPath(value)
	}
	if value := os.Getenv("TOOLSNIFF_CARGO_BIN_DIR"); value != "" {
		settings.CargoBinDir = expandPath(value)
	}
	if value := os.Getenv("TOOLSNIFF_EXEC_TIMEOUT"); value != "" {
		timeout, err := time.ParseDuration(value)
		if err != nil || timeout <= 0 {
			return Settings{}, fmt.Errorf("config: invalid TOOLSNIFF_EXEC_TIMEOUT %q", value)
		}
		settings.ExecTimeout = timeout
	}
	return settings, nil
}

// DefaultThemeSettings returns the original toolsniff palette.
func DefaultThemeSettings() ThemeSettings {
	colors, _ := presetColors("toolsniff")
	return ThemeSettings{Preset: "toolsniff", Colors: colors}
}

// ThemePresets returns the names users can select in configuration.
func ThemePresets() []string {
	return []string{"toolsniff", "midnight", "nord", "mono", "high-contrast"}
}

// ThemeSettingsForPreset returns a complete theme configuration for a named
// built-in preset.
func ThemeSettingsForPreset(name string) (ThemeSettings, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	colors, ok := presetColors(name)
	if !ok {
		return ThemeSettings{}, fmt.Errorf("config: unknown theme preset %q", name)
	}
	return ThemeSettings{Preset: name, Colors: colors}, nil
}

// SaveTheme updates the theme section of the TOML configuration. The rest of
// the decoded configuration is preserved, and the write is atomic.
func SaveTheme(path string, theme ThemeSettings) error {
	if path == "" {
		return fmt.Errorf("config: empty config path")
	}
	path = expandPath(path)
	raw := make(map[string]any)
	if data, err := os.ReadFile(path); err == nil {
		if err := toml.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("config: parsing %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("config: reading %s: %w", path, err)
	}
	raw["theme"] = map[string]any{
		"preset": theme.Preset,
		"colors": map[string]any{
			"accent":               theme.Colors.Accent,
			"secondary":            theme.Colors.Secondary,
			"muted":                theme.Colors.Muted,
			"border":               theme.Colors.Border,
			"text":                 theme.Colors.Text,
			"selection_foreground": theme.Colors.SelectionForeground,
			"selection_background": theme.Colors.SelectionBackground,
			"warning":              theme.Colors.Warning,
			"warning_background":   theme.Colors.WarningBackground,
			"error":                theme.Colors.Error,
			"success":              theme.Colors.Success,
		},
	}
	data, err := toml.Marshal(raw)
	if err != nil {
		return fmt.Errorf("config: serializing %s: %w", path, err)
	}
	return writeConfigAtomically(path, data)
}

func writeConfigAtomically(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("config: creating directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("config: creating temporary file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("config: securing temporary file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("config: writing temporary file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("config: syncing temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("config: closing temporary file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("config: replacing %s: %w", path, err)
	}
	return nil
}

func applyThemeConfig(settings *ThemeSettings, preset string, overrides ThemeColors) error {
	if preset != "" {
		colors, ok := presetColors(strings.ToLower(strings.TrimSpace(preset)))
		if !ok {
			return fmt.Errorf("config: unknown theme preset %q", preset)
		}
		settings.Preset = strings.ToLower(strings.TrimSpace(preset))
		settings.Colors = colors
	}
	mergeThemeColors(&settings.Colors, overrides)
	if err := validateThemeColors(settings.Colors); err != nil {
		return err
	}
	return nil
}

func presetColors(name string) (ThemeColors, bool) {
	palette := map[string]ThemeColors{
		"toolsniff": {
			Accent:              "#ffb454",
			Secondary:           "#7fd8c4",
			Muted:               "#5c6577",
			Border:              "#384152",
			Text:                "#e6edf3",
			SelectionForeground: "#081018",
			SelectionBackground: "#7fd8c4",
			Warning:             "#ffb454",
			WarningBackground:   "#4a3215",
			Error:               "#ff6b6b",
			Success:             "#7ee787",
		},
		"midnight": {
			Accent:              "#82aaff",
			Secondary:           "#c792ea",
			Muted:               "#64748b",
			Border:              "#263449",
			Text:                "#e5e7eb",
			SelectionForeground: "#07111f",
			SelectionBackground: "#82aaff",
			Warning:             "#f6c177",
			WarningBackground:   "#493923",
			Error:               "#f7768e",
			Success:             "#9ece6a",
		},
		"nord": {
			Accent:              "#88c0d0",
			Secondary:           "#81a1c1",
			Muted:               "#616e88",
			Border:              "#3b4252",
			Text:                "#eceff4",
			SelectionForeground: "#2e3440",
			SelectionBackground: "#88c0d0",
			Warning:             "#ebcb8b",
			WarningBackground:   "#4c4635",
			Error:               "#bf616a",
			Success:             "#a3be8c",
		},
		"mono": {
			Accent:              "#ffffff",
			Secondary:           "#d0d0d0",
			Muted:               "#808080",
			Border:              "#606060",
			Text:                "#ffffff",
			SelectionForeground: "#000000",
			SelectionBackground: "#ffffff",
			Warning:             "#ffffff",
			WarningBackground:   "#404040",
			Error:               "#ffffff",
			Success:             "#ffffff",
		},
		"high-contrast": {
			Accent:              "#ffff00",
			Secondary:           "#00ffff",
			Muted:               "#c0c0c0",
			Border:              "#ffffff",
			Text:                "#ffffff",
			SelectionForeground: "#000000",
			SelectionBackground: "#00ffff",
			Warning:             "#ffff00",
			WarningBackground:   "#400000",
			Error:               "#ff8080",
			Success:             "#00ff00",
		},
	}
	colors, ok := palette[name]
	return colors, ok
}

func mergeThemeColors(base *ThemeColors, overrides ThemeColors) {
	if overrides.Accent != "" {
		base.Accent = overrides.Accent
	}
	if overrides.Secondary != "" {
		base.Secondary = overrides.Secondary
	}
	if overrides.Muted != "" {
		base.Muted = overrides.Muted
	}
	if overrides.Border != "" {
		base.Border = overrides.Border
	}
	if overrides.Text != "" {
		base.Text = overrides.Text
	}
	if overrides.SelectionForeground != "" {
		base.SelectionForeground = overrides.SelectionForeground
	}
	if overrides.SelectionBackground != "" {
		base.SelectionBackground = overrides.SelectionBackground
	}
	if overrides.Warning != "" {
		base.Warning = overrides.Warning
	}
	if overrides.WarningBackground != "" {
		base.WarningBackground = overrides.WarningBackground
	}
	if overrides.Error != "" {
		base.Error = overrides.Error
	}
	if overrides.Success != "" {
		base.Success = overrides.Success
	}
}

func validateThemeColors(colors ThemeColors) error {
	values := map[string]string{
		"accent": colors.Accent, "secondary": colors.Secondary, "muted": colors.Muted,
		"border": colors.Border, "text": colors.Text, "selection_foreground": colors.SelectionForeground,
		"selection_background": colors.SelectionBackground, "warning": colors.Warning,
		"warning_background": colors.WarningBackground, "error": colors.Error, "success": colors.Success,
	}
	for name, value := range values {
		if !isHexColor(value) {
			return fmt.Errorf("config: theme color %s must be a #RRGGBB value, got %q", name, value)
		}
	}
	return nil
}

func isHexColor(value string) bool {
	if len(value) != 7 || value[0] != '#' {
		return false
	}
	for _, r := range value[1:] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

func defaultRegistryPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".toolsniff", "registry.json")
}

func defaultNPXDir() string {
	home, _ := os.UserHomeDir()
	if cache := os.Getenv("NPM_CONFIG_CACHE"); cache != "" {
		return filepath.Join(expandPath(cache), "_npx")
	}
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".npm", "_npx")
}

func defaultCargoBinDir() string {
	home, _ := os.UserHomeDir()
	if cargoHome := os.Getenv("CARGO_HOME"); cargoHome != "" {
		return filepath.Join(expandPath(cargoHome), "bin")
	}
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".cargo", "bin")
}

func expandPath(path string) string {
	if path == "~" {
		home, _ := os.UserHomeDir()
		if home != "" {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		if home != "" {
			path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return filepath.Clean(path)
}

func uniquePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	unique := make([]string, 0, len(paths))
	for _, path := range paths {
		path = expandPath(strings.TrimSpace(path))
		if path == "" || path == "." {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		unique = append(unique, path)
	}
	return unique
}

func uniqueNames(names []string) []string {
	seen := make(map[string]struct{}, len(names))
	unique := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		unique = append(unique, name)
	}
	return unique
}
