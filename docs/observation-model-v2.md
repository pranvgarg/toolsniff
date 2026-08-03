# toolsniff Observation Model v2

## Status

This document is the implementation contract for branch:

```text
feat/observation-model-v2
```

It is a design and execution plan. Application code must not be changed until
this plan is approved.

## Executive Decision

The current `model.Tool` shape is too flat for the observations toolsniff now
collects. A package, an application bundle, a PATH executable, and an npx cache
entry do not have the same meaning even when all of them have a name.

The v2 model will represent an **observation** rather than pretending every
result is a conventional versioned tool.

```text
Observation
  logical identity
  display and command names
  kind
  installation origin
  version state and evidence
  one or more locations
  availability state
  package metadata when applicable
  application metadata when applicable
  history metadata when applicable
  timestamps and evidence
```

The implementation will use typed optional structures instead of an untyped
metadata map. This keeps the JSON contract discoverable, testable, and safe to
evolve.

## Current Findings

The existing model is:

```go
type Tool struct {
    Name    string
    Source  string
    Role    SourceRole
    Version string
    Path    string
}
```

The current scanner behavior is inconsistent by design, but the output model
does not make those differences explicit:

| Source | Current fields | Problem |
| --- | --- | --- |
| npm | Name, Version | Package location and executable links are missing |
| Homebrew formula | Name | Homebrew version and Cellar location are missing |
| Homebrew cask | Name | Bundle path, bundle ID, app version, signing, and architecture are missing |
| pipx | Name, Version | Venv and executable locations are missing |
| Cargo | Name, Path | Package identity and version are missing |
| Bun | Name, Path | Package identity and version are missing |
| applications | Name, Path | Info.plist metadata is missing |
| PATH | Name, Path | Installation origin and version are unknown |
| npx history | Name, Version=date | A usage date is incorrectly stored in the version field |

The current UI also uses `Path` as the fallback value for the Version column.
That is useful for display, but it mixes two different concepts in the output
contract.

The current identity rule uses `Source + Path` when a path exists and
`Source + Name` otherwise. This makes relocation look like removal plus
addition even when the package or application identity is unchanged.

## Goals

- Represent versioned and non-versioned discoveries honestly.
- Distinguish unknown, unavailable, not-applicable, and not-reported versions.
- Separate package/application identity from filesystem locations.
- Preserve installed, available, and history semantics.
- Preserve existing registry files through an explicit migration.
- Provide richer metadata without making every scanner fabricate fields.
- Keep renderers downstream of one shared report model.
- Make future provenance, snapshots, health checks, and profiles possible.
- Keep scanner tests hermetic and release tests safe.

## Non-Goals

- Do not execute arbitrary tools during a normal scan.
- Do not guess installation provenance from a matching name.
- Do not update or uninstall discovered tools as part of this model change.
- Do not introduce SQLite.
- Do not make all scanners implement a single forced metadata abstraction.
- Do not break existing JSON consumers without a schema version and migration.
- Do not add network lookups to the default scan.

## Target Domain Model

### Observation

The final name should be `Observation`. During migration, the existing `Tool`
name may remain as a compatibility alias or adapter until all consumers move.

```go
type Observation struct {
    ID           string             `json:"id"`
    DisplayName  string             `json:"display_name"`
    CommandName  string             `json:"command_name,omitempty"`
    Kind         ObservationKind    `json:"kind"`
    Role         SourceRole         `json:"role"`
    Origin       Origin             `json:"origin"`
    Version      VersionInfo        `json:"version"`
    Locations    []Location         `json:"locations,omitempty"`
    Availability AvailabilityInfo   `json:"availability,omitempty"`
    Package      *PackageInfo       `json:"package,omitempty"`
    Application  *ApplicationInfo   `json:"application,omitempty"`
    History      *HistoryInfo       `json:"history,omitempty"`
    Evidence     []Evidence         `json:"evidence,omitempty"`
    FirstSeen    *time.Time         `json:"first_seen,omitempty"`
    LastSeen     *time.Time         `json:"last_seen,omitempty"`
}
```

The exact Go field tags and pointer choices may be adjusted during Wave 1, but
the semantic separation is mandatory.

### Observation Kind

```go
type ObservationKind string

const (
    KindCLI        ObservationKind = "cli"
    KindPackage    ObservationKind = "package"
    KindApplication ObservationKind = "application"
    KindExecutable ObservationKind = "executable"
    KindHistory    ObservationKind = "history"
)
```

Kinds describe what was observed. They do not claim how it was installed.

### Origin

```go
type Origin struct {
    Provider string `json:"provider"`
    Manager  string `json:"manager,omitempty"`
    Package  string `json:"package,omitempty"`
    Manifest string `json:"manifest,omitempty"`
}
```

Examples:

```text
npm:
  provider: npm
  manager: global
  package: opencode-ai

brew-formula:
  provider: homebrew
  manager: formula
  package: gh

path:
  provider: unknown
  manager: manual-or-unknown
```

Origin must be evidence-backed. A PATH result must not be relabeled as npm or
Homebrew merely because the command name matches.

### VersionInfo

```go
type VersionState string

const (
    VersionKnown         VersionState = "known"
    VersionUnknown       VersionState = "unknown"
    VersionNotApplicable VersionState = "not-applicable"
    VersionNotReported   VersionState = "not-reported"
)

type VersionScheme string

const (
    SchemeSemver    VersionScheme = "semver"
    SchemeCalver    VersionScheme = "calver"
    SchemeNumeric   VersionScheme = "numeric"
    SchemeGit       VersionScheme = "git"
    SchemeOpaque    VersionScheme = "opaque"
    SchemeUnknown   VersionScheme = "unknown"
)

type VersionInfo struct {
    Value        string        `json:"value,omitempty"`
    State        VersionState  `json:"state"`
    Scheme       VersionScheme `json:"scheme,omitempty"`
    Comparable   bool          `json:"comparable"`
    Confidence   Confidence    `json:"confidence"`
    RetrievedBy  string        `json:"retrieved_by,omitempty"`
}
```

The states have strict meanings:

| State | Meaning |
| --- | --- |
| `known` | A version value was obtained from reliable evidence |
| `unknown` | A version may exist, but toolsniff could not obtain it |
| `not-applicable` | A version does not meaningfully apply, such as a history date |
| `not-reported` | The source did not provide version data and no probe ran |

An npx cache entry will use `History.LastUsed`, never `Version`.

### Confidence

```go
type Confidence string

const (
    ConfidenceHigh   Confidence = "high"
    ConfidenceMedium Confidence = "medium"
    ConfidenceLow    Confidence = "low"
)
```

Confidence describes evidence quality, not whether the tool is trusted.

### Location

```go
type LocationType string

const (
    LocationExecutable LocationType = "executable"
    LocationApplication LocationType = "application-bundle"
    LocationPackagePrefix LocationType = "package-prefix"
    LocationVirtualEnv LocationType = "virtual-environment"
    LocationCache LocationType = "cache"
)

type Location struct {
    Path          string       `json:"path"`
    Type          LocationType `json:"type"`
    IsSymlink     bool         `json:"is_symlink,omitempty"`
    SymlinkTarget string       `json:"symlink_target,omitempty"`
    Executable    bool         `json:"executable,omitempty"`
    ModifiedAt    *time.Time   `json:"modified_at,omitempty"`
    Architectures []string     `json:"architectures,omitempty"`
    SHA256        string       `json:"sha256,omitempty"`
}
```

Hashes are optional and must never be computed by default for an entire PATH.
They should be opt-in because of cost and privacy implications.

### Availability

```go
type AvailabilityState string

const (
    AvailabilityAvailable   AvailabilityState = "available"
    AvailabilityUnavailable AvailabilityState = "unavailable"
    AvailabilityUnknown     AvailabilityState = "unknown"
)

type AvailabilityInfo struct {
    State        AvailabilityState `json:"state"`
    ResolvedPath string            `json:"resolved_path,omitempty"`
    PATHIndex    int               `json:"path_index,omitempty"`
    ShadowedBy   []string          `json:"shadowed_by,omitempty"`
    Probe        *ProbeResult      `json:"probe,omitempty"`
}
```

PATH discovery should eventually retain every matching executable, not only the
one that wins shell resolution. This enables command-shadow diagnostics.

### PackageInfo

```go
type PackageInfo struct {
    Name             string   `json:"name,omitempty"`
    Version          string   `json:"version,omitempty"`
    Executables      []string `json:"executables,omitempty"`
    Prefix           string   `json:"prefix,omitempty"`
    VirtualEnv       string   `json:"virtual_env,omitempty"`
    DependencyCount  int      `json:"dependency_count,omitempty"`
}
```

Package metadata is optional and source-specific.

### ApplicationInfo

```go
type ApplicationInfo struct {
    BundleID        string   `json:"bundle_id,omitempty"`
    DisplayVersion  string   `json:"display_version,omitempty"`
    ShortVersion    string   `json:"short_version,omitempty"`
    MinimumOS       string   `json:"minimum_os,omitempty"`
    Architectures   []string `json:"architectures,omitempty"`
    Signed          *bool    `json:"signed,omitempty"`
    SigningTeamID   string   `json:"signing_team_id,omitempty"`
}
```

Application metadata should be read from the bundle’s `Info.plist` and code
signature tools only when configured or when the scan mode requests details.

### HistoryInfo

```go
type HistoryInfo struct {
    LastUsed   *time.Time `json:"last_used,omitempty"`
    CachePath  string     `json:"cache_path,omitempty"`
}
```

History remains informational and stays outside installed and availability
baselines.

### Evidence

```go
type Evidence struct {
    Type        string     `json:"type"`
    Source      string     `json:"source"`
    Description string     `json:"description,omitempty"`
    RetrievedAt *time.Time `json:"retrieved_at,omitempty"`
}
```

Examples:

```text
package-metadata: npm ls -g JSON
package-metadata: brew info --json=v2
bundle-metadata: Info.plist
filesystem: executable bit and path scan
probe: command --version
```

Evidence must not include secrets, environment variables, command arguments that
contain tokens, or full arbitrary command output.

## Identity Contract

Identity and location are separate.

Identity priority:

1. Package manager plus package name.
2. Application bundle identifier.
3. Explicit executable path for a manually discovered binary.
4. Name plus origin when no stronger identity exists.
5. Content hash only as an opt-in fallback.

Examples:

```text
brew-formula + gh
npm + opencode-ai
brew-cask + com.anthropic.claudefordesktop
application + com.google.Chrome
path + /Users/name/bin/internal-tool
```

A relocation changes `Locations`, not necessarily `ID`.

The diff engine must distinguish:

```text
ADDED       No matching identity existed before.
REMOVED     An existing identity disappeared.
UPDATED     Version or meaningful metadata changed.
RELOCATED   Identity stayed stable but location changed.
REPAIRED    A previously broken location became usable.
BROKEN      An identity remains but its location is unusable.
SHADOWED    A command is hidden by an earlier PATH entry.
```

The first implementation may expose `RELOCATED`, `REPAIRED`, `BROKEN`, and
`SHADOWED` in JSON before adding dedicated table/TUI sections.

## Scanner Mapping

| Scanner | Kind | Required v2 evidence |
| --- | --- | --- |
| npm | package/CLI | Package name, version, prefix, executable links |
| Homebrew formula | package/CLI | Formula name, installed version, Cellar/opt paths |
| Homebrew cask | application or package | Cask version, app path, bundle metadata when present |
| pipx | package/CLI | Package version, venv path, executable paths |
| Cargo | executable/package | Binary path first; package metadata when cheaply available |
| Bun | executable/package | Binary path first; package metadata when cheaply available |
| applications | application | Bundle ID, display version, architectures, signature status |
| PATH | executable | Every matching path, active path index, safe probe result |
| npx history | history | Package name, cache path, last-used timestamp |

No scanner should populate fields by guessing. An absent field should be
represented by the correct state rather than an invented value.

## Safe Version Probing

Version probing is useful for PATH, Cargo, Bun, and manually installed tools,
but it can execute arbitrary software. It must be opt-in or constrained.

Default policy:

- Package metadata first.
- Bundle metadata second.
- No arbitrary executable probing in the default fast scan.
- Optional probe mode only for executable observations.
- One process per probe.
- Configurable timeout, default 2 seconds.
- Captured stdout/stderr limited to a small byte budget.
- No shell interpolation.
- Arguments are fixed by a probe adapter.
- Non-zero exit is evidence of probe failure, not scanner failure.

Supported initial probe forms:

```text
--version
version
-v
```

Probe output must be parsed into `VersionInfo` with `RetrievedBy: probe` and
medium or low confidence unless the adapter is source-specific.

## Persistence and Migration

The current files remain:

```text
~/.toolsniff/registry.json
~/.toolsniff/availability.json
```

Version 2 will use an envelope:

```json
{
  "schema_version": 2,
  "observations": []
}
```

Migration rules:

1. Read the current legacy array as schema version 1.
2. Convert each legacy `Tool` into an `Observation`.
3. Convert non-empty legacy `Version` to `VersionInfo.state=known`.
4. Convert empty legacy versions to `not-reported` for package observations and
   `unknown` for executable observations.
5. Convert legacy `Path` to a `Location`.
6. Convert legacy `Source` and `Role` into `Origin` and role fields.
7. Never treat an npx history date as a version during migration. Detect the
   npx source and move it to `History.LastUsed`.
8. Write schema version 2 only after a successful atomic save.
9. Preserve corrupt-file warning behavior.
10. Keep installed and availability files separate.

The migration must be idempotent and must never discard unknown fields from a
future schema without a warning.

## Output Contract

### Table and TUI

The TUI must use progressive disclosure. The overview answers what needs
attention, the list supports scanning, and the detail view preserves every
available fact without forcing all fields into one row.

The primary navigation should be user-intent-oriented rather than source-only:

```text
ALL              1,423
INSTALLED          207
AVAILABLE        1,209
CHANGES              4
ISSUES               7
HISTORY             60
```

Sources remain available as filters:

```text
npm · brew-formula · brew-cask · pipx · cargo · bun · applications · path
```

The primary row should become:

```text
Name                    Version       Status       Source
gh                      2.75.0        installed    brew-formula
opencode-ai             1.18.11       installed    npm
custom-tool             unknown       available    path
create-vite             n/a           history      npx-history
```

Do not show a path in a column named Version. A location may appear as a
dedicated column on wide terminals, but normally belongs in the detail view.
The TUI must display explicit version states such as `unknown`, `n/a`, and
`not reported` rather than silently falling back to a path.

Press `enter` to open a complete detail view:

```text
Tool Details

Name:          @google/gemini-cli
Kind:          CLI
Status:        installed and available
Source:        npm global
Package:       @google/gemini-cli
Version:       0.53.1
Version state: known
Confidence:    high

Locations:
  package:    ~/.npm-global/lib/node_modules/@google/gemini-cli
  executable: ~/.npm-global/bin/gemini

Availability:
  active command: yes
  PATH index: 3

Evidence:
  npm package metadata
  executable link discovery
```

The detail view is read-only. It may support copying a path, copying the
selected observation as JSON, and revealing a location in Finder.

### User-Friendly Filtering

Simple search must work without a query language. Press `/` and type normal
text to search globally across display name, command name, package name, source,
version, status, and path:

```text
/gemini
Filter: gemini                         4 matches
```

Press `f` to open a visual filter drawer:

```text
Filter Inventory

Search       gemini
View         Installed
Source       All
Kind         CLI
Version      Any
Status       Any

              Apply     Clear     Cancel
```

The filter drawer must support:

- Views: All, Installed, Available, Changes, Issues, History.
- Sources: npm, brew-formula, brew-cask, pipx, cargo, bun, applications, path.
- Kinds: CLI, application, package, executable, history.
- Version states: known, unknown, not reported, not applicable.
- Statuses: installed, available, updated, shadowed, broken, relocated.

Power users may use optional structured filters with simple AND semantics:

```text
source:npm gemini
kind:application claude
version:unknown
source:brew-formula status:updated
```

Regex is not the default. The default interaction should be plain text and
facets, not syntax memorization.

Active filters must remain visible as removable chips:

```text
Filter: gemini  [source:npm x] [status:updated x]    2 matches
```

Empty results must explain the active filters and provide clear actions:

```text
No matches

Current filters:
  source:npm
  status:updated
  search: gemini

Try: clear status filter or search all sources
```

Filtering only changes visibility. It must never remove observations from the
report or affect JSON output.

### TUI Interaction and Responsive Layout

The key map should provide:

```text
enter     open details
esc       close details or clear the current mode
/         global search
f         filter drawer
d         changes view
i         issues view
s         save baseline
p         copy selected path
c         copy selected observation JSON
o         reveal selected location
r         refresh scan
?         help
q         quit
```

The layout should adapt without losing information:

- Wide terminal: sidebar, inventory list, and optional detail pane.
- Medium terminal: sidebar and inventory list, with details as a modal view.
- Narrow terminal: compact view strip and inventory list.
- Very narrow terminal: selected row plus detail view as the primary screen.

Large PATH inventories must remain responsive. Metadata enrichment and probes
must be lazy for selected rows or explicitly requested views, not performed for
every row while the user types.

### Changes and Issues Views

The changes view must retain event types and before/after context rather than
turning events into ordinary tool rows:

```text
CHANGES

UPDATED
  opencode-ai
  1.18.10 -> 1.18.11

RELOCATED
  gh
  /usr/local/bin/gh
  -> /opt/homebrew/bin/gh

BROKEN
  internal-tool
  ~/.local/bin/internal-tool

SHADOWED
  node
  active: /opt/homebrew/bin/node
  hidden: /usr/local/bin/node
```

The Issues view should summarize actionable health findings:

```text
ISSUES

3 shadowed commands
2 broken executable paths
1 unknown application version
1 architecture mismatch
```

### TUI Presentation Types

The TUI must not format domain observations directly. Add a presentation layer:

```go
type InventoryRow struct {
    ObservationID string
    Name          string
    Version       string
    VersionState  string
    Status        string
    Source        string
    Kind          string
}

type DetailViewModel struct {
    Title    string
    Sections []DetailSection
}

type FilterState struct {
    Text         string
    Sources      map[string]bool
    Roles        map[model.SourceRole]bool
    Kinds        map[ObservationKind]bool
    VersionState map[VersionState]bool
    Statuses     map[Status]bool
}
```

This keeps domain fields stable while allowing the UI to evolve independently.

Filters should eventually include:

```text
kind:cli
role:available
source:npm
version:unknown
status:shadowed
```

### JSON

The JSON report must expose:

```json
{
  "schema_version": 2,
  "installed": [],
  "available": [],
  "history": [],
  "changes": {
    "added": [],
    "removed": [],
    "updated": [],
    "relocated": [],
    "broken": [],
    "repaired": [],
    "shadowed": []
  },
  "warnings": []
}
```

The initial v2 implementation may retain the existing top-level aliases for a
single compatibility release, but all new fields must use the v2 structure.

## Innovation Backlog

These features become possible once the observation model is stable.

### Command Shadow Map

Show every executable with the same command name and identify which one wins
shell resolution:

```text
gh
  active: /opt/homebrew/bin/gh
  shadowed:
    /Users/name/.local/bin/gh
    /usr/local/bin/gh
```

### Provenance Graph

Connect package, executable, symlink, and application relationships:

```text
gh
  Homebrew formula gh 2.75.0
  Cellar binary /opt/homebrew/Cellar/gh/2.75.0/bin/gh
  active link /opt/homebrew/bin/gh
```

### Environment Profiles

Export and compare machines:

```bash
toolsniff export --profile work.json
toolsniff compare work.json personal.json
```

This should support sanitized profiles with optional path redaction.

### Timeline

Use snapshots to answer “what changed my machine?”:

```text
09:14  Homebrew upgraded gh 2.74.0 -> 2.75.0
09:21  PATH gained ~/.local/bin/internal-tool
10:05  Claude.app changed 1.2.2 -> 1.2.3
```

### Read-Only Doctor

```bash
toolsniff --doctor
```

Doctor should report broken links, shadowed commands, architecture mismatches,
missing package-manager executables, outdated Homebrew prerequisites, and
configuration/registry health without mutating the machine.

### Capability Inventory

Optional adapters can report capabilities rather than only identity:

```text
claude
  kind: ai-cli
  capabilities: interactive, mcp, version-probe

gh
  kind: developer-cli
  capabilities: git-hosting, version-probe
```

Capabilities must be explicit adapter declarations or safe metadata, not
guesses from names.

### Support Bundles

Generate a sanitized bug-report bundle:

```bash
toolsniff support-bundle
```

It should include platform, toolsniff version, scanner warnings, package
manager availability, and selected observations while excluding tokens,
environment secrets, and arbitrary command output.

## Wave Plan

Wave assignment is based on produced interfaces, not task size.

### Wave 1: Domain Contracts

**Gate:** New model types compile in isolation; table-driven tests cover version
states, origin, identity, location, evidence, and observation kind. No scanner
or renderer changes are started until these semantics are stable.

#### Task 1: Introduce Observation Types

**Files:**

- `model/observation.go`
- `model/version.go`
- `model/location.go`
- `model/provenance.go`
- Existing `model/tool.go` compatibility adapter
- Model tests

**Consumes:** Existing `model.Tool`, `SourceRole`, `ToolIdentity`, and JSON
field conventions.

**Produces:** Typed `Observation`, `VersionInfo`, `Location`, `Origin`,
`ApplicationInfo`, `PackageInfo`, `AvailabilityInfo`, `HistoryInfo`, and
`Evidence` contracts.

**Steps:**

1. Define enums and validation helpers.
2. Define state-aware version semantics.
3. Define stable identity components separately from location.
4. Keep typed optional metadata structures.
5. Add conversion from legacy `Tool` to `Observation`.
6. Reject invalid combinations such as history with installed baseline status.

#### Task 2: Define Report and Change Events

**Files:**

- `registry/diff.go`
- New report/change model files
- Registry tests

**Consumes:** Observation contracts from Task 1.

**Produces:** Change events for added, removed, updated, relocated, broken,
repaired, and shadowed observations.

**Steps:**

1. Replace path-only identity matching with identity-plus-location semantics.
2. Preserve version update behavior.
3. Define deterministic change ordering.
4. Add explicit “not-applicable” and unknown version handling.
5. Keep legacy `Diff` compatibility during migration.

### Wave 2: Persistence and Scanner Evidence

**Depends on:** Wave 1

**Gate:** Existing v1 registry arrays load into v2 observations, v2 files save
atomically, corrupt files warn safely, and every scanner fixture produces valid
observation states without fabricating metadata.

#### Task 3: Versioned Registry Envelope

**Files:**

- `registry/registry.go`
- New migration helpers
- `registry/*_test.go`

**Consumes:** Observation and change contracts from Wave 1.

**Produces:** Schema version 2 installed and availability registries.

**Steps:**

1. Detect legacy array versus v2 envelope.
2. Migrate legacy observations deterministically.
3. Preserve separate installed and availability files.
4. Preserve warning and atomic-save semantics.
5. Add migration, idempotence, corruption, and permission tests.

#### Task 4: Package Manager Metadata

**Files:**

- `scanner/npm.go`
- `scanner/homebrew.go`
- `scanner/pipx.go`
- `scanner/cargo.go`
- `scanner/bun.go`
- Scanner fixtures/tests

**Consumes:** Observation contracts from Wave 1.

**Produces:** Package origins, package metadata, versions, prefixes, and
executable locations where the package manager can provide them.

**Steps:**

1. Use structured package-manager output where available.
2. Use source-specific parsers, not a universal command parser.
3. Keep missing metadata state-aware.
4. Preserve tolerant warning behavior.
5. Add fixture-driven tests for all response shapes.

#### Task 5: Application Bundle Metadata

**Files:**

- `scanner/applications.go`
- New bundle metadata helper
- Application scanner tests

**Consumes:** Observation and `ApplicationInfo` contracts from Wave 1.

**Produces:** Bundle ID, display version, short version, architectures, and
optional signing evidence.

**Steps:**

1. Read `Info.plist` without recursively scanning bundle contents.
2. Use stable bundle ID as the preferred identity.
3. Distinguish missing metadata from invalid metadata.
4. Add fixtures for valid, missing, malformed, and nested bundles.

#### Task 6: Executable and PATH Evidence

**Files:**

- `scanner/filesystem.go`
- `scanner/path.go`
- New probe package or scanner helper
- PATH/filesystem tests

**Consumes:** Location, availability, version, and evidence contracts from
Wave 1.

**Produces:** Structured executable locations, all PATH candidates, active PATH
index, shadowing information, and optional safe probe results.

**Steps:**

1. Preserve executable and symlink metadata.
2. Track every matching command, not only the first one.
3. Mark the active command according to PATH order.
4. Add opt-in bounded version probing.
5. Enforce timeouts, output limits, no shell execution, and fixed arguments.
6. Add tests for shadowing, broken links, permissions, and probe failures.

### Wave 3: Shared Report and TUI Experience

**Depends on:** Wave 2

**Gate:** `--list`, `--json`, TUI, `--save`, `--diff`, and
`--diff --available` expose the same v2 report semantics with no legacy field
confusion. UI tests cover simple search, filter facets, detail views, changes,
issues, responsive layouts, and large PATH inventories.

#### Task 7: Shared Report Model

**Files:**

- `output/report.go`
- `output/json.go`
- `output/table.go`
- `output/tui_model.go`
- `output/tui_frame.go`
- `output/filter.go`
- `output/filter_parser.go`
- `output/filter_drawer.go`
- `output/tui_detail.go`
- `output/tui_changes.go`
- Output tests

**Consumes:** v2 observations, registry changes, and migrated baselines.

**Produces:** Installed, available, history, and typed change report output.

**Steps:**

1. Stop displaying paths in a Version column.
2. Render version state labels such as `unknown` and `not-applicable`.
3. Add a presentation layer with `InventoryRow` and `DetailViewModel` instead
   of formatting domain observations directly.
4. Add view-based navigation: All, Installed, Available, Changes, Issues, and
   History.
5. Add global plain-text search through `/` across name, command, package,
   source, version, status, and path.
6. Add a visual facet filter drawer through `f` for source, role, kind, version
   state, and status.
7. Add optional structured filters with simple AND semantics, such as
   `source:npm gemini` and `status:shadowed`.
8. Show active filters as removable chips with match counts.
9. Add clear empty-result explanations and filter reset actions.
10. Add a read-only detail view through `enter` with metadata, locations,
    availability, evidence, and safe copy/reveal actions.
11. Preserve typed change events in the Changes view, including before/after
    versions and relocation, broken, repaired, and shadowed states.
12. Add an Issues view for actionable health findings.
13. Preserve machine-readable v2 JSON.
14. Keep large inventories responsive through lazy metadata loading and
    bounded filtering work.
15. Add table, JSON, filter, detail, changes, issues, responsive, and large
    inventory tests.

### Wave 4: CLI Compatibility and Migration Flow

**Depends on:** Waves 2 and 3

**Gate:** Existing CLI modes continue to work, v1 registries migrate safely,
and JSON schema versioning is explicit.

#### Task 8: CLI Compatibility and Migration Flow

**Files:**

- `internal/cli/cli.go`
- `internal/cli/cli_test.go`
- `main_integration_test.go`

**Consumes:** Registry migration APIs from Wave 2 and report/TUI APIs from Wave
3.

**Produces:** Stable CLI behavior during the v1-to-v2 transition.

**Steps:**

1. Keep existing flags working.
2. Add an explicit JSON schema version.
3. Ensure save migrates only after a successful scan.
4. Preserve warnings and exit status behavior.
5. Add end-to-end migration tests using temporary registries.

### Wave 5: Health, Provenance, and Timeline

**Depends on:** Wave 4

**Gate:** New diagnostic features are read-only, privacy-safe, and backed by
fixture tests before being exposed in the default UI.

#### Task 9: Provenance and Health Graph

**Files:**

- New `model/provenance.go` extensions
- New `diagnostics` or `health` package
- TUI and JSON report extensions

**Consumes:** Locations, origins, evidence, availability, and changes.

**Produces:** Shadow map, broken/repaired status, package-to-executable graph,
architecture mismatch, and signing diagnostics.

#### Task 10: Snapshots and Profiles

**Files:**

- New `registry/snapshots.go`
- New profile/export package
- CLI commands and tests
- Documentation

**Consumes:** Versioned report model from Wave 4.

**Produces:** Timeline, machine comparison, sanitized export, and support bundle
capabilities.

### Wave 6: Capability Adapters

**Depends on:** Wave 5

**Gate:** Each capability adapter has explicit scope, no arbitrary execution by
default, a timeout, and fixture coverage.

#### Task 11: Capability Inventory

**Files:**

- New capability adapter package
- Source-specific adapters
- CLI and output integration
- Adapter tests

**Consumes:** Stable observations and safe probe infrastructure.

**Produces:** Explicit capabilities such as MCP, interactive CLI, Git hosting,
LSP, and version probing.

## Dependency Self-Review

- Task 1 consumes only existing model and source-role contracts, so it is Wave
  1.
- Task 2 consumes Task 1's observation types, so it is Wave 1.
- Task 3 consumes Tasks 1 and 2, so it is Wave 2.
- Tasks 4, 5, and 6 consume Task 1 and are independent of Task 3 and each
  other, so they are also Wave 2.
- Task 7 consumes the persistence and scanner outputs from Wave 2, so it is
  Wave 3.
- Task 8 consumes registry migration from Wave 2 and the report/TUI contract
  from Task 7, so it is Wave 4.
- Tasks 9 and 10 consume the stable CLI/report model from Wave 4, so they are
  Wave 5.
- Task 11 consumes the safe probe and stable observation infrastructure from
  Wave 5, so it is Wave 6.

No task consumes an interface produced in the same wave or a later wave.

## Test Strategy

Tests stay online in Git and remain close to the code they cover.

Required tests:

- Model validation and legacy conversion tests.
- Version state and scheme tests.
- Identity and relocation tests.
- Registry v1-to-v2 migration tests.
- Package-manager fixture tests.
- Application bundle metadata fixtures.
- PATH shadow and broken-link fixtures.
- Safe probe timeout and output-limit tests.
- JSON schema contract tests.
- Table and TUI state rendering tests.
- Global plain-text search tests.
- Structured filter parser and facet drawer tests.
- Filter chip, empty-result, and clear-filter tests.
- Detail view tests for every optional metadata section.
- Changes and Issues view tests preserving event types.
- Responsive layout tests for wide, medium, narrow, and very narrow terminals.
- Large inventory tests with at least 1,000 available PATH observations.
- Full CLI end-to-end tests with temporary registries and fake commands.
- Snapshot and profile redaction tests.
- Race, vet, and build gates for every load-bearing wave.

No test may depend on the real Homebrew database, real npm cache, real user
applications, credentials, network access, or machine-specific paths.

## Privacy and Security Rules

- Never persist environment variables.
- Never persist command-line arguments that may contain secrets.
- Do not upload inventory data automatically.
- Make hashes opt-in.
- Make executable probes opt-in or strictly adapter-controlled.
- Redact home directory prefixes in exported support bundles by default.
- Keep source paths local unless the user explicitly exports them.
- Keep update and uninstall actions separate from inventory collection.

## Performance Rules

- The default scan remains concurrent.
- Metadata enrichment must not turn every executable into a subprocess.
- Package metadata should be collected in one command per manager where possible.
- Bundle metadata should be read only for discovered `.app` roots.
- Probes must be bounded and parallelism-limited.
- Hashing must be opt-in.
- Snapshots must be written atomically.

## Acceptance Criteria

The implementation is complete only when:

1. A legacy v1 registry loads without data loss.
2. A v2 registry round-trips all typed observation fields.
3. npm, Homebrew, pipx, Cargo, Bun, applications, PATH, and npx history each
   use accurate version states.
4. npx dates are no longer stored as versions.
5. Application versions and bundle IDs appear when available.
6. Homebrew versions are collected from Homebrew metadata.
7. PATH rows never claim an installation origin without evidence.
8. A relocation is distinguishable from removal plus addition.
9. The TUI never displays a path under a Version heading.
10. JSON reports include an explicit schema version.
11. Existing CLI modes continue to work.
12. All tests are hermetic and pass with race detection, vet, and build checks.
13. The MCP graph is refreshed after each wave and shows the intended package
    boundaries.
14. Documentation explains the new observation model without reintroducing a
    stale planning document.

## Execution Rule

Implement one wave at a time. Run the gate yourself after each wave. Do not
start the next wave because a subagent reports completion; start it only after
the wave gate passes.
