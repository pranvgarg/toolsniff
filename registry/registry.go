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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("registry: creating directory: %w", err)
	}
	data, err := json.MarshalIndent(tools, "", "  ")
	if err != nil {
		return fmt.Errorf("registry: marshaling: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("registry: writing %s: %w", path, err)
	}
	return nil
}
