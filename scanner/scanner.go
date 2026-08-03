// scanner/scanner.go
package scanner

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"sync"
	"time"

	"github.com/pranvgarg/toolsniff/model"
)

// DefaultExecTimeout bounds how long an external command (brew, npm, pipx, ...) is
// allowed to run. Without this, a hung external tool (cold Homebrew index,
// slow NFS mount, etc.) would hang the whole program forever with no
// feedback.
const DefaultExecTimeout = 8 * time.Second

// Scanner discovers tools from a single source (a package manager, a
// directory, a cache). Implementations must be safe to call concurrently
// with other scanners.
type Scanner interface {
	Name() string
	Scan() ([]model.Tool, error)
}

// ObservationScanner is implemented by scanners that can preserve source
// metadata while producing the v2 observation model.
type ObservationScanner interface {
	ScanObservations() ([]model.Observation, error)
}

// SourceInfo is the metadata shared by scan orchestration and renderers.
// Keeping it beside Scanner prevents source ordering and source semantics from
// being reimplemented in main and output packages.
type SourceInfo struct {
	ID            string
	Order         int
	Role          model.SourceRole
	Informational bool
}

// Registration binds a scanner implementation to the metadata that describes
// its observations.
type Registration struct {
	SourceInfo
	Scanner Scanner
}

// CommandRunner runs an external command and returns its captured stdout.
// Scanners that shell out take one of these instead of calling os/exec
// directly, so tests can inject fixture output instead of depending on the
// real binary being installed.
type CommandRunner func(name string, args ...string) ([]byte, error)

// ExecRunner is the real CommandRunner used outside of tests.
func ExecRunner(name string, args ...string) ([]byte, error) {
	return NewExecRunner(DefaultExecTimeout)(name, args...)
}

// NewExecRunner returns a command runner with an explicit timeout. The
// timeout is injected so installations with slower package-manager commands
// can configure it without changing scanner behavior.
func NewExecRunner(timeout time.Duration) CommandRunner {
	if timeout <= 0 {
		timeout = DefaultExecTimeout
	}
	return func(name string, args ...string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, name, args...)
		return cmd.Output()
	}
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
		allTools = append(allTools, r.tools...)
		if r.err != nil {
			warnings = append(warnings, Warning{Source: r.name, Err: r.err})
			continue
		}
	}
	sort.Slice(warnings, func(i, j int) bool { return warnings[i].Source < warnings[j].Source })
	return allTools, warnings
}

// RunObservations prefers a scanner's v2 adapter and falls back to the legacy
// tool model for sources that have not been migrated yet. Registration roles
// are authoritative so adapters cannot accidentally change installed,
// available, or history semantics.
func RunObservations(registrations []Registration) ([]model.Observation, []Warning) {
	type result struct {
		name         string
		observations []model.Observation
		err          error
	}

	results := make(chan result, len(registrations))
	var wg sync.WaitGroup
	for _, registration := range registrations {
		wg.Add(1)
		go func(registration Registration) {
			defer wg.Done()
			observations, err := scanRegistrationObservations(registration)
			results <- result{
				name:         registration.ID,
				observations: observations,
				err:          err,
			}
		}(registration)
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	var all []model.Observation
	var warnings []Warning
	for result := range results {
		if result.err != nil {
			warnings = append(warnings, Warning{Source: result.name, Err: result.err})
			continue
		}
		all = append(all, result.observations...)
	}

	unique := all[:0]
	seen := make(map[string]struct{}, len(all))
	for i := range all {
		all[i].ID = model.ObservationIdentity(all[i])
		if _, ok := seen[all[i].ID]; ok {
			continue
		}
		seen[all[i].ID] = struct{}{}
		unique = append(unique, all[i])
	}
	all = unique
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].ID != all[j].ID {
			return all[i].ID < all[j].ID
		}
		return all[i].DisplayName < all[j].DisplayName
	})
	sort.Slice(warnings, func(i, j int) bool { return warnings[i].Source < warnings[j].Source })
	return all, warnings
}

func scanRegistrationObservations(registration Registration) ([]model.Observation, error) {
	var (
		observations []model.Observation
		err          error
	)
	role := registrationRole(registration)

	switch adapted := registration.Scanner.(type) {
	case ObservationScanner:
		observations, err = adapted.ScanObservations()
	case *PathScanner:
		observations, err = adapted.ScanObservations(PathScanOptions{})
	default:
		var tools []model.Tool
		tools, err = registration.Scanner.Scan()
		if err == nil {
			observations = make([]model.Observation, 0, len(tools))
			for _, tool := range tools {
				tool.Role = role
				observations = append(observations, model.ObservationFromTool(tool))
			}
		}
	}
	if err != nil {
		return nil, err
	}
	for i := range observations {
		observations[i].Role = role
		if observations[i].ID == "" {
			observations[i].ID = model.ObservationIdentity(observations[i])
		}
	}
	return observations, nil
}

func registrationRole(registration Registration) model.SourceRole {
	if registration.Informational {
		return model.RoleHistory
	}
	return registration.Role
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
