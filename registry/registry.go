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
	observations, warning := LoadObservations(path)
	if warning != "" {
		return nil, warning
	}
	return toolsFromObservations(observations), ""
}

// LoadObservations reads either a v2 envelope or a legacy v1 tool array. A
// missing file is not an error; malformed and unsupported files return a
// warning and an empty baseline without modifying the file.
func LoadObservations(path string) (observations []model.Observation, warning string) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ""
		}
		return nil, fmt.Sprintf("registry: reading %s: %v (treating as empty baseline)", path, err)
	}

	observations, err = decodeRegistry(data)
	if err != nil {
		return nil, fmt.Sprintf("registry: parsing %s: %v (treating as empty baseline)", path, err)
	}
	return observations, ""
}

// Save writes the current scan as the new baseline, creating the parent
// directory if needed.
func Save(path string, tools []model.Tool) error {
	return saveObservations(path, observationsFromTools(tools), false)
}

// SaveObservations writes a v2 registry envelope atomically. The input is
// copied before canonical sorting so callers retain ownership of their slice.
func SaveObservations(path string, observations []model.Observation) error {
	return saveObservations(path, observations, true)
}

func saveObservations(path string, observations []model.Observation, canonicalize bool) error {
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
	canonical := append([]model.Observation(nil), observations...)
	if canonical == nil {
		canonical = []model.Observation{}
	}
	for i, observation := range canonical {
		if err := observation.Validate(); err != nil {
			return fmt.Errorf("registry: validating observation %d: %w", i, err)
		}
	}
	if canonicalize {
		sortObservations(canonical)
	}
	data, err := json.MarshalIndent(Envelope{
		SchemaVersion: CurrentSchemaVersion,
		Observations:  canonical,
	}, "", "  ")
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
