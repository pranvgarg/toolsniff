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
