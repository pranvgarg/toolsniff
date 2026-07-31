// scanner/scanner.go
package scanner

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/pranvgarg/toolsniff/model"
)

// execTimeout bounds how long an external command (brew, npm, pipx, ...) is
// allowed to run. Without this, a hung external tool (cold Homebrew index,
// slow NFS mount, etc.) would hang the whole program forever with no
// feedback.
const execTimeout = 8 * time.Second

// Scanner discovers tools from a single source (a package manager, a
// directory, a cache). Implementations must be safe to call concurrently
// with other scanners.
type Scanner interface {
	Name() string
	Scan() ([]model.Tool, error)
}

// CommandRunner runs an external command and returns its captured stdout.
// Scanners that shell out take one of these instead of calling os/exec
// directly, so tests can inject fixture output instead of depending on the
// real binary being installed.
type CommandRunner func(name string, args ...string) ([]byte, error)

// ExecRunner is the real CommandRunner used outside of tests.
func ExecRunner(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), execTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Output()
}

// Warning records that one scanner failed without aborting the whole run.
type Warning struct {
	Source string
	Err    error
}

// RunAll runs every scanner concurrently and collects successful results
// and warnings separately. A failing scanner never prevents the others
// from completing.
func RunAll(scanners []Scanner) ([]model.Tool, []Warning) {
	type result struct {
		name  string
		tools []model.Tool
		err   error
	}

	results := make(chan result, len(scanners))
	var wg sync.WaitGroup
	for _, s := range scanners {
		wg.Add(1)
		go func(s Scanner) {
			defer wg.Done()
			tools, err := s.Scan()
			results <- result{name: s.Name(), tools: tools, err: err}
		}(s)
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	var allTools []model.Tool
	var warnings []Warning
	for r := range results {
		if r.err != nil {
			warnings = append(warnings, Warning{Source: r.name, Err: r.err})
			continue
		}
		allTools = append(allTools, r.tools...)
	}
	return allTools, warnings
}

// runTolerant runs a command via runner and returns its output, treating a
// non-nil error as fatal only when stdout is empty. Many CLI tools (npm,
// brew, pipx) exit non-zero on warnings — e.g. npm ls -g on unmet peer
// deps — while still emitting valid, usable output on stdout. Silently
// discarding runErr when out is non-empty is deliberate, not a missed
// error check.
func runTolerant(runner CommandRunner, source string, args ...string) ([]byte, error) {
	out, runErr := runner(args[0], args[1:]...)
	if len(out) == 0 {
		if runErr != nil {
			return nil, fmt.Errorf("%s: %w", source, runErr)
		}
		return nil, nil
	}
	return out, nil
}
