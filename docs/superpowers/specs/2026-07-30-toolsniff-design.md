# toolsniff — Design

## Purpose

A single-binary CLI/TUI that inventories dev and AI CLI tools installed on a
machine across multiple package managers and install locations, so the user
always has an answer to "what do I actually have installed, and what's new
since I last checked." Personal tool first; built cleanly enough to open-source
and eventually distribute via Homebrew.

## Scope (v1)

- macOS only. Linux support (different package managers, no `/Applications`)
  is an explicit v2 concern — the scanner interface is designed to make that
  additive, not a rewrite, but no Linux code ships in v1.
- Sources scanned: npm global, npx run-history, Homebrew formulae, Homebrew
  casks, pipx, cargo, Bun global binaries, user application roots, and raw
  `$PATH` executable discovery.
- No daemon, no background service, no auto-update. Invoked on demand, exits
  when done (or when the user quits the TUI).

## Architecture

Single Go module, `main` plus four packages:

- **`model`** — the shared `Tool` struct: `Name`, `Source`, `Version`, `Path`.
  `Source` is the scanner's own identifier and `Role` distinguishes installed,
  available-on-PATH, and informational history observations. Same-name tools
  from different installation sources remain separate records.

- **`scanner`** — one file per source, each implementing:
  ```go
  type Scanner interface {
      Name() string
      Scan() ([]model.Tool, error)
  }
  ```
  `main` runs all registered scanners concurrently (goroutines + `sync.WaitGroup`,
  fan-in via channel or mutex-guarded slice) and collects `(tools, err)` per
  scanner. A failing scanner (tool not installed, directory missing) does not
  abort the run — see Error Handling below.

  `npx.go` walks npm's configured npx cache and resolves package names through
  `.bin` symlinks. Bun discovery asks `bun pm bin -g` for its global bin
  directory instead of assuming a fixed path.

- **`registry`** — persists and diffs the baseline snapshot.
  - Location: `~/.toolsniff/registry.json`.
  - `Save(tools []model.Tool) error` — writes the current scan (excluding
    `npx-history` — see Decisions below) as the new baseline.
  - `Load() ([]model.Tool, error)` — reads the saved baseline; a missing or
    malformed file is not an error condition, it's treated as "no baseline
    yet" (empty slice + warning), so diff naturally shows everything as new
    on first run.
  - `Diff(old, new []model.Tool) (added, removed []model.Tool)` — pure
   function, keyed on `(Source, Path)` when a path is available and
   `(Source, Name)` otherwise.

- **`output`** — three renderers over the same `[]model.Tool` + diff result:
  - TUI (Bubbletea + Lipgloss) — default when no flags given.
  - `--list` — plain grouped table to stdout, exits.
  - `--json` — full scan (including warnings and diff) as JSON, exits.

  No renderer reaches back into `scanner` or `registry` internals — they only
  see the data structures handed to them by `main`.

## CLI Surface

```
toolsniff                 # launch TUI (default)
toolsniff --list          # plain grouped table, exits
toolsniff --json          # full scan as JSON, exits
toolsniff --save          # scan, write result as new registry baseline, exit
toolsniff --diff          # scan, print only what changed vs registry, exit
```

TUI keybindings: `↑↓` move · `tab` switch pane · `/` filter · `d` toggle diff
view · `s` save baseline · `q` quit.

## Data Flow

1. `main` builds the list of registered `Scanner`s.
2. Runs them concurrently; collects `[]model.Tool` + any per-scanner errors.
3. Splits the combined result: `npx-history` tools go into a separate bucket,
   everything else is "real installs."
4. Loads the saved registry (real installs only).
5. Computes `Diff(registryTools, currentRealInstalls)`.
6. Hands `{realInstalls, npxHistory, diff, warnings}` to whichever `output`
   renderer the flags selected.

## Decisions Worth Recording

- **npx history is informational, not inventory.** It reflects one-off
  `npx <pkg>` runs — much of it generated automatically by MCP clients
  spinning up servers, not deliberate installs. It gets its own TUI pane and
  its own `--json` field, but:
  - it is excluded from the top-line tool count,
  - it is excluded from `registry.Save` (never becomes part of the baseline),
  - it is excluded from `Diff` (never triggers a "new since last scan" alert).
  Rationale: npx cache churns constantly and isn't a deliberate action: 
  treating it as a peer of "you ran `brew install x`" would make the diff
  noisy to the point of being ignored.

- **Registry format is JSON, not YAML.** No new dependency, trivial to
  `encoding/json` marshal/unmarshal, and it's a machine-written/machine-read
  file — human-editability wasn't a requirement.

- **Project name: `toolsniff`**, not `drivers-finder`. Checked against GitHub
  search: `drivers-finder` was technically unclaimed but every real-world use
  of that phrase is a hardware/ROS device-driver tool, which misleads at a
  glance. `toolscope`, `stackscan`, `cliradar`, `pkgradar`, and `devscan` were
  all already taken (`devscan` in particular is doing something very close to
  this). `toolsniff` had zero collisions and its name directly describes the
  action (sniffing out installed tools across sources).

## Error Handling

- A scanner failing (binary not on `$PATH`, expected directory missing) never
  aborts the whole run. Its `Scan()` returns `(nil, err)`; `main` records the
  error as a warning tied to that source and continues collecting from the
  rest.
- Warnings surface as a dim status line in the TUI and a `"warnings"` array
  in `--json` output — visible, never silent, never fatal.
- A missing or corrupt `registry.json` is treated as "empty baseline," not an
  error: first run (or a wiped registry) naturally shows every real install
  as "new."

## Testing

- `scanner/*_test.go`: each scanner tested against fixture command output
  (not a live `brew`/`npm` install) so tests are hermetic and don't depend on
  what's installed in CI.
- `registry/diff_test.go`: pure function, table-driven tests covering
  added/removed/unchanged/empty-baseline cases.
- No automated tests for the Bubbletea view layer in v1 — low value for a
  personal tool without an SLA; verified manually by running it.

## Out of Scope (v1)

- Linux/Windows support.
- Auto-refresh / watch mode.
- Homebrew tap / `goreleaser` packaging (planned once the tool is proven
  locally and pushed to GitHub — tracked as follow-up work, not part of this
  spec).
