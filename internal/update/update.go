package update

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	// DefaultExecTimeout bounds each Homebrew command run by the real runner.
	DefaultExecTimeout = 8 * time.Second

	// ToolName is the Homebrew package managed by this service.
	ToolName = "toolsniff"
)

var (
	ErrHomebrewNotInstalled     = errors.New("homebrew is not installed")
	ErrToolsniffNotInstalled    = errors.New("toolsniff is not installed with homebrew")
	ErrAmbiguousInstallation    = errors.New("toolsniff is installed as both a formula and a cask")
	ErrCommandLineToolsOutdated = errors.New("apple command line tools are outdated")
)

// CommandRunner runs an external command and returns its captured output.
// Keeping command execution behind a function makes every update decision
// testable without requiring Homebrew on the test machine.
type CommandRunner func(name string, args ...string) ([]byte, error)

// ExecRunner is the real command runner used by callers outside tests.
func ExecRunner(name string, args ...string) ([]byte, error) {
	return NewExecRunner(DefaultExecTimeout)(name, args...)
}

// NewExecRunner returns a command runner with an explicit timeout.
func NewExecRunner(timeout time.Duration) CommandRunner {
	if timeout <= 0 {
		timeout = DefaultExecTimeout
	}
	return func(name string, args ...string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		return exec.CommandContext(ctx, name, args...).CombinedOutput()
	}
}

// Source identifies the Homebrew packaging source for toolsniff.
type Source string

const (
	SourceFormula Source = "formula"
	SourceCask    Source = "cask"
)

// Status is the result of checking the local Homebrew installation.
type Status struct {
	Source         Source
	Outdated       bool
	OutdatedOutput string
}

// UpdateInfo is passed to a prompt before an upgrade is started.
type UpdateInfo struct {
	Name           string
	Source         Source
	OutdatedOutput string
}

// Prompt decides whether an identified outdated installation should be
// upgraded. A nil prompt is treated as a declined update.
type Prompt func(UpdateInfo) (bool, error)

// Options controls the reusable update decision. Yes takes precedence over
// Prompt and is intended for non-interactive callers.
type Options struct {
	Yes    bool
	Prompt Prompt
}

// Result describes what Run decided and whether it changed the installation.
type Result struct {
	Status  Status
	Updated bool
	Skipped bool
}

// CommandLineToolsError explains the local prerequisite problem separately
// from ordinary Homebrew command failures.
type CommandLineToolsError struct {
	Operation string
	Cause     error
}

func (e *CommandLineToolsError) Error() string {
	return fmt.Sprintf("brew %s failed because Apple's Command Line Tools are outdated; update them through Software Update, then retry", e.Operation)
}

func (e *CommandLineToolsError) Unwrap() error {
	return ErrCommandLineToolsOutdated
}

// Service contains the Homebrew self-update behavior for toolsniff.
type Service struct {
	runner CommandRunner
}

// NewService constructs an update service with an injected command runner.
func NewService(runner CommandRunner) *Service {
	if runner == nil {
		runner = ExecRunner
	}
	return &Service{runner: runner}
}

// DetectHomebrew verifies that brew can be executed.
func (s *Service) DetectHomebrew() error {
	out, err := s.runner("brew", "--version")
	if err == nil {
		return nil
	}
	if classified := classifyCommandError("--version", out, err); classified != nil {
		return classified
	}
	return fmt.Errorf("%w: %v", ErrHomebrewNotInstalled, err)
}

// DetectInstallation identifies whether toolsniff is installed as a formula
// or cask. Installing both forms is rejected instead of choosing arbitrarily.
func (s *Service) DetectInstallation() (Source, error) {
	if err := s.DetectHomebrew(); err != nil {
		return "", err
	}
	return s.detectInstallation()
}

// Check detects the installation source and checks its outdated state.
func (s *Service) Check() (Status, error) {
	if err := s.DetectHomebrew(); err != nil {
		return Status{}, err
	}

	source, err := s.detectInstallation()
	if err != nil {
		return Status{}, err
	}
	flag, err := outdatedFlag(source)
	if err != nil {
		return Status{}, err
	}
	out, err := s.run("outdated", flag, ToolName)
	if err != nil {
		return Status{}, err
	}
	return Status{
		Source:         source,
		Outdated:       len(strings.TrimSpace(string(out))) > 0,
		OutdatedOutput: strings.TrimSpace(string(out)),
	}, nil
}

// IsOutdated checks one known Homebrew source without redetecting it.
func (s *Service) IsOutdated(source Source) (bool, error) {
	flag, err := outdatedFlag(source)
	if err != nil {
		return false, err
	}
	out, err := s.run("outdated", flag, ToolName)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) != "", nil
}

// Upgrade upgrades toolsniff using the source-specific Homebrew command.
func (s *Service) Upgrade(source Source) error {
	args := []string{"upgrade", ToolName}
	if source == SourceCask {
		args = []string{"upgrade", "--cask", ToolName}
	}
	if source != SourceFormula && source != SourceCask {
		return fmt.Errorf("unknown Homebrew source %q", source)
	}
	_, err := s.run(args...)
	return err
}

// Run checks toolsniff, obtains a prompt/yes decision when needed, and runs
// the source-specific upgrade. It never prompts when the installation is
// already current.
func (s *Service) Run(options Options) (Result, error) {
	status, err := s.Check()
	if err != nil {
		return Result{}, err
	}
	result := Result{Status: status}
	if !status.Outdated {
		return result, nil
	}

	confirmed := options.Yes
	if !confirmed && options.Prompt != nil {
		confirmed, err = options.Prompt(UpdateInfo{
			Name:           ToolName,
			Source:         status.Source,
			OutdatedOutput: status.OutdatedOutput,
		})
		if err != nil {
			return result, fmt.Errorf("confirm toolsniff update: %w", err)
		}
	}
	if !confirmed {
		result.Skipped = true
		return result, nil
	}
	if err := s.Upgrade(status.Source); err != nil {
		return result, err
	}
	result.Updated = true
	return result, nil
}

func (s *Service) detectInstallation() (Source, error) {
	formula, formulaErr := s.runner("brew", "list", "--formula", ToolName)
	if err := classifyCommandError("list --formula", formula, formulaErr); err != nil {
		return "", err
	}
	cask, caskErr := s.runner("brew", "list", "--cask", ToolName)
	if err := classifyCommandError("list --cask", cask, caskErr); err != nil {
		return "", err
	}

	hasFormula := formulaErr == nil && strings.TrimSpace(string(formula)) != ""
	hasCask := caskErr == nil && strings.TrimSpace(string(cask)) != ""
	switch {
	case hasFormula && hasCask:
		return "", ErrAmbiguousInstallation
	case hasFormula:
		return SourceFormula, nil
	case hasCask:
		return SourceCask, nil
	default:
		return "", ErrToolsniffNotInstalled
	}
}

func (s *Service) run(args ...string) ([]byte, error) {
	out, err := s.runner("brew", args...)
	if err == nil {
		return out, nil
	}
	if classified := classifyCommandError(strings.Join(args, " "), out, err); classified != nil {
		return nil, classified
	}
	return nil, fmt.Errorf("brew %s: %w", strings.Join(args, " "), err)
}

func outdatedFlag(source Source) (string, error) {
	if source == SourceCask {
		return "--cask", nil
	}
	if source != SourceFormula {
		return "", fmt.Errorf("unknown Homebrew source %q", source)
	}
	return "--formula", nil
}

func classifyCommandError(operation string, output []byte, err error) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(string(output) + " " + err.Error())
	if strings.Contains(message, "command line tools") &&
		(strings.Contains(message, "outdated") || strings.Contains(message, "out of date") || strings.Contains(message, "too old")) {
		return &CommandLineToolsError{Operation: operation, Cause: err}
	}
	return nil
}
