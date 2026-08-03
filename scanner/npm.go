package scanner

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pranvgarg/toolsniff/model"
)

// NPMScanner discovers globally installed npm packages via `npm ls -g`.
type NPMScanner struct {
	runner CommandRunner
}

type npmOutput struct {
	Path         string                `json:"path"`
	Prefix       string                `json:"prefix"`
	Dependencies map[string]npmPackage `json:"dependencies"`
}

type npmPackage struct {
	Version  string          `json:"version"`
	Path     string          `json:"path"`
	Prefix   string          `json:"prefix"`
	Bin      json.RawMessage `json:"bin"`
	BinLinks json.RawMessage `json:"bin_links"`
}

func NewNPMScanner(runner CommandRunner) *NPMScanner {
	return &NPMScanner{runner: runner}
}

func (s *NPMScanner) Name() string { return model.SourceNPM }

func (s *NPMScanner) Scan() ([]model.Tool, error) {
	out, err := runTolerant(s.runner, "npm", "npm", "ls", "-g", "--depth=0", "--json")
	if err != nil {
		return nil, err
	}
	if out == nil {
		return nil, nil
	}

	parsed, err := parseNPMOutput(out)
	if err != nil {
		return nil, fmt.Errorf("npm: parsing output: %w", err)
	}

	tools := make([]model.Tool, 0, len(parsed.Dependencies))
	for name, dep := range parsed.Dependencies {
		tools = append(tools, model.Tool{Name: name, Source: "npm", Version: dep.Version})
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return tools, nil
}

// ScanObservations exposes the same npm discovery as Scan with source-specific
// package metadata. Scan remains the compatibility API used by orchestration.
func (s *NPMScanner) ScanObservations() ([]model.Observation, error) {
	out, err := runTolerant(s.runner, "npm", "npm", "ls", "-g", "--depth=0", "--json")
	if err != nil {
		return nil, err
	}
	return NPMObservationsFromJSON(out)
}

// NPMObservationsFromJSON converts npm ls --global JSON into v2 observations.
func NPMObservationsFromJSON(data []byte) ([]model.Observation, error) {
	if len(data) == 0 {
		return nil, nil
	}
	parsed, err := parseNPMOutput(data)
	if err != nil {
		return nil, fmt.Errorf("npm: parsing output: %w", err)
	}

	observations := make([]model.Observation, 0, len(parsed.Dependencies))
	for name, dep := range parsed.Dependencies {
		observation, err := npmObservation(name, dep, parsed)
		if err != nil {
			return nil, fmt.Errorf("npm: package %q: %w", name, err)
		}
		observations = append(observations, observation)
	}
	sort.Slice(observations, func(i, j int) bool {
		return observations[i].DisplayName < observations[j].DisplayName
	})
	return observations, nil
}

func parseNPMOutput(data []byte) (npmOutput, error) {
	var parsed struct {
		Path         string                `json:"path"`
		Prefix       string                `json:"prefix"`
		Dependencies map[string]npmPackage `json:"dependencies"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return npmOutput{}, err
	}
	return npmOutput{
		Path:         parsed.Path,
		Prefix:       parsed.Prefix,
		Dependencies: parsed.Dependencies,
	}, nil
}

func npmObservation(name string, dep npmPackage, root npmOutput) (model.Observation, error) {
	prefix := firstNonEmpty(dep.Prefix, dep.Path, root.Prefix, root.Path)
	executableTargets, err := npmBinTargets(dep.Bin, prefix)
	if err != nil {
		return model.Observation{}, fmt.Errorf("invalid bin metadata: %w", err)
	}
	linkTargets, err := npmBinTargets(dep.BinLinks, prefix)
	if err != nil {
		return model.Observation{}, fmt.Errorf("invalid bin link metadata: %w", err)
	}
	for executable, target := range linkTargets {
		executableTargets[executable] = target
	}

	executables := make([]string, 0, len(executableTargets))
	for executable := range executableTargets {
		executables = append(executables, executable)
	}
	sort.Strings(executables)

	locations := make([]model.Location, 0, 1+len(executableTargets))
	if prefix != "" {
		locations = append(locations, model.Location{Path: prefix, Type: model.LocationPackagePrefix})
	}
	for _, executable := range executables {
		target := executableTargets[executable]
		if target == "" {
			continue
		}
		locations = append(locations, model.Location{
			Path:       target,
			Type:       model.LocationExecutable,
			Executable: true,
		})
	}

	kind := model.KindPackage
	if len(executables) > 0 {
		kind = model.KindCLI
	}
	packageInfo := &model.PackageInfo{
		Name:        name,
		Version:     dep.Version,
		Executables: executables,
		Prefix:      prefix,
	}
	observation := model.Observation{
		DisplayName: name,
		CommandName: name,
		Kind:        kind,
		Role:        model.RoleInstalled,
		Origin: model.Origin{
			Provider: "npm",
			Manager:  "global",
			Package:  name,
		},
		Version:      packageVersionInfo(dep.Version, "npm ls -g --depth=0 --json"),
		Locations:    locations,
		Availability: model.AvailabilityInfo{State: model.AvailabilityUnknown},
		Package:      packageInfo,
		Evidence: []model.Evidence{{
			Type:   "package-metadata",
			Source: "npm ls -g --depth=0 --json",
		}},
	}
	observation.ID = model.ObservationIdentity(observation)
	return observation, nil
}

func npmBinTargets(raw json.RawMessage, prefix string) (map[string]string, error) {
	targets := make(map[string]string)
	if len(raw) == 0 || string(raw) == "null" {
		return targets, nil
	}

	var links map[string]string
	if err := json.Unmarshal(raw, &links); err == nil {
		for name, target := range links {
			if target == "" {
				targets[name] = ""
				continue
			}
			if !filepath.IsAbs(target) && prefix != "" {
				target = filepath.Join(prefix, target)
			}
			targets[name] = filepath.Clean(target)
		}
		return targets, nil
	}

	var link string
	if err := json.Unmarshal(raw, &link); err != nil {
		return nil, err
	}
	if link != "" && !filepath.IsAbs(link) && prefix != "" {
		link = filepath.Join(prefix, link)
	}
	if link != "" {
		targets[filepath.Base(link)] = filepath.Clean(link)
	}
	return targets, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func packageVersionInfo(value, retrievedBy string) model.VersionInfo {
	if value == "" {
		return model.VersionInfo{
			State:       model.VersionNotReported,
			Scheme:      model.SchemeUnknown,
			Confidence:  model.ConfidenceLow,
			RetrievedBy: retrievedBy,
		}
	}
	scheme := model.SchemeOpaque
	comparable := false
	if looksLikeVersion(value) {
		scheme = model.SchemeSemver
		comparable = true
	}
	return model.VersionInfo{
		Value:       value,
		State:       model.VersionKnown,
		Scheme:      scheme,
		Comparable:  comparable,
		Confidence:  model.ConfidenceHigh,
		RetrievedBy: retrievedBy,
	}
}

func looksLikeVersion(value string) bool {
	if value == "" || value[0] < '0' || value[0] > '9' || !strings.Contains(value, ".") {
		return false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && r != '.' && r != '-' && r != '+' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
			return false
		}
	}
	return true
}
