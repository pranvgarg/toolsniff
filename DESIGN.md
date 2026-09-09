---
name: toolsniff
description: Terminal-native dark design system for the toolsniff TUI
version: 1.0.0
platform: terminal
medium: character-cell grid, monospace, 16/256/truecolor
colors:
  # Structure — never carry meaning on their own.
  background: "#0B0D10"
  surface: "#12161C"
  surface-selected: "#1E2733"
  border: "#566171"
  border-focus: "#FFB454"
  # Text.
  foreground: "#E6EAF0"
  muted: "#8B95A5"
  # The single accent. Brand, active tab, selection bar, focus.
  accent: "#FFB454"
  accent-contrast: "#0B0D10"
  # Semantic status.
  status-installed: "#5FD68A"
  status-available: "#93A4C0"
  status-broken: "#FF7A7A"
  status-shadowed: "#FFC24B"
  status-history: "#B49BE0"
  status-warning: "#FFC24B"
  status-warning-surface: "#4A3215"
  status-error: "#FF7A7A"
  status-badge: "#FFB454"
spacing:
  # 4px-step scale, expressed in character cells at ~8px cell advance.
  # A cell is the atomic unit; sm/md/lg are cell counts, not pixels.
  none: 0
  sm: 1   #  4px — intra-cell gutter, cell padding
  md: 2   #  8px — pane padding, column gutter
  lg: 3   # 12px — section separation
  xl: 4   # 16px — modal padding
typography:
  family: monospace (terminal-provided; never assume a specific face)
  scale: fixed — one cell, one glyph, no size axis
  roles:
    header:  { weight: bold,   color: accent,     transform: none }
    column:  { weight: bold,   color: muted,      transform: uppercase }
    body:    { weight: normal, color: foreground, transform: none }
    meta:    { weight: normal, color: muted,      transform: none }
    badge:   { weight: bold,   color: accent,     transform: none }
    warning: { weight: bold,   color: status-warning, transform: none }
layout:
  min-terminal: 80x24
  compact-threshold: 60
  frame: single bordered region; header embedded in top border
  regions: [header, sidebar, content, separator, footer]
  sidebar: fixed-width, label left-aligned, count right-aligned to shared column
  content: responsive column ladder at 45 / 75 cells
elevation:
  # The terminal has no z-axis. "Elevation" is background lift + border weight.
  0: background        # the frame interior
  1: surface           # panels, drawers, detail panes
  2: surface-selected  # the selected row, accent bar at column 0
  3: border-focus      # modal / focused pane border
shapes:
  frame: rounded (╭ ╮ ╰ ╯)
  panel: rounded
  divider: light single line (─ │ ├ ┤ ┴)
  ascii-fallback: + - |
---

# toolsniff — Terminal Design System

## Overview

toolsniff is an inventory scanner. Its TUI answers one question at a glance:
**what is on this machine, and what state is it in?** Everything below serves
that question.

The aesthetic is *terminal-native dark* in the Linear / VoltAgent lineage:
near-black ground, one restrained accent, high information density, crisp
hierarchy, monospace throughout. No gradients, no decoration, no second accent.
Color is a semantic channel, not a mood.

Four rules govern every decision:

1. **Color encodes state, never appearance.** `installed`, `available`,
   `broken` are tokens. They are also always paired with a glyph and a Status
   column, so the UI survives `NO_COLOR`, monochrome terminals, and red-green
   colour vision deficiency.
2. **One accent.** Amber `#FFB454` marks the brand, the active tab, the
   selection bar, and focus. Nothing else competes for it.
3. **A PATH-only command is a weaker signal than an installed one.** It is
   merely PATH-visible — not something the user chose to install. It is rendered
   in a cool desaturated slate, deliberately quieter than the positive green
   used for installed tools.
4. **Navigation is kind-first, and speaks English.** The primary axis is *what
   each thing is* — CLI tools, packages, applications, things on your PATH, npx
   history — never which bucket a scanner filled. A status is a filter, not a
   place. Every user-facing noun and its one plain sentence live in
   `output/kinds.go`; see Components > "No jargon".

The token block in the front matter is the contract.
`output/tui_styles.go` is its only implementation.

## Colors

All ratios are computed against the frame background `#0B0D10` unless stated
otherwise, using the WCAG 2.x relative-luminance formula. **Every foreground
token clears 4.5:1 (AA normal text).**

### Structure

| Token | Hex | Role | Contrast vs `background` |
|---|---|---|---|
| `background` | `#0B0D10` | frame interior, terminal ground | — |
| `surface` | `#12161C` | panels, filter drawer, detail pane | 1.46:1 (lift only) |
| `surface-selected` | `#1E2733` | the selected row's block | 1.29:1 vs `surface` |
| `border` | `#566171` | frame, sidebar divider, separators | **3.10:1** (WCAG 1.4.11 non-text ✓) |
| `border-focus` | `#FFB454` | focused pane / modal border | 11.03:1 |

`border` is deliberately pushed to 3.10:1 rather than the more fashionable
near-invisible `#232A33` (1.33:1) so pane boundaries satisfy WCAG 1.4.11 for
non-text UI components. Structure is never the *only* signal — every pane is
also labelled — but a boundary you cannot see is not a boundary.

### Text

| Token | Hex | Role | Contrast |
|---|---|---|---|
| `foreground` | `#E6EAF0` | primary content, tool names | **16.11:1** |
| `muted` | `#8B95A5` | column headers, metadata, hints, inactive tabs | **6.43:1** |

`muted` is `#8B95A5`, not the more common `#5C6577`. The latter measures
**3.23:1** on this ground and fails AA — it is the single most common accessibility
defect in dark terminal themes. Dimness is achieved with hue desaturation, not
by dropping luminance below the floor.

### Accent

| Token | Hex | Role | Contrast |
|---|---|---|---|
| `accent` | `#FFB454` | wordmark, active tab, selection bar, counts | **11.03:1** |
| `accent-contrast` | `#0B0D10` | text *on* an accent block | 11.03:1 (inverse pair) |

### Semantic status

| Token | Hex | Meaning | Glyph | vs `background` | vs `surface-selected` |
|---|---|---|---|---|---|
| `status-installed` | `#5FD68A` | user installed it; it is theirs | `●` | **10.63:1** | **8.24:1** |
| `status-available` | `#93A4C0` | PATH-visible only; weaker signal | `○` | **7.70:1** | **5.97:1** |
| `status-broken` | `#FF7A7A` | recorded but no longer resolvable | `✗` | **7.71:1** | **5.97:1** |
| `status-shadowed` | `#FFC24B` | resolvable, but another entry wins PATH | `▲` | **12.11:1** | **8.68:1** |
| `status-history` | `#B49BE0` | npx cache; ran once, not installed | `·` | **8.07:1** | **6.25:1** |
| `status-warning` | `#FFC24B` | scanner degraded, partial result | `⚠` | **12.11:1** | — |
| `status-error` | `#FF7A7A` | operation failed | `✗` | **7.71:1** | — |

`status-warning` on `status-warning-surface` (`#4A3215`) measures **7.44:1** —
that pair is used for the warning-tinted tab chip only.

Note that `accent` (`#FFB454`) and `status-warning` (`#FFC24B`) share a hue
family. They are never confusable in practice because they never appear in the
same role: accent is *positional* (the thing you are on), warning is
*prefixed* (always led by `⚠` and, on the tab chip, backed by a tinted block).

### Runtime contrast guard

Users may select a preset (`toolsniff`, `midnight`, `nord`, `mono`,
`high-contrast`) or override individual colors in `~/.config/toolsniff/config.toml`.
Those values are *not* trusted to be accessible. `output/tui_styles.go` runs
every resolved text token through `ensureContrast(fg, bg, 4.5)`, which walks the
color toward the far end of the luminance axis in small steps until it clears
the ratio. `border` is guarded at 3.0. A user theme therefore cannot render the
UI unreadable; it can only shift hue.

### Adaptive downsampling

`colorprofile.Detect(os.Stdout, os.Environ())` runs once. Each token is then
resolved through `lipgloss.Complete(profile)(ansi, ansi256, truecolor)` with
hand-picked ANSI (16-color) and ANSI256 fallbacks, so the semantic separation
survives on a 256-color or 16-color terminal:

| Token | ANSI | ANSI256 | TrueColor |
|---|---|---|---|
| `foreground` | 7 | 253 | `#E6EAF0` |
| `muted` | 8 | 246 | `#8B95A5` |
| `accent` | 3 | 215 | `#FFB454` |
| `status-installed` | 2 | 114 | `#5FD68A` |
| `status-available` | 4 | 110 | `#93A4C0` |
| `status-broken` | 1 | 210 | `#FF7A7A` |
| `status-shadowed` | 3 | 221 | `#FFC24B` |
| `status-history` | 5 | 141 | `#B49BE0` |
| `border` | 8 | 243 | `#566171` |
| `surface-selected` | 0 | 236 | `#1E2733` |

`NO_COLOR` is honoured by `colorprofile.Detect`, which returns
`colorprofile.NoTTY`/`Ascii`; every style then degrades to weight and glyph,
both of which carry the same information.

## Typography

The terminal has one type size. Hierarchy comes from weight, color, case,
position, and whitespace — nothing else.

| Role | Weight | Color | Case | Used for |
|---|---|---|---|---|
| header | bold | `accent` | as-written | `◆ toolsniff` wordmark, modal titles |
| tagline | normal | `muted` | as-written | `dev & AI CLI inventory` |
| stats | normal | `muted` | as-written | `47 installed · 68 available · 6 sources` |
| column | bold | `muted` | UPPERCASE | table column headers |
| body | normal | `foreground` | as-written | tool names, values |
| status | normal | *semantic* | lowercase | the Status cell and its row tint |
| meta | normal | `muted` | as-written | versions, sources, counts, hints |
| badge | bold | `accent` | as-written | tab counts, `13/13 shown` |
| warning | bold | `status-warning` | as-written | `⚠ warning: …` |

Never use blink. Never use italic as the only signal. Underline is reserved for
the active tab as a monochrome-safe secondary cue.

## Layout

```
╭─ ◆ toolsniff ─ dev & AI CLI inventory ──── 47 installed · 68 available ─╮
│  1 all         203 │ NAME            VERSION   STATUS     SOURCE       │
│ ▎2 installed    47 │ ● gh            2.62.0    installed  brew-formula │
│  3 available    68 │ ○ awk           unknown   available  path         │
│  4 history      12 │ ✗ oldtool       1.0.0     broken     npm          │
│  5 changes       0 │                                                   │
│  6 issues        2 │                                     13/13 shown   │
├────────────────────┴───────────────────────────────────────────────────┤
│ ⚠ warning: homebrew: exit status 1                                     │
│ ↑/↓ move · ←/→ tab · / filter · f facets · ? help · q quit              │
╰────────────────────────────────────────────────────────────────────────╯
```

**Regions, in fixed positions. Panels never move without explicit user action.**

- **Header** — embedded in the top border. Wordmark, tagline, stats stamp.
  Degrades by dropping the tagline, then the stats, before it can overflow.
- **Sidebar** — one row per view. `▎` accent bar marks the active row, `N`
  gives the jump key, label left-aligned, count **right-aligned to a shared
  column** so the numbers scan vertically.
- **Content** — filter/summary bar, then the responsive table, then a
  right-aligned `n/m shown` badge when the rows do not fill the pane.
- **Separator** — `├───┴───┤` with the `┴` at the measured sidebar boundary.
- **Footer** — warnings first (severity, `⚠` prefix, `status-warning`), then
  status messages, then keybinding hints in `muted`. A user's eye must be able
  to jump to a real problem without reading the hint row.

### Intent-based navigation (`ui.mode = intent`)

`ui.mode = "intent"` (`output/tui_v2.go`, `intentReportTabs`) replaces the eight
kind-first sidebar rows with four named for what the user came to **do**. This
is a **re-grouping of the same observations — not new data, and not a second
visual language.** Every tab's title and gloss comes from `ViewLabel` /
`ViewMeaning` in `output/kinds.go`, and every count from `CountForView`, exactly
as in legacy.

| Tab | Holds | Grouped by |
|---|---|---|
| **Manage** — *everything installed by a manager, in one place* | the union of `ViewCLI` + `ViewPackages` + `ViewApplications` (`manageViews`) | the manager that installed the row, ranked by `sourceGroupOrder` |
| **Discover** — *what's on your PATH that no manager installed* | the union of `ViewPathExecutables` + `ViewNpxHistory` (`discoverViews`) | the directory the binary sits in, then the npx cache last (`discoverGroupOrder`) |
| **Review** — *what changed and what needs fixing* | typed change events, partitioned by `reviewSections` into **Changes** / **Issues** / **New** | section, in that order |
| **Health** — *updates, issues, and reclaimable space at a glance* | four cards: Updatable, Issues, Manage, Reclaimable | not a list — three cards count a view, the fourth is a placeholder |

`manageViews` and `discoverViews` are complements and together cover every kind
view, so no observation falls between the two tabs and none is listed twice.
`reviewSections` partitions the change categories the same way — every status a
change event can carry appears in exactly one block — which is why Review's
count is simply the number of events.

**No token moves for intent.** Every role is one legacy already defined:

- the active tab is `ActiveTab` — the same `accent` + bold + underline — with
  the same `▎` `accent` bar at column 0 (`SelectionBar`);
- Manage, Discover, and Review render through the same inventory table, so the
  status glyphs, semantic row tones, and `ColumnHeader` sub-group headings are
  unchanged; only the grouping key differs (manager · directory · section);
- Health's cards are Overview group rows, drawn by `renderOverviewBlock` — the
  same table, so the two dashboards align identically. Only its `Issues` card
  borrows a semantic tone, and only when non-zero;
- the Reclaimable card's `—` sits in the ordinary count cell (`Body`) with its
  gloss in `muted`, exactly like every other card. Nothing is styled to mark it
  as special; the wording carries that — see "Reclaimable placeholder".

Intent mode adds four names to the vocabulary. It adds **no color, no glyph, and no
second accent.**

### Spacing

The 4px scale maps to cell counts: `sm`=1 cell (~4px of advance in a typical
8px cell at half-step), `md`=2, `lg`=3, `xl`=4. In practice:

- cell padding inside table columns: `sm` (1)
- pane padding (sidebar and content, horizontal): `sm` (1) each side
- panel/drawer padding: `sm` vertical, `md` horizontal
- modal padding: `md` vertical, `xl` horizontal
- blank separator rows between sections: `sm` (1 row)

Every horizontal budget is **measured**, never hard-coded:
`style.GetHorizontalFrameSize()` sums borders and padding. No `-4`, no `-2`,
no `frameFixedCols = 7`.

### Responsive ladder

| Pane width | Columns | Behaviour |
|---|---|---|
| ≥ 105 | Name · Version · Kind · Status · Source · Action | full |
| 72–104 | Name · Version · Kind · Status · Source | Action drops |
| 60–71 | Name · Version · Kind · Status | Source drops |
| 48–59 | Name · Version · Status | Kind drops |
| < 48 | Name · Version | narrowest table form |

Below 80×24 the frame still renders; the sidebar collapses at 60 cells. The
status glyph column is never dropped — it is the last line of defence when
color is unavailable. Kind outranks Source in the ladder: the source is
recoverable from the kind marker plus the detail pane, but nothing else in a
row says whether a name is a package, an app, or a bare executable.

## Elevation

The terminal has no z-axis. Depth is background lift plus border weight.

| Level | Realised as | Used by |
|---|---|---|
| 0 | `background`, no border | frame interior, table body |
| 1 | `surface` + `border` rounded | filter drawer, detail pane |
| 2 | `surface-selected` + `▎` accent bar at column 0 | the selected row |
| 3 | `surface` + `border-focus` rounded | theme picker modal, splash |

Never nest more than one border between the terminal edge and the content. The
outer frame *is* the one border; panes inside it are separated by a single
divider line, not by their own boxes.

## Shapes

- **Frame and panels**: rounded `╭ ╮ ╰ ╯` — the modern Charm register, softer
  than sharp corners without reading as decorative.
- **Dividers**: light single line `─ │ ├ ┤ ┴`. Never double-line (`═ ║`) — it
  reads as DOS.
- **Selection bar**: `▎` (U+258E, left one-quarter block) in `accent`, column 0
  of the selected row. It is a *bar*, not a full-width inverse block, so the
  row's own semantic color survives selection.
- **Status glyphs**: `●` installed, `○` available, `✗` broken, `▲` shadowed,
  `·` history, `⚠` warning, `+`/`-`/`~` change events.
- **Kind glyphs**: `❯` cli, `◇` package, `▤` application, `▸` executable,
  `↺` history. Always followed by the text label — see Components.
- **ASCII fallback**: `+ - |` for the frame, `* o x ^ .` for status glyphs, and
  `> # @ = ~` for kind glyphs when the detected profile has no Unicode
  confidence.

## Components

Each component maps to exactly one recipe in `output/tui_styles.go`. Component
code never picks a color.

| Component | Style field | lipgloss v2 recipe |
|---|---|---|
| Wordmark | `HeaderTitle` | `Foreground(accent).Bold(true)` |
| Tagline | `HeaderTagline` | `Foreground(muted)` |
| Stats stamp | `HeaderStats` | `Foreground(muted)` |
| Frame border | `HeaderBorder` | `Foreground(border)` |
| Sidebar, inactive | `Tab` | `Foreground(muted)` |
| Sidebar, active | `ActiveTab` | `Foreground(accent).Bold(true).Underline(true)` |
| Sidebar, issues tab | `NewTab` | `Foreground(warning).Background(warningSurface)` |
| Sidebar, active issues | `ActiveNewTab` | `Foreground(accentContrast).Background(warning).Bold(true)` |
| Sidebar count | `Count` | `Foreground(muted)` (accent + bold when active) |
| Selection bar | `SelectionBar` | `Foreground(accent)` rendering `▎` |
| Selected row | `SelectedRow` | `Background(surfaceSelected).Bold(true)` |
| Table header | `Table.Header` | `Foreground(muted).Bold(true).Padding(0,1)` |
| Table cell | `Table.Cell` | `Foreground(foreground).Padding(0,1)` |
| Row — installed | `RowInstalled` | `Foreground(statusInstalled)` |
| Row — available | `RowAvailable` | `Foreground(statusAvailable)` |
| Row — broken | `RowBroken` | `Foreground(statusBroken).Bold(true)` |
| Row — shadowed | `RowShadowed` | `Foreground(statusShadowed)` |
| Row — history | `RowHistory` | `Foreground(statusHistory)` |
| Panel / drawer | `Panel` | `Border(RoundedBorder()).BorderForeground(border).Padding(1,2)` |
| Modal | `Modal` | `Border(RoundedBorder()).BorderForeground(accent).Padding(1,2)` |
| Detail heading | `DetailHeading` | `Foreground(accent).Bold(true)` |
| Detail label | `DetailLabel` | `Foreground(muted)` |
| Detail value | `DetailValue` | `Foreground(foreground)` |
| Footer hint | `Footer` | `Foreground(muted)` |
| Footer key | `FooterKey` | `Foreground(foreground).Bold(true)` |
| Footer warning | `Warning` | `Foreground(warning).Bold(true)` prefixed `⚠` |
| Status message | `Status` | `Foreground(accent)` |
| Row-count badge | `Badge` | `Foreground(accent).Bold(true)` |
| Empty state | `EmptyState` | `Foreground(muted).Italic(true)` |
| Kind marker | *(row's own style)* | glyph + label, no color of its own |
| Detail type line | `Badge` | `Foreground(accent).Bold(true)`, kind glyph prefixed |
| What this is line | `DetailLabel` + `DetailValue` | muted label, `Foreground(foreground)` sentence |
| Action row — label | `Badge` | `Foreground(accent).Bold(true)` |
| Action row — command | `Meta` | `Foreground(muted)`, prefixed `⧉` |
| Action row — inert | `DetailLabel` | `Foreground(muted)`, note in place of a command |
| Pane caption | `Badge` + `Meta` | accent title, then `· count · plain meaning` in muted |
| Overview group row | `Body` + `Meta` | body label, right-aligned count, muted gloss |
| Overview group row — nested | `Meta` | `Foreground(muted)`, label indented two cells |
| Overview group row — alerting | `RowBroken` | only when its count is non-zero |
| Kind sub-group header | `ColumnHeader` | `Foreground(muted).Bold(true)`, count in the VERSION cell |
| ACTION cell (inline) | *(row's cell style)* | `Foreground(muted)` override, ≥ 105 cells only |
| Multi-select mark | `SelectionBar` | `Foreground(accent)` rendering `▎` — the cursor's own bar |
| Health card | `Body` + `Meta` | an Overview group row: label, count, muted gloss |
| Unmeasured card value | `Body` | the count cell's own style rendering `—`, never `0` |

Layout composition uses `lipgloss.JoinHorizontal` / `JoinVertical` and
bordered styles. Tabular data uses `charm.land/lipgloss/v2/table` with a
`StyleFunc` keyed on row status — never hand-rolled `strings.Repeat` spacing.

### Kind marker

The KIND column answers "what sort of thing is this" — the question the name
and version cannot. It is a **glyph plus a short label**, never one or the
other:

| Kind | Marker | ASCII |
|---|---|---|
| `cli` | `❯ cli` | `> cli` |
| `package` | `◇ pkg` | `# pkg` |
| `application` | `▤ app` | `@ app` |
| `executable` | `▸ exe` | `= exe` |
| `history` | `↺ hist` | `~ hist` |

The marker carries **no color of its own** — it inherits the row's semantic
status style, so kind and status stay orthogonal and the accent still marks
exactly one thing (the selection). The label alone is sufficient under
`NO_COLOR`; the glyph alone is never relied upon. A row with no kind (a change
event) renders an empty cell rather than a misleading default.

The same glyph prefixes the detail pane's type line, so "the `◇ pkg` row" and
"npm global package" are visibly the same claim.

### Overview group row

The landing pane answers "what's on this machine", grouped by **what each thing
is** rather than by which scanner bucket it came from. One row per group: label,
right-aligned count, and a parenthesised plain-English gloss.

```
◆ toolsniff — what's on this machine

  CLI tools            12  (commands installed by a package manager)
  Packages             79  (installed for you by a package manager)
    npm                13  (Node.js packages installed globally)
    Homebrew           60  (formulae + casks)
    Cargo               2  (Rust binaries)
    pipx                4  (Python apps)
  Applications         47  (macOS .app bundles you open from Finder)
  On your PATH          9  (not installed by a manager)
  npx history           3  (run once via npx, not installed)
  ───────────────────────
  Changes               2  (since the last saved baseline)
  Issues                1  (broken or shadowed)

→ or 1-8 open a view · ↑/↓ then enter for details
```

- Every count comes from `CountForView`, the same predicate the pane it links to
  filters with, so a dashboard number can never disagree with its tab.
- The nested manager rows sum to the group above them. They are also the first
  thing dropped when the pane is too short — a truncated top-level list would
  hide a whole category, a collapsed breakdown hides nothing the Packages tab
  does not already show.
- Only `Issues` borrows a semantic tone, and only when it is non-zero. The
  accent still marks exactly one thing: where the user is.

### Kind sub-group header

Inside the CLI-tools and Packages panes, rows are grouped by the manager that
installed them, because "79 packages" is not a fact anyone can act on and
"60 Homebrew formulae, 13 npm packages" is.

```
   NAME                       VERSION  KIND    STATUS
   npm packages                     1
▎● left-pad                     1.0.0  ◇ pkg   installed
   Homebrew formulae                1
 ● wget                        1.24.5  ◇ pkg   installed
```

- A header is a table row whose NAME cell carries the label and whose VERSION
  cell carries the count — the numeric column is already right-aligned, and
  lipgloss has no colspan. One table render keeps every column aligned across
  every block.
- Headers are **not selectable**: only real rows carry an index, so selection
  always maps to an observation and the selected-row background never lands on
  a heading.
- The gloss (`npm packages — Node.js packages installed globally`) is appended
  only when it fits the NAME column. A narrow pane loses the explanation, never
  the label.

### What this is line

The detail pane's second line, under the type badge:

```
wget Details
◇ Homebrew formula
What this is       Homebrew formula — a command-line program installed via
                   Homebrew.
```

This is the **one place in the TUI that wraps instead of truncating**: half a
sentence explains nothing. Continuation lines are indented to the value column.

The sentence comes from the plain-language table in `output/kinds.go`, keyed by
kind *and* by origin, so a cask and a formula get different answers.

### No jargon

`installed` / `available` / `history` are scanner words. Two of them are
actively misleading on their own — "available" reads as "available to install"
when it means *"your shell can already run this; no manager installed it"*.

- The former `available` tab is now **"On your PATH (not installed by a
  manager)"** everywhere it is named: the tab strip (`on your PATH`), the pane
  caption, the header stamp (`7 installed · 1 on your PATH`), the group header,
  and the STATUS cell (`on PATH`).
- Never render one of those words without its plain meaning within a line or
  two. Every pane's first line is its caption — `Label · count · meaning` —
  which is what pays for that guarantee.
- The relabels are **display only**. `ObservationStatus`, `InventoryRow.Status`,
  the JSON, the plain-text table, and the `status:` / `view:` filter facets all
  keep the raw vocabulary, so nothing machine-readable moves and every status
  lens the kind-first tabs replaced is still reachable through the filter
  drawer.

### Action row

The detail pane's ACTIONS section lists what the user can *do* with an entry,
derived per kind in `output/actions.go`. It sits directly under the heading
block, **above** the metadata sections: the pane does not scroll, and an entry
with locations, availability, package, and evidence blocks is taller than the
pane.

```
ACTIONS
  Upgrade            ⧉ brew upgrade --cask visual-studio-code
  Uninstall          ⧉ brew uninstall --cask visual-studio-code
  Reveal in Finder   ⧉ open -R /Applications/Visual Studio Code.app
  Copy path            /Applications/Visual Studio Code.app
```

- **Label** in `accent` + bold (`Badge`) when the action is runnable.
- **Command** in `muted` (`Meta`), behind the copy glyph `⧉` (ascii `$`) — the
  glyph is the affordance that says "this is yankable", and it is what keeps
  a runnable row distinguishable from an inert one without color.
- **Inert rows** (an npx cache entry, a copy-path target) drop the label to
  `muted` and show the note where the command would be. Nothing that isn't
  runnable is styled as though it were.

Commands are display text built from already-captured metadata. `KindActions`
starts no process, touches no filesystem, and does no shell quoting — the
caller decides whether to show, copy, or run what it gets.

### ACTION column

At **≥ 105 cells** — the last rung of the responsive ladder — the inventory
table gains an inline ACTION column carrying the verb the detail pane would
offer for that row (`Upgrade`, `Uninstall`, …). The wording comes from
`output/actions.go` through `InventoryRowFromObservation`, so the cell can never
name an operation the Action row would not; a row with nothing runnable renders
an empty cell.

```
   NAME              VERSION   KIND    STATUS     SOURCE        ACTION
▎● gh                 2.62.0   ❯ cli   installed  brew-formula  Upgrade
 ● left-pad            1.0.0   ◇ pkg   installed  npm           Uninstall
 ○ awk               unknown   ▸ exe   on PATH    path
```

- The cell is **`muted`**, applied over the row's own cell style
  (`muteActionCell`). It is deliberately *not* the `Badge` accent the detail
  pane's "Action row — label" uses: in the pane the label is the one thing to
  read, whereas in a 200-row table a column of accent verbs would compete with
  the selection bar for the eye. Same two tokens, opposite ranking, because the
  surrounding density is opposite.
- The row's semantic status tone still owns every other cell, and selection is
  applied afterwards, so a selected row keeps its raised background.
- Action sits **well above** Source on the ladder rather than beside it. Every
  fixed-width column is paid for out of NAME; 105 is the first width at which
  NAME still holds the longest manager-group heading with ACTION present.

### Multi-select mark

Marking rows for a bulk action lights **the same `▎` `accent` bar the cursor
uses** (`rowMarks` + `styles.Glyph.Selection` in
`output/tui_inventory_table.go`). A marked row and the cursor row make the same
claim — *"this one"* — and the cursor is still told apart by its raised
`surface-selected` background.

```
▎● gh              2.62.0    ← cursor: accent bar + raised background
▎● left-pad         1.0.0    ← marked: accent bar only
 ● wget            1.24.5
```

Multi-select therefore introduces **no second highlight color**. A group heading
never carries a mark: only real rows hold a flat index, and a mark is keyed by
that index.

### Reclaimable placeholder

The Health dashboard's fourth card shows `—` (`healthUnknownValue`) where the
other three show a count, with the gloss *"caches and leftovers — not measured
yet"* (`healthReclaimableMeaning`, `output/kinds.go`).

**This is a deliberate deferred placeholder, not a missed value.** No scanner on
this machine measures reclaimable disk yet, so the card has no view behind it
the way Updatable, Issues, and Manage have `CountForView`. A dash is the only
honest render: `0` is a measurement, and would claim a scan ran and found
nothing.

Do not "fix" the card by hard-coding a number, and do not hide it — the gloss is
what tells a user the work is pending. When a disk scanner lands, the card gains
a real count and the wording in `output/kinds.go` retires with it.

## Do's and Don'ts

**Do**

- Pair every color with a glyph and a text label. A user in `NO_COLOR` must
  read the same information.
- Right-align numerics (counts, versions). Left-align text. Truncate, never
  wrap, inside a cell.
- Derive every width from `lipgloss.Width` or `GetHorizontalFrameSize()`.
- Show a count when the view is filtered or under-full (`13/203 shown`), so an
  empty pane is never ambiguous with a broken one.
- Keep `q`, `Esc`, `/`, `?`, `1`–`9`, `hjkl`+arrows bound to their conventional
  meanings.
- Let the accent mark exactly one thing at a time: where the user is.

**Don't**

- Don't use `%-*s` to pad a string that may contain wide or non-ASCII runes.
  Go's `fmt` width counts runes; the terminal counts cells. This is what
  misaligned the sidebar counts.
- Don't introduce a second accent. If something needs to stand out and isn't
  the current selection, it is a *status*, and it has a token already.
- Don't nest borders. One frame, then dividers.
- Don't signal a state with color alone, or with weight alone.
- Don't hard-code a magic column budget. If you write `- 4`, you have moved a
  layout constant out of the style that owns it.
- Don't put a number in a card that nothing measures. The Health dashboard's
  Reclaimable `—` is a placeholder on purpose; a `0` there would be a lie.
- Don't render a PATH-only command with the same weight as an installed tool.
  They are different strengths of evidence and must look it.
- Don't put a scanner word in front of a user. `available`, `history`, `role`,
  and `observation` are internal vocabulary; the user-facing name and its one
  plain sentence live in `output/kinds.go`.
- Don't organize navigation by which bucket a scanner filled. Lead with what
  each thing *is*; a status is a filter, not a place.
