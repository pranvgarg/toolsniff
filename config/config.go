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
