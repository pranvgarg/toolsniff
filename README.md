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
brew install toolsniff
```

### Build From Source

```bash
git clone https://github.com/pranvgarg/toolsniff.git
cd toolsniff
go build -o toolsniff .
```

The current release targets macOS and requires Go 1.26 or newer for source
builds.

### Homebrew Troubleshooting

The formula installs a prebuilt macOS binary. It does not compile toolsniff
from source. Homebrew itself may require current Apple Command Line Tools,
even for a prebuilt formula.

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
brew install toolsniff
```

Only if the installer remains stuck on an outdated standalone Command Line
Tools installation should you reinstall it:

```bash
sudo rm -rf /Library/Developer/CommandLineTools
xcode-select --install
```

## Quick Start

Run a one-time report first:

```bash
toolsniff --list
```

Save the current installed-tool inventory as your baseline:

```bash
toolsniff --save
```

Later, check what was installed or removed:

```bash
toolsniff --diff
```

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

`--json` includes source role and path information when available:

```json
{
  "tools": [
    {
      "name": "jq",
      "source": "brew-formula",
      "role": "installed",
      "version": "1.7.1",
      "path": ""
    },
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
  "warnings": []
}
```

## Baseline and Diff

Run `toolsniff --save` to create a baseline. Later, `toolsniff --diff` reports
installed observations that were added or removed. The registry is stored at
`~/.toolsniff/registry.json` by default and can be relocated through config or
environment variables.

The registry keeps different sources and executable paths separate. Installing
the same package through two package managers is therefore represented as two
real observations rather than one merged, ambiguous record.

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
