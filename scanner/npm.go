package scanner

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/pranvgarg/toolsniff/model"
)

// NPMScanner discovers globally installed npm packages via `npm ls -g`.
type NPMScanner struct {
	runner CommandRunner
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

	var parsed struct {
		Dependencies map[string]struct {
			Version string `json:"version"`
		} `json:"dependencies"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, fmt.Errorf("npm: parsing output: %w", err)
	}

	tools := make([]model.Tool, 0, len(parsed.Dependencies))
	for name, dep := range parsed.Dependencies {
		tools = append(tools, model.Tool{Name: name, Source: "npm", Version: dep.Version})
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return tools, nil
}
