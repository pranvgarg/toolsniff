# toolsniff Next Implementation Roadmap

This document records the next product and engineering work after the
`v0.1.0` release. It is intended to be the starting point for the next
implementation session.

## Current Product Direction

toolsniff is a macOS inventory tool for developer tools and AI applications.
It discovers tools through package managers, application roots, Bun global
binaries, and executable files on `PATH`.

The product promise is:

> Show what is installed, where it came from, what is currently available, and
> what changed over time.

The current release is distributed through Homebrew as `v0.1.0`.

## Current Architecture

```text
main.go
  -> config.Load()
  -> buildScanners()
  -> scanner.RunAll()
  -> model.DeduplicateTools()
  -> splitByRole()
  -> registry.Load()/ComputeDiff()
  -> output.RenderTable()
  -> output.RenderJSON()
  -> output.RunTUI()
```

Current packages:

- `model` — tool observations, source roles, identity, deduplication.
- `scanner` — concurrent scanner orchestration and scanner implementations.
- `config` — TOML loading, platform defaults, environment overrides, themes.
- `registry` — baseline persistence and change detection.
- `output` — table output, JSON output, Bubble Tea TUI, and Lip Gloss themes.
- `internal/version` — development fallback and release-time version injection.

## Important Correctness Work First

These items should be completed before adding many new scanners or UI
features.

### 1. Separate Installed Baselines from PATH Availability

Current `main.go` sends every non-history observation into `realTools`. This
includes `RoleAvailable` observations from the PATH scanner. The baseline
therefore currently stores both installations and PATH availability.

That can create misleading behavior:

- A PATH directory change can look like an installation change.
- A command becoming unavailable can appear as a removed installed tool.
- `--diff` does not clearly distinguish installation changes from availability
  changes.

Recommended model:

```text
Installed observations
  npm
  brew-formula
  brew-cask
  pipx
  cargo
  bun
  applications

Availability observations
  path

History observations
  npx-history
```

Recommended behavior:

```text
toolsniff --save              save installed observations
toolsniff --diff              show installed changes
toolsniff --diff --available  show PATH availability changes
```

Implementation direction:

- Split tools by `SourceRole` before registry load/save.
- Keep separate availability data if PATH history is desired.
- Do not merge installed and available observations.
- Keep the UI source tabs truthful.
- Update README and JSON output semantics.
- Add tests proving PATH observations do not enter the installed baseline.

### 2. Detect Version Upgrades and Downgrades

Current registry identity is path-aware:

```text
Source + Path
or
Source + Name
```

That correctly preserves different installation sources, but it means a
version change with the same identity is currently treated as unchanged.

Example that is currently missed:

```text
opencode-ai  1.18.10 -> 1.19.0
```

Extend the registry diff model:

```go
type Diff struct {
    Added   []model.Tool
    Removed []model.Tool
    Updated []ToolChange
}

type ToolChange struct {
    Before model.Tool
    After  model.Tool
}
```

Define update behavior:

- Version changed: report an update.
- Path changed: report removal plus addition, unless the source explicitly
  supports relocation semantics.
- Role changed: report an update or replacement, depending on source identity.
- Empty-to-known version: report an update when useful.
- Known-to-empty version: do not create noisy changes unless configured.

Update all renderers:

```text
UPDATED
  opencode-ai  1.18.10 -> 1.19.0
```

JSON should include:

```json
{
  "updated": [
    {
      "before": {"name": "tool", "version": "1.0.0"},
      "after": {"name": "tool", "version": "1.1.0"}
    }
  ]
}
```

## High-Value Features

### 3. Tool Details Panel

When a user selects a tool and presses `enter`, open a detail view.

Example:

```text
Tool Details

Name:       gh
Source:     brew-formula
Role:       installed
Version:    2.75.0
Path:       /opt/homebrew/bin/gh
```

Application details can include:

```text
Bundle ID:  com.anthropic.claudefordesktop
Version:    1.2.3
Signed:     yes
```

Suggested controls:

- `enter` — open details.
- `esc` — close details.
- `p` — copy the path.
- `o` — reveal the item in Finder.
- `c` — copy a source/install description.

Keep actions read-only and safe. Do not add uninstall or update actions to the
first version of this panel.

Likely implementation:

- Add `ToolDetails` rendering in `output`.
- Keep the selected `model.Tool` in `tuiModel`.
- Use `atotto/clipboard` for copy behavior, or a small output abstraction.
- Use `open -R <path>` only for Finder reveal.
- Add tests for detail formatting and missing fields.

### 4. Global Search

The current `/` filter searches only the active source. Add a global search
mode that searches across all observations.

Search fields:

- Tool name.
- Source.
- Role.
- Version.
- Path.

Example:

```text
/gh

gh  brew-formula  installed
gh  path          available
```

Suggested behavior:

- Keep the current source filter unchanged.
- Add a separate global search command.
- Preserve source tabs in global results.
- Show match counts and the source for every result.
- Make `esc` return to the previous tab.

### 5. Proper Command Palette

The current `/theme` implementation is a special case inside the filter
state. It works, but it does not scale well as more commands are added.

Replace it with a dedicated command mode:

```text
/

theme       Change the TUI theme
search      Search all sources
refresh     Run a new scan
save        Save the installed baseline
doctor      Diagnose scanner and configuration state
quit        Exit toolsniff
```

Recommended architecture:

```go
type Command struct {
    Name        string
    Description string
    Run         func(*tuiModel) tea.Cmd
}
```

Do not add more string comparisons to the existing filter switch. Keep command
input and tool filtering as separate state machines.

### 6. `toolsniff --doctor`

Add a diagnostic command:

```bash
toolsniff --doctor
```

Expected output:

```text
toolsniff doctor

Platform:       macOS arm64
Version:        0.1.0
Config:         found and readable
Registry:       writable
PATH:           12 directories
Applications:   2 roots found
npm:            available
brew:           available
pipx:           missing
cargo:          available
bun:            missing
TUI colors:     truecolor
```

Diagnostics should cover:

- Operating system and architecture.
- toolsniff version.
- Configuration path and parse status.
- Registry path and write permissions.
- PATH directories and exclusions.
- Application roots.
- External command availability.
- Bun global bin discovery.
- Terminal color capability.
- Tap/release installation metadata when detectable.

This should be read-only and safe to run in bug reports.

### 7. Snapshot History

The current registry stores one baseline. Add historical snapshots under:

```text
~/.toolsniff/snapshots/
```

Example:

```text
2026-08-01T10-30-00.json
2026-08-05T09-15-00.json
```

Suggested commands:

```bash
```

Recommended design:

- Keep the current registry format for compatibility.
- Add a snapshot metadata envelope with timestamp and toolsniff version.
- Do not introduce SQLite yet; JSON files are sufficient for this scale.
- Add retention configuration later if snapshot growth becomes a problem.

### 8. More Package Managers and Tool Locations

Potential scanners, in priority order:

1. pnpm global packages.
2. Yarn global packages.
3. `uv tool list`.
4. `mise` tools.
5. `asdf` tools.
6. `go install` binaries.
7. MacPorts.
8. Docker CLI plugins.
9. Terraform plugins.
10. Additional user binary directories such as `~/.local/bin` and `~/bin`.

The existing `Scanner` interface supports additive scanners. For similar
command-output scanners, consider a reusable adapter:

```go
type CommandScanner struct {
    Source  string
    Role    model.SourceRole
    Command []string
    Parse   func([]byte) ([]model.Tool, error)
}
```

Do not force scanners with different filesystem or metadata semantics into the
same abstraction merely to reduce file count.

### 9. Watch and Refresh Mode

Add manual refresh first:

```text
r  refresh now
```

Then consider automatic refresh:

```bash
```

The TUI should display:

```text
Last scan: 10:42:15
Next scan: 10:43:15
```

Implementation requirements:

- Reuse the existing concurrent scanner runner.
- Do not block the Bubble Tea event loop.
- Preserve the selected tab when results refresh.
- Recompute diffs against the saved installed baseline.
- Avoid writing registry state automatically unless explicitly requested.

### 10. Export Formats

JSON already exists. Additional formats could include:

```bash
```

Potential uses:

- Machine inventory reports.
- Onboarding documentation.
- Security audits.
- Comparing two laptops.
- CI environment checks.

Keep all exporters downstream of a shared report model so each format exposes
the same installed, available, history, added, removed, and updated data.

### 11. Installation Provenance and Metadata

Make source relationships easier to understand without merging truthful
observations.

Example:

```text
gh
  installed: brew-formula
  available: /opt/homebrew/bin/gh

gh
  installed: npm
```

Potential metadata:

- Package manager installation command.
- Executable path.
- Application bundle identifier.
- Application bundle version.
- File modification time.
- Code-signing status.
- Architecture.
- SHA256 hash for manually installed binaries.

Use separate optional metadata fields rather than guessing installation
provenance from names.

## Theme Roadmap

The current TUI has built-in presets and an OpenCode-style picker:

- `t` opens the picker.
- `/theme` opens the picker through the command prompt.
- `?` shows the theme key.
- `enter` applies and persists a preset.

Future theme work:

- User-defined theme files under `~/.config/toolsniff/themes/`.
- Project-local themes only if there is a clear use case.
- Theme preview before applying.
- Dark/light variants.
- ANSI and terminal-native colors.
- A `system` theme that respects the terminal palette.
- Import/export of themes.

Avoid putting Lip Gloss values directly into individual TUI components. Keep
semantic tokens in the config layer and style construction in
`output.NewThemeStyles`.

## Suggested Version Roadmap

### `v0.2.0`

Prioritize correctness and daily usability:

1. Separate installed baselines from PATH availability.
2. Detect version upgrades and downgrades.
3. Add the tool details panel.
4. Add global search.
5. Add `--doctor`.
6. Replace the `/theme` filter special case with a real command palette.
7. Add an end-to-end TUI test for `? -> t -> picker -> enter -> save`.

### `v0.3.0`

Expand inventory coverage and history:

1. Add pnpm, Yarn, and uv.
2. Add mise/asdf/go-installed tools.
3. Add snapshot history.
4. Add export formats.
5. Add manual refresh and watch mode.
6. Add richer application metadata.

### `v1.0.0`

Stabilize the public product:

1. Version the JSON schema.
2. Add registry migrations.
3. Support custom theme files.
4. Add Linux support.
5. Expand integration and TUI test coverage.
6. Maintain stable Homebrew distribution.

## Implementation Order

Use dependency-ordered waves for each feature group.

### Wave 1: Inventory Correctness

- Separate installed and available registry data.
- Add updated/version-change diff records.
- Update JSON, table, TUI, and tests.

Gate:

```bash
go test ./...
```

### Wave 2: TUI Navigation and Details

- Add the detail panel.
- Add global search.
- Add a proper command palette.
- Add end-to-end key-flow tests.

Gate:

```bash
go test ./...
```

Manual checks must cover source navigation, selection, details, search,
theme selection, save, diff, and quit.

### Wave 3: Diagnostics and History

- Add `--doctor`.
- Add snapshot persistence.
- Add history and historical diff commands.

Gate:

- Registry migration tests.
- Read-only doctor behavior.
- Snapshot corruption and retention tests.

### Wave 4: Scanner Expansion

- Add package-manager scanners one at a time.
- Add fixture-driven tests for every command parser.
- Add registrations and source roles.
- Update README and configuration examples.

Gate:

```bash
go test ./...
```

### Wave 5: Release

- Update the user documentation.
- Run the full release verification suite.
- Create a semantic version tag.
- Verify GitHub artifacts and Homebrew formula.
- Test installation on Apple Silicon and Intel where possible.

## Engineering Rules

- Do not reintroduce curated tool-name inclusion lists.
- Keep different installation sources separate and truthful.
- Treat PATH as availability, not proof of installation.
- Keep npx history outside the installed baseline.
- Do not silently change registry identity semantics.
- Add migration logic before changing persisted formats.
- Keep all scanner tests hermetic.
- Keep TUI styling centralized through semantic theme tokens.
- Prefer a shared report model over format-specific business logic.
- Do not add destructive uninstall/update actions without explicit safety
  design.
- Run tests, race tests, vet, and build gates at the end of each wave.
- Do not create or push release tags from dirty worktrees.
