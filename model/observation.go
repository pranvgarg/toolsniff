package model

import (
	"fmt"
	"strings"
	"time"
)

type ObservationKind string

const (
	KindCLI         ObservationKind = "cli"
	KindPackage     ObservationKind = "package"
	KindApplication ObservationKind = "application"
	KindExecutable  ObservationKind = "executable"
	KindHistory     ObservationKind = "history"
)

func (k ObservationKind) IsValid() bool {
	switch k {
	case KindCLI, KindPackage, KindApplication, KindExecutable, KindHistory:
		return true
	default:
		return false
	}
}

type Observation struct {
	ID           string           `json:"id"`
	DisplayName  string           `json:"display_name"`
	CommandName  string           `json:"command_name,omitempty"`
	Kind         ObservationKind  `json:"kind"`
	Role         SourceRole       `json:"role"`
	Origin       Origin           `json:"origin"`
	Version      VersionInfo      `json:"version"`
	Locations    []Location       `json:"locations,omitempty"`
	Availability AvailabilityInfo `json:"availability,omitempty"`
	Package      *PackageInfo     `json:"package,omitempty"`
	Application  *ApplicationInfo `json:"application,omitempty"`
	History      *HistoryInfo     `json:"history,omitempty"`
	Evidence     []Evidence       `json:"evidence,omitempty"`
	FirstSeen    *time.Time       `json:"first_seen,omitempty"`
	LastSeen     *time.Time       `json:"last_seen,omitempty"`
}

func (r SourceRole) IsValid() bool {
	switch r {
	case RoleInstalled, RoleAvailable, RoleHistory:
		return true
	default:
		return false
	}
}

func (o Observation) Validate() error {
	if o.ID == "" {
		return fmt.Errorf("observation: id is required")
	}
	if o.DisplayName == "" {
		return fmt.Errorf("observation: display name is required")
	}
	if !o.Kind.IsValid() {
		return fmt.Errorf("observation: invalid kind %q", o.Kind)
	}
	if !o.Role.IsValid() {
		return fmt.Errorf("observation: invalid role %q", o.Role)
	}
	if err := o.Origin.Validate(); err != nil {
		return err
	}
	if err := o.Version.Validate(); err != nil {
		return err
	}
	if err := o.Availability.Validate(); err != nil {
		return err
	}
	for i, location := range o.Locations {
		if err := location.Validate(); err != nil {
			return fmt.Errorf("observation: location %d: %w", i, err)
		}
	}
	for i, evidence := range o.Evidence {
		if err := evidence.Validate(); err != nil {
			return fmt.Errorf("observation: evidence %d: %w", i, err)
		}
	}
	if o.Kind == KindHistory && o.Role != RoleHistory {
		return fmt.Errorf("observation: history kind requires history role")
	}
	if o.Role == RoleHistory && o.Kind != KindHistory {
		return fmt.Errorf("observation: history role requires history kind")
	}
	if o.Kind == KindHistory && o.History == nil {
		return fmt.Errorf("observation: history kind requires history metadata")
	}
	if o.Role == RoleInstalled && o.Kind == KindHistory {
		return fmt.Errorf("observation: history cannot be installed")
	}
	if o.FirstSeen != nil && o.LastSeen != nil && o.LastSeen.Before(*o.FirstSeen) {
		return fmt.Errorf("observation: last seen precedes first seen")
	}
	return nil
}

// IdentityComponents are the stable parts of an observation identity. A
// location is included only for an executable with no stronger identity.
type IdentityComponents struct {
	Provider string
	Manager  string
	Package  string
	BundleID string
	Path     string
	Name     string
}

func (o Observation) IdentityComponents() IdentityComponents {
	components := IdentityComponents{
		Provider: o.Origin.Provider,
		Manager:  o.Origin.Manager,
		Name:     o.DisplayName,
	}
	if o.Origin.Package != "" {
		components.Package = o.Origin.Package
	} else if o.Package != nil {
		components.Package = o.Package.Name
	}
	if o.Application != nil {
		components.BundleID = o.Application.BundleID
	}
	if components.Package != "" || components.BundleID != "" {
		return components
	}
	if o.Kind == KindExecutable && (o.Origin.Provider == "unknown" || o.Origin.Manager == "manual-or-unknown") {
		for _, location := range o.Locations {
			if location.Type == LocationExecutable {
				components.Path = location.Path
				break
			}
		}
	}
	return components
}

// Key returns a stable, unambiguous key for the selected identity components.
func (c IdentityComponents) Key() string {
	switch {
	case c.Package != "":
		return strings.Join([]string{"package", c.Provider, c.Manager, c.Package}, "\x00")
	case c.BundleID != "":
		return strings.Join([]string{"application", c.BundleID}, "\x00")
	case c.Path != "":
		return strings.Join([]string{"path", c.Path}, "\x00")
	default:
		return strings.Join([]string{"name", c.Provider, c.Manager, c.Name}, "\x00")
	}
}

func ObservationIdentity(o Observation) string { return o.IdentityComponents().Key() }

// ObservationFromTool adapts the legacy flat model without changing Tool or
// any existing scanner, registry, or renderer APIs.
func ObservationFromTool(tool Tool) Observation {
	role := tool.Role
	if !role.IsValid() {
		switch tool.Source {
		case SourceNPXHistory:
			role = RoleHistory
		case SourcePath:
			role = RoleAvailable
		default:
			role = RoleInstalled
		}
	}

	kind := legacyKind(tool.Source)
	origin := legacyOrigin(tool)
	version := VersionInfo{
		State:       VersionNotReported,
		Scheme:      SchemeUnknown,
		Confidence:  ConfidenceLow,
		RetrievedBy: "legacy",
	}
	if tool.Source == SourceNPXHistory {
		version.State = VersionNotApplicable
	} else if tool.Version != "" {
		version.State = VersionKnown
		version.Value = tool.Version
		version.Confidence = ConfidenceMedium
	} else if kind == KindExecutable {
		version.State = VersionUnknown
	}

	observation := Observation{
		DisplayName:  tool.Name,
		CommandName:  tool.Name,
		Kind:         kind,
		Role:         role,
		Origin:       origin,
		Version:      version,
		Availability: legacyAvailability(tool, role),
	}
	if tool.Path != "" {
		observation.Locations = []Location{legacyLocation(tool)}
	}
	if kind == KindPackage || kind == KindHistory {
		observation.Package = &PackageInfo{Name: tool.Name}
		if kind == KindPackage {
			observation.Package.Version = tool.Version
		}
	}
	if kind == KindHistory {
		history := &HistoryInfo{CachePath: tool.Path}
		if lastUsed, err := time.Parse("2006-01-02", tool.Version); err == nil {
			history.LastUsed = &lastUsed
		}
		observation.History = history
	}
	observation.ID = ObservationIdentity(observation)
	return observation
}

// ToObservation is the method form of the explicit legacy adapter.
func (tool Tool) ToObservation() Observation { return ObservationFromTool(tool) }

func legacyKind(source string) ObservationKind {
	switch source {
	case SourceNPXHistory:
		return KindHistory
	case SourceApplications:
		return KindApplication
	case SourcePath, SourceCargo, SourceBun:
		return KindExecutable
	default:
		return KindPackage
	}
}

func legacyOrigin(tool Tool) Origin {
	origin := Origin{Provider: tool.Source}
	switch tool.Source {
	case SourceBrewFormula:
		origin = Origin{Provider: "homebrew", Manager: "formula", Package: tool.Name}
	case SourceBrewCask:
		origin = Origin{Provider: "homebrew", Manager: "cask", Package: tool.Name}
	case SourceNPM:
		origin = Origin{Provider: "npm", Manager: "global", Package: tool.Name}
	case SourcePipx:
		origin = Origin{Provider: "pipx", Manager: "global", Package: tool.Name}
	case SourceNPXHistory:
		origin = Origin{Provider: "npm", Manager: "npx", Package: tool.Name}
	case SourcePath:
		origin = Origin{Provider: "unknown", Manager: "manual-or-unknown"}
	case SourceApplications:
		origin = Origin{Provider: "applications"}
	case SourceCargo:
		origin = Origin{Provider: "cargo", Manager: "install"}
	case SourceBun:
		origin = Origin{Provider: "bun", Manager: "global"}
	}
	return origin
}

func legacyLocation(tool Tool) Location {
	location := Location{Path: tool.Path, Type: LocationPackagePrefix}
	switch tool.Source {
	case SourceApplications:
		location.Type = LocationApplication
	case SourcePath, SourceCargo, SourceBun:
		location.Type = LocationExecutable
		location.Executable = true
	case SourceNPXHistory:
		location.Type = LocationCache
	}
	return location
}

func legacyAvailability(tool Tool, role SourceRole) AvailabilityInfo {
	if role == RoleAvailable || tool.Source == SourcePath {
		return AvailabilityInfo{State: AvailabilityAvailable, ResolvedPath: tool.Path}
	}
	return AvailabilityInfo{State: AvailabilityUnknown}
}
