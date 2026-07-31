```
████████╗ ██████╗  ██████╗ ██╗         ███████╗███╗   ██╗██╗███████╗███████╗
╚══██╔══╝██╔═══██╗██╔═══██╗██║         ██╔════╝████╗  ██║██║██╔════╝██╔════╝
   ██║   ██║   ██║██║   ██║██║         ███████╗██╔██╗ ██║██║█████╗  █████╗
   ██║   ██║   ██║██║   ██║██║         ╚════██║██║╚██╗██║██║██╔══╝  ██╔══╝
   ██║   ╚██████╔╝╚██████╔╝███████╗    ███████║██║ ╚████║██║██║     ██║
   ╚═╝    ╚═════╝  ╚═════╝ ╚══════╝    ╚══════╝╚═╝  ╚═══╝╚═╝╚═╝     ╚═╝
```

# toolsniff

Scans this machine for installed dev/AI CLI tools across npm, npx history,
Homebrew (formulae + casks), pipx, cargo, `/Applications`, and `$PATH`, and
shows them in an interactive terminal UI. Tracks what's new since the last
scan via a saved baseline.

## What it does

toolsniff runs eight scanners in parallel, each looking at a different place
tools tend to get installed, and merges the results into one view:

- **npm global** (`npm ls -g`) — packages installed globally with npm.
- **npx history** — packages you've run via `npx` without installing them.
  This is shown separately and *not* counted as "installed" — it's cache
  from `~/.npm/_npx`, not a deliberate install, and it churns constantly as
  you run one-off commands. Informational only.
- **Homebrew formula** (`brew list --formula`) — CLI tools installed via
  Homebrew.
- **Homebrew cask** (`brew list --cask`) — GUI apps installed via Homebrew.
- **pipx** — Python CLI tools installed in isolated environments via pipx.
- **cargo** — Rust binaries installed with `cargo install` (read from your
  cargo bin directory).
- **Applications** — a keyword-filtered scan of `/Applications`, looking for
  bundle names matching known dev/AI-relevant terms (e.g. `claude`,
  `chatgpt`, `cursor`, `ollama`, `docker`, `windsurf`, `github desktop`).
  This is filtered, not a full inventory of every installed app.
- **`$PATH`** — a curated list of known dev/AI CLI tool names (e.g. `claude`,
  `gh`, `ollama`, `codex`, `aider`, `ngrok`, `uv`) checked against your
  `$PATH` with `exec.LookPath`. This is curated, not exhaustive — it checks
  for specific known tools rather than scanning every binary on the system.

## Install

Requires Go 1.26+.

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

### `--list`

Groups tools by source, uppercased, with a count per group. `npx-history`
gets its own labeled block since it's informational, and a "new since last
scan" block appears if a baseline exists and something changed. A trimmed
example:

```
BREW-FORMULA (2)
  jq                             1.7.1
  ripgrep                        14.1.0

PATH (1)
  gh

NPX HISTORY (2, informational)
  create-react-app               2024-03-11
  cowsay                         2023-11-02

3 tools across 2 sources
```

### `--json`

Full machine-readable report: real tools, npx history, the added/removed
diff against your saved baseline, and any scanner warnings. A trimmed
example:

```json
{
  "tools": [
    { "name": "jq", "source": "brew-formula", "version": "1.7.1", "path": "" },
    { "name": "gh", "source": "path", "version": "", "path": "/opt/homebrew/bin/gh" }
  ],
  "npx_history": [
    { "name": "cowsay", "source": "npx-history", "version": "2023-11-02", "path": "" }
  ],
  "added": [],
  "removed": [],
  "warnings": []
}
```

## Why the baseline/diff matters

Run `toolsniff --save` once to record everything currently installed as a
baseline. Any time after that, `toolsniff --diff` (or just glancing at the
TUI's "new" tab) tells you exactly what's changed since — what got added or
removed. That's useful for noticing when something got installed by a setup
script, an AI coding agent, or a `brew bundle` run you didn't watch closely,
without having to manually diff your tool list in your head.

## TUI walkthrough

- `↑` / `↓` — move the selection up and down within the current tab's list.
- `tab` — cycle to the next source tab (npm, brew-formula, brew-cask, pipx,
  cargo, applications, path, npx-history, and "new" if there's a diff).
- `/` — start filtering the current tab's list by typed text.
- `d` — jump straight to the "new since last scan" tab. This tab only
  exists (and the key only does something) if there's an actual diff
  against your saved baseline.
- `s` — save the current scan as the new baseline; a status message at the
  bottom confirms success or reports the failure.
- `q` — quit.

## Scope

macOS only for now. Linux support (different package managers, no
`/Applications` equivalent) is a planned follow-up — see
`docs/superpowers/specs/2026-07-30-toolsniff-design.md`.
