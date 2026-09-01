# Mole Integration Plan

## Executive Summary

Mole (`tw93/mole`, GPL-3.0) is a macOS cleaner/optimizer installed on this machine as `mo` v1.49.1 via Homebrew. This plan defines how to fold Mole's capabilities into toolsniff **natively** (no GPL code copied) as a read-only **Opportunities** layer — surfacing cleanup candidates with copy-ready / `--dry-run` commands, never auto-mutating.

**GPL-3.0 Compliance**: toolsniff is MIT. Mole is GPL-3.0. This plan re-implements capabilities from OS filesystem facts (path lists, plist keys, extension globs) — not copyrightable expression. Shelling out to the installed `mo` binary is explicitly permitted. **Risk**: do not translate Mole's `lib/*.sh` into Go verbatim.

---

## 1. Mole Feature Inventory (from `mo` v1.49.1 + `libexec/bin/*.sh`)

| Command | What It Does | Paths Touched (from `libexec/`) | Mutating? | Read-Only Surface |
|---------|-------------|----------------------------------|-----------|-------------------|
| `mo clean` | Free disk: caches, logs, leftovers | `~/Library/Caches/*`, `~/Library/Logs/*`, `~/Library/Application Support/*`, `~/Library/Containers/*`, `~/Library/Developer/Xcode/DerivedData`, `~/Library/Developer/CoreSimulator`, `~/.Trash`, browser/dev caches (`.gradle`, `.m2`, `~/.cache/pip`, `~/.cache/go-build`, Homebrew, mise, `~/go/pkg/mod`) | **YES** (with `--dry-run`) | **YES** — `--dry-run` outputs reclaimable bytes per category |
| `mo uninstall` | Remove apps + launch agents, prefs, remnants | `/Applications`, `~/Library/{LaunchAgents,LaunchDaemons}`, `/opt/homebrew/Caskroom`, `launchctl bootout` | **YES** (`--list` is read-only) | **YES** — `--list` shows what would be removed |
| `mo optimize` | Refresh caches & system services (20 tasks) | Spotlight, DNS, QuickLook, SQLite compress, LaunchServices, permissions, periodic scripts | **YES** | NO (maintenance, not inventory) |
| `mo analyze` | Visual disk usage explorer, large files | Read-only walk; `mo analyze /Volumes` | **NO** (read-only) | N/A (exploration paradigm) |
| `mo status` | Live CPU/GPU/mem/disk/network dashboard | sysctl/ps/df | **NO** (read-only) | N/A (monitoring paradigm) |
| `mo history` | Cleanup activity log, `--json` | `~/Library/Logs/mole/*` | **NO** (read-only) | N/A (self-management) |
| `mo purge` | Remove build artifacts (`node_modules`, `target`, `dist`, `.next`, `DerivedData`) | `~/Projects`, `~/GitHub`, `~/dev`, … (configurable via `--paths`) | **YES** (with `--dry-run`) | **YES** — `--dry-run` outputs size per artifact |
| `mo installer` | Find & remove `.dmg/.pkg/.iso/.xip/.zip` | `~/Downloads`, `~/Desktop`, … | **YES** (with `--dry-run`) | **YES** — `--dry-run` lists installers with sizes |
| `mo touchid` / `completion` / `update` / `remove` | Self-management | `/etc/pam.d/sudo`, shell rc, binary | **YES** | N/A (self-management) |

**Proof point**: `mo clean --dry-run` on this machine emitted:
```
→ User app cache · 71 items, 8.45GB dry
→ Chrome cache · 3.08GB dry
→ System log · 12 items, 234MB dry
→ Xcode DerivedData · 4.1GB dry
→ Go module cache · 892MB dry
→ npm cache · 1.2GB dry
```
This exact data model is what toolsniff will rebuild natively.

---

## 2. Mapping Matrix

| Mole Capability | Already in toolsniff? | Native Read-Only Equivalent | Decision |
|-----------------|----------------------|----------------------------|----------|
| App caches/logs (`~/Library/Caches/<bundleid>`, `~/Library/Logs`) | Partial (apps found, caches not) | New `scanner/caches.go` → `KindOpportunity` + copy-ready `mo clean --dry-run` | **BUILD** |
| Orphaned App Support/Containers leftovers (dirs for uninstalled bundle IDs) | No | Diff installed bundle-IDs vs leftover dirs in `~/Library/Application Support`, `~/Library/Containers` | **BUILD** |
| Dev-tool caches (`.gradle`, `.m2`, `~/.cache/pip`, `~/.cache/go-build`, `~/go/pkg/mod`) | No | Size-inventory scanner over configurable roots | **BUILD** |
| Uninstall remnants (launch agents/daemons, Caskroom) | Partial (casks detected) | Extend app actions: "find leftovers" → Opportunities | **EXTEND** |
| Build artifacts (`node_modules`, `target`, `dist`, `.next`, `DerivedData`, `.gradle`) | No | New `scanner/buildartifacts.go`, size-grouped, configurable roots | **BUILD** |
| Installer/pkg files (`.dmg`, `.pkg`, `.iso`, `.xip`, `.zip`) | No | New `scanner/installers.go` (cheap extension glob) | **BUILD** |
| `optimize` (Spotlight/LaunchServices/SQLite/permissions) | No | **SKIP** (maintenance, not inventory) |
| `analyze` (large-file explorer) | No | **SKIP** (exploration, not inventory) |
| `status` (live monitoring) | No | **SKIP** (out of paradigm; `--doctor` is analogue) |
| `history` / `touchid` / `completion` / `update` / `remove` | No | **SKIP** (self-management) |

---

## 3. Gap Analysis

### Core Missing Primitive
toolsniff computes **no byte sizes** (`Observation`/`Location` have no `SizeBytes`) and **doesn't enumerate junk directories**. All gaps require this foundation.

| Gap | New Scanner | Model Changes | UI Surface |
|-----|-------------|---------------|------------|
| **Per-app cache & leftover detection** | `scanner/caches.go` + `scanner/leftovers.go` | Add `KindOpportunity` to `model/source.go`; add `SizeBytes int64` to `model/location.go`; new `OpportunityInfo{Category, RecoverableBytes, Protected, Description}` in `model/observation.go` | New **Opportunities** tab (or Health sub-tab), grouped by category (caches, logs, leftovers) |
| **Build-artifact size discovery** (`node_modules`, `target`, `DerivedData`, `.next`, `dist`, `.gradle`) | `scanner/buildartifacts.go` (lazy, bounded walk with configurable roots via TOML) | Same `KindOpportunity` + `OpportunityInfo{Category="build-artifact"}` | Opportunities tab, sort by reclaimable bytes desc |
| **Installer/pkg finder** | `scanner/installers.go` (extension glob, cheap) | Same `KindOpportunity` + `OpportunityInfo{Category="installer"}` | Opportunities tab |
| **Orphaned launch agents/daemons** | `scanner/launchagents.go` (parse plists for missing binaries) | Emit as **Issue** (diagnostic) + Opportunity (reclaim plist) | Issues view + Opportunities |
| **Large-file finder** | — | — | **SKIP v1** (exploration, not inventory) |

---

## 4. Phased Implementation Plan

### P-A — Model & Vocabulary Foundations
| | |
|---|---|
| **Goal** | Add `KindOpportunity`, `SizeBytes`, `OpportunityInfo`; wire into `kinds.go` + `DESIGN.md` |
| **New files** | `model/opportunity.go` (new), modify `model/observation.go`, `model/location.go`, `model/source.go` |
| **UI surface** | New "Opportunities" tab placeholder in `output/tui_v2.go` |
| **Reuses** | `kinds.go` vocabulary pattern, `DESIGN.md` tokens (amber accent, status colors) |

### P-B — Caches & Leftovers Scanner
| | |
|---|---|
| **Goal** | Detect per-app caches, logs, orphaned `Application Support`/`Containers` dirs |
| **New files** | `scanner/caches.go`, `scanner/leftovers.go` |
| **Model changes** | `OpportunityInfo.Category ∈ {"cache", "log", "leftover"}` |
| **UI surface** | Opportunities tab, grouped by category; detail shows paths + `mo clean --dry-run` command |
| **Reuses** | `KindActions` pattern (non-executing), `output/actions.go` structure |

### P-C — Build Artifacts Scanner
| | |
|---|---|
| **Goal** | Find `node_modules`, `target`, `DerivedData`, `.next`, `dist`, `.gradle` with sizes |
| **New files** | `scanner/buildartifacts.go` (lazy walk, bounded by config `scan.build_artifact_roots`) |
| **Model changes** | `OpportunityInfo.Category = "build-artifact"`, `OpportunityInfo.ToolHint` (npm/cargo/go/next) |
| **UI surface** | Opportunities tab, sortable by size; detail shows `rm -rf` + `mo purge --dry-run` |
| **Reuses** | Config TOML pattern, `kinds.go` |

### P-D — Installer Finder
| | |
|---|---|
| **Goal** | Find `.dmg`, `.pkg`, `.iso`, `.xip`, `.zip` in `~/Downloads`, `~/Desktop` |
| **New files** | `scanner/installers.go` |
| **Model changes** | `OpportunityInfo.Category = "installer"` |
| **UI surface** | Opportunities tab; detail shows `rm` + `mo installer --dry-run` |
| **Reuses** | Extension glob, cheap scan |

### P-E — Orphaned Launch Agents
| | |
|---|---|
| **Goal** | Detect `~/Library/LaunchAgents`, `/Library/LaunchDaemons` plists pointing to missing binaries |
| **New files** | `scanner/launchagents.go` |
| **Model changes** | Emit as **Issue** (`kind=launch-agent-orphaned`) + Opportunity (`category="leftover"`) |
| **UI surface** | Issues view (diagnostic) + Opportunities (reclaim) |
| **Reuses** | Existing Issues pipeline, `KindActions` |

### P-F — Action Hardening & Mole Bridge
| | |
|---|---|
| **Goal** | Extend `KindActions` for `KindOpportunity`; verify all copy-ready commands |
| **New files** | Modify `output/actions.go` (add `OpportunityActions`), `output/tui_detail.go` |
| **UI surface** | Detail view: "What you can do" → copy `mo clean --dry-run`, `rm -rf`, `open -R` |
| **Reuses** | Non-executing `KindActions` pattern, `DESIGN.md` amber for actions |

### P-G — Overview Roll-up
| | |
|---|---|
| **Goal** | "X GB reclaimable" card on Overview dashboard |
| **New files** | Modify `output/tui_overview.go` |
| **UI surface** | Overview dashboard → "Cleanup Opportunities" card with reclaimable total + link to tab |
| **Reuses** | Phase 3 Overview, `DESIGN.md` tokens |

---

## 5. GPL-3.0 Compliance Note

✅ **Compliant**: This plan specifies **native Go re-implementation** of OS filesystem facts (path enumerations, plist parsing, extension globs, size summation). No Mole source code is copied. The algorithms are re-derived independently.

✅ **Shelling out to `mo` is permitted**: Invoking the installed binary (`mo clean --dry-run`, `mo purge --dry-run`) as a subprocess does not incorporate GPL code into toolsniff.

⚠️ **Only real risk**: If any implementation translates Mole's `libexec/lib/*.sh` logic into Go **verbatim** (e.g., exact cache path lists, exact exclusion patterns), that could be considered derivative. Mitigation: use Apple's documented standard paths + configurable user roots, not Mole's hardcoded lists.

---

## 6. File Tree After Full Integration

```
toolsniff/
├── model/
│   ├── observation.go       ← +KindOpportunity, +OpportunityInfo
│   ├── location.go          ← +SizeBytes
│   ├── source.go            ← +KindOpportunity
│   └── opportunity.go       ← NEW: OpportunityInfo, Category enum
├── scanner/
│   ├── caches.go            ← NEW
│   ├── leftovers.go         ← NEW
│   ├── buildartifacts.go    ← NEW
│   ├── installers.go        ← NEW
│   └── launchagents.go      ← NEW
├── output/
│   ├── actions.go           ← +OpportunityActions
│   ├── kinds.go             ← +Opportunity definition
│   ├── tui_v2.go            ← +Opportunities tab
│   ├── tui_overview.go      ← +Reclaimable card
│   └── tui_detail.go        ← +Opportunity detail
└── config/
    └── config.go            ← +build_artifact_roots, cache_scan_roots
```

---

## 7. Acceptance Criteria Per Phase

| Phase | Verify |
|-------|--------|
| P-A | `go build ./...`; `KindOpportunity` appears in `kinds.go`; `SizeBytes` on `Location` |
| P-B | `./toolsniff` → Opportunities tab shows caches/logs/leftovers with sizes; `o` key copies `mo clean --dry-run` |
| P-C | Opportunities tab shows `node_modules` (size), `DerivedData` (size); `y` yanks `rm -rf` |
| P-D | Installers listed with sizes; `o` copies `mo installer --dry-run` |
| P-E | Issues shows orphaned launch agents; Opportunities shows reclaimable plists |
| P-F | All Opportunity actions are copy-ready; no executing commands |
| P-G | Overview shows "X GB reclaimable" card; clicking navigates to Opportunities tab |

---

## 8. Configuration Additions (TOML)

```toml
[scan]
# Roots to search for build artifacts (node_modules, target, DerivedData, etc.)
build_artifact_roots = ["~/Projects", "~/GitHub", "~/dev", "~/work"]
# Additional cache roots beyond standard Library locations
cache_roots = ["~/.cache", "~/Library/Caches"]
# Enable/disable specific opportunity scanners
enable_caches = true
enable_build_artifacts = true
enable_installers = true
enable_launch_agents = true
```

---

*Generated from deep research (read-only `mo` inspection, toolsniff source analysis). Ready for implementation.*