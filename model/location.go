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
