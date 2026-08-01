package scanner

import (
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

func NewHomebrewFormulaScanner(runner CommandRunner) *HomebrewFormulaScanner {
	return &HomebrewFormulaScanner{runner: runner}
}

func (s *HomebrewFormulaScanner) Name() string { return model.SourceBrewFormula }

func (s *HomebrewFormulaScanner) Scan() ([]model.Tool, error) {
	return runBrewList(s.runner, model.SourceBrewFormula, "--formula")
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
