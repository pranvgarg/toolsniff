package scanner

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/pranvgarg/toolsniff/model"
)

// PipxScanner discovers tools installed via pipx.
type PipxScanner struct {
	runner CommandRunner
}

type pipxOutput struct {
	Venvs map[string]pipxVenv `json:"venvs"`
}

type pipxVenv struct {
	VenvDir  string `json:"venv_dir"`
	Metadata struct {
		MainPackage struct {
			PackageOrURL   string `json:"package_or_url"`
			Package        string `json:"package"`
			PackageVersion string `json:"package_version"`
		} `json:"main_package"`
	} `json:"metadata"`
	AppPaths json.RawMessage `json:"app_paths"`
	BinPaths json.RawMessage `json:"bin_paths"`
	Apps     json.RawMessage `json:"apps"`
}

func NewPipxScanner(runner CommandRunner) *PipxScanner {
	return &PipxScanner{runner: runner}
}

func (s *PipxScanner) Name() string { return model.SourcePipx }

func (s *PipxScanner) Scan() ([]model.Tool, error) {
	out, err := runTolerant(s.runner, "pipx", "pipx", "list", "--json")
	if err != nil {
		return nil, err
	}
	if out == nil {
		return nil, nil
	}

	parsed, err := parsePipxOutput(out)
	if err != nil {
		return nil, fmt.Errorf("pipx: parsing output: %w", err)
	}

	tools := make([]model.Tool, 0, len(parsed.Venvs))
	for name, venv := range parsed.Venvs {
		tools = append(tools, model.Tool{
			Name:    name,
			Source:  "pipx",
			Version: venv.Metadata.MainPackage.PackageVersion,
		})
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return tools, nil
}

// ScanObservations exposes pipx venv and executable metadata while retaining
// the legacy []model.Tool Scanner contract.
func (s *PipxScanner) ScanObservations() ([]model.Observation, error) {
	out, err := runTolerant(s.runner, "pipx", "pipx", "list", "--json")
	if err != nil {
		return nil, err
	}
	return PipxObservationsFromJSON(out)
}

// PipxObservationsFromJSON converts pipx list --json output into v2 package
// observations. Missing optional paths do not make the whole scan fail.
func PipxObservationsFromJSON(data []byte) ([]model.Observation, error) {
	if len(data) == 0 {
		return nil, nil
	}
	parsed, err := parsePipxOutput(data)
	if err != nil {
		return nil, fmt.Errorf("pipx: parsing output: %w", err)
	}

	observations := make([]model.Observation, 0, len(parsed.Venvs))
	for name, venv := range parsed.Venvs {
		packageName := firstNonEmpty(venv.Metadata.MainPackage.PackageOrURL, venv.Metadata.MainPackage.Package, name)
		paths, executableNames, err := pipxPaths(venv)
		if err != nil {
			return nil, fmt.Errorf("pipx: venv %q: %w", name, err)
		}
		for _, path := range paths {
			executableNames = append(executableNames, filepath.Base(path))
		}
		sort.Strings(executableNames)
		executableNames = uniqueStrings(executableNames)

		locations := make([]model.Location, 0, 1+len(paths))
		if venv.VenvDir != "" {
			locations = append(locations, model.Location{Path: venv.VenvDir, Type: model.LocationVirtualEnv})
		}
		for _, path := range paths {
			locations = append(locations, model.Location{Path: path, Type: model.LocationExecutable, Executable: true})
		}
		kind := model.KindPackage
		if len(executableNames) > 0 {
			kind = model.KindCLI
		}
		observation := model.Observation{
			DisplayName: name,
			CommandName: name,
			Kind:        kind,
			Role:        model.RoleInstalled,
			Origin: model.Origin{
				Provider: "pipx",
				Manager:  "global",
				Package:  packageName,
			},
			Version:      packageVersionInfo(venv.Metadata.MainPackage.PackageVersion, "pipx list --json"),
			Locations:    locations,
			Availability: model.AvailabilityInfo{State: model.AvailabilityUnknown},
			Package: &model.PackageInfo{
				Name:        packageName,
				Version:     venv.Metadata.MainPackage.PackageVersion,
				Executables: executableNames,
				VirtualEnv:  venv.VenvDir,
			},
			Evidence: []model.Evidence{{
				Type:   "package-metadata",
				Source: "pipx list --json",
			}},
		}
		observation.ID = model.ObservationIdentity(observation)
		observations = append(observations, observation)
	}
	sort.Slice(observations, func(i, j int) bool {
		return observations[i].DisplayName < observations[j].DisplayName
	})
	return observations, nil
}

func parsePipxOutput(data []byte) (pipxOutput, error) {
	var parsed pipxOutput
	if err := json.Unmarshal(data, &parsed); err != nil {
		return pipxOutput{}, err
	}
	return parsed, nil
}

func pipxPaths(venv pipxVenv) ([]string, []string, error) {
	var paths []string
	var names []string
	for field, raw := range map[string]json.RawMessage{
		"app_paths": venv.AppPaths,
		"bin_paths": venv.BinPaths,
	} {
		values, err := stringList(raw)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid %s: %w", field, err)
		}
		paths = append(paths, values...)
	}
	if len(venv.Apps) > 0 && string(venv.Apps) != "null" {
		values, err := stringList(venv.Apps)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid apps: %w", err)
		}
		names = append(names, values...)
	}
	paths = uniqueStrings(paths)
	return paths, names, nil
}

func stringList(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	return values, nil
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	return unique
}
