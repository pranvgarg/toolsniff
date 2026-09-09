# Tool Sniff UI Enhancement Plan

## Executive Summary

Post-P3, toolsniff has **8 tabs**: Overview · CLI tools · Packages · Applications · PATH executables · npx history · Changes · Issues. This plan reduces cognitive load by collapsing to **4 intent-based tabs**, making the Overview actionable, unifying "installed" views, and surfacing actions inline — all grounded in the current Charmbracelet v2 + DESIGN.md stack.

---

## 1. Current State (Grounded in Code)

| File | Role |
|------|------|
| `output/tui_v2.go:56-64` | `reportTabs = []string{"Overview", "CLI tools", "Packages", "Applications", "PATH executables", "npx history", "Changes", "Issues"}` |
| `output/kinds.go:36-108` | `KindDefinition` map — plain English for every kind/origin |
| `output/actions.go` | `KindActions` per kind (Install/Update/Uninstall/Open/Reveal) |
| `output/tui_overview.go` | Dashboard: grouped counts by kind |
| `output/tui_inventory_table.go` | `lipgloss.Table` with semantic row styling (status colors) |
| `output/tui_panes.go` / `tui_detail.go` | Detail pane: "What this is" + "What you can do" |
| `DESIGN.md` | WCAG-AA tokens: `ColorPrimary` (amber), `ColorSuccess/Warning/Error`, `ColorMuted`, selection bg/fg |

**Current flow**: User lands on Overview (passive counts) → must know which tab → Enter for detail → `o`/`y`/`p` for actions.

---

## 2. Pain Points (Validated)

1. **8 tabs = high cognitive load** — user must map mental model ("I want to update") to internal Kind
2. **Overview is passive** — shows "147 installed, 23 available" but not "7 updates available"
3. **CLI tools vs Packages split is artificial** — both are "things I installed"; split by `Kind=cli` vs `Kind=package` (internal)
4. **PATH executables = dumping ground** — 1000+ items, no grouping, no "what do I do?"
5. **Changes + Issues = orphaned** — not part of "manage tools" flow
6. **Actions hidden behind Enter** — no inline preview, no bulk, no "update all"
7. **No "needs attention" signal** — must hunt across tabs

---

## 3. Proposed Navigation: 4 Intent-Based Tabs

| New Tab | Contains | Rationale |
|---------|----------|-----------|
| **Manage** | Unified "Installed" view: CLI tools + Packages + Applications, grouped by **manager** (npm, Homebrew, Cargo, pipx, Bun, Applications) with kind chips (`cli`/`pkg`/`app`) | Single place for "what I installed & can act on" |
| **Discover** | PATH executables (grouped by directory) + npx history | "What I found on this machine" — not installed by a manager |
| **Review** | Changes (diff since baseline) + Issues (broken/shadowed/unknown) + New since last scan | "What changed / what's wrong" — single review surface |
| **Health** | Overview dashboard (actionable) + Mole Opportunities (reclaimable space) | "What needs attention" — updates, broken, reclaimable |

**Migration**: Old tabs → New tabs
- Overview → Health (transformed)
- CLI tools + Packages + Applications → Manage
- PATH executables + npx history → Discover
- Changes + Issues → Review

---

## 4. Tab Specifications

### 4.1 Manage Tab (Primary)

**Structure**: Single list, grouped by manager → kind chip → item

```
▼ npm (12)
  ● cli  opencode-ai        1.18.11  update→1.19.0
  ● pkg  @anthropic/claude  0.8.2    -
▼ Homebrew (47)
  ● cli  gh                 2.65.0   -
  ● cask  visual-studio-code  1.92.0  update→1.93.0
  ● fml  docker             27.1.0   -
▼ Cargo (8)
  ● cli  ripgrep            14.1.0   -
▼ Applications (31)
  ● app  Chrome                    128.0.0
  ● app  VS Code                   1.92.0
```

**Inline actions** (visible without Enter):
- `u` / `U` → Update selected / Update ALL updatable
- `x` → Uninstall selected
- `o` → Open (app) / Reveal (pkg)
- `Enter` → Deep detail (current behavior)

**Grouping logic** (in `output/tui_inventory_table.go`):
```go
// Sort key: ManagerPriority → KindPriority → Name
// ManagerPriority: npm=1, homebrew-formula=2, homebrew-cask=3, cargo=4, pipx=5, bun=6, applications=7, manual=99
```

### 4.2 Discover Tab

**Structure**: Grouped by source directory

```
▼ /opt/homebrew/bin (234)
  ● gh          → Homebrew formula (install: brew install gh)
  ● node        → Unknown (install: brew install node)
▼ ~/.local/bin (12)
  ● my-script   → Manual (no manager)
▼ /usr/local/bin (8)
  ● docker      → Homebrew cask (install: brew install --cask docker)
▼ npx history (7)
  ● create-react-app  2024-01-15  (reveal: open -R ~/.npm/_npx/...)
```

**Smart suggestions**: For each PATH executable with no manager, run heuristic match against known Homebrew formulae / npm packages → show "Install via: `brew install X`" inline.

### 4.3 Review Tab

**Structure**: Three sections, collapsible

```
▼ Changes since 2024-08-28 (5)
  + added   npm: opencode-ai@1.18.11
  ~ updated brew: gh 2.64.0 → 2.65.0
  - removed cargo: old-tool@1.0.0
  ! relocated app: Chrome moved to ~/Applications
  ! shadowed  node: /opt/homebrew/bin/node shadows ~/.nvm/node

▼ Issues (3)
  ⚠ broken      pipx: black (executable missing)
  ⚠ shadowed    npm: prettier (shadowed by local ./node_modules/.bin/prettier)
  ⚠ unknown-ver manual: my-script (no version probe)

▼ New since last scan (2)
  + bun: bunx@1.1.0
  + path: new-binary (manual)
```

**Actions**: `u` on updated → update; `x` on broken → uninstall; `Enter` → detail.

### 4.4 Health Tab (Transformed Overview)

**Current**: Passive counts per kind
**New**: Actionable cards

```
┌─────────────────────────────────────────────────────────────┐
│  📊  Tool Sniff Health                    [t] theme  [?] help │
├─────────────────────────────────────────────────────────────┤
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐          │
│  │  Updates     │ │  Issues      │ │  Reclaimable │          │
│  │  7 available │ │  3 need fix  │ │  12.4 GB     │ ← Mole P-G│
│  │  [Manage]    │ │  [Review]    │ │  [Cleanup]   │          │
│  └──────────────┘ └──────────────┘ └──────────────┘          │
│                                                               │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐          │
│  │  Installed   │ │  Discovered  │ │  Last Scan   │          │
│  │  234 items   │ │  1,847 bins  │ │  2h ago      │          │
│  └──────────────┘ └──────────────┘ └──────────────┘          │
│                                                               │
│  Quick actions:  [u] Update all  [s] Scan now  [c] Config    │
└─────────────────────────────────────────────────────────────┘
```

**Cards are clickable** (Enter/`o`) → navigate to relevant tab with filter pre-applied.

---

## 5. Keyboard Efficiency (New Bindings)

| Key | Context | Action |
|-----|---------|--------|
| `u` | Manage/Review | Update selected |
| `U` | Manage | Update ALL updatable (with confirmation) |
| `x` | Manage/Review | Uninstall selected |
| `X` | Manage | Uninstall ALL selected (multi-select) |
| `o` | Any row | Open / Reveal in Finder |
| `y` | Any row | Yank primary command (current) |
| `p` | Any row | Copy path (current) |
| `m` | Any | Toggle multi-select mode |
| `a` | Multi-select | Select all in group |
| `?` | Any | Help overlay (shows contextual bindings) |
| `1`–`4` | Any | Jump to tab (Manage/Discover/Review/Health) |

**Multi-select**: `m` enters mode, `↑/↓` + `space` selects, `U`/`X`/`o` acts on all selected.

---

## 6. Visual Hierarchy (Using DESIGN.md Tokens)

| Element | Token | Example |
|---------|-------|---------|
| Manager group header | `ColorPrimary` (amber) + bold | `▼ Homebrew (47)` |
| Kind chip | `ColorMuted` bg + `ColorForeground` fg, rounded | `[cli]` `[pkg]` `[app]` |
| Status badge (updatable) | `ColorWarning` bg + `ColorBackground` fg | `↑ 1.19.0` |
| Status badge (broken) | `ColorError` bg + `ColorBackground` fg | `✗ broken` |
| Status badge (ok) | `ColorSuccess` bg + `ColorBackground` fg | `✓ current` |
| Reclaimable size | `ColorPrimary` + right-aligned | `12.4 GB` |
| Selection | `SelectionBackground` / `SelectionForeground` | (current) |
| Focus ring | `ColorPrimary` 2px | (accessibility) |

**Table columns** (Manage tab):
```
[Kind]  Name                    Version        Status          Manager        Size
[cli]   gh                      2.65.0         ↑ 2.66.0        Homebrew       45 MB
[pkg]   @types/node             20.12.0        ✓ current       npm            -
[app]   Visual Studio Code      1.92.0         ✓ current       Applications   1.2 GB
```

---

## 7. Mole Opportunities Integration

| Option | Recommendation |
|--------|----------------|
| **New "Cleanup" tab** | Adds 5th tab → back to cognitive load problem |
| **Inside Health tab** | ✅ **Preferred** — Health already shows "Reclaimable" card; clicking navigates to Opportunities section within Health (or sub-view) |
| **Inside Manage tab** | Confuses "installed tools" with "cleanup candidates" |

**Implementation**: Health tab has two sub-views (toggle with `Tab` or `Ctrl+h`):
1. Dashboard (cards) — default
2. Opportunities list — when Mole P-G lands

---

## 8. File Changes Required

| File | Change |
|------|--------|
| `output/tui_v2.go` | Replace `reportTabs` with 4 tabs; update tab-switch logic |
| `output/kinds.go` | Add `KindManageGroup` helper (manager → priority → label) |
| `output/tui_inventory_table.go` | New `ManageTableModel` with manager grouping, inline action columns, multi-select |
| `output/tui_overview.go` | Transform to `HealthDashboard` with actionable cards + click navigation |
| `output/tui_panes.go` / `tui_detail.go` | Keep for deep detail; add inline action row in table |
| `output/actions.go` | Add `BulkActions` (UpdateAll, UninstallAll) |
| `output/tui_model.go` | Add `MultiSelectMode`, `HealthSubView` state |
| `scanner/path.go` | Add heuristic manager suggestion for PATH executables |
| `DESIGN.md` | Document new tokens: kind chip, status badge, card styles |

---

## 9. Migration Path (Non-Breaking)

1. **Phase 1**: Add new tab logic behind feature flag (`ui.mode = "intent"` in config)
2. **Phase 2**: Default to intent; keep old tabs accessible via `--legacy-tabs`
3. **Phase 3**: Remove legacy code

**Config addition**:
```toml
[ui]
mode = "intent"  # "legacy" = current 8-tab, "intent" = 4-tab intent-based
```

---

## 10. Accessibility

- **High-contrast mode**: Already in themes (`mono`, `high-contrast` presets) — ensure new chips/badges respect it
- **Focus visible**: 2px `ColorPrimary` ring on selected row (already in `lipgloss.Table` selection)
- **Screen reader**: Add `aria-label` equivalents in detail pane text ("Update available for gh, version 2.66.0")
- **Keyboard-only**: All actions reachable via keys; no mouse required

---

## 11. Acceptance Criteria

| Criteria | Verify |
|----------|--------|
| Tab count | `./toolsniff` shows exactly 4 tabs: Manage, Discover, Review, Health |
| Manage grouping | Items grouped by manager; kind chip visible; inline status badge |
| Inline actions | `u` updates selected; `U` updates all (confirm); `x` uninstalls |
| Health cards | Updates/Issues/Reclaimable cards clickable → navigate with filter |
| Discover suggestions | PATH items show "Install via: brew install X" where heuristic matches |
| Review unification | Changes + Issues + New since scan in one tab, collapsible sections |
| Multi-select | `m` → space → `U`/`X` works |
| Build/tests | `go build ./...` OK; `go test ./...` 13+ packages OK |
| Theme compatibility | All new elements render correctly in `midnight`, `nord`, `mono`, `high-contrast` |

---

## 12. Future Extensions (Post-v3)

- **Bulk export**: `y` on multi-select → JSON/CSV of selected items
- **Scheduled scan**: Background scan + notification on Health tab
- **Profiles**: Work / Personal / CI machine profiles with different scanner roots
- **Plugin actions**: User-defined `KindActions` via config (e.g., "run `mise install`")

---

*Ready for implementation. Grounded in current `output/` codebase, DESIGN.md tokens, and Mole integration plan.*