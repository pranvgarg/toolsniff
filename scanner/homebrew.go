package scanner

import (
	"fmt"
	"strings"

	"github.com/pranvgarg/toolsniff/model"
)

func runBrewList(runner CommandRunner, source, flag string) ([]model.Tool, error) {
	out, runErr := runner("brew", "list", flag, "-1")
	if len(out) == 0 {
		if runErr != nil {
			return nil, fmt.Errorf("brew: %w", runErr)
		}
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
	return tools, nil
}

// HomebrewFormulaScanner discovers installed Homebrew formulae.
type HomebrewFormulaScanner struct {
	runner CommandRunner
}

func NewHomebrewFormulaScanner(runner CommandRunner) *HomebrewFormulaScanner {
	return &HomebrewFormulaScanner{runner: runner}
}

func (s *HomebrewFormulaScanner) Name() string { return "brew-formula" }

func (s *HomebrewFormulaScanner) Scan() ([]model.Tool, error) {
	return runBrewList(s.runner, "brew-formula", "--formula")
}

// HomebrewCaskScanner discovers installed Homebrew casks (GUI apps).
type HomebrewCaskScanner struct {
	runner CommandRunner
}

func NewHomebrewCaskScanner(runner CommandRunner) *HomebrewCaskScanner {
	return &HomebrewCaskScanner{runner: runner}
}

func (s *HomebrewCaskScanner) Name() string { return "brew-cask" }

func (s *HomebrewCaskScanner) Scan() ([]model.Tool, error) {
	return runBrewList(s.runner, "brew-cask", "--cask")
}
