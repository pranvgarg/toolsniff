package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pranvgarg/toolsniff/model"
)

// DefaultRegistryPath returns ~/.toolsniff/registry.json.
func DefaultRegistryPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".toolsniff", "registry.json")
}

// AvailabilityPath returns the sibling registry used for PATH availability
// observations. Keeping it separate prevents command availability from being
// treated as proof of installation in the installed baseline.
func AvailabilityPath(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(path), "availability.json")
}

// Load reads the saved baseline. A missing file is not an error — it just
// means there's no baseline yet, so every real install will show as new. A
// corrupt file is treated the same way, but with a warning explaining why.
func Load(path string) (tools []model.Tool, warning string) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ""
		}
		return nil, fmt.Sprintf("registry: reading %s: %v (treating as empty baseline)", path, err)
	}

	if err := json.Unmarshal(data, &tools); err != nil {
		return nil, fmt.Sprintf("registry: parsing %s: %v (treating as empty baseline)", path, err)
	}
	return tools, ""
}

// Save writes the current scan as the new baseline, creating the parent
// directory if needed.
func Save(path string, tools []model.Tool) error {
	if path == "" {
		return fmt.Errorf("registry: empty path")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("registry: creating directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("registry: securing directory: %w", err)
	}
	data, err := json.MarshalIndent(tools, "", "  ")
	if err != nil {
		return fmt.Errorf("registry: marshaling: %w", err)
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(dir, ".registry-*.tmp")
	if err != nil {
		return fmt.Errorf("registry: creating temporary file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("registry: securing temporary file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("registry: writing temporary file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("registry: syncing temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("registry: closing temporary file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("registry: replacing %s: %w", path, err)
	}
	return nil
}
