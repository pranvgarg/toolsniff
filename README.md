```
████████╗ ██████╗  ██████╗ ██╗         ███████╗███╗   ██╗██╗███████╗███████╗
╚══██╔══╝██╔═══██╗██╔═══██╗██║         ██╔════╝████╗  ██║██║██╔════╝██╔════╝
   ██║   ██║   ██║██║   ██║██║         ███████╗██╔██╗ ██║██║█████╗  █████╗
   ██║   ██║   ██║██║   ██║██║         ╚════██║██║╚██╗██║██║██╔══╝  ██╔══╝
   ██║   ╚██████╔╝╚██████╔╝███████╗    ███████║██║ ╚████║██║██║     ██║
   ╚═╝    ╚═════╝  ╚═════╝ ╚══════╝    ╚══════╝╚═╝  ╚═══╝╚═╝╚═╝     ╚═╝
```

# toolsniff

`toolsniff` discovers developer tools and AI applications installed or
available on a macOS machine and presents them as one inventory. It scans
package managers, application roots, Bun's global bin directory, and the
user's `PATH`, then tracks changes against a saved baseline.

The inventory is discovery-based. It does not contain a compiled list of
known AI tools. A new CLI or application is found automatically if it appears
in one of the scanned locations.

## What It Scans

The scanners run concurrently:

- **npm global** — top-level packages reported by `npm ls -g`.
- **npx history** — packages found in npm's npx cache. Informational only; it
  is not treated as an installed tool or included in the baseline.
- **Homebrew formulae** — packages reported by `brew list --formula`.
- **Homebrew casks** — applications reported by `brew list --cask`.
- **pipx** — isolated Python applications reported by `pipx list --json`.
- **Cargo** — executable files in Cargo's bin directory.
- **Bun** — executable files in the directory returned by `bun pm bin -g`.
- **Applications** — every `.app` bundle under `/Applications` and
  `~/Applications`, without keyword filtering. Application bundles are not
  recursively inspected internally.
- **PATH** — executable files in the directories in `$PATH`, excluding the
  standard macOS system directories.

Sources are intentionally kept separate. If the same command is installed
through Homebrew and npm, both observations remain visible. A PATH result is
reported as an available command, not falsely presented as another package
manager installation.

## Install

### Homebrew

Homebrew installation is available from the toolsniff tap:

```bash
brew tap pranvgarg/toolsniff
# Homebrew 6 may require this once for a custom tap:
brew trust pranvgarg/toolsniff
brew install pranvgarg/toolsniff/toolsniff
```

The formula is the preferred Homebrew installation. Releases provide a
prebuilt macOS binary, and the formula can be served through a manually
published Homebrew bottle. If the formula bottle is not usable on a particular
Homebrew/macOS combination, the tap also provides a cask fallback:

```bash
brew install --cask pranvgarg/toolsniff/toolsniff
```

Choose either the formula or the cask, rather than installing both. The
formula and cask are separate Homebrew installations and toolsniff will report
them as separate sources.

### Direct Installer

The release installer does not require Go or Homebrew. It detects Apple
Silicon or Intel, downloads the matching GitHub release archive, verifies its
SHA-256 checksum, and installs the binary at `~/.local/bin/toolsniff`:

```bash
curl -fsSL https://raw.githubusercontent.com/pranvgarg/toolsniff/main/install.sh | sh
```

For a checked-out repository, run the same installer directly:

```bash
./install.sh
```

Add `~/.local/bin` to `PATH` if the installer reports that it is not already
there. Direct installations are not managed by Homebrew, so `toolsniff
--update` does not update them; rerun `install.sh` to install a newer release.

### Build From Source

```bash
git clone https://github.com/pranvgarg/toolsniff.git
cd toolsniff
go build -o toolsniff .
```

The current release targets macOS and requires Go 1.26 or newer for source
builds.

### Homebrew Troubleshooting

The formula and cask install a prebuilt macOS binary. They do not compile
toolsniff from source. Homebrew itself may require current Apple Command Line
Tools, even for a prebuilt formula or cask.

If Homebrew reports that the Command Line Tools are outdated:

```bash
xcode-select --install
```

If macOS reports that they are already installed but Homebrew still rejects
them, update Command Line Tools through **System Settings > General > Software
Update**. You can inspect the active developer tools path with:

```bash
xcode-select -p
```

After updating, refresh Homebrew and retry:

```bash
brew update
brew install pranvgarg/toolsniff/toolsniff
```

Only if the installer remains stuck on an outdated standalone Command Line
Tools installation should you reinstall it:

```bash
sudo rm -rf /Library/Developer/CommandLineTools
xcode-select --install
```

These Command Line Tools requirements are Homebrew prerequisites, not a
runtime requirement of the prebuilt toolsniff binary. If Homebrew cannot be
made usable on the Mac, use the direct installer above instead.

## Quick Start

Run a one-time report first:

```bash
toolsniff --list
```

Save the current installed-tool inventory as your baseline:

```bash
toolsniff --save
```

Saving also records current PATH availability separately from installed tools.

Later, check what was installed or removed:

```bash
toolsniff --diff
```

Include PATH availability changes when needed:

```bash
toolsniff --diff --available
```

Update a Homebrew installation when a newer release is available:

```bash
toolsniff --update
```

This checks whether toolsniff was installed as a formula or cask, checks that
source for an update, and asks for confirmation only when an update is
available. Use `--yes` for a non-interactive confirmation:

```bash
toolsniff --update --yes
```

Formula installations use `brew upgrade toolsniff`; cask installations use
`brew upgrade --cask toolsniff`. If both the formula and cask are installed,
the update is rejected as ambiguous. If toolsniff was installed directly or
from source, these flags do not update it.

Launch the interactive interface when you want to browse sources, filter
results, or change the theme:

```bash
toolsniff
```

The first scan may show warnings for package managers that are not installed.
Those warnings do not prevent the other scanners from completing.

## Usage

```text
toolsniff                  # launch the interactive TUI
toolsniff --list           # print a grouped table and exit
toolsniff --json           # print a machine-readable report and exit
toolsniff --save           # save the current installed observations
toolsniff --diff           # show changes since the saved baseline
toolsniff --update         # update a Homebrew-managed toolsniff installation
toolsniff --update --yes   # update without prompting
toolsniff --version        # print the installed release version
toolsniff --config FILE    # use an explicit TOML configuration file
```

Only one report mode can be selected at a time.

## Configuration

The default configuration file is:

```text
~/.config/toolsniff/config.toml
```

The file is optional. A default installation works without it. Configuration
is for discovery policy and noise control, not for maintaining a list of tool
names.

Example:

```toml
[applications]
roots = ["/Applications", "~/Applications"]
ignore_paths = ["~/Applications/Old Tools"]

[path]
exclude_directories = ["/custom/system/bin"]
ignore_names = ["internal-debug-tool"]

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

Configuration precedence is:

1. Command-line flags
2. `TOOLSNIFF_*` environment variables
3. The TOML configuration file
4. Platform-aware defaults

Useful environment overrides include:

```bash
TOOLSNIFF_CONFIG=/tmp/toolsniff.toml toolsniff --list
TOOLSNIFF_REGISTRY=/tmp/registry.json toolsniff --save
TOOLSNIFF_EXEC_TIMEOUT=15s toolsniff --json
```

The scanner also respects standard environment values such as
`NPM_CONFIG_CACHE`, `CARGO_HOME`, and `PATH`.

### Themes

The TUI includes built-in `toolsniff`, `midnight`, `nord`, `mono`, and
`high-contrast` themes.

Inside the TUI:

- Press `?` to open the full help view, then use `t` to open the theme picker.
- Press `t` directly to open the theme picker.
- Type `/theme` and press `enter` to open it through the command prompt.
- Use `↑` / `↓` or `j` / `k` to choose a theme.
- Press `enter` to apply and persist it.
- Press `esc` to cancel.

The selected theme is saved to the configured TOML file. You can also select
one before starting the TUI:

```toml
[theme]
preset = "nord"
```

Individual selection colors can be overridden under `[theme.colors]` without
changing the source code.

## Output

`--list` groups observations by source. Installed observations and PATH
availability are counted separately:

```text
BREW-FORMULA (2)
  jq                             1.7.1
  ripgrep                        14.1.0

PATH (1)
  gh                             /opt/homebrew/bin/gh

NPX HISTORY (1, informational)
  create-vite                    2026-07-31

2 installed tools and 1 available command across 2 sources
```

`--json` keeps installed tools and current PATH availability in separate
collections and includes source role and path information when available:

```json
{
  "tools": [
    {
      "name": "jq",
      "source": "brew-formula",
      "role": "installed",
      "version": "1.7.1",
      "path": ""
    }
  ],
  "available": [
    {
      "name": "gh",
      "source": "path",
      "role": "available",
      "version": "",
      "path": "/opt/homebrew/bin/gh"
    }
  ],
  "npx_history": [],
  "added": [],
  "removed": [],
  "updated": [],
  "available_added": [],
  "available_removed": [],
  "available_updated": [],
  "warnings": []
}
```

## Baseline and Diff

Run `toolsniff --save` to create separate installed and PATH availability
baselines. Later, `toolsniff --diff` reports installed observations that were
added, removed, or updated. Use `toolsniff --diff --available` to include PATH
availability changes. The installed registry is stored at
`~/.toolsniff/registry.json` and the availability registry at
`~/.toolsniff/availability.json` by default. The installed registry can be
relocated through config or environment variables; the availability registry
remains its sibling file.

The registries keep different sources and executable paths separate. Installing
the same package through two package managers is therefore represented as two
real observations rather than one merged, ambiguous record.

When an observation keeps the same source and identity but changes from one
known version to another, the diff reports an update. Path changes remain a
removal plus an addition.

The baseline is intentionally separate from npx history. npx history records
one-off cached executions and can change frequently, so it is shown for
reference but does not create installation-change alerts.

## TUI Controls

- `↑` / `↓` — move through the active source
- `←` / `→` / `tab` — switch source
- `1`–`9` — jump to a source
- `/` — filter the active source temporarily
- `d` — jump to the new-observations tab
- `s` — save the installed baseline
- `t` — open the theme picker
- `/theme` — open the theme picker through the command-style filter prompt
- `?` — toggle help
- `q` — quit

The theme picker applies a preset immediately and saves the selected preset to
the configured TOML file. Use `↑` / `↓` to choose, `enter` to apply, and
`esc` to cancel.

## Scope

The current release targets macOS. Linux support will add platform-specific
package-manager and application-root scanners without changing the scanner
interface.
