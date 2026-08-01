package scanner

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/pranvgarg/toolsniff/model"
)

// PipxScanner discovers tools installed via pipx.
type PipxScanner struct {
	runner CommandRunner
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

	var parsed struct {
		Venvs map[string]struct {
			Metadata struct {
				MainPackage struct {
					PackageVersion string `json:"package_version"`
				} `json:"main_package"`
			} `json:"metadata"`
		} `json:"venvs"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
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
