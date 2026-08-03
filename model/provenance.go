package model

import (
	"fmt"
	"time"
)

type Origin struct {
	Provider string `json:"provider"`
	Manager  string `json:"manager,omitempty"`
	Package  string `json:"package,omitempty"`
	Manifest string `json:"manifest,omitempty"`
}

func (o Origin) Validate() error {
	if o.Provider == "" {
		return fmt.Errorf("origin: provider is required")
	}
	return nil
}

type PackageInfo struct {
	Name            string   `json:"name,omitempty"`
	Version         string   `json:"version,omitempty"`
	Executables     []string `json:"executables,omitempty"`
	Prefix          string   `json:"prefix,omitempty"`
	VirtualEnv      string   `json:"virtual_env,omitempty"`
	DependencyCount int      `json:"dependency_count,omitempty"`
}

type ApplicationInfo struct {
	BundleID       string   `json:"bundle_id,omitempty"`
	DisplayVersion string   `json:"display_version,omitempty"`
	ShortVersion   string   `json:"short_version,omitempty"`
	MinimumOS      string   `json:"minimum_os,omitempty"`
	Architectures  []string `json:"architectures,omitempty"`
	Signed         *bool    `json:"signed,omitempty"`
	SigningTeamID  string   `json:"signing_team_id,omitempty"`
}

type HistoryInfo struct {
	LastUsed  *time.Time `json:"last_used,omitempty"`
	CachePath string     `json:"cache_path,omitempty"`
}

// ProbeResult is deliberately small. Probe output is parsed into VersionInfo;
// raw command output does not belong in an observation.
type ProbeResult struct {
	Version     string     `json:"version,omitempty"`
	Error       string     `json:"error,omitempty"`
	RetrievedAt *time.Time `json:"retrieved_at,omitempty"`
}

type AvailabilityState string

const (
	AvailabilityAvailable   AvailabilityState = "available"
	AvailabilityUnavailable AvailabilityState = "unavailable"
	AvailabilityUnknown     AvailabilityState = "unknown"
)

func (s AvailabilityState) IsValid() bool {
	switch s {
	case AvailabilityAvailable, AvailabilityUnavailable, AvailabilityUnknown:
		return true
	default:
		return false
	}
}

type AvailabilityInfo struct {
	State        AvailabilityState `json:"state"`
	ResolvedPath string            `json:"resolved_path,omitempty"`
	PATHIndex    int               `json:"path_index,omitempty"`
	ShadowedBy   []string          `json:"shadowed_by,omitempty"`
	Probe        *ProbeResult      `json:"probe,omitempty"`
}

func (a AvailabilityInfo) Validate() error {
	if !a.State.IsValid() {
		return fmt.Errorf("availability: invalid state %q", a.State)
	}
	return nil
}

type Evidence struct {
	Type        string     `json:"type"`
	Source      string     `json:"source"`
	Description string     `json:"description,omitempty"`
	RetrievedAt *time.Time `json:"retrieved_at,omitempty"`
}

func (e Evidence) Validate() error {
	if e.Type == "" {
		return fmt.Errorf("evidence: type is required")
	}
	if e.Source == "" {
		return fmt.Errorf("evidence: source is required")
	}
	return nil
}
