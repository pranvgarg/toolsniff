# toolsniff Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a single Go binary that scans a macOS machine for installed dev/AI CLI tools across eight sources, shows them in an interactive Bubbletea TUI (with `--list`/`--json`/`--save`/`--diff` non-interactive modes), and tracks what's new since the last scan via a saved JSON baseline.

**Architecture:** Four packages behind clean interfaces — `model` (shared `Tool` struct), `scanner` (one file per source, all implementing a `Scanner` interface, run concurrently by `RunAll`), `registry` (JSON baseline load/save/diff), `output` (table, JSON, and Bubbletea TUI renderers, all consuming the same `[]model.Tool`). `main` wires flags to the right renderer.

**Tech Stack:** Go 1.22+ (installed: 1.26.2), stdlib `flag`/`encoding/json`/`os/exec` for everything except the TUI, which uses `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/bubbles` (for filtering/scrolling list), and `github.com/charmbracelet/lipgloss` (styling).

## Global Constraints

- Module path: `github.com/pranvgarg/toolsniff` (GitHub username confirmed via `gh api user`).
- macOS-only for v1 — no Linux package-manager scanners, no cross-platform path handling beyond what Go's stdlib gives for free.
- `npx-history` is informational only: excluded from `registry.Save`, excluded from `registry.ComputeDiff` inputs, excluded from the top-line tool count in every renderer. It gets its own tab/section everywhere it appears.
- Registry baseline lives at `~/.toolsniff/registry.json`, plain JSON (no YAML dependency).
- A scanner failing (tool not installed, directory missing) must never crash the program — it becomes a warning attached to that source, collected by `scanner.RunAll` and surfaced by every renderer, while every other scanner still completes.
- A missing or corrupt registry file is not an error — `registry.Load` returns an empty slice plus an informational message, never a hard failure, so a first run (or a wiped registry) just shows everything as new.
- Scanner tests must be hermetic: no test may depend on `brew`, `npm`, `pipx`, or any other tool actually being installed on the machine running `go test`. Every scanner that shells out takes an injectable `CommandRunner`; every scanner that reads the filesystem takes an injectable path.
- No automated tests for the Bubbletea TUI view layer in v1 — verified manually (documented as an explicit manual-verification step, not skipped silently).

---

## Wave Overview

Derived from each task's Interfaces block (Consumes/Produces), not authoring order. Skipped codebase research (step 1) — this is a greenfield project, nothing in the repo yet to ground dependency inference against. No clarifying questions were needed either: the dependency graph below falls out directly from the Interfaces blocks already written into each task, with one non-ambiguous judgment call noted under Wave 5.

| Wave | Delivers | Tasks | Depends on |
|---|---|---|---|
| 1 | Shared data model | 1 | — |
| 2 | Scanner contract, filesystem-based scanners, registry | 2, 4, 7, 8, 9, 10, 11 | Wave 1 |
| 3 | Command-based scanners, all three renderers | 3, 5, 6, 12, 13, 14 | Wave 2 |
| 4 | main.go wiring | 15 | Wave 3 |
| 5 | README | 16 | Wave 4 |

**Why Wave 2 groups scanner orchestration with registry and filesystem scanners:** none of `npx`/`cargo`/`applications`/`path`/`registry` consume `CommandRunner` — they only need `model.Tool` (Wave 1) — so per the Interfaces blocks they're exactly as unblocked as `scanner.go` itself. They land in the same wave as the `Scanner` interface because nothing in this set depends on anything else in it, which is what makes a wave valid, not because they're related in subject matter.

**Why Wave 3 groups npm/homebrew/pipx with table/JSON/TUI:** the three command-based scanners need `CommandRunner` (Wave 2). The three renderers need `registry.Diff` and `scanner.Warning` (also Wave 2) — but nothing in the renderers depends on any specific scanner's output, only on the generic `[]model.Tool` shape. Six tasks, no interdependency between any of them, same wave.

**The one judgment call — Task 16 (README) in its own Wave 5:** its Interfaces block says "Consumes: none" — by the letter of the algorithm it could sit in Wave 1. That's not a real option: the README documents install/usage instructions for the *built binary*, so it only makes sense to write after Wave 4 produces one. Not treating this as a question for you — there's no real second option here, just the algorithm's blind spot for documentation tasks that don't consume a Go interface but do depend on the thing being done.

---

## Wave 1: Shared Data Model

**Gate:** `go test ./model/...` passes. This is the single type every other package imports — nothing downstream is trustworthy until this is verified green.

### Task 1: Go module + `model.Tool`

**Files:**
- Create: `go.mod`
- Create: `model/tool.go`
- Test: `model/tool_test.go`

**Interfaces:**
- Produces: `model.Tool{ Name, Source, Version, Path string }` with JSON tags `name`, `source`, `version`, `path` — every other package in this plan imports and uses this exact struct.

- [ ] **Step 1: Initialize the Go module**

Run: `cd /Users/pranvgarg/Developer/poc/toolsniff && go mod init github.com/pranvgarg/toolsniff`
Expected: creates `go.mod` with `module github.com/pranvgarg/toolsniff` and a `go` directive.

- [ ] **Step 2: Write the failing test**

```go
// model/tool_test.go
package model

import (
	"encoding/json"
	"testing"
)

func TestToolJSONRoundTrip(t *testing.T) {
	original := Tool{Name: "ollama", Source: "applications", Version: "", Path: "/Applications/Ollama.app"}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Tool
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded != original {
		t.Errorf("round trip mismatch: got %+v, want %+v", decoded, original)
	}

	if got := string(data); got != `{"name":"ollama","source":"applications","version":"","path":"/Applications/Ollama.app"}` {
		t.Errorf("unexpected JSON shape: %s", got)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./model/...`
Expected: FAIL — `model` package / `Tool` type does not exist yet.

- [ ] **Step 4: Write minimal implementation**

```go
// model/tool.go
package model

// Tool is a single installed CLI tool or application discovered by a scanner.
type Tool struct {
	Name    string `json:"name"`
	Source  string `json:"source"`
	Version string `json:"version"`
	Path    string `json:"path"`
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./model/...`
Expected: `ok  	github.com/pranvgarg/toolsniff/model`

- [ ] **Step 6: Commit**

```bash
git add go.mod model/tool.go model/tool_test.go
git commit -m "Add model.Tool with JSON tags"
```

---

## Wave 2: Package Contracts (scanner orchestration, filesystem scanners, registry)

**Depends on:** Wave 1

**Gate:** `go build ./...` succeeds and the following all pass:
`go test ./scanner/... -run 'TestRunAll|TestNPXScanner|TestCargoScanner|TestApplicationsScanner|TestPathScanner'`
`go test ./registry/...`
Load-bearing: Wave 3 depends directly on the `Scanner` interface, `CommandRunner`, `RunAll`, and every `registry` function defined here. A silent failure here surfaces as confusing breakage two waves later.

### Task 2: Scanner interface + concurrent runner

**Files:**
- Create: `scanner/scanner.go`
- Test: `scanner/scanner_test.go`

**Interfaces:**
- Consumes: `model.Tool` (Task 1).
- Produces: `scanner.Scanner` interface (`Name() string`, `Scan() ([]model.Tool, error)`), `scanner.CommandRunner` type (`func(name string, args ...string) ([]byte, error)`), `scanner.ExecRunner` (a `CommandRunner` implementation), `scanner.Warning{ Source string, Err error }`, `scanner.RunAll(scanners []Scanner) ([]model.Tool, []Warning)`. Every scanner built in Tasks 3–9 implements `Scanner`; every `output` renderer in Tasks 12–14 consumes `[]Warning`.

- [ ] **Step 1: Write the failing test**

```go
// scanner/scanner_test.go
package scanner

import (
	"errors"
	"sort"
	"testing"

	"github.com/pranvgarg/toolsniff/model"
)

type stubScanner struct {
	name  string
	tools []model.Tool
	err   error
}

func (s stubScanner) Name() string                    { return s.name }
func (s stubScanner) Scan() ([]model.Tool, error) { return s.tools, s.err }

func TestRunAllCollectsToolsAndWarnings(t *testing.T) {
	scanners := []Scanner{
		stubScanner{name: "ok-a", tools: []model.Tool{{Name: "foo", Source: "ok-a"}}},
		stubScanner{name: "ok-b", tools: []model.Tool{{Name: "bar", Source: "ok-b"}}},
		stubScanner{name: "broken", err: errors.New("boom")},
	}

	tools, warnings := RunAll(scanners)

	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d: %+v", len(tools), tools)
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	if tools[0].Name != "bar" || tools[1].Name != "foo" {
		t.Errorf("unexpected tools: %+v", tools)
	}

	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %+v", len(warnings), warnings)
	}
	if warnings[0].Source != "broken" || warnings[0].Err.Error() != "boom" {
		t.Errorf("unexpected warning: %+v", warnings[0])
	}
}

func TestRunAllEmptyInput(t *testing.T) {
	tools, warnings := RunAll(nil)
	if len(tools) != 0 || len(warnings) != 0 {
		t.Errorf("expected empty results for empty input, got tools=%+v warnings=%+v", tools, warnings)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./scanner/...`
Expected: FAIL — `scanner` package does not exist yet.

- [ ] **Step 3: Write minimal implementation**

```go
// scanner/scanner.go
package scanner

import (
	"os/exec"
	"sync"

	"github.com/pranvgarg/toolsniff/model"
)

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
	cmd := exec.Command(name, args...)
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./scanner/... -run TestRunAll -v`
Expected: both tests PASS.

- [ ] **Step 5: Commit**

```bash
git add scanner/scanner.go scanner/scanner_test.go
git commit -m "Add Scanner interface and concurrent RunAll orchestrator"
```

---

### Task 4: npx history scanner

**Files:**
- Create: `scanner/npx.go`
- Test: `scanner/npx_test.go`

**Interfaces:**
- Consumes: `model.Tool` (Task 1).
- Produces: `scanner.NewNPXScanner(npxDir string) *NPXScanner`, `scanner.DefaultNPXDir() string`, implementing `Scanner` with `Name() == "npx-history"`.

**Implementation note:** npm's `_npx` cache directories don't reliably distinguish "top-level requested package" from "transitive dependency" in `package.json`'s `dependencies` field for every entry (verified against this machine's real cache — some entries list hundreds of transitive packages flattened). The reliable signal is `node_modules/.bin/*`: npx itself resolves what to execute by finding a same-named binary there, so reading those symlinks back to their owning package is the same mechanism npx uses internally, not a heuristic guess.

- [ ] **Step 1: Write the failing test**

```go
// scanner/npx_test.go
package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

// makeNPXFixture builds a fake ~/.npm/_npx directory with one hash dir
// containing a node_modules/.bin symlink pointing at a package, mimicking
// what `npx <pkg>` leaves behind.
func makeNPXFixture(t *testing.T, root string, hash, binName, pkgName string) {
	t.Helper()
	binDir := filepath.Join(root, hash, "node_modules", ".bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	pkgDir := filepath.Join(root, hash, "node_modules", pkgName)
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	target := filepath.Join("..", pkgName, "bin", "cli.js")
	if err := os.Symlink(target, filepath.Join(binDir, binName)); err != nil {
		t.Fatalf("symlink: %v", err)
	}
}

func TestNPXScannerResolvesPackageFromBinSymlink(t *testing.T) {
	root := t.TempDir()
	makeNPXFixture(t, root, "hash1", "create-vite", "create-vite")
	makeNPXFixture(t, root, "hash2", "prisma", "@prisma/cli")

	s := NewNPXScanner(root)
	if s.Name() != "npx-history" {
		t.Errorf("expected Name() == \"npx-history\", got %q", s.Name())
	}

	tools, err := s.Scan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d: %+v", len(tools), tools)
	}
	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.Name] = true
		if tool.Source != "npx-history" {
			t.Errorf("expected Source == \"npx-history\", got %q", tool.Source)
		}
	}
	if !names["create-vite"] || !names["@prisma/cli"] {
		t.Errorf("unexpected tool names: %+v", tools)
	}
}

func TestNPXScannerMissingDirIsNotAnError(t *testing.T) {
	s := NewNPXScanner(filepath.Join(t.TempDir(), "does-not-exist"))
	tools, err := s.Scan()
	if err != nil {
		t.Fatalf("expected no error for missing directory, got %v", err)
	}
	if len(tools) != 0 {
		t.Errorf("expected no tools, got %+v", tools)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./scanner/... -run TestNPXScanner`
Expected: FAIL — `NewNPXScanner` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
// scanner/npx.go
package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pranvgarg/toolsniff/model"
)

// NPXScanner reads npm's npx run-cache to find what packages have been
// invoked via `npx` (not npm-installed, just cached from a one-off run).
type NPXScanner struct {
	npxDir string
}

func NewNPXScanner(npxDir string) *NPXScanner {
	return &NPXScanner{npxDir: npxDir}
}

// DefaultNPXDir returns ~/.npm/_npx, npm's actual cache location.
func DefaultNPXDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".npm", "_npx")
}

func (s *NPXScanner) Name() string { return "npx-history" }

func (s *NPXScanner) Scan() ([]model.Tool, error) {
	entries, err := os.ReadDir(s.npxDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("npx-history: %w", err)
	}

	seen := map[string]time.Time{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		hashDir := filepath.Join(s.npxDir, entry.Name())
		binDir := filepath.Join(hashDir, "node_modules", ".bin")
		binEntries, err := os.ReadDir(binDir)
		if err != nil {
			continue
		}

		var modTime time.Time
		if info, err := entry.Info(); err == nil {
			modTime = info.ModTime()
		}

		for _, bin := range binEntries {
			target, err := os.Readlink(filepath.Join(binDir, bin.Name()))
			if err != nil {
				continue
			}
			pkgName := packageNameFromBinTarget(target)
			if pkgName == "" {
				continue
			}
			if existing, ok := seen[pkgName]; !ok || modTime.After(existing) {
				seen[pkgName] = modTime
			}
		}
	}

	tools := make([]model.Tool, 0, len(seen))
	for name, modTime := range seen {
		version := ""
		if !modTime.IsZero() {
			version = modTime.Format("2006-01-02")
		}
		tools = append(tools, model.Tool{Name: name, Source: "npx-history", Version: version})
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return tools, nil
}

// packageNameFromBinTarget extracts the owning package name from a
// node_modules/.bin symlink target, handling scoped packages (@scope/pkg).
func packageNameFromBinTarget(target string) string {
	parts := strings.Split(filepath.ToSlash(target), "/")
	for i, p := range parts {
		if p != "node_modules" || i+1 >= len(parts) {
			continue
		}
		if strings.HasPrefix(parts[i+1], "@") && i+2 < len(parts) {
			return parts[i+1] + "/" + parts[i+2]
		}
		return parts[i+1]
	}
	return ""
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./scanner/... -run TestNPXScanner -v`
Expected: both tests PASS.

- [ ] **Step 5: Commit**

```bash
git add scanner/npx.go scanner/npx_test.go
git commit -m "Add npx history scanner using .bin symlink resolution"
```

---

### Task 7: cargo scanner

**Files:**
- Create: `scanner/cargo.go`
- Test: `scanner/cargo_test.go`

**Interfaces:**
- Consumes: `model.Tool` (Task 1).
- Produces: `scanner.NewCargoScanner(binDir string) *CargoScanner`, `scanner.DefaultCargoBinDir() string`, `Name() == "cargo"`.

- [ ] **Step 1: Write the failing test**

```go
// scanner/cargo_test.go
package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCargoScannerListsBinaries(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"ripgrep", "bat"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatalf("write fixture binary: %v", err)
		}
	}

	s := NewCargoScanner(dir)
	if s.Name() != "cargo" {
		t.Errorf("expected Name() == \"cargo\", got %q", s.Name())
	}
	tools, err := s.Scan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 2 || tools[0].Name != "bat" || tools[0].Source != "cargo" {
		t.Errorf("unexpected tools: %+v", tools)
	}
}

func TestCargoScannerMissingDirIsNotAnError(t *testing.T) {
	s := NewCargoScanner(filepath.Join(t.TempDir(), "does-not-exist"))
	tools, err := s.Scan()
	if err != nil {
		t.Fatalf("expected no error for missing directory, got %v", err)
	}
	if len(tools) != 0 {
		t.Errorf("expected no tools, got %+v", tools)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./scanner/... -run TestCargoScanner`
Expected: FAIL — `NewCargoScanner` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
// scanner/cargo.go
package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/pranvgarg/toolsniff/model"
)

// CargoScanner lists binaries installed via `cargo install` into ~/.cargo/bin.
type CargoScanner struct {
	binDir string
}

func NewCargoScanner(binDir string) *CargoScanner {
	return &CargoScanner{binDir: binDir}
}

// DefaultCargoBinDir returns ~/.cargo/bin.
func DefaultCargoBinDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cargo", "bin")
}

func (s *CargoScanner) Name() string { return "cargo" }

func (s *CargoScanner) Scan() ([]model.Tool, error) {
	entries, err := os.ReadDir(s.binDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cargo: %w", err)
	}

	tools := make([]model.Tool, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		tools = append(tools, model.Tool{
			Name:   entry.Name(),
			Source: "cargo",
			Path:   filepath.Join(s.binDir, entry.Name()),
		})
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return tools, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./scanner/... -run TestCargoScanner -v`
Expected: both tests PASS.

- [ ] **Step 5: Commit**

```bash
git add scanner/cargo.go scanner/cargo_test.go
git commit -m "Add cargo scanner"
```

---

### Task 8: Applications scanner

**Files:**
- Create: `scanner/applications.go`
- Test: `scanner/applications_test.go`

**Interfaces:**
- Consumes: `model.Tool` (Task 1).
- Produces: `scanner.NewApplicationsScanner(dir string, keywords []string) *ApplicationsScanner`, `scanner.DefaultApplicationsDir() string`, `scanner.DefaultApplicationKeywords() []string`, `Name() == "applications"`.

**Implementation note:** `/Applications` on a real machine contains dozens of unrelated apps. The scanner filters to `.app` bundles whose lowercased name contains one of a curated keyword list, passed in explicitly (not a package-private var) so the caller in `main.go` can extend it later without touching this file.

- [ ] **Step 1: Write the failing test**

```go
// scanner/applications_test.go
package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplicationsScannerFiltersByKeyword(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"Ollama.app", "Preview.app", "Claude.app", "Calculator.app"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}

	s := NewApplicationsScanner(dir, []string{"ollama", "claude"})
	if s.Name() != "applications" {
		t.Errorf("expected Name() == \"applications\", got %q", s.Name())
	}
	tools, err := s.Scan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 matched apps, got %d: %+v", len(tools), tools)
	}
	names := map[string]bool{tools[0].Name: true, tools[1].Name: true}
	if !names["Ollama.app"] || !names["Claude.app"] {
		t.Errorf("unexpected matched apps: %+v", tools)
	}
}

func TestApplicationsScannerMissingDirIsNotAnError(t *testing.T) {
	s := NewApplicationsScanner(filepath.Join(t.TempDir(), "nope"), DefaultApplicationKeywords())
	tools, err := s.Scan()
	if err != nil {
		t.Fatalf("expected no error for missing directory, got %v", err)
	}
	if len(tools) != 0 {
		t.Errorf("expected no tools, got %+v", tools)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./scanner/... -run TestApplicationsScanner`
Expected: FAIL — `NewApplicationsScanner` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
// scanner/applications.go
package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pranvgarg/toolsniff/model"
)

// ApplicationsScanner finds dev/AI-relevant apps in /Applications, filtered
// by keyword since the directory otherwise contains everything on the machine.
type ApplicationsScanner struct {
	dir      string
	keywords []string
}

func NewApplicationsScanner(dir string, keywords []string) *ApplicationsScanner {
	return &ApplicationsScanner{dir: dir, keywords: keywords}
}

// DefaultApplicationsDir returns the standard macOS /Applications path.
func DefaultApplicationsDir() string { return "/Applications" }

// DefaultApplicationKeywords is the curated list of substrings (matched
// case-insensitively against app bundle names) considered dev/AI-relevant.
// Extend this list to widen what the scanner picks up.
func DefaultApplicationKeywords() []string {
	return []string{
		"claude", "chatgpt", "gpt", "cursor", "devin", "ollama",
		"lm studio", "antigravity", "finetune", "agents", "wispr",
		"copilot", "codex", "windsurf", "docker", "github desktop",
	}
}

func (s *ApplicationsScanner) Name() string { return "applications" }

func (s *ApplicationsScanner) Scan() ([]model.Tool, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("applications: %w", err)
	}

	tools := make([]model.Tool, 0)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".app") {
			continue
		}
		lower := strings.ToLower(name)
		matched := false
		for _, kw := range s.keywords {
			if strings.Contains(lower, kw) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		tools = append(tools, model.Tool{
			Name:   name,
			Source: "applications",
			Path:   filepath.Join(s.dir, name),
		})
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return tools, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./scanner/... -run TestApplicationsScanner -v`
Expected: both tests PASS.

- [ ] **Step 5: Commit**

```bash
git add scanner/applications.go scanner/applications_test.go
git commit -m "Add applications scanner with keyword filtering"
```

---

### Task 9: `$PATH` scanner

**Files:**
- Create: `scanner/path.go`
- Test: `scanner/path_test.go`

**Interfaces:**
- Consumes: `model.Tool` (Task 1).
- Produces: `scanner.PathLookup` type (`func(name string) (string, error)`, matching `exec.LookPath`'s signature), `scanner.NewPathScanner(lookup PathLookup, candidates []string) *PathScanner`, `scanner.DefaultPathCandidates() []string`, `Name() == "path"`.

**Implementation note:** `$PATH` can contain thousands of binaries (every system utility). Rather than enumerating everything, this scanner checks a curated candidate list — the same approach used to explore this machine earlier in the project. `main.go` wires the real lookup via `exec.LookPath`, which satisfies the `PathLookup` type directly.

- [ ] **Step 1: Write the failing test**

```go
// scanner/path_test.go
package scanner

import (
	"errors"
	"testing"
)

func TestPathScannerChecksCandidates(t *testing.T) {
	lookup := func(name string) (string, error) {
		switch name {
		case "claude":
			return "/opt/homebrew/bin/claude", nil
		case "ollama":
			return "/usr/local/bin/ollama", nil
		default:
			return "", errors.New("not found")
		}
	}

	s := NewPathScanner(lookup, []string{"claude", "ollama", "nonexistent-tool"})
	if s.Name() != "path" {
		t.Errorf("expected Name() == \"path\", got %q", s.Name())
	}
	tools, err := s.Scan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 found tools, got %d: %+v", len(tools), tools)
	}
	if tools[0].Name != "claude" || tools[0].Path != "/opt/homebrew/bin/claude" || tools[0].Source != "path" {
		t.Errorf("unexpected first tool: %+v", tools[0])
	}
}

func TestPathScannerEmptyCandidates(t *testing.T) {
	s := NewPathScanner(func(string) (string, error) { return "", errors.New("nope") }, nil)
	tools, err := s.Scan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 0 {
		t.Errorf("expected no tools, got %+v", tools)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./scanner/... -run TestPathScanner`
Expected: FAIL — `NewPathScanner` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
// scanner/path.go
package scanner

import (
	"sort"

	"github.com/pranvgarg/toolsniff/model"
)

// PathLookup resolves a command name to its full path, matching the
// signature of exec.LookPath so it can be passed directly in production.
type PathLookup func(name string) (string, error)

// PathScanner checks a curated candidate list against $PATH rather than
// enumerating every binary on the system.
type PathScanner struct {
	lookup     PathLookup
	candidates []string
}

func NewPathScanner(lookup PathLookup, candidates []string) *PathScanner {
	return &PathScanner{lookup: lookup, candidates: candidates}
}

// DefaultPathCandidates is the curated list of dev/AI CLI tool names to
// check for on $PATH. Extend this list to widen what the scanner picks up.
func DefaultPathCandidates() []string {
	return []string{
		"claude", "ollama", "opencode", "gemini", "codex", "pi", "mo",
		"gh", "vercel", "flyctl", "azd", "whisper-cpp", "aider",
		"cursor", "cline", "continue", "llm", "sgpt", "cody", "warp",
		"uv", "uvx", "ngrok",
	}
}

func (s *PathScanner) Name() string { return "path" }

func (s *PathScanner) Scan() ([]model.Tool, error) {
	tools := make([]model.Tool, 0)
	for _, name := range s.candidates {
		path, err := s.lookup(name)
		if err != nil {
			continue
		}
		tools = append(tools, model.Tool{Name: name, Source: "path", Path: path})
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return tools, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./scanner/... -run TestPathScanner -v`
Expected: both tests PASS.

- [ ] **Step 5: Run the full scanner suite**

Run: `go test ./scanner/... -v`
Expected: every test across Tasks 2–9 PASSes.

- [ ] **Step 6: Commit**

```bash
git add scanner/path.go scanner/path_test.go
git commit -m "Add \$PATH scanner with curated candidate list"
```

---

### Task 10: Registry load/save

**Files:**
- Create: `registry/registry.go`
- Test: `registry/registry_test.go`

**Interfaces:**
- Consumes: `model.Tool` (Task 1).
- Produces: `registry.DefaultRegistryPath() string`, `registry.Load(path string) (tools []model.Tool, warning string)`, `registry.Save(path string, tools []model.Tool) error`.

- [ ] **Step 1: Write the failing test**

```go
// registry/registry_test.go
package registry

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pranvgarg/toolsniff/model"
)

func TestSaveThenLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "registry.json")
	tools := []model.Tool{
		{Name: "gh", Source: "brew-formula"},
		{Name: "ollama", Source: "applications", Path: "/Applications/Ollama.app"},
	}

	if err := Save(path, tools); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, warning := Load(path)
	if warning != "" {
		t.Errorf("expected no warning, got %q", warning)
	}
	if len(loaded) != 2 || loaded[0].Name != "gh" || loaded[1].Path != "/Applications/Ollama.app" {
		t.Errorf("unexpected loaded tools: %+v", loaded)
	}
}

func TestLoadMissingFileReturnsEmptyNoWarning(t *testing.T) {
	tools, warning := Load(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if len(tools) != 0 {
		t.Errorf("expected no tools, got %+v", tools)
	}
	if warning != "" {
		t.Errorf("expected no warning for a first run, got %q", warning)
	}
}

func TestLoadCorruptFileReturnsEmptyWithWarning(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")
	if err := os.WriteFile(path, []byte("{not valid json"), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	tools, warning := Load(path)
	if len(tools) != 0 {
		t.Errorf("expected no tools for corrupt file, got %+v", tools)
	}
	if warning == "" {
		t.Error("expected a warning for a corrupt registry file")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./registry/...`
Expected: FAIL — `registry` package does not exist yet.

- [ ] **Step 3: Write minimal implementation**

```go
// registry/registry.go
package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pranvgarg/toolsniff/model"
)

// DefaultRegistryPath returns ~/.toolsniff/registry.json.
func DefaultRegistryPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".toolsniff", "registry.json")
}

// Load reads the saved baseline. A missing file is not an error — it just
// means there's no baseline yet, so every real install will show as new. A
// corrupt file is treated the same way, but with a warning explaining why.
func Load(path string) (tools []model.Tool, warning string) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ""
		}
		return nil, fmt.Sprintf("registry: reading %s: %v (treating as empty baseline)", path, err)
	}

	if err := json.Unmarshal(data, &tools); err != nil {
		return nil, fmt.Sprintf("registry: parsing %s: %v (treating as empty baseline)", path, err)
	}
	return tools, ""
}

// Save writes the current scan as the new baseline, creating the parent
// directory if needed.
func Save(path string, tools []model.Tool) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("registry: creating directory: %w", err)
	}
	data, err := json.MarshalIndent(tools, "", "  ")
	if err != nil {
		return fmt.Errorf("registry: marshaling: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("registry: writing %s: %w", path, err)
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./registry/... -v`
Expected: all three tests PASS.

- [ ] **Step 5: Commit**

```bash
git add registry/registry.go registry/registry_test.go
git commit -m "Add registry load/save at ~/.toolsniff/registry.json"
```

---

### Task 11: Registry diff

**Files:**
- Create: `registry/diff.go`
- Test: `registry/diff_test.go`

**Interfaces:**
- Consumes: `model.Tool` (Task 1).
- Produces: `registry.Diff{ Added, Removed []model.Tool }`, `registry.ComputeDiff(old, new []model.Tool) Diff`.

- [ ] **Step 1: Write the failing test**

```go
// registry/diff_test.go
package registry

import (
	"testing"

	"github.com/pranvgarg/toolsniff/model"
)

func TestComputeDiffAddedAndRemoved(t *testing.T) {
	old := []model.Tool{
		{Name: "gh", Source: "brew-formula"},
		{Name: "wget", Source: "brew-formula"},
	}
	current := []model.Tool{
		{Name: "gh", Source: "brew-formula"},
		{Name: "flyctl", Source: "brew-formula"},
	}

	diff := ComputeDiff(old, current)

	if len(diff.Added) != 1 || diff.Added[0].Name != "flyctl" {
		t.Errorf("unexpected Added: %+v", diff.Added)
	}
	if len(diff.Removed) != 1 || diff.Removed[0].Name != "wget" {
		t.Errorf("unexpected Removed: %+v", diff.Removed)
	}
}

func TestComputeDiffSameSourceDifferentNameNotConfused(t *testing.T) {
	old := []model.Tool{{Name: "gh", Source: "brew-formula"}}
	current := []model.Tool{{Name: "gh", Source: "path"}}

	diff := ComputeDiff(old, current)

	if len(diff.Added) != 1 || diff.Added[0].Source != "path" {
		t.Errorf("expected same-name-different-source to count as added: %+v", diff.Added)
	}
	if len(diff.Removed) != 1 || diff.Removed[0].Source != "brew-formula" {
		t.Errorf("expected same-name-different-source to count as removed: %+v", diff.Removed)
	}
}

func TestComputeDiffEmptyBaseline(t *testing.T) {
	current := []model.Tool{{Name: "gh", Source: "brew-formula"}}
	diff := ComputeDiff(nil, current)

	if len(diff.Added) != 1 || len(diff.Removed) != 0 {
		t.Errorf("expected everything to be Added on empty baseline, got %+v", diff)
	}
}

func TestComputeDiffNoChanges(t *testing.T) {
	tools := []model.Tool{{Name: "gh", Source: "brew-formula"}}
	diff := ComputeDiff(tools, tools)

	if len(diff.Added) != 0 || len(diff.Removed) != 0 {
		t.Errorf("expected no changes, got %+v", diff)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./registry/... -run TestComputeDiff`
Expected: FAIL — `ComputeDiff` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
// registry/diff.go
package registry

import (
	"sort"

	"github.com/pranvgarg/toolsniff/model"
)

// Diff describes what changed between a saved baseline and a fresh scan.
type Diff struct {
	Added   []model.Tool
	Removed []model.Tool
}

func toolKey(t model.Tool) string { return t.Source + "\x00" + t.Name }

// ComputeDiff compares an old baseline against a new scan, keyed on
// (Source, Name) so the same tool name from two different sources is never
// confused with itself.
func ComputeDiff(old, new []model.Tool) Diff {
	oldSet := make(map[string]model.Tool, len(old))
	for _, t := range old {
		oldSet[toolKey(t)] = t
	}
	newSet := make(map[string]model.Tool, len(new))
	for _, t := range new {
		newSet[toolKey(t)] = t
	}

	var added, removed []model.Tool
	for key, t := range newSet {
		if _, ok := oldSet[key]; !ok {
			added = append(added, t)
		}
	}
	for key, t := range oldSet {
		if _, ok := newSet[key]; !ok {
			removed = append(removed, t)
		}
	}

	sort.Slice(added, func(i, j int) bool { return added[i].Name < added[j].Name })
	sort.Slice(removed, func(i, j int) bool { return removed[i].Name < removed[j].Name })
	return Diff{Added: added, Removed: removed}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./registry/... -v`
Expected: all seven registry tests (Task 10 + 11) PASS.

- [ ] **Step 5: Commit**

```bash
git add registry/diff.go registry/diff_test.go
git commit -m "Add registry diff keyed on (Source, Name)"
```

---

## Wave 3: Command-Based Scanners and Renderers

**Depends on:** Wave 2

**Gate:** `go build ./...` succeeds and the following all pass:
`go test ./scanner/... -run 'TestNPMScanner|TestHomebrew|TestPipx'`
`go test ./output/...`
Plus the Task 14 manual TUI smoke test (tabs render, filter works, save/quit work) actually run and confirmed, not assumed. Load-bearing: `main.go` in Wave 4 wires every scanner constructor and every renderer produced here directly.

### Task 3: npm global scanner

**Files:**
- Create: `scanner/npm.go`
- Test: `scanner/npm_test.go`

**Interfaces:**
- Consumes: `model.Tool`, `scanner.CommandRunner` (Task 2).
- Produces: `scanner.NewNPMScanner(runner CommandRunner) *NPMScanner`, implementing `Scanner` with `Name() == "npm"`.

- [ ] **Step 1: Write the failing test**

```go
// scanner/npm_test.go
package scanner

import (
	"errors"
	"testing"
)

func TestNPMScannerParsesGlobalPackages(t *testing.T) {
	fixture := []byte(`{
  "name": "lib",
  "dependencies": {
    "npm": { "version": "10.9.2" },
    "opencode-ai": { "version": "1.18.4" }
  }
}`)
	runner := func(name string, args ...string) ([]byte, error) {
		if name != "npm" {
			t.Fatalf("expected command 'npm', got %q", name)
		}
		return fixture, nil
	}

	s := NewNPMScanner(runner)
	if s.Name() != "npm" {
		t.Errorf("expected Name() == \"npm\", got %q", s.Name())
	}

	tools, err := s.Scan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d: %+v", len(tools), tools)
	}
	if tools[0].Name != "npm" || tools[0].Version != "10.9.2" || tools[0].Source != "npm" {
		t.Errorf("unexpected first tool: %+v", tools[0])
	}
	if tools[1].Name != "opencode-ai" || tools[1].Version != "1.18.4" {
		t.Errorf("unexpected second tool: %+v", tools[1])
	}
}

func TestNPMScannerNotInstalled(t *testing.T) {
	runner := func(name string, args ...string) ([]byte, error) {
		return nil, errors.New("exec: \"npm\": executable file not found in $PATH")
	}
	s := NewNPMScanner(runner)
	tools, err := s.Scan()
	if err == nil {
		t.Fatal("expected an error when npm is not found")
	}
	if len(tools) != 0 {
		t.Errorf("expected no tools on error, got %+v", tools)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./scanner/... -run TestNPMScanner`
Expected: FAIL — `NewNPMScanner` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
// scanner/npm.go
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

func (s *NPMScanner) Name() string { return "npm" }

func (s *NPMScanner) Scan() ([]model.Tool, error) {
	out, runErr := s.runner("npm", "ls", "-g", "--depth=0", "--json")
	if len(out) == 0 {
		if runErr != nil {
			return nil, fmt.Errorf("npm: %w", runErr)
		}
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./scanner/... -run TestNPMScanner -v`
Expected: both tests PASS.

- [ ] **Step 5: Commit**

```bash
git add scanner/npm.go scanner/npm_test.go
git commit -m "Add npm global scanner"
```

---

### Task 5: Homebrew formula + cask scanners

**Files:**
- Create: `scanner/homebrew.go`
- Test: `scanner/homebrew_test.go`

**Interfaces:**
- Consumes: `model.Tool`, `scanner.CommandRunner` (Task 2).
- Produces: `scanner.NewHomebrewFormulaScanner(runner CommandRunner) *HomebrewFormulaScanner` (`Name() == "brew-formula"`), `scanner.NewHomebrewCaskScanner(runner CommandRunner) *HomebrewCaskScanner` (`Name() == "brew-cask"`).

- [ ] **Step 1: Write the failing test**

```go
// scanner/homebrew_test.go
package scanner

import (
	"errors"
	"testing"
)

func TestHomebrewFormulaScannerParsesOnePerLine(t *testing.T) {
	runner := func(name string, args ...string) ([]byte, error) {
		if name != "brew" {
			t.Fatalf("expected command 'brew', got %q", name)
		}
		if len(args) < 2 || args[0] != "list" || args[1] != "--formula" {
			t.Fatalf("expected 'brew list --formula ...', got %v", args)
		}
		return []byte("gh\nmole\nwget\n"), nil
	}

	s := NewHomebrewFormulaScanner(runner)
	if s.Name() != "brew-formula" {
		t.Errorf("expected Name() == \"brew-formula\", got %q", s.Name())
	}

	tools, err := s.Scan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d: %+v", len(tools), tools)
	}
	if tools[0].Name != "gh" || tools[0].Source != "brew-formula" {
		t.Errorf("unexpected first tool: %+v", tools[0])
	}
}

func TestHomebrewCaskScannerParsesOnePerLine(t *testing.T) {
	runner := func(name string, args ...string) ([]byte, error) {
		if len(args) < 2 || args[1] != "--cask" {
			t.Fatalf("expected 'brew list --cask ...', got %v", args)
		}
		return []byte("ollama\nlm-studio\n"), nil
	}

	s := NewHomebrewCaskScanner(runner)
	if s.Name() != "brew-cask" {
		t.Errorf("expected Name() == \"brew-cask\", got %q", s.Name())
	}
	tools, err := s.Scan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 2 || tools[0].Source != "brew-cask" {
		t.Errorf("unexpected tools: %+v", tools)
	}
}

func TestHomebrewScannerNotInstalled(t *testing.T) {
	runner := func(name string, args ...string) ([]byte, error) {
		return nil, errors.New("exec: \"brew\": executable file not found in $PATH")
	}
	s := NewHomebrewFormulaScanner(runner)
	tools, err := s.Scan()
	if err == nil {
		t.Fatal("expected an error when brew is not found")
	}
	if len(tools) != 0 {
		t.Errorf("expected no tools on error, got %+v", tools)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./scanner/... -run TestHomebrew`
Expected: FAIL — `NewHomebrewFormulaScanner` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
// scanner/homebrew.go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./scanner/... -run TestHomebrew -v`
Expected: all three tests PASS.

- [ ] **Step 5: Commit**

```bash
git add scanner/homebrew.go scanner/homebrew_test.go
git commit -m "Add Homebrew formula and cask scanners"
```

---

### Task 6: pipx scanner

**Files:**
- Create: `scanner/pipx.go`
- Test: `scanner/pipx_test.go`

**Interfaces:**
- Consumes: `model.Tool`, `scanner.CommandRunner` (Task 2).
- Produces: `scanner.NewPipxScanner(runner CommandRunner) *PipxScanner`, `Name() == "pipx"`.

- [ ] **Step 1: Write the failing test**

```go
// scanner/pipx_test.go
package scanner

import "testing"

func TestPipxScannerParsesVenvs(t *testing.T) {
	fixture := []byte(`{
  "venvs": {
    "black": {
      "metadata": { "main_package": { "package_version": "24.1.0" } }
    }
  }
}`)
	runner := func(name string, args ...string) ([]byte, error) {
		if name != "pipx" {
			t.Fatalf("expected command 'pipx', got %q", name)
		}
		return fixture, nil
	}

	s := NewPipxScanner(runner)
	if s.Name() != "pipx" {
		t.Errorf("expected Name() == \"pipx\", got %q", s.Name())
	}
	tools, err := s.Scan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "black" || tools[0].Version != "24.1.0" || tools[0].Source != "pipx" {
		t.Errorf("unexpected tools: %+v", tools)
	}
}

func TestPipxScannerEmptyVenvs(t *testing.T) {
	runner := func(name string, args ...string) ([]byte, error) {
		return []byte(`{"venvs": {}}`), nil
	}
	s := NewPipxScanner(runner)
	tools, err := s.Scan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 0 {
		t.Errorf("expected no tools, got %+v", tools)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./scanner/... -run TestPipx`
Expected: FAIL — `NewPipxScanner` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
// scanner/pipx.go
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

func (s *PipxScanner) Name() string { return "pipx" }

func (s *PipxScanner) Scan() ([]model.Tool, error) {
	out, runErr := s.runner("pipx", "list", "--json")
	if len(out) == 0 {
		if runErr != nil {
			return nil, fmt.Errorf("pipx: %w", runErr)
		}
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./scanner/... -run TestPipx -v`
Expected: both tests PASS.

- [ ] **Step 5: Commit**

```bash
git add scanner/pipx.go scanner/pipx_test.go
git commit -m "Add pipx scanner"
```

---

### Task 12: Table output (`--list`)

**Files:**
- Create: `output/table.go`
- Test: `output/table_test.go`

**Interfaces:**
- Consumes: `model.Tool` (Task 1), `registry.Diff` (Task 11), `scanner.Warning` (Task 2).
- Produces: `output.RenderTable(tools, npxHistory []model.Tool, diff registry.Diff, warnings []scanner.Warning) string`, `output.RenderDiff(diff registry.Diff) string`.

- [ ] **Step 1: Write the failing test**

```go
// output/table_test.go
package output

import (
	"strings"
	"testing"

	"github.com/pranvgarg/toolsniff/model"
	"github.com/pranvgarg/toolsniff/registry"
	"github.com/pranvgarg/toolsniff/scanner"
)

func TestRenderTableGroupsBySourceAndShowsCounts(t *testing.T) {
	tools := []model.Tool{
		{Name: "gh", Source: "brew-formula"},
		{Name: "wget", Source: "brew-formula"},
		{Name: "opencode-ai", Source: "npm", Version: "1.18.4"},
	}
	npxHistory := []model.Tool{{Name: "create-vite", Source: "npx-history", Version: "2026-06-13"}}
	diff := registry.Diff{Added: []model.Tool{{Name: "flyctl", Source: "brew-formula"}}}
	warnings := []scanner.Warning{{Source: "pipx", Err: errFixture{"pipx not found"}}}

	out := RenderTable(tools, npxHistory, diff, warnings)

	for _, want := range []string{
		"BREW-FORMULA (2)",
		"gh",
		"wget",
		"NPM (1)",
		"opencode-ai",
		"1.18.4",
		"NPX HISTORY (1, informational)",
		"create-vite",
		"NEW SINCE LAST SCAN",
		"+ flyctl",
		"warning: pipx",
		"3 tools across 2 sources",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

type errFixture struct{ msg string }

func (e errFixture) Error() string { return e.msg }
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./output/... -run TestRenderTable`
Expected: FAIL — `output` package does not exist yet.

- [ ] **Step 3: Write minimal implementation**

```go
// output/table.go
package output

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pranvgarg/toolsniff/model"
	"github.com/pranvgarg/toolsniff/registry"
	"github.com/pranvgarg/toolsniff/scanner"
)

func groupBySource(tools []model.Tool) map[string][]model.Tool {
	grouped := map[string][]model.Tool{}
	for _, t := range tools {
		grouped[t.Source] = append(grouped[t.Source], t)
	}
	return grouped
}

func sortedSourceKeys(grouped map[string][]model.Tool) []string {
	keys := make([]string, 0, len(grouped))
	for k := range grouped {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// RenderTable produces a plain grouped-by-source table for --list.
func RenderTable(tools, npxHistory []model.Tool, diff registry.Diff, warnings []scanner.Warning) string {
	var b strings.Builder

	grouped := groupBySource(tools)
	sources := sortedSourceKeys(grouped)
	for _, src := range sources {
		fmt.Fprintf(&b, "%s (%d)\n", strings.ToUpper(src), len(grouped[src]))
		for _, t := range grouped[src] {
			if t.Version != "" {
				fmt.Fprintf(&b, "  %-30s %s\n", t.Name, t.Version)
			} else {
				fmt.Fprintf(&b, "  %s\n", t.Name)
			}
		}
		b.WriteString("\n")
	}

	if len(npxHistory) > 0 {
		fmt.Fprintf(&b, "NPX HISTORY (%d, informational)\n", len(npxHistory))
		for _, t := range npxHistory {
			fmt.Fprintf(&b, "  %-30s %s\n", t.Name, t.Version)
		}
		b.WriteString("\n")
	}

	if len(diff.Added) > 0 || len(diff.Removed) > 0 {
		b.WriteString("NEW SINCE LAST SCAN\n")
		b.WriteString(RenderDiff(diff))
		b.WriteString("\n")
	}

	for _, w := range warnings {
		fmt.Fprintf(&b, "warning: %s: %v\n", w.Source, w.Err)
	}

	fmt.Fprintf(&b, "%d tools across %d sources\n", len(tools), len(sources))
	return b.String()
}

// RenderDiff renders just the added/removed tools, used by --diff and
// embedded into RenderTable.
func RenderDiff(diff registry.Diff) string {
	if len(diff.Added) == 0 && len(diff.Removed) == 0 {
		return "no changes since last scan\n"
	}
	var b strings.Builder
	for _, t := range diff.Added {
		fmt.Fprintf(&b, "  + %s (%s)\n", t.Name, t.Source)
	}
	for _, t := range diff.Removed {
		fmt.Fprintf(&b, "  - %s (%s)\n", t.Name, t.Source)
	}
	return b.String()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./output/... -run TestRenderTable -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add output/table.go output/table_test.go
git commit -m "Add plain table renderer for --list"
```

---

### Task 13: JSON output (`--json`)

**Files:**
- Create: `output/json.go`
- Test: `output/json_test.go`

**Interfaces:**
- Consumes: `model.Tool`, `registry.Diff`, `scanner.Warning` (as in Task 12).
- Produces: `output.JSONReport{ Tools, NPXHistory, Added, Removed []model.Tool; Warnings []string }`, `output.RenderJSON(tools, npxHistory []model.Tool, diff registry.Diff, warnings []scanner.Warning) ([]byte, error)`.

- [ ] **Step 1: Write the failing test**

```go
// output/json_test.go
package output

import (
	"encoding/json"
	"testing"

	"github.com/pranvgarg/toolsniff/model"
	"github.com/pranvgarg/toolsniff/registry"
	"github.com/pranvgarg/toolsniff/scanner"
)

func TestRenderJSONShape(t *testing.T) {
	tools := []model.Tool{{Name: "gh", Source: "brew-formula"}}
	npxHistory := []model.Tool{{Name: "create-vite", Source: "npx-history"}}
	diff := registry.Diff{Added: []model.Tool{{Name: "flyctl", Source: "brew-formula"}}}
	warnings := []scanner.Warning{{Source: "pipx", Err: errFixture{"not found"}}}

	data, err := RenderJSON(tools, npxHistory, diff, warnings)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var report JSONReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, data)
	}

	if len(report.Tools) != 1 || report.Tools[0].Name != "gh" {
		t.Errorf("unexpected Tools: %+v", report.Tools)
	}
	if len(report.NPXHistory) != 1 || report.NPXHistory[0].Name != "create-vite" {
		t.Errorf("unexpected NPXHistory: %+v", report.NPXHistory)
	}
	if len(report.Added) != 1 || report.Added[0].Name != "flyctl" {
		t.Errorf("unexpected Added: %+v", report.Added)
	}
	if len(report.Warnings) != 1 || report.Warnings[0] != "pipx: not found" {
		t.Errorf("unexpected Warnings: %+v", report.Warnings)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./output/... -run TestRenderJSON`
Expected: FAIL — `RenderJSON` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
// output/json.go
package output

import (
	"encoding/json"
	"fmt"

	"github.com/pranvgarg/toolsniff/model"
	"github.com/pranvgarg/toolsniff/registry"
	"github.com/pranvgarg/toolsniff/scanner"
)

// JSONReport is the full --json output shape.
type JSONReport struct {
	Tools      []model.Tool `json:"tools"`
	NPXHistory []model.Tool `json:"npx_history"`
	Added      []model.Tool `json:"added"`
	Removed    []model.Tool `json:"removed"`
	Warnings   []string     `json:"warnings"`
}

// RenderJSON produces the full scan report as indented JSON.
func RenderJSON(tools, npxHistory []model.Tool, diff registry.Diff, warnings []scanner.Warning) ([]byte, error) {
	report := JSONReport{
		Tools:      tools,
		NPXHistory: npxHistory,
		Added:      diff.Added,
		Removed:    diff.Removed,
		Warnings:   make([]string, 0, len(warnings)),
	}
	for _, w := range warnings {
		report.Warnings = append(report.Warnings, fmt.Sprintf("%s: %v", w.Source, w.Err))
	}
	return json.MarshalIndent(report, "", "  ")
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./output/... -v`
Expected: all table + JSON tests PASS.

- [ ] **Step 5: Commit**

```bash
git add output/json.go output/json_test.go
git commit -m "Add JSON renderer for --json"
```

---

### Task 14: Bubbletea TUI

**Files:**
- Create: `output/tui_model.go`
- Create: `output/tui_item.go`
- Create: `output/tui_styles.go`

**Interfaces:**
- Consumes: `model.Tool`, `registry.Diff`, `scanner.Warning`, `registry.Save` (Tasks 1, 2, 10, 11).
- Produces: `output.RunTUI(realTools, npxHistory []model.Tool, diff registry.Diff, warnings []scanner.Warning, regPath string) error`.

This task has no automated tests per the spec's decision to skip TUI testing in v1 — verification is manual (Step 4 below). Everything else follows the same commit discipline as prior tasks.

- [ ] **Step 1: Add the TUI dependencies**

Run:
```bash
cd /Users/pranvgarg/Developer/poc/toolsniff
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/bubbles@latest
go get github.com/charmbracelet/lipgloss@latest
go mod tidy
```
Expected: `go.mod` and `go.sum` gain the three dependencies with no errors.

- [ ] **Step 2: Write the item and styles files**

```go
// output/tui_item.go
package output

import "github.com/pranvgarg/toolsniff/model"

// toolItem adapts model.Tool to bubbles/list's Item interface.
type toolItem struct {
	tool model.Tool
}

func (i toolItem) Title() string { return i.tool.Name }

func (i toolItem) Description() string {
	if i.tool.Version != "" {
		return i.tool.Version
	}
	return i.tool.Path
}

func (i toolItem) FilterValue() string { return i.tool.Name }
```

```go
// output/tui_styles.go
package output

import "github.com/charmbracelet/lipgloss"

var (
	colorAmber = lipgloss.Color("#ffb454")
	colorCyan  = lipgloss.Color("#7fd8c4")
	colorMuted = lipgloss.Color("#5c6577")

	activeTabStyle = lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Underline(true)
	tabStyle       = lipgloss.NewStyle().Foreground(colorMuted)
	footerStyle    = lipgloss.NewStyle().Foreground(colorMuted).MarginTop(1)
	statusStyle    = lipgloss.NewStyle().Foreground(colorAmber)
)
```

```go
// output/tui_model.go
package output

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/list"
	"github.com/pranvgarg/toolsniff/model"
	"github.com/pranvgarg/toolsniff/registry"
	"github.com/pranvgarg/toolsniff/scanner"
)

// tabOrder is the fixed, meaningful order tabs appear in: real sources
// first (in scan-priority order), then npx-history (informational), then
// "new" (only present when there's an actual diff).
var tabOrder = []string{
	"npm", "brew-formula", "brew-cask", "pipx", "cargo",
	"applications", "path", "npx-history",
}

type tuiModel struct {
	tabs       []string
	toolsBySrc map[string][]model.Tool
	activeTab  int
	list       list.Model
	realTools  []model.Tool
	regPath    string
	statusMsg  string
}

func newTUIModel(realTools, npxHistory []model.Tool, diff registry.Diff, regPath string) tuiModel {
	toolsBySrc := map[string][]model.Tool{}
	for _, t := range realTools {
		toolsBySrc[t.Source] = append(toolsBySrc[t.Source], t)
	}
	toolsBySrc["npx-history"] = npxHistory

	if len(diff.Added) > 0 || len(diff.Removed) > 0 {
		newTab := append([]model.Tool{}, diff.Added...)
		newTab = append(newTab, diff.Removed...)
		toolsBySrc["new"] = newTab
	}

	tabs := make([]string, 0, len(tabOrder)+1)
	for _, src := range tabOrder {
		if _, ok := toolsBySrc[src]; ok {
			tabs = append(tabs, src)
		}
	}
	if _, ok := toolsBySrc["new"]; ok {
		tabs = append(tabs, "new")
	}
	if len(tabs) == 0 {
		tabs = []string{"npm"}
	}

	l := list.New(itemsFor(toolsBySrc[tabs[0]]), list.NewDefaultDelegate(), 0, 0)
	l.Title = tabs[0]

	return tuiModel{
		tabs:       tabs,
		toolsBySrc: toolsBySrc,
		list:       l,
		realTools:  realTools,
		regPath:    regPath,
	}
}

func itemsFor(tools []model.Tool) []list.Item {
	items := make([]list.Item, len(tools))
	for i, t := range tools {
		items[i] = toolItem{tool: t}
	}
	return items
}

func (m tuiModel) Init() tea.Cmd { return nil }

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height-4)
		return m, nil
	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			return m, cmd
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.activeTab = (m.activeTab + 1) % len(m.tabs)
			m.list.SetItems(itemsFor(m.toolsBySrc[m.tabs[m.activeTab]]))
			m.list.Title = m.tabs[m.activeTab]
			return m, nil
		case "s":
			if err := registry.Save(m.regPath, m.realTools); err != nil {
				m.statusMsg = "save failed: " + err.Error()
			} else {
				m.statusMsg = fmt.Sprintf("saved baseline: %d tools", len(m.realTools))
			}
			return m, nil
		case "d":
			for i, t := range m.tabs {
				if t == "new" {
					m.activeTab = i
					m.list.SetItems(itemsFor(m.toolsBySrc["new"]))
					m.list.Title = "new"
					break
				}
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m tuiModel) View() string {
	tabBar := renderTabBar(m.tabs, m.activeTab, m.toolsBySrc)
	footer := footerStyle.Render("↑↓ move · tab switch · / filter · d diff · s save · q quit")
	status := ""
	if m.statusMsg != "" {
		status = statusStyle.Render(m.statusMsg)
	}
	parts := []string{tabBar, m.list.View()}
	if status != "" {
		parts = append(parts, status)
	}
	parts = append(parts, footer)
	return strings.Join(parts, "\n")
}

func renderTabBar(tabs []string, active int, toolsBySrc map[string][]model.Tool) string {
	parts := make([]string, len(tabs))
	for i, t := range tabs {
		label := fmt.Sprintf("%s (%d)", t, len(toolsBySrc[t]))
		if i == active {
			parts[i] = activeTabStyle.Render(label)
		} else {
			parts[i] = tabStyle.Render(label)
		}
	}
	return strings.Join(parts, "  ")
}

// RunTUI launches the interactive Bubbletea program. warnings is accepted
// for interface symmetry with the other renderers; v1 surfaces warnings via
// the status line only when the save action itself fails (see "s" handler
// above) — a startup warnings banner is a documented follow-up, not part of
// this task.
func RunTUI(realTools, npxHistory []model.Tool, diff registry.Diff, warnings []scanner.Warning, regPath string) error {
	p := tea.NewProgram(newTUIModel(realTools, npxHistory, diff, regPath), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
```

- [ ] **Step 3: Verify it builds**

Run: `go build ./...`
Expected: no compile errors.

- [ ] **Step 4: Manual smoke test**

Run: `go run . ` (from `/Users/pranvgarg/Developer/poc/toolsniff`, with a `main.go` stub — if Task 15 isn't done yet, temporarily add a throwaway `main.go` that calls `output.RunTUI` with a couple of fixture tools to test in isolation; delete the stub before Task 15's real `main.go` is written)

Verify manually:
- Tabs render, `tab` key cycles through them, active tab is visually distinct (cyan/underlined).
- `↑`/`↓` move the selection in the list.
- `/` opens the built-in filter, typing narrows the list, `esc` clears it.
- `s` shows a "saved baseline" status message.
- `q` exits cleanly back to the shell prompt (no leftover alt-screen garbage).

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum output/tui_model.go output/tui_item.go output/tui_styles.go
git commit -m "Add Bubbletea TUI with tab navigation, filtering, and save"
```

---

## Wave 4: Assembly

**Depends on:** Wave 3

**Gate:** `go build -o toolsniff .` succeeds, `go test ./...` is green across every package, and all six manual end-to-end checks in Task 15 Step 4 are run against this real machine and confirmed (--list, --json, --diff before/after --save, and the bare TUI). This is the actual deliverable — verify it for real, not by reading the task's own completion report.

### Task 15: `main.go` wiring + end-to-end verification

**Files:**
- Create: `main.go`

**Interfaces:**
- Consumes: everything produced by Tasks 1–14.
- Produces: the `toolsniff` binary's entry point — no further consumers.

- [ ] **Step 1: Write `main.go`**

```go
// main.go
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"

	"github.com/pranvgarg/toolsniff/model"
	"github.com/pranvgarg/toolsniff/output"
	"github.com/pranvgarg/toolsniff/registry"
	"github.com/pranvgarg/toolsniff/scanner"
)

func buildScanners() []scanner.Scanner {
	return []scanner.Scanner{
		scanner.NewNPMScanner(scanner.ExecRunner),
		scanner.NewNPXScanner(scanner.DefaultNPXDir()),
		scanner.NewHomebrewFormulaScanner(scanner.ExecRunner),
		scanner.NewHomebrewCaskScanner(scanner.ExecRunner),
		scanner.NewPipxScanner(scanner.ExecRunner),
		scanner.NewCargoScanner(scanner.DefaultCargoBinDir()),
		scanner.NewApplicationsScanner(scanner.DefaultApplicationsDir(), scanner.DefaultApplicationKeywords()),
		scanner.NewPathScanner(exec.LookPath, scanner.DefaultPathCandidates()),
	}
}

func splitNPXHistory(tools []model.Tool) (real, npxHistory []model.Tool) {
	for _, t := range tools {
		if t.Source == "npx-history" {
			npxHistory = append(npxHistory, t)
		} else {
			real = append(real, t)
		}
	}
	return real, npxHistory
}

func main() {
	listFlag := flag.Bool("list", false, "print a plain grouped table and exit")
	jsonFlag := flag.Bool("json", false, "print the full scan as JSON and exit")
	saveFlag := flag.Bool("save", false, "scan, save the result as the new baseline, and exit")
	diffFlag := flag.Bool("diff", false, "scan, print only what changed since the last save, and exit")
	flag.Parse()

	tools, warnings := scanner.RunAll(buildScanners())
	realTools, npxHistory := splitNPXHistory(tools)

	regPath := registry.DefaultRegistryPath()
	baseline, regWarning := registry.Load(regPath)
	if regWarning != "" {
		warnings = append(warnings, scanner.Warning{Source: "registry", Err: errors.New(regWarning)})
	}
	diff := registry.ComputeDiff(baseline, realTools)

	switch {
	case *saveFlag:
		if err := registry.Save(regPath, realTools); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("saved baseline: %d tools\n", len(realTools))

	case *diffFlag:
		fmt.Print(output.RenderDiff(diff))

	case *jsonFlag:
		data, err := output.RenderJSON(realTools, npxHistory, diff, warnings)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(string(data))

	case *listFlag:
		fmt.Print(output.RenderTable(realTools, npxHistory, diff, warnings))

	default:
		if err := output.RunTUI(realTools, npxHistory, diff, warnings, regPath); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
```

If a throwaway `main.go` stub was created for Task 14's manual test, delete it before adding this file.

- [ ] **Step 2: Build the binary**

Run: `cd /Users/pranvgarg/Developer/poc/toolsniff && go build -o toolsniff .`
Expected: produces a `toolsniff` binary with no errors (already gitignored from Task setup).

- [ ] **Step 3: Run the full test suite**

Run: `go test ./...`
Expected: every package (`model`, `scanner`, `registry`, `output`) reports `ok`.

- [ ] **Step 4: End-to-end manual verification against the real machine**

Run each of these from `/Users/pranvgarg/Developer/poc/toolsniff` and confirm the described behavior:

1. `./toolsniff --list` — prints a grouped table; counts should roughly match what was found manually earlier in this project (13 npm globals, ~75 brew formulae, 9 casks, etc.), plus an `NPX HISTORY (..., informational)` section, and no crash even though there's no registry yet.
2. `./toolsniff --json | python3 -m json.tool > /dev/null` — exits 0, meaning the output is valid JSON.
3. `./toolsniff --diff` — since there's no saved baseline yet, prints `no changes since last scan` (per the spec: no baseline is treated as empty, and `ComputeDiff(nil, current)` would normally show everything as "added" — confirm which behavior actually happens and make sure it's not confusing; if it shows every tool as "added" on a fresh machine, note that as expected first-run behavior, not a bug).
4. `./toolsniff --save` — prints `saved baseline: N tools`, and `~/.toolsniff/registry.json` now exists.
5. `./toolsniff --diff` again — now prints `no changes since last scan` (nothing changed between steps 3 and 5).
6. `./toolsniff` (bare) — launches the TUI; confirm the same manual checks from Task 14 Step 4 still pass, and additionally that pressing `d` jumps to a tab (even if empty, since nothing changed) without crashing.

- [ ] **Step 5: Commit**

```bash
git add main.go
git commit -m "Wire scanners, registry, and output renderers together in main"
```

---

## Wave 5: Documentation

**Depends on:** Wave 4

**Gate:** Leaf wave, nothing downstream depends on it — light gate: README committed, and the PATH-check step actually run once so the instructions match this machine's real state.

### Task 16: README and PATH setup

**Files:**
- Create: `README.md`

**Interfaces:**
- None — this is documentation, no code consumes or produces anything here.

- [ ] **Step 1: Write the README**

```markdown
# toolsniff

Scans this machine for installed dev/AI CLI tools across npm, npx history,
Homebrew (formulae + casks), pipx, cargo, `/Applications`, and `$PATH`, and
shows them in an interactive terminal UI. Tracks what's new since the last
scan via a saved baseline.

## Install

Requires Go 1.22+.

```bash
git clone https://github.com/pranvgarg/toolsniff.git
cd toolsniff
go build -o toolsniff .
```

Put the binary on your `$PATH`, e.g.:

```bash
mkdir -p ~/go/bin
mv toolsniff ~/go/bin/
# add once to ~/.zshrc if not already there:
echo 'export PATH="$HOME/go/bin:$PATH"' >> ~/.zshrc
```

## Usage

```
toolsniff                 # launch the interactive TUI (default)
toolsniff --list          # plain grouped table, exits
toolsniff --json          # full scan as JSON, exits
toolsniff --save          # save current scan as the new baseline
toolsniff --diff          # show only what changed since the last save
```

TUI keys: `↑↓` move · `tab` switch pane · `/` filter · `d` jump to diff ·
`s` save baseline · `q` quit.

## Scope

macOS only for now. Linux support (different package managers, no
`/Applications` equivalent) is a planned follow-up — see
`docs/superpowers/specs/2026-07-30-toolsniff-design.md`.
```

- [ ] **Step 2: Verify the PATH instructions actually work on this machine**

Run: `echo $PATH | tr ':' '\n' | grep "go/bin"`
Expected: no output yet (confirms the gap flagged during planning is still real, so the README instructions are worth keeping — don't skip this check, since if it's already on PATH the README should say so instead).

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "Add README with install and usage instructions"
```

---

## Post-Plan Follow-Ups (not part of this implementation)

- Linux scanner support (v2).
- Homebrew tap / `goreleaser` packaging for public distribution.
- Startup-warnings banner in the TUI (currently warnings only surface via `--list`/`--json`; the TUI only shows a status message after `s` fails).
