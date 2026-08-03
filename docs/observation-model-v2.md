# toolsniff Observation Model v2

## Status

The v2 observation model is implemented. This document describes the shipped
JSON contracts, scanner semantics, change behavior, CLI modes, TUI controls,
privacy rules, and v1 registry migration. It is a reference, not an
implementation plan.

## Why Observations

toolsniff discovers packages, application bundles, executables, and cache
history. Those things can share a name without having the same identity or
version semantics. v2 therefore represents each result as an observation and
keeps these concerns separate:

```text
Observation
  stable identity
  display and command names
  kind and role
  evidence-backed origin
  typed version state
  one or more locations
  availability and PATH state
  optional package, application, or history metadata
  evidence and timestamps
```

The model uses typed optional structures rather than an untyped metadata map.
Absent information remains absent or is represented by an explicit state; it is
not filled with a guessed value.

## Observation Schema

An observation has this shape. Optional fields are omitted from JSON when they
are empty:

```json
{
  "id": "package\u0000npm\u0000global\u0000opencode-ai",
  "display_name": "opencode-ai",
  "command_name": "opencode-ai",
  "kind": "cli",
  "role": "installed",
  "origin": {
    "provider": "npm",
    "manager": "global",
    "package": "opencode-ai"
  },
  "version": {
    "value": "1.18.11",
    "state": "known",
    "scheme": "semver",
    "comparable": true,
    "confidence": "high",
    "retrieved_by": "npm ls -g --depth=0 --json"
  },
  "locations": [
    {"path": "$HOME/.npm-global", "type": "package-prefix"}
  ],
  "availability": {"state": "unknown"},
  "package": {
    "name": "opencode-ai",
    "version": "1.18.11",
    "prefix": "$HOME/.npm-global"
  },
  "evidence": [
    {"type": "package-metadata", "source": "npm ls -g --depth=0 --json"}
  ]
}
```

### Kind

`kind` describes what was observed, not how it was installed:

| Kind | Meaning |
| --- | --- |
| `cli` | A package or installation with executable links |
| `package` | Package metadata without executable links |
| `application` | A macOS application bundle |
| `executable` | An executable discovered from a bin directory or PATH |
| `history` | Informational npx cache history |

### Role

`role` controls report grouping and baseline behavior:

| Role | Meaning | Baseline |
| --- | --- | --- |
| `installed` | A package manager, bin directory, or application root reported an installation | Included in the installed baseline |
| `available` | An executable is available through PATH | Included in the separate availability baseline |
| `history` | A cached one-off execution was observed | Excluded from both baselines |

A PATH result has origin `unknown` / `manual-or-unknown` unless stronger
evidence exists. Matching a command name to an npm package or Homebrew formula
does not establish provenance.

### Origin

```json
{
  "provider": "homebrew",
  "manager": "formula",
  "package": "gh",
  "manifest": "..."
}
```

`provider`, `manager`, and `package` describe only evidence supplied by the
source. Current source labels map as follows:

| Source | Origin |
| --- | --- |
| `npm` | `provider=npm`, `manager=global` |
| `brew-formula` | `provider=homebrew`, `manager=formula` |
| `brew-cask` | `provider=homebrew`, `manager=cask` |
| `pipx` | `provider=pipx`, `manager=global` |
| `cargo` | `provider=cargo`, `manager=install` |
| `bun` | `provider=bun`, `manager=global` |
| `applications` | `provider=applications` |
| `path` | `provider=unknown`, `manager=manual-or-unknown` |
| `npx-history` | history kind and role; source is retained as npx history |

### Locations

Each location has a path and one of these types:

| Type | Use |
| --- | --- |
| `executable` | A command or executable link |
| `application-bundle` | An `.app` bundle |
| `package-prefix` | An npm or Homebrew package prefix |
| `virtual-environment` | A pipx environment |
| `cache` | A history or package cache |

Locations may include symlink, executable, modification time, architecture,
and optional SHA-256 fields. Hashes are not computed or exported by default.
Identity and location are deliberately independent: moving a package or bundle
can produce a relocation instead of a removal and an addition.

### Optional Metadata

`package` can contain package name, package version, executable links, prefix,
virtual environment, and dependency count. `application` can contain bundle
ID, display and short versions, minimum OS, architectures, signing status, and
signing team. `history` contains `last_used` and `cache_path`.

`evidence` records compact source references such as package metadata,
filesystem discovery, bundle metadata, or a bounded probe. It never stores
full arbitrary command output.

## Version Semantics

Every observation has a `version` object. A version state is not inferred from
whether the display name or path is available:

| State | Meaning | `value` allowed? | Comparable? |
| --- | --- | --- | --- |
| `known` | A version value was obtained from reliable evidence | Yes | Only with a compatible scheme |
| `unknown` | A version may exist, but toolsniff could not obtain it | No | No |
| `not-applicable` | Version has no useful meaning for this observation | No | No |
| `not-reported` | The source did not report a version and no probe ran | No | No |

The implementation also records `scheme`, `comparable`, `confidence`, and
`retrieved_by`. Supported schemes are `semver`, `calver`, `numeric`, `git`,
`opaque`, and `unknown`. Confidence (`high`, `medium`, or `low`) describes the
quality of evidence, not whether a tool is trusted.

The list renderer uses these labels:

```text
known value       1.18.11
unknown           unknown
not-applicable    n/a
not-reported     not reported
```

A path is never substituted into a Version column. An npx history date is
stored as `history.last_used`, and its version state is `not-applicable`.

### Probing

Normal scans use package-manager metadata, bundle metadata, and filesystem
evidence. They do not execute arbitrary discovered executables for versions.
The bounded probe helper is used only when explicitly enabled:

- The default argument is `--version`; arguments are passed directly, without a shell.
- One probe has a 2-second default timeout.
- Captured stdout and stderr share a 4 KiB default limit.
- The first non-empty stdout line becomes the version value.
- A timeout, non-zero exit, empty output, or output limit produces `unknown` and compact error evidence.

The normal scanner command timeout is separate and is configured by
`execution.timeout` (8 seconds by default).

## Identity and Changes

Identity components are selected in this order:

1. Provider, manager, and package identity.
2. Application bundle ID.
3. Explicit executable path for an unknown/manual executable.
4. Name plus origin when no stronger identity exists.

The resulting ID is stable and unambiguous. A location is not part of a package
or bundle identity unless the executable has no stronger identity.

The diff engine compares identities first, then compares locations, version,
metadata, availability, and shadowing:

| Event | When it is emitted |
| --- | --- |
| `added` | No matching identity existed before |
| `removed` | An existing identity disappeared |
| `updated` | Known version or meaningful non-location metadata changed; a non-known state becoming known also counts |
| `relocated` | Identity stayed stable but location type/path changed |
| `broken` | An existing observation became unavailable |
| `repaired` | An unavailable observation became available |
| `shadowed` | Shadowing changed and the current observation is shadowed |

Losing a known version is intentionally not an update, which avoids noisy
changes after transient probe failures. Change events retain `before` and
`after` observations where both exist and are ordered deterministically.

## Report JSON

`--json` emits the shared report contract:

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

Filtering is presentation-only. It does not mutate this report or change JSON
output. Installed, available, and history are separate arrays even though the
TUI can show them together.

## Scanner Mapping

The v2 scanner adapters preserve source-specific evidence:

| Scanner | v2 behavior |
| --- | --- |
| npm | Reads `npm ls -g --depth=0 --json`; records package version, prefix, executable targets, and package evidence |
| Homebrew formula | Reads `brew info --json=v2 --installed --formula`; records installed version and prefix |
| Homebrew cask | Reads `brew info --json=v2 --installed --cask`; records cask version. Bundle metadata belongs to the application scanner |
| pipx | Reads `pipx list --json`; records package version, virtual environment, and executable paths |
| Cargo | Scans the configured Cargo bin directory; records executable paths without fabricating package or version metadata |
| Bun | Resolves `bun pm bin -g`; records executable paths without fabricating package or version metadata |
| applications | Finds `.app` bundles under configured roots and records application-bundle locations |
| PATH | Retains every matching executable, PATH order, active path, shadowed paths, symlinks, and broken candidates |
| npx history | Reads the npx cache and records package name and last-used date as history metadata; it never treats the date as a version |

Scanner failures are retained as warnings while other sources continue. The
default scan is concurrent and deduplicates observations by stable ID.

## CLI Modes

### `--doctor`

`--doctor` runs a scan and prints a read-only report. It does not save or
migrate registries, update configuration, inspect the filesystem beyond the
scan evidence, or execute arbitrary diagnostic commands. The output includes
observation and issue counts, typed issues, and explicit package-to-location
provenance edges.

Current issue kinds are:

- `shadowed-command`
- `broken-location`
- `architecture-mismatch`
- `unsigned-application`
- `unknown-signing-status`
- `missing-executable-link`
- `unknown-version`

Diagnostics are derived from observations. For example, an unknown version is
an issue, while a not-reported package version is not treated as an unknown
version merely because it is absent.

### `--snapshot` and `--snapshots`

`--snapshot` saves installed and available observations, never history, to:

```text
~/.toolsniff/snapshots/snapshot-YYYYMMDDTHHMMSS.nnnnnnnnnZ.json
```

The snapshot format is immutable and has its own schema version:

```json
{
  "schema_version": 1,
  "version": "1.2.3",
  "created_at": "2026-08-02T12:00:00Z",
  "observations": []
}
```

`version` identifies the toolsniff producer; `schema_version` identifies the
snapshot JSON contract. Saves are atomic, directories and files are created
with owner-only permissions, and observation order is canonical. `--snapshots`
lists valid snapshots newest-first and ignores temporary, unrelated, or corrupt
files without deleting them.

### Profiles and support bundles

`--export-profile FILE` writes a sanitized profile containing the current
observations and the v2 report. Its profile schema is version 1:

```json
{
  "schema_version": 1,
  "version": "1.2.3",
  "created_at": "2026-08-02T12:00:00Z",
  "observations": [],
  "report": {}
}
```

`--compare-profile FILE` loads and sanitizes the selected profile, scans the
current machine, and renders deterministic observation change events. The
comparison uses the same identity and version rules as registry diffs.

`--support-bundle FILE` writes the narrower support-bundle schema. It includes
sanitized observations and the sanitized report, but does not include raw
command output, probe payloads, evidence descriptions, scanner warnings, or
location hashes. Both export modes write atomically and do not upload data.

## Capabilities

`--capabilities` emits JSON with a `capabilities` array. Each result identifies
the observation, capability kind, detected state, and compact evidence. The
default adapters cover:

- `interactive-cli`
- `mcp`
- `git-hosting`
- `lsp`
- `version-probe`

Capability matching requires explicit package metadata, origin metadata, or
capability evidence. It never infers a capability from `display_name` or
`command_name`. There is no network adapter.

`--capabilities-probe` is opt-in and valid only with `--capabilities`. It adds
bounded probe evidence for active executable observations. A disabled or absent
probe option never starts a process through the capability registry.

## TUI Behavior

The report TUI is organized around user intent rather than requiring a source
tab for every question. The header exposes counts for:

```text
ALL  INSTALLED  AVAILABLE  CHANGES  ISSUES  HISTORY
```

Rows contain Name, explicit Version, Status, Source, and Kind in the available
width. Narrow terminals drop Source and then Status before ever replacing a
version with a path. Press `Enter` for the read-only detail view, which can
show:

- Overview: name, command, kind, status, and source.
- Version: value, state, scheme, confidence, and retrieval method.
- Origin: provider, manager, package, and manifest.
- Locations: all recorded paths and location types.
- Availability: active path, PATH index, and shadowed paths.
- Package, Application, or History metadata when present.
- Evidence references when present.

### Controls

| Key | Action |
| --- | --- |
| `Up` / `Down`, `k` / `j` | Move through rows |
| `a` | All view |
| `d` | Changes view |
| `i` | Issues view |
| `/` | Start plain search or structured filter input |
| `f` | Open the filter drawer |
| `Enter` | Open selected observation details |
| `Esc` | Close details, cancel filtering, or clear filters |
| `p` | Prepare a selected path for copying |
| `c` | Prepare selected observation JSON for copying |
| `o` | Prepare a safe `open -R PATH` command |
| `q` / `Ctrl-C` | Quit |

The drawer shows Search, View, Source, Role, Kind, Version, and Status. Plain
text searches across display name, command name, source, version, status, kind,
and path. Structured filters use simple AND semantics:

```text
source:npm gemini
role:available
kind:application
version:unknown
status:shadowed
view:history
```

Values separated by commas are alternatives within one facet. Active filters
are shown as removable chips with a match count. Empty results list the active
filters and suggest clearing them. Regex is not enabled by default.

The detail actions do not access a clipboard or execute a command themselves;
they return a value or argument-separated command for the caller to present or
integrate. This keeps the TUI read-only and avoids shell interpolation.

## Persistence and v1 Migration

The default persistence files are:

```text
~/.toolsniff/registry.json
~/.toolsniff/availability.json
```

The v2 registry envelope is:

```json
{
  "schema_version": 2,
  "observations": []
}
```

The loader accepts either this envelope or a legacy v1 array of flat `Tool`
objects. Migration is deterministic and keeps every input item:

1. Treat a top-level JSON array as schema version 1.
2. Convert each `Tool` to an `Observation` through the legacy adapter.
3. Map `Source` and `Role` to evidence-backed origin, kind, and role.
4. Map `Path` to a typed location.
5. Map a non-empty ordinary `Version` to `version.state=known`.
6. Map an empty package version to `not-reported`.
7. Map an empty executable version to `unknown`.
8. Detect npx history dates and move them to `history.last_used`; never keep them as versions.
9. Recompute the stable observation ID from the v2 identity components.
10. Keep installed and availability registries separate.

Loading a v1 file does not modify it. `--save` writes a validated v2 envelope
only after a successful scan and uses an atomic replacement. A corrupt or
unsupported file becomes an empty baseline with a warning; it is not silently
overwritten. v2 decoding is strict about the schema version, required arrays,
observation validation, unknown fields, and trailing JSON values.

## Privacy and Security

Normal inventory output is local data and includes paths because locations are
part of the inventory. Exported data has a stricter contract:

- Home-directory prefixes become `$HOME`.
- Likely assignments containing passwords, tokens, API keys, authorization, bearer values, or private keys are redacted.
- Probe results, probe errors, evidence arrays, and warning strings are omitted from sanitized exports.
- Raw command output is never persisted as observation data.
- Location SHA-256 values are cleared from exports.
- No inventory or support bundle is uploaded automatically.
- Executable probes and hashes are opt-in.
- Update and uninstall behavior is separate from collection.

The sanitizer is also applied before profile comparison, so a profile loaded
from disk is compared in the same privacy-safe representation used for export.

## Configuration and Compatibility

The TOML file controls scanner roots, exclusions, Bun, themes, npx and Cargo
locations, registry path, and the external command timeout. Environment
variables override the corresponding TOML values. See
[`configuration.md`](configuration.md) for the complete settings and mode
reference.

The following schema versions are intentionally independent:

| Contract | Version |
| --- | ---: |
| Registry and availability | 2 |
| Report JSON | 2 |
| Snapshot | 1 |
| Profile and support bundle | 1 |

The application version identifies the toolsniff binary and is recorded in
snapshots, profiles, and support bundles separately from their schema versions.
Release compatibility rules are documented in [`releasing.md`](releasing.md).
