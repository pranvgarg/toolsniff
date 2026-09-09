// Package profile provides additive, machine-readable profile and support
// bundle contracts without coupling them to CLI or renderer behavior.
package profile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/pranvgarg/toolsniff/model"
	"github.com/pranvgarg/toolsniff/output"
	"github.com/pranvgarg/toolsniff/registry"
)

// SchemaVersion is the version of the profile JSON contract.
const SchemaVersion = 1

// HomeRedaction is used for paths that begin in the exporting user's home.
const HomeRedaction = "$HOME"

// Profile is a portable representation of observations and their optional
// presentation report. Version is the producer version; SchemaVersion is the
// JSON contract version.
type Profile struct {
	SchemaVersion int                       `json:"schema_version"`
	Version       string                    `json:"version,omitempty"`
	CreatedAt     time.Time                 `json:"created_at"`
	Observations  []model.Observation       `json:"observations"`
	Report        *output.ObservationReport `json:"report,omitempty"`
}

// SupportBundle is deliberately narrower than a profile. It contains only
// sanitized observations and a sanitized report, never raw command output.
type SupportBundle struct {
	SchemaVersion int                       `json:"schema_version"`
	Version       string                    `json:"version,omitempty"`
	CreatedAt     time.Time                 `json:"created_at"`
	Observations  []model.Observation       `json:"observations"`
	Report        *output.ObservationReport `json:"report,omitempty"`
}

// New creates a profile and copies its observation slice. A nil slice becomes
// an empty JSON array to make round trips unambiguous.
func New(observations []model.Observation, report *output.ObservationReport) Profile {
	result := Profile{
		SchemaVersion: SchemaVersion,
		CreatedAt:     time.Now().UTC(),
		Observations:  append([]model.Observation(nil), observations...),
		Report:        report,
	}
	if result.Observations == nil {
		result.Observations = []model.Observation{}
	}
	return canonicalProfile(result)
}

// NewProfile is a descriptive constructor alias.
func NewProfile(observations []model.Observation, report *output.ObservationReport) Profile {
	return New(observations, report)
}

// Marshal serializes a validated profile with stable observation ordering.
func Marshal(value Profile) ([]byte, error) {
	value = canonicalProfile(value)
	if err := validateProfile(value); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("profile: marshaling: %w", err)
	}
	return append(data, '\n'), nil
}

// MarshalProfile is an explicit alias for callers that prefer type-oriented
// function names.
func MarshalProfile(value Profile) ([]byte, error) { return Marshal(value) }

// Unmarshal parses and validates a profile without accepting trailing JSON or
// unknown top-level fields.
func Unmarshal(data []byte) (Profile, error) {
	var value Profile
	if err := decodeStrict(data, &value); err != nil {
		return Profile{}, fmt.Errorf("profile: parsing: %w", err)
	}
	if err := validateProfile(value); err != nil {
		return Profile{}, err
	}
	return canonicalProfile(value), nil
}

// UnmarshalProfile is an explicit alias for callers that prefer type-oriented
// function names.
func UnmarshalProfile(data []byte) (Profile, error) { return Unmarshal(data) }

// Compare returns deterministic additions, removals, and updates between two
// profiles. If a profile was created with only a report, the report's complete
// observation set is used as its comparison input.
func Compare(before, after Profile) registry.ObservationDiff {
	return registry.ComputeObservationDiff(profileObservations(before), profileObservations(after))
}

// CompareProfiles is the descriptive comparison alias.
func CompareProfiles(before, after Profile) registry.ObservationDiff {
	return Compare(before, after)
}

// Sanitize removes diagnostic payloads that can contain command output and
// redacts home paths before creating a support bundle.
func Sanitize(value Profile) Profile {
	value = canonicalProfile(value)
	result := Profile{
		SchemaVersion: value.SchemaVersion,
		Version:       sanitizeText(value.Version),
		CreatedAt:     value.CreatedAt,
		Observations:  sanitizeObservations(value.Observations),
	}
	if value.Report != nil {
		report := sanitizeReport(*value.Report)
		result.Report = &report
	}
	return result
}

// CreateSupportBundle returns a deterministic, privacy-safe JSON bundle.
func CreateSupportBundle(value Profile) ([]byte, error) {
	sanitized := Sanitize(value)
	bundle := SupportBundle{
		SchemaVersion: SchemaVersion,
		Version:       sanitized.Version,
		CreatedAt:     sanitized.CreatedAt,
		Observations:  sanitized.Observations,
		Report:        sanitized.Report,
	}
	if bundle.CreatedAt.IsZero() {
		bundle.CreatedAt = time.Now().UTC()
	}
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("profile: marshaling support bundle: %w", err)
	}
	return append(data, '\n'), nil
}

// MarshalSupportBundle is an alias for CreateSupportBundle.
func MarshalSupportBundle(value Profile) ([]byte, error) {
	return CreateSupportBundle(value)
}

// WriteSupportBundle atomically writes a support bundle to path. It is useful
// to library callers while keeping file output out of the CLI layer.
func WriteSupportBundle(path string, value Profile) error {
	if path == "" {
		return fmt.Errorf("profile: empty support bundle path")
	}
	data, err := CreateSupportBundle(value)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("profile: creating support bundle directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".support-bundle-*.tmp")
	if err != nil {
		return fmt.Errorf("profile: creating support bundle temporary file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("profile: securing support bundle temporary file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("profile: writing support bundle temporary file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("profile: syncing support bundle temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("profile: closing support bundle temporary file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("profile: replacing support bundle %s: %w", path, err)
	}
	return nil
}

func validateProfile(value Profile) error {
	if value.SchemaVersion != SchemaVersion {
		return fmt.Errorf("profile: unsupported schema version %d", value.SchemaVersion)
	}
	if value.CreatedAt.IsZero() {
		return fmt.Errorf("profile: created_at is required")
	}
	if value.Observations == nil {
		return fmt.Errorf("profile: observations must be an array")
	}
	for i, observation := range value.Observations {
		if err := observation.Validate(); err != nil {
			return fmt.Errorf("profile: observation %d: %w", i, err)
		}
	}
	return nil
}

func canonicalProfile(value Profile) Profile {
	if value.SchemaVersion == 0 {
		value.SchemaVersion = SchemaVersion
	}
	if value.Observations == nil {
		value.Observations = []model.Observation{}
	} else {
		value.Observations = append([]model.Observation(nil), value.Observations...)
	}
	sort.SliceStable(value.Observations, func(i, j int) bool {
		left := model.ObservationIdentity(value.Observations[i])
		right := model.ObservationIdentity(value.Observations[j])
		if left != right {
			return left < right
		}
		leftJSON, _ := json.Marshal(value.Observations[i])
		rightJSON, _ := json.Marshal(value.Observations[j])
		return string(leftJSON) < string(rightJSON)
	})
	return value
}

func profileObservations(value Profile) []model.Observation {
	if value.Observations != nil {
		return value.Observations
	}
	if value.Report == nil {
		return nil
	}
	return value.Report.AllObservations()
}

func sanitizeObservations(observations []model.Observation) []model.Observation {
	result := make([]model.Observation, 0, len(observations))
	for _, observation := range observations {
		result = append(result, sanitizeObservation(observation))
	}
	return canonicalProfile(Profile{Observations: result}).Observations
}

func sanitizeObservation(value model.Observation) model.Observation {
	value.DisplayName = sanitizeText(value.DisplayName)
	value.CommandName = sanitizeText(value.CommandName)
	value.Origin.Provider = sanitizeText(value.Origin.Provider)
	value.Origin.Manager = sanitizeText(value.Origin.Manager)
	value.Origin.Package = sanitizeText(value.Origin.Package)
	value.Origin.Manifest = sanitizePath(value.Origin.Manifest)
	value.Version.Value = sanitizeText(value.Version.Value)
	value.Version.RetrievedBy = sanitizeText(value.Version.RetrievedBy)
	value.Availability.ResolvedPath = sanitizePath(value.Availability.ResolvedPath)
	value.Availability.ShadowedBy = sanitizePaths(value.Availability.ShadowedBy)
	// Probe errors and evidence descriptions are diagnostic output, not support
	// bundle data. Omitting them is safer than trying to recognize every secret.
	value.Availability.Probe = nil
	value.Evidence = nil
	if value.Package != nil {
		packageInfo := *value.Package
		packageInfo.Name = sanitizeText(packageInfo.Name)
		packageInfo.Version = sanitizeText(packageInfo.Version)
		packageInfo.Executables = sanitizeTexts(packageInfo.Executables)
		packageInfo.Prefix = sanitizePath(packageInfo.Prefix)
		packageInfo.VirtualEnv = sanitizePath(packageInfo.VirtualEnv)
		value.Package = &packageInfo
	}
	if value.Application != nil {
		application := *value.Application
		application.BundleID = sanitizeText(application.BundleID)
		application.DisplayVersion = sanitizeText(application.DisplayVersion)
		application.ShortVersion = sanitizeText(application.ShortVersion)
		application.MinimumOS = sanitizeText(application.MinimumOS)
		application.Architectures = sanitizeTexts(application.Architectures)
		application.SigningTeamID = sanitizeText(application.SigningTeamID)
		value.Application = &application
	}
	if value.History != nil {
		history := *value.History
		history.CachePath = sanitizePath(history.CachePath)
		value.History = &history
	}
	value.Locations = append([]model.Location(nil), value.Locations...)
	for i := range value.Locations {
		value.Locations[i].Path = sanitizePath(value.Locations[i].Path)
		value.Locations[i].SymlinkTarget = sanitizePath(value.Locations[i].SymlinkTarget)
		value.Locations[i].Architectures = sanitizeTexts(value.Locations[i].Architectures)
		// Hashes are local diagnostics and remain opt-in for exports.
		value.Locations[i].SHA256 = ""
	}
	value.ID = model.ObservationIdentity(value)
	return value
}

func sanitizeReport(value output.ObservationReport) output.ObservationReport {
	value.Installed = sanitizeObservations(value.Installed)
	value.Available = sanitizeObservations(value.Available)
	value.History = sanitizeObservations(value.History)
	value.Warnings = []string{}
	value.Changes = sanitizeChanges(value.Changes)
	return value
}

func sanitizeChanges(value output.ChangeReport) output.ChangeReport {
	return output.ChangeReport{
		Added:     sanitizeEvents(value.Added),
		Removed:   sanitizeEvents(value.Removed),
		Updated:   sanitizeEvents(value.Updated),
		Relocated: sanitizeEvents(value.Relocated),
		Broken:    sanitizeEvents(value.Broken),
		Repaired:  sanitizeEvents(value.Repaired),
		Shadowed:  sanitizeEvents(value.Shadowed),
	}
}

func sanitizeEvents(events []registry.ChangeEvent) []registry.ChangeEvent {
	result := make([]registry.ChangeEvent, 0, len(events))
	for _, event := range events {
		event.Identity = sanitizeText(event.Identity)
		if event.Before != nil {
			before := sanitizeObservation(*event.Before)
			event.Before = &before
		}
		if event.After != nil {
			after := sanitizeObservation(*event.After)
			event.After = &after
		}
		result = append(result, event)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Identity != result[j].Identity {
			return result[i].Identity < result[j].Identity
		}
		return result[i].Kind < result[j].Kind
	})
	return result
}

func sanitizePath(value string) string {
	return sanitizeText(value)
}

func sanitizePaths(values []string) []string {
	result := sanitizeTexts(values)
	sort.Strings(result)
	return result
}

func sanitizeTexts(values []string) []string {
	if values == nil {
		return nil
	}
	result := make([]string, len(values))
	for i, value := range values {
		result[i] = sanitizeText(value)
	}
	return result
}

var secretTextPattern = regexp.MustCompile(`(?i)(password|passwd|secret|token|api[_-]?key|authorization|bearer|private[_-]?key)(\s*[:=]\s*|\s+)[^\s,;]+`)

func sanitizeText(value string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		value = strings.ReplaceAll(value, home, HomeRedaction)
	}
	return secretTextPattern.ReplaceAllString(value, `$1=<redacted>`)
}

func decodeStrict(data []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}
