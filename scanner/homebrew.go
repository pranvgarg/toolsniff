package scanner

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/pranvgarg/toolsniff/model"
)

func runBrewList(runner CommandRunner, source, flag string) ([]model.Tool, error) {
	out, err := runTolerant(runner, source, "brew", "list", flag, "-1")
	if err != nil {
		return nil, err
	}
	if out == nil {
		return nil, nil
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	tools := make([]model.Tool, 0, len(lines))
	for _, line := range lines {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		tools = append(tools, model.Tool{Name: name, Source: source})
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return tools, nil
}

// HomebrewFormulaScanner discovers installed Homebrew formulae.
type HomebrewFormulaScanner struct {
	runner CommandRunner
}

type brewInfoOutput struct {
	Formulae []brewFormula `json:"formulae"`
	Casks    []brewCask    `json:"casks"`
}

type brewFormula struct {
	Name      string `json:"name"`
	FullName  string `json:"full_name"`
	Prefix    string `json:"prefix"`
	LinkedKeg string `json:"linked_keg"`
	Versions  struct {
		Stable string `json:"stable"`
	} `json:"versions"`
	Installed []struct {
		Version string `json:"version"`
		Prefix  string `json:"prefix"`
	} `json:"installed"`
}

type brewCask struct {
	Token     string `json:"token"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	Installed string `json:"installed"`
}

func NewHomebrewFormulaScanner(runner CommandRunner) *HomebrewFormulaScanner {
	return &HomebrewFormulaScanner{runner: runner}
}

func (s *HomebrewFormulaScanner) Name() string { return model.SourceBrewFormula }

func (s *HomebrewFormulaScanner) Scan() ([]model.Tool, error) {
	return runBrewList(s.runner, model.SourceBrewFormula, "--formula")
}

// ScanObservations reads Homebrew's structured installed metadata without
// changing the compatibility list-based Scan method.
func (s *HomebrewFormulaScanner) ScanObservations() ([]model.Observation, error) {
	return s.scanInfo(model.SourceBrewFormula, "--formula")
}

func (s *HomebrewFormulaScanner) scanInfo(source, kindFlag string) ([]model.Observation, error) {
	out, err := runTolerant(s.runner, source, "brew", "info", "--json=v2", "--installed", kindFlag)
	if err != nil {
		return nil, err
	}
	return HomebrewFormulaObservationsFromJSON(out)
}

// HomebrewCaskScanner discovers installed Homebrew casks (GUI apps).
type HomebrewCaskScanner struct {
	runner CommandRunner
}

func NewHomebrewCaskScanner(runner CommandRunner) *HomebrewCaskScanner {
	return &HomebrewCaskScanner{runner: runner}
}

func (s *HomebrewCaskScanner) Name() string { return model.SourceBrewCask }

func (s *HomebrewCaskScanner) Scan() ([]model.Tool, error) {
	return runBrewList(s.runner, model.SourceBrewCask, "--cask")
}

// ScanObservations exposes cask versions from brew info --json=v2.
func (s *HomebrewCaskScanner) ScanObservations() ([]model.Observation, error) {
	out, err := runTolerant(s.runner, model.SourceBrewCask, "brew", "info", "--json=v2", "--installed", "--cask")
	if err != nil {
		return nil, err
	}
	return HomebrewCaskObservationsFromJSON(out)
}

func parseBrewInfo(data []byte) (brewInfoOutput, error) {
	if len(data) == 0 {
		return brewInfoOutput{}, nil
	}
	var parsed brewInfoOutput
	if err := json.Unmarshal(data, &parsed); err != nil {
		return brewInfoOutput{}, fmt.Errorf("homebrew: parsing output: %w", err)
	}
	return parsed, nil
}

// HomebrewFormulaObservationsFromJSON converts brew formula metadata into v2
// package observations.
func HomebrewFormulaObservationsFromJSON(data []byte) ([]model.Observation, error) {
	parsed, err := parseBrewInfo(data)
	if err != nil {
		return nil, err
	}
	observations := make([]model.Observation, 0, len(parsed.Formulae))
	for _, formula := range parsed.Formulae {
		name := firstNonEmpty(formula.Name, formula.FullName)
		if name == "" {
			continue
		}
		version := formula.Versions.Stable
		prefix := formula.Prefix
		if len(formula.Installed) > 0 {
			version = firstNonEmpty(formula.Installed[0].Version, version)
			prefix = firstNonEmpty(formula.Installed[0].Prefix, prefix)
		}
		observations = append(observations, homebrewObservation(name, version, prefix, "formula"))
	}
	return observations, nil
}

// HomebrewCaskObservationsFromJSON converts brew cask metadata into v2
// package observations. Bundle metadata is intentionally left to the
// application scanner.
func HomebrewCaskObservationsFromJSON(data []byte) ([]model.Observation, error) {
	parsed, err := parseBrewInfo(data)
	if err != nil {
		return nil, err
	}
	observations := make([]model.Observation, 0, len(parsed.Casks))
	for _, cask := range parsed.Casks {
		name := firstNonEmpty(cask.Token, cask.Name)
		if name == "" {
			continue
		}
		observations = append(observations, homebrewObservation(name, cask.Version, "", "cask"))
	}
	return observations, nil
}

func homebrewObservation(name, version, prefix, manager string) model.Observation {
	locations := []model.Location(nil)
	if prefix != "" {
		locations = []model.Location{{Path: prefix, Type: model.LocationPackagePrefix}}
	}
	observation := model.Observation{
		DisplayName: name,
		CommandName: name,
		Kind:        model.KindPackage,
		Role:        model.RoleInstalled,
		Origin: model.Origin{
			Provider: "homebrew",
			Manager:  manager,
			Package:  name,
		},
		Version:      packageVersionInfo(version, "brew info --json=v2 --installed"),
		Locations:    locations,
		Availability: model.AvailabilityInfo{State: model.AvailabilityUnknown},
		Package: &model.PackageInfo{
			Name:    name,
			Version: version,
			Prefix:  prefix,
		},
		Evidence: []model.Evidence{{
			Type:   "package-metadata",
			Source: "brew info --json=v2 --installed",
		}},
	}
	observation.ID = model.ObservationIdentity(observation)
	return observation
}
