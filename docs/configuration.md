# Configuration

toolsniff is discovery-first. It does not require a list of known AI or
developer tools. It discovers application bundles and executable files from
the configured user locations.

## File Location

The default file is:

```text
~/.config/toolsniff/config.toml
```

Use `--config` or `TOOLSNIFF_CONFIG` to select another file.

## Settings

```toml
[applications]
roots = ["/Applications", "~/Applications"]
ignore_paths = []

[path]
exclude_directories = []
ignore_names = []

[npx]
dir = "~/.npm/_npx"

[cargo]
bin_dir = "~/.cargo/bin"

[bun]
enabled = true

[theme]
preset = "toolsniff"

[theme.colors]
# Optional overrides. Colors must use #RRGGBB values.
# selection_background = "#7fd8c4"
# selection_foreground = "#081018"

[registry]
path = "~/.toolsniff/registry.json"

[execution]
timeout = "8s"
```

`applications.roots` controls user application roots. By default toolsniff
uses `/Applications` and `~/Applications`, and does not scan
`/System/Applications`.

`applications.ignore_paths` excludes application paths without affecting
other application roots.

`path.exclude_directories` adds directories to the default system-directory
exclusions. `path.ignore_names` hides specific executable names after PATH
discovery. These are optional noise controls, not required tool inventories.

`bun.enabled` controls discovery through Bun's `bun pm bin -g` command. Bun's
reported global bin directory is used; toolsniff does not assume a fixed Bun
installation path.

The configured registry path stores installed observations. PATH availability
is stored separately in a sibling `availability.json` file, so executable
availability cannot become an installation claim. For example, the default
files are:

```text
~/.toolsniff/registry.json
~/.toolsniff/availability.json
```

`toolsniff --save` updates both files. `toolsniff --diff` compares installed
observations, while `toolsniff --diff --available` also compares PATH
availability.

Both files use the v2 registry envelope after a save:

```json
{
  "schema_version": 2,
  "observations": []
}
```

Legacy v1 JSON arrays are accepted and converted in memory. A successful
`--save` writes the converted v2 form atomically. Read-only modes such as
`--doctor`, `--json`, and `--snapshot` do not rewrite the registry.

`theme.preset` controls the complete TUI palette. Available presets are
`toolsniff`, `midnight`, `nord`, `mono`, and `high-contrast`. Individual
semantic colors can override a preset without changing source code. The active
source and selected table row both use the selection foreground/background
tokens, so the two-pane selection remains visually consistent.

The same presets can be selected from inside the TUI with `t` or by entering
`/theme`. Applying a preset immediately rebuilds the Bubble Tea/Lip Gloss
styles and saves the selected preset to the configured TOML file. Use `esc` to
close the picker without changing the theme.

## Environment Overrides

Environment values take priority over the TOML file:

| Variable | Purpose |
| --- | --- |
| `TOOLSNIFF_CONFIG` | Configuration file path |
| `TOOLSNIFF_THEME` | TUI theme preset |
| `TOOLSNIFF_APPLICATION_ROOTS` | PATH-list of application roots |
| `TOOLSNIFF_PATH_DIRECTORIES` | PATH-list of directories to scan |
| `TOOLSNIFF_PATH_EXCLUDE` | PATH-list of additional exclusions |
| `TOOLSNIFF_NPX_DIR` | npx history directory |
| `TOOLSNIFF_CARGO_BIN_DIR` | Cargo binary directory |
| `TOOLSNIFF_REGISTRY` | Registry file path |
| `TOOLSNIFF_EXEC_TIMEOUT` | External command timeout, such as `15s` |

Standard environment values are also respected where applicable:

- `PATH`
- `NPM_CONFIG_CACHE`
- `CARGO_HOME`

## Source Roles

Each result has a source and a role:

- `installed` means a package manager or application root reported an
  installation.
- `available` means an executable was found on PATH. This does not claim how
  it was installed.
- `history` means a cached one-off execution such as npx history.

The same name from two installation sources remains separate. Exact duplicate
observations from the same source and path are deduplicated.

## Command Modes

The scan-backed modes use the configured scanners and registry unless noted:

| Mode | Behavior |
| --- | --- |
| `--doctor` | Prints typed, read-only health issues and package-to-location provenance. It does not save baselines or alter configuration. |
| `--snapshot` | Saves installed and available observations, excluding history, under `~/.toolsniff/snapshots/`. Snapshot files are timestamped and written atomically. |
| `--snapshots` | Lists valid snapshots newest-first without scanning and without loading the TOML configuration. |
| `--export-profile FILE` | Scans and writes a sanitized profile to `FILE`. |
| `--compare-profile FILE` | Scans, loads a profile, and prints typed additions, removals, updates, and other observation changes. |
| `--support-bundle FILE` | Scans and writes the narrower sanitized support-bundle form to `FILE`. |
| `--capabilities` | Scans and prints JSON results only for capabilities supported by explicit metadata or evidence. It does not run executable probes. |
| `--capabilities --capabilities-probe` | Enables bounded `--version` probes for active executable observations. The probe flag is only valid with `--capabilities`. |

Only one report, snapshot, profile, support, or update mode can be selected at
once. `FILE` may be relative or absolute; parent directories are created with
owner-only permissions when toolsniff writes an export.

## TUI Filters

The interactive TUI has intent-oriented views for `all`, `installed`,
`available`, `changes`, `issues`, and `history`. Press `/` for plain search or
`f` for the filter drawer. The input accepts normal text and optional ANDed
facets:

```text
gemini
source:npm gemini
kind:application
version:unknown
status:updated
view:history
```

Facet values within one facet are alternatives; populated facets are combined
with AND semantics. Regex is not enabled. Press `Enter` to open read-only
details for a row; `Esc` closes the current mode or clears active filters.
`p`, `c`, and `o` prepare a path, observation JSON value, or Finder reveal
command for the selected row without executing a shell command.

## Safe Execution and Privacy

Package-manager commands are executed directly with the configured
`execution.timeout` (8 seconds by default). The normal scan does not probe
arbitrary executables for versions. Capability detection is evidence-based;
names alone are never treated as capability evidence.

`--export-profile` and `--support-bundle` redact home paths as `$HOME`, redact
likely secret assignments, omit raw evidence and probe payloads, and clear
location hashes. Normal local inventory output is not redacted because paths
are part of its purpose. See
[`observation-model-v2.md`](observation-model-v2.md) for the complete schema
and privacy contract.
