package model

import (
	"fmt"
	"time"
)

type LocationType string

const (
	LocationExecutable    LocationType = "executable"
	LocationApplication   LocationType = "application-bundle"
	LocationPackagePrefix LocationType = "package-prefix"
	LocationVirtualEnv    LocationType = "virtual-environment"
	LocationCache         LocationType = "cache"
)

func (t LocationType) IsValid() bool {
	switch t {
	case LocationExecutable, LocationApplication, LocationPackagePrefix, LocationVirtualEnv, LocationCache:
		return true
	default:
		return false
	}
}

type Location struct {
	Path          string       `json:"path"`
	Type          LocationType `json:"type"`
	IsSymlink     bool         `json:"is_symlink,omitempty"`
	SymlinkTarget string       `json:"symlink_target,omitempty"`
	Executable    bool         `json:"executable,omitempty"`
	ModifiedAt    *time.Time   `json:"modified_at,omitempty"`
	Architectures []string     `json:"architectures,omitempty"`
	SHA256        string       `json:"sha256,omitempty"`
	// SizeBytes is computed on demand (see output.DirectorySize), never
	// persisted to the registry baseline, and zero when not yet measured.
	// Shared field name/type with docs/mole/integration-plan.md's P-A --
	// both efforts read the same primitive.
	SizeBytes int64 `json:"size_bytes,omitempty"`
}

func (l Location) Validate() error {
	if l.Path == "" {
		return fmt.Errorf("location: path is required")
	}
	if !l.Type.IsValid() {
		return fmt.Errorf("location: invalid type %q", l.Type)
	}
	if l.SymlinkTarget != "" && !l.IsSymlink {
		return fmt.Errorf("location: symlink target requires is_symlink")
	}
	return nil
}
