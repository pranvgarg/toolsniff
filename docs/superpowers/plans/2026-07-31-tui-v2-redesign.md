# toolsniff TUI: v2 migration + redesign — Wave Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan wave by wave. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild the post-splash TUI view as a bordered sidebar+content layout with real keyboard tab navigation, on Bubbletea v2 (current stable, not the v1 this project shipped on), using official `bubbles` components (`table`, `help`, `key`, `timer`) instead of hand-rolled equivalents.

**Architecture:** Same `output` package, same `RunTUI` entry point and signature. Internals change: `charm.land/bubbletea/v2` + `charm.land/bubbles/v2` + `charm.land/lipgloss/v2` replace the `github.com/charmbracelet/...` v1 imports; `bubbles/table` replaces `bubbles/list` for the content pane; `bubbles/help` + `key.Map` replace the hand-written footer string; a `debounce`-pattern wrapper replaces raw `tea.WindowSizeMsg` handling.

**Tech Stack:** Go 1.26 (already satisfies v2's `go 1.25.0` floor), `charm.land/bubbletea/v2` v2.0.8, `charm.land/bubbles/v2` v2.1.1, `charm.land/lipgloss/v2` v2.0.5.

## Global Constraints

- Scope is limited to `output/tui_model.go`, `output/tui_item.go`, `output/tui_styles.go`, `output/tui_splash.go`, `go.mod`, `go.sum`. `main.go` never imports `bubbletea` directly (it only calls `output.RunTUI(...)`) and must not need changes.
- Color tokens are fixed and already correct: `colorAmber = #ffb454`, `colorCyan = #7fd8c4`, `colorMuted = #5c6577` (in `output/tui_styles.go`) — reuse them, do not introduce new hex values.
- The amber-for-"new"-tab convention is already established in `output/table.go`'s `NEW SINCE LAST SCAN` handling — match it, don't invent a different treatment for the "new" tab.
- Tab cycling is wrap-around (from the last tab, `→` goes back to the first) — this is a deliberate deviation from the upstream `tabs` example, which clamps at the edges instead. Confirmed with the user.
- No automated tests exist for the TUI view layer by design (matches the pattern already used for `tui_splash.go`) — every task is verified by build/vet/gofmt plus a manual smoke test via tmux (or equivalent), not new `_test.go` files, unless a task explicitly says otherwise.
- Every implementer must confirm `pwd` and `git branch --show-current` show `worktree-implement-toolsniff` as its literal first command — this project has twice had a subagent's work land in the wrong repo location.
- End-user package quality: idiomatic Go, `gofmt`-clean, `go vet`-clean, no swallowed errors.

---

## Wave 1: v2 migration (mechanical only, foundation)

**Gate:** `go build ./...`, `go vet ./...`, `gofmt -l .` all clean. `go test ./...` still green (existing `output/table_test.go`/`output/json_test.go` must not break — they don't touch the TUI). Manual smoke test: launch the program, confirm splash still plays (hold → dissolve → main view) and the pre-existing keys (`tab`, `s`, `d`, `q`, `/`) still work exactly as before — this wave changes nothing user-visible, only the plumbing underneath. Load-bearing: every later wave is written directly against v2 APIs.

### Task 1: Migrate to Bubbletea v2

**Files:**
- Modify: `go.mod`, `go.sum`
- Modify: `output/tui_model.go`
- Modify: `output/tui_item.go`
- Modify: `output/tui_styles.go`
- Modify: `output/tui_splash.go`

**Interfaces:**
- Consumes: nothing new — this is a like-for-like migration of what already exists.
- Produces: the same `RunTUI(realTools, npxHistory []model.Tool, diff registry.Diff, warnings []scanner.Warning, regPath string) error` signature, now built on v2. Every later task in this plan is written against the v2 APIs this task establishes.

The current file contents (read in full before starting, this is the exact starting point):
- `output/tui_model.go` — 179 lines, defines `tuiModel`, `newTUIModel`, `itemsFor`, `Init`, `Update`, `View`, `renderTabBar`, `RunTUI`.
- `output/tui_splash.go` — defines `splashPhase`, the splash state machine (`updateSplash`, `dissolveWordmark`, `renderSplash`), using `tea.Tick`.
- `output/tui_styles.go` — just color/style vars, no `bubbletea`/`bubbles` imports at all (only `lipgloss`) — still needs its `lipgloss` import path bumped to v2, nothing else in this file changes.
- `output/tui_item.go` — `toolItem` implementing `list.Item` (`Title()`, `Description()`, `FilterValue()`) — stays as-is in this task (still used by `list.Model` in v2 form); it gets deleted in Wave 3 when `table` replaces `list` entirely. Don't delete it here.

**Steps:**

- [ ] **Step 1: Bump dependencies**

Run from the worktree root:
```bash
go get charm.land/bubbletea/v2@v2.0.8
go get charm.land/bubbles/v2@v2.1.1
go get charm.land/lipgloss/v2@v2.0.5
go mod tidy
```
Expected: `go.mod` now requires `charm.land/bubbletea/v2 v2.0.8`, `charm.land/bubbles/v2 v2.1.1`, `charm.land/lipgloss/v2 v2.0.5`; the old `github.com/charmbracelet/{bubbletea,bubbles,lipgloss}` v1 direct requires are gone (they may briefly remain as indirect until Step 2 removes all v1 imports — that's fine, `go mod tidy` after Step 2 will clean them up).

- [ ] **Step 2: Update every import**

In all four files, replace:
- `"github.com/charmbracelet/bubbletea"` → `tea "charm.land/bubbletea/v2"`
- `"github.com/charmbracelet/bubbles/list"` → `"charm.land/bubbles/v2/list"`
- `"github.com/charmbracelet/lipgloss"` → `"charm.land/lipgloss/v2"`

- [ ] **Step 3: Apply the three mechanical v2 API changes**

Confirmed against the official `UPGRADE_GUIDE_V2.md` and the v2.0.8-tagged `tabs`/`window-size` examples:

1. **`tea.KeyMsg` → `tea.KeyPressMsg`.** Every `case tea.KeyMsg:` type-switch case, and every place a variable is typed as `tea.KeyMsg`, becomes `tea.KeyPressMsg`. `msg.String()` still exists and behaves the same way on the new type — no changes needed to the string comparisons themselves (`"q"`, `"ctrl+c"`, `"tab"`, etc. all still match).
2. **`View() string` → `View() tea.View`.** Both `tuiModel.View()` (in `tui_model.go`) and `renderSplash` (in `tui_splash.go`, called from `View()`) currently return a plain `string`. Wrap the final string in `tea.NewView(...)` before returning from `tuiModel.View()`. `renderSplash` itself can keep returning a plain `string` (it's a helper, not the `tea.Model` interface method) — only the actual `View() tea.View` method on `tuiModel` needs the wrapper.
3. **Alt-screen mode.** `RunTUI` currently does `tea.NewProgram(newTUIModel(...), tea.WithAltScreen())`. In v2, `tea.WithAltScreen()` as a `NewProgram` option is gone — alt-screen is now a field set on the returned `tea.View` (`v.AltScreen = true`, per the upgrade guide's `tea.NewView` documentation). Set this every time `tuiModel.View()` builds its `tea.View`, both in the splash branch and the main branch, so alt-screen mode is active for the whole program lifetime, matching current behavior exactly.

- [ ] **Step 4: Verify it builds**

Run: `go build ./...`
Expected: clean build, no errors. If there are compile errors beyond the three changes above, they're additional v1→v2 API differences this task's research didn't anticipate — read the actual compiler error, consult `https://raw.githubusercontent.com/charmbracelet/bubbletea/main/UPGRADE_GUIDE_V2.md` for the specific API in question, and fix it minimally. Do not guess.

- [ ] **Step 5: Verify no swallowed regressions**

Run: `go vet ./...` and `gofmt -l .` — both must be silent.
Run: `go test ./...` — all packages must still report `ok`.

- [ ] **Step 6: Manual smoke test**

Build the real binary (`go build -o toolsniff .`) and run it (or `go run .`) in a terminal or tmux session. Confirm, in order: splash appears immediately, holds ~1.5s, dissolves, main view takes over; a keypress during the splash hold skips straight to the main view without also triggering that key's normal action; `tab` still cycles forward through sources one at a time; `s` still saves and shows a status message; `d` still jumps to the "new" tab if one exists; `q` still quits; `/` still opens the list's built-in filter. This is a pure migration — if any of this behaves differently than before the migration, something was missed in Step 3.

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum output/tui_model.go output/tui_item.go output/tui_styles.go output/tui_splash.go
git commit -m "Migrate TUI to Bubbletea v2 (charm.land/bubbletea/v2, bubbles/v2, lipgloss/v2)"
```

---

## Wave 2: Splash timer + layout foundation

**Depends on:** Wave 1
**Gate:** `go build ./...`/`vet`/`gofmt`/`test` clean for both tasks' combined result. Manual smoke test of each task's own surface (see each task) plus a combined check: splash still works correctly stacked on top of the new bordered layout underneath it. Load-bearing: Wave 3's table sizing and Wave 4's keybindings both build directly on Task 3's frame/sidebar structure.

### Task 2: Splash hold timer → `bubbles/timer`

**Files:**
- Modify: `output/tui_splash.go`

**Interfaces:**
- Consumes: nothing new beyond what Wave 1 already migrated to v2.
- Produces: same public behavior (`updateSplash`, `renderSplash`, `splashHoldCmd` may be renamed/removed as an implementation detail — nothing outside `tui_splash.go` calls these directly except `tuiModel.Init()`'s call to kick off the hold, and `tuiModel.Update()`'s delegation to `updateSplash` when `splashPhase != splashDone`, both of which must keep working with whatever the new internal names are).

**Steps:**

- [ ] **Step 1: Replace the hold-phase timer**

Currently, `splashHoldCmd()` returns `tea.Tick(splashHoldDuration, func(time.Time) tea.Msg { return splashHoldDoneMsg{} })`, and `Init()` calls it directly. Replace this with `charm.land/bubbles/v2/timer`, matching the v2 `timer` example's pattern: create a `timer.Model` via `timer.NewWithInterval(splashHoldDuration, splashHoldDuration)` (a single-shot timer — interval equal to timeout means it fires once and stops), store it as a field on `tuiModel` (or pass it through the splash state — your call on exact placement, keep it scoped to splash-only state), start it from `Init()` via its own `Init()` command, and listen for `timer.TimeoutMsg` in place of the current custom `splashHoldDoneMsg` to transition from `splashHold` to `splashDissolve1`. The dissolve-phase logic (`splashDissolveCmd`, `dissolveWordmark`, the `splashDissolveTickMsg` handling) is unaffected — leave it exactly as-is, this task only touches the initial hold.

- [ ] **Step 2: Verify it builds and behaves identically**

`go build ./...`, `go vet ./...`, `gofmt -l .` clean.

- [ ] **Step 3: Manual smoke test**

Run the program, confirm the hold is still ~1.5s (a stopwatch-precise check isn't required, just "clearly about a second and a half, not instant, not much longer"), confirm the dissolve still plays after it, confirm a keypress during the hold still skips straight to the main view.

- [ ] **Step 4: Commit**

```bash
git add output/tui_splash.go
git commit -m "Use bubbles/timer for the splash hold phase instead of raw tea.Tick"
```

### Task 3: Bordered sidebar + header layout

**Files:**
- Modify: `output/tui_model.go`
- Modify: `output/tui_styles.go` (new styles for border, header, sidebar rows)

**Interfaces:**
- Consumes: `m.width`, `m.height` (already populated from `tea.WindowSizeMsg` in `Update`), `m.tabs`, `m.activeTab`, `m.toolsBySrc` (all already on `tuiModel`).
- Produces: the new `View()` structure — a bordered frame containing a header line and a sidebar — that Wave 3's Task 4 will render its content pane into (via `lipgloss.JoinHorizontal`), and that Wave 4's Task 5/6 will attach keybindings and debounced resizing to. Establish a clear width constant or field for the sidebar's fixed width (e.g. a package-level `const sidebarWidth = 22`) and expose however much width remains for the content pane (e.g. a computed value passed into whatever renders the content area) — Task 4 needs this number.

**Steps:**

- [ ] **Step 1: Build the bordered frame**

Wrap the whole post-splash view in a single `lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(colorMuted)` frame sized to `m.width`/`m.height` (same border style already used for the splash screen in `tui_splash.go` — reuse `splashBorder`'s style definition or define an equivalent, don't diverge visually between splash and main view).

- [ ] **Step 2: Build the header**

Render `◆ toolsniff` in `colorAmber` bold, ` dev & AI CLI inventory` in `colorMuted`, and a right-aligned stats stamp showing the total real-tool count and source count (`N tools · M sources`, numbers in `colorCyan`) — compute these from `m.toolsBySrc`/`m.tabs` (sum of counts across non-"npx-history" sources for "tools", count of tabs excluding "npx-history" and "new" for "sources" — match the counting convention already used in `output/table.go`'s "N tools across M sources" line for the `--list` renderer, don't invent a different counting rule). Place this header as the first content line inside the top border, or overlaid on the border itself — your call on the exact `lipgloss` mechanism, but it must read as one continuous top edge, not a separate box stacked above the frame.

- [ ] **Step 3: Build the vertical sidebar**

Fixed width (~22 cols, matching `sidebarWidth`). One row per tab: number (`1`–`9`, only as many digits as tabs exist, max 9 tabs), source name, right-aligned count. Active row: background/left-border tint in `colorCyan`, except when the active tab is `"new"`, which uses `colorAmber` and a `⚠` marker instead — matching `output/table.go`'s established amber-for-new convention. Inactive rows: `colorMuted`.

- [ ] **Step 4: Pressure-test the floor**

When `m.width < 60`, don't render the vertical sidebar at all — instead render a single compact top strip above the (still-to-come, Wave 3) content area: the active tab shown as `[N name·count]`, every other tab shown as a bare number, the "new" tab (if present) still marked with `⚠` regardless of active/inactive. Implement as a width-conditional branch in `View()` (or a helper it calls) — an explicit `if m.width < 60 { ... } else { ... }`, not an attempt to make one layout gracefully degrade automatically.

- [ ] **Step 5: Leave the content area as a placeholder for now**

This task does not touch what's rendered to the right of the sidebar — Wave 3 Task 4 replaces that. For this task, it's fine for the content area to still show whatever `m.list.View()` currently produces (the pre-existing `list.Model`, now on v2 per Wave 1) — the goal here is the frame/header/sidebar chrome around it, not the content itself. Compose via `lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)`.

- [ ] **Step 6: Verify**

`go build ./...`, `go vet ./...`, `gofmt -l .` clean, `go test ./...` still green.

- [ ] **Step 7: Manual smoke test**

Run the program (skip the splash with a keypress), confirm: bordered frame renders at the terminal's actual size, header shows the wordmark/tagline/stats stamp, sidebar shows all tabs with correct counts and correct active-tab styling (cyan normally, amber+⚠ when "new" is active), old `tab` key still cycles the active tab and the sidebar highlight follows it. Then resize the terminal (tmux `resize-window` or manually) below 60 columns and confirm the sidebar collapses to the compact top-strip form; resize back above 60 and confirm it returns to the full vertical sidebar.

- [ ] **Step 8: Commit**

```bash
git add output/tui_model.go output/tui_styles.go
git commit -m "Add bordered sidebar+header layout with narrow-terminal fallback"
```

---

## Wave 3: Content pane on `bubbles/table`

**Depends on:** Wave 2 (needs Task 3's content-pane width)
**Gate:** `go build ./...`/`vet`/`gofmt`/`test` clean. Manual smoke test: content pane renders single-line rows (name left, version/path right-aligned), selection highlight visible and moves with `↑`/`↓`, switching tabs rebuilds the table with the new source's rows, `/` filtering still narrows the visible rows live. Load-bearing: Wave 4's keybinding task dispatches around this table's own key handling and must not double-handle what `table.Model` already does internally.

### Task 4: Replace `list.Model` with `table.Model`

**Files:**
- Modify: `output/tui_model.go`
- Delete: `output/tui_item.go` (its `toolItem`/`itemsFor` become dead code once nothing constructs `list.Item`s anymore)

**Interfaces:**
- Consumes: `model.Tool{Name, Source, Version, Path}` (unchanged), the content-pane width established in Wave 2 Task 3.
- Produces: the content pane's final rendering, which Wave 4's help/keybinding task and debounce task both build around without needing to know table internals beyond "it's a `table.Model` field on `tuiModel`."

**Steps:**

- [ ] **Step 1: Replace the `list.Model` field**

Remove the `list list.Model` field from `tuiModel`, remove the `github.com/charmbracelet/bubbles/list` (now `charm.land/bubbles/v2/list`) import, add `charm.land/bubbles/v2/table` and add a `content table.Model` field (name your choice, keep it distinct from any future field named plain `table` to avoid shadowing the package name).

- [ ] **Step 2: Define columns and row-building**

Two columns: `{Title: "Name", Width: <content width minus version column width minus padding>}`, `{Title: "Version", Width: <fixed, e.g. 12>}` — matching the v2 `table` example's `table.Column{Title, Width}` shape. Since `table` renders each column left-aligned by default and has no per-column alignment option in this version, right-align the version/path column yourself: when building each `table.Row{name, versionOrPath}`, right-pad... no — right-*align* the version string within its column width by left-padding with spaces (e.g. `fmt.Sprintf("%*s", width, value)`) before inserting it into the row. For entries where `Version == ""`, fall back to `Path` (matching `toolItem.Description()`'s existing fallback behavior in the file you're deleting — preserve that behavior here). Write a helper (e.g. `rowsFor(tools []model.Tool, width int) []table.Row`) that both the initial construction and every tab-switch call into.

- [ ] **Step 3: Wire selection, focus, and construction**

`table.New(table.WithColumns(...), table.WithRows(rowsFor(...)), table.WithFocused(true), table.WithHeight(...))`. Apply `table.DefaultStyles()` with the selected-row style overridden to use `colorCyan` (matching the sidebar's active-row convention), via `table.WithStyles(...)` or `m.content.SetStyles(...)` — whichever the v2 API exposes (check the actual v2 `table` package if the v1-pattern doesn't translate directly; the `table` example fetched at the v2.0.8 tag shows the exact current API, defer to that over any v1-era assumption).

- [ ] **Step 4: Rebuild on tab switch**

Wherever the old code called `m.list.SetItems(itemsFor(...))` and `m.list.Title = ...` (the `tab`/`d` key handlers, soon to be replaced in Wave 4 but still present at the start of this task), replace with `m.content.SetRows(rowsFor(m.toolsBySrc[m.tabs[m.activeTab]], contentWidth))`. Selection should reset to the first row on tab switch (matches current `list.Model` behavior when items are replaced) — confirm this is `table.Model`'s default behavior when `SetRows` is called; if it preserves the old cursor position instead, explicitly reset it (`m.content.SetCursor(0)` or equivalent — check the actual v2 API for the right call).

- [ ] **Step 5: Filter (`/`)**

`table.Model` has no built-in fuzzy filter, unlike `list.Model`. Add a `filterQuery string` and `filtering bool` field to `tuiModel`. On `/`, enter filtering mode. While filtering, printable keypresses append to `filterQuery`, `backspace` removes the last character, `esc` clears the query and exits filtering, `enter` exits filtering but keeps the current query active. On every change to `filterQuery`, recompute the visible rows as the subset of the active tab's tools whose `Name` contains the query (case-insensitive substring match, matching the convention already used elsewhere in this project — check `scanner/applications.go`'s keyword matching for the established case-insensitive-substring idiom) and call `m.content.SetRows(...)` with that filtered subset. Show a live match count somewhere sensible (header stats line or a small indicator near the content pane) — exact placement is your call, but the count must be visible while filtering, matching the fzf-style "shown/total" convention.

- [ ] **Step 6: Delete `tui_item.go`**

`rm output/tui_item.go` — confirm nothing else in the package still references `toolItem` or `itemsFor` before deleting (`grep -rn "toolItem\|itemsFor" output/`).

- [ ] **Step 7: Verify**

`go build ./...`, `go vet ./...`, `gofmt -l .` clean, `go test ./...` still green (confirm `output/table_test.go` — note the *different* `table.go`/`table_test.go`, the `--list` renderer, is completely unrelated to `bubbles/table` and must not be confused with it or accidentally touched).

- [ ] **Step 8: Manual smoke test**

Run the program, confirm: content pane shows one row per tool, name left, version/path right-aligned; `↑`/`↓` move the selection with visible highlighting; switching tabs (`tab` key, still the old handler at this point) rebuilds the table with the new source's rows, selection reset to the top; `/` opens filtering, typing narrows the visible rows live with a visible match count, `esc` clears it.

- [ ] **Step 9: Commit**

```bash
git add -A
git commit -m "Replace list.Model content pane with bubbles/table"
```

---

## Wave 4: Keybindings, help, and resize debouncing

**Depends on:** Wave 3
**Gate (final):** Full verification — `go build ./...`, `go vet ./...`, `gofmt -l .`, `go test ./...`, `go test -race ./...` all clean. Manual smoke test covering the complete feature set end to end (see Task 5/6's own smoke tests plus a combined pass). This is the last wave — nothing downstream depends on it, but it's still gated with full rigor since it's the actual user-facing deliverable.

### Task 5: `key.Map` + `bubbles/help` + tab/jump navigation

**Files:**
- Modify: `output/tui_model.go`

**Interfaces:**
- Consumes: `table.Model`'s own built-in key handling (Wave 3) — must not double-handle `↑`/`↓`/`j`/`k`, which `table.Model.Update` already processes internally when it has focus.
- Produces: the final footer/help rendering and the final keybinding set — nothing later in this plan builds on top of this, it's a leaf within the wave but the wave itself gates the whole plan's completion.

**Steps:**

- [ ] **Step 1: Define the key map**

Add `charm.land/bubbles/v2/key` and `charm.land/bubbles/v2/help` imports. Define a `keyMap` struct with `key.Binding` fields for: `Up`, `Down` (informational only — actually handled by `table.Model`, included here just so `help` can display them), `PrevTab` (keys `"left"`, `"h"`), `NextTab` (keys `"right"`, `"l"`, `"tab"` — `tab` stays as a synonym for next, matching the pre-existing key), `Filter` (`"/"`), `JumpTab` (a synthetic entry for the help display representing `1`-`9`, since `key.Binding` doesn't cleanly express "any digit" as one binding — check whether the `help` example's pattern handles this, or use `key.WithKeys("1","2","3","4","5","6","7","8","9")` explicitly, which does work), `Diff` (`"d"`), `Save` (`"s"`), `Help` (`"?"`), `Quit` (`"q"`, `"ctrl+c"`). Each `key.Binding` via `key.NewBinding(key.WithKeys(...), key.WithHelp("short", "description"))`, matching the v2 `help` example exactly.

- [ ] **Step 2: Implement `ShortHelp()`/`FullHelp()`**

`ShortHelp() []key.Binding` returns the 5 most useful: `Up/Down` collapsed into one combined binding for the footer (or however the `help` example handles a combined up/down entry — check its exact pattern), `PrevTab/NextTab` combined, `Filter`, `Help`, `Quit`. `FullHelp() [][]key.Binding` returns everything, grouped in a couple of columns per the `help` example's convention.

- [ ] **Step 3: Wire `help.Model`**

Add a `help help.Model` field (`help.New()`), and a way to toggle `help.ShowAll` on `?`. Replace the current hand-written `footerStyle.Render("↑↓ move · tab switch · / filter · d diff · s save · q quit")` line in `View()` with `m.help.View(m.keys)`.

- [ ] **Step 4: Wire navigation**

Replace the current `case "tab":` handler (which only advances one tab, one direction) with `key.Matches(msg, m.keys.PrevTab)`/`key.Matches(msg, m.keys.NextTab)` — both wrap around (from tab 0, prev goes to the last tab; from the last tab, next goes to tab 0 — confirmed deliberate deviation from the upstream `tabs` example's clamping behavior). Add digit-key handling (`1`-`9`) that jumps directly to that 1-indexed tab if it exists, silently ignored otherwise. On any tab change (prev/next/jump/the old `d`-jumps-to-new), call the same row-rebuild helper from Wave 3 Task 4 and reset `filterQuery`/`filtering` state (switching tabs should not carry a stale filter over from the previous tab — confirm this matches user expectation; if you think carrying it over is actually better, flag it in your report rather than silently deciding either way).

- [ ] **Step 5: Dispatch via `key.Matches`, not raw `msg.String()`**

Convert the remaining handlers (`Filter`/`Diff`/`Save`/`Help`/`Quit`) to `key.Matches(msg, m.keys.X)` checks, matching the `help` example's dispatch style, replacing the old `switch msg.String() { case "q", "ctrl+c": ... }` raw-string pattern.

- [ ] **Step 6: Verify**

`go build ./...`, `go vet ./...`, `gofmt -l .` clean, `go test ./...` still green.

- [ ] **Step 7: Manual smoke test**

Run the program, confirm: `←`/`h` and `→`/`l` cycle tabs with wrap-around at both ends; `1`-`9` jump directly (including confirming a digit beyond the tab count does nothing, no crash, no error); `tab` still works as next-tab; `↑`/`↓`/`j`/`k` still move the table selection (via table's own built-in handling, not double-triggered by anything you added); `?` toggles between short footer and full help; `/`, `d`, `s`, `q` all still work exactly as before.

- [ ] **Step 8: Commit**

```bash
git add output/tui_model.go
git commit -m "Replace hand-rolled keybindings with key.Map + bubbles/help, add wrap-around tab nav and numeric jump"
```

### Task 6: Debounce resize handling

**Files:**
- Modify: `output/tui_model.go`

**Interfaces:**
- Consumes: the existing `tea.WindowSizeMsg` handling in `Update` (currently: set `m.width`/`m.height`, resize the content component, return immediately).
- Produces: nothing anything else in this plan depends on — this is the last task, purely a robustness improvement.

**Steps:**

- [ ] **Step 1: Add a resize-generation tag**

Matching the v2 `debounce` example's pattern: add a `resizeTag int` field to `tuiModel`. On every `tea.WindowSizeMsg`, increment the tag, store the new width/height as before, and return a `tea.Tick`-based command carrying the current tag value (e.g. a `resizeSettledMsg{tag: m.resizeTag}` fired after a short delay, ~100-150ms per the example).

- [ ] **Step 2: Only actually re-layout on a matching tag**

When a `resizeSettledMsg` arrives, only trigger the (potentially expensive) re-layout work — resizing the `table.Model`, recomputing sidebar-vs-compact-strip mode — if `msg.tag == m.resizeTag` (i.e., no newer resize event has arrived since this one was scheduled). Stale settle messages from superseded resize events are silently dropped.

Note: `m.width`/`m.height` should still update immediately on every raw `WindowSizeMsg` (so `View()` always has the latest terminal dimensions for its own border sizing) — it's specifically the *derived* layout work (table column widths, sidebar-vs-compact-strip mode) that gets debounced, not the dimensions themselves. Read the `debounce` example closely for the exact split between "update immediately" and "settle before acting."

- [ ] **Step 3: Verify**

`go build ./...`, `go vet ./...`, `gofmt -l .` clean, `go test ./...` still green, `go test -race ./...` clean (this task introduces timing-sensitive message passing — confirm no race).

- [ ] **Step 4: Manual smoke test**

In tmux, resize the pane rapidly several times in succession (or drag a real terminal window's edge quickly) and confirm the layout doesn't visibly stutter/flicker/thrash — it should settle cleanly on the final size shortly after resizing stops, not re-render on every intermediate size.

- [ ] **Step 5: Commit**

```bash
git add output/tui_model.go
git commit -m "Debounce window-resize re-layout to avoid thrashing on rapid resize"
```

---

## Final verification (after Wave 4)

1. `go build -o toolsniff .` from the worktree root — succeeds.
2. `go test ./...` and `go test -race ./...` — both clean.
3. Run the real binary: confirm splash → main frame transition, bordered sidebar+content layout matches the agreed ASCII mockup, every keybinding from Task 5's smoke test still works, resize floor (<60 cols) still collapses correctly, resize debouncing (Task 6) doesn't stutter.
4. Re-run `./toolsniff --list`/`--json`/`--diff`/`--save` — unaffected by this plan, but cheap to confirm nothing regressed in `main.go` or the non-TUI `output` renderers.
5. Whole-branch review (per this project's established process) before any merge decision, same as the original 16-task build.
