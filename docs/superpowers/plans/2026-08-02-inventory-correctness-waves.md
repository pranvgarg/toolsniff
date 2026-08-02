# Inventory Correctness Wave Plan

## Scope

Implement Wave 1 of `Next implementation markdown file/next-implementation-roadmap.md`:

- Keep installed observations separate from PATH availability observations.
- Preserve the existing installed registry format.
- Persist PATH availability in a sibling registry file.
- Detect version upgrades and downgrades.
- Update CLI, JSON, table, TUI, documentation, and tests.

The selected product decisions are:

- Installed observations remain in the configured registry path, normally
  `~/.toolsniff/registry.json`.
- PATH observations use a sibling file, normally
  `~/.toolsniff/availability.json`.
- Existing registry files remain readable without migration.
- `--diff` reports installed changes only.
- `--diff --available` adds PATH availability changes.
- JSON and the TUI continue to expose current availability separately from
  installed observations.

## Dependency Summary

```text
Wave 1
  Diff model and algorithm
  Availability registry persistence
  Role-based observation partitioning
       |
       v
Wave 2
  Report and renderer support
       |
       v
Wave 3
  CLI integration and documentation
```

## Wave 1: Inventory Foundations

**Gate:** Run `gofmt -w` on changed Go files, then run `go test ./...`,
`go test -race ./...`, and `go vet ./...`. All commands must pass before Wave
2 begins.

### Task 1: Version-Aware Registry Diff

**Files:**

- `registry/diff.go`
- `registry/diff_test.go`

**Interfaces:**

- **Consumes:** `model.ToolIdentity` and the existing `registry.ComputeDiff`
  inputs.
- **Produces:** `registry.ToolChange`, `registry.Diff.Updated`, and
  version-aware `registry.ComputeDiff` behavior for all downstream consumers.

**Steps:**

1. Add `ToolChange` with `Before` and `After` `model.Tool` values.
2. Add `Updated []ToolChange` to `Diff`.
3. Match old and current observations by the existing source-plus-identity
   key.
4. Report a matching observation as updated when its version changes from one
   known value to another or from empty to known.
5. Ignore known-to-empty version changes to avoid noisy reports.
6. Keep path changes as removal plus addition because path-aware identity is
   intentionally preserved.
7. Sort updated results deterministically by source and tool name, with stable
   before/after values where needed.
8. Add tests for upgrades, downgrades, empty-to-known changes, known-to-empty
   changes, unchanged versions, path changes, and source separation.

### Task 2: Availability Registry Persistence

**Files:**

- `registry/registry.go`
- `registry/registry_test.go`

**Interfaces:**

- **Consumes:** The existing `Save(path, tools)` and `Load(path)` JSON-array
  persistence behavior and the configured installed registry path.
- **Produces:** A deterministic sibling availability-path helper and separate
  load/save calls for PATH observations without changing the installed
  registry format.

**Steps:**

1. Add a helper that derives `availability.json` in the same directory as the
   configured installed registry path.
2. Preserve the existing path's directory and support custom registry paths.
3. Reuse the existing atomic, permission-safe JSON persistence behavior for
   availability observations.
4. Keep missing availability files warning-free on first use.
5. Add tests for the default-shaped path, custom paths, round trips, missing
   files, corrupt files, and independent installed/availability contents.

### Task 3: Role-Based Observation Partitioning

**Files:**

- `main.go`
- `main_test.go`

**Interfaces:**

- **Consumes:** `scanner.Registration`, `scanner.SourceInfo.Role`, and the
  existing installed, history, and available role constants.
- **Produces:** Separate installed, available, and history slices for registry,
  reporting, and TUI orchestration.

**Steps:**

1. Replace the current history-only split with a role-aware partition.
2. Put only `RoleInstalled` observations in the installed slice.
3. Put `RoleAvailable` observations in the availability slice.
4. Put informational and `RoleHistory` observations in the history slice.
5. Preserve input order in the partition helper; keep global sorting where the
   scan pipeline currently performs it.
6. Add tests proving PATH observations never enter the installed slice and npx
   history never enters either persisted baseline.

## Wave 2: Reports and Interactive Views

**Depends on:** Wave 1

**Gate:** Run `go test ./...`, `go test -race ./...`, and `go vet ./...`. Tests
must cover installed, available, history, added, removed, and updated data in
the machine-readable and human-readable outputs.

### Task 4: Shared Report and Renderer Semantics

**Files:**

- `output/json.go`
- `output/json_test.go`
- `output/table.go`
- `output/table_test.go`
- `output/tui_model.go`
- `output/tui_frame.go`
- Relevant TUI tests under `output/`

**Interfaces:**

- **Consumes:** The three role-separated observation slices from Task 3 and
  `registry.Diff`, including `Updated` from Task 1.
- **Produces:** Renderer APIs and output shapes consumed by CLI integration in
  Task 5.

**Steps:**

1. Extend the JSON report with a separate current `available` collection and
   an `updated` collection containing before/after tool values.
2. Add explicit availability diff fields to the JSON report so
   `--diff --available` is machine-readable without mixing it into installed
   changes.
3. Keep npx history separate and informational.
4. Update table output to render installed additions, removals, and updates in
   distinct sections, including `before -> after` versions.
5. Add a separate availability-change section that is rendered only when the
   availability diff is requested.
6. Ensure installed counts exclude PATH observations while current available
   counts remain visible.
7. Update the TUI model to receive availability separately and keep PATH as a
   truthful source tab.
8. Include updated installed observations in the TUI change tab without
   treating PATH availability as installed inventory.
9. Keep current TUI save behavior limited to installed tools until CLI
   orchestration supplies the separate availability baseline data.
10. Add renderer tests for updated versions, separate availability, empty
    sections, and compatibility with existing report data.

## Wave 3: CLI Integration and User Documentation

**Depends on:** Wave 2

**Gate:** Run `go test ./...`, `go test -race ./...`, `go vet ./...`, and
`go build ./...`. Perform manual command checks for `--list`, `--json`,
`--save`, `--diff`, `--diff --available`, and the TUI. Confirm existing
registry files remain readable and PATH observations are absent from the
installed baseline.

### Task 5: CLI Baseline and Diff Orchestration

**Files:**

- `main.go`
- `main_test.go`

**Interfaces:**

- **Consumes:** Role partitioning from Task 3, availability path persistence
  from Task 2, updated diff behavior from Task 1, and renderer APIs from Task
  4.
- **Produces:** End-to-end command behavior for installed and availability
  baselines.

**Steps:**

1. Add an `--available` flag with help text explaining that it augments
   `--diff` with PATH availability changes.
2. Reject `--available` unless `--diff` is also selected.
3. Load the installed baseline from the configured registry path.
4. Load the availability baseline from the derived sibling path.
5. Compute installed and availability diffs independently.
6. Make `--save` save installed observations to `registry.json` and PATH
   observations to `availability.json`.
7. Keep `--diff` focused on installed changes unless `--available` is present.
8. Pass installed, available, and history collections separately to JSON,
   table, and TUI renderers.
9. Preserve warning behavior for corrupt or unreadable baselines, identifying
   whether the installed or availability registry caused the warning.
10. Update main tests for flag validation, separate save inputs, independent
    diff inputs, and version updates.

### Task 6: Documentation and Configuration Semantics

**Files:**

- `README.md`
- `docs/configuration.md`
- `Next implementation markdown file/next-implementation-roadmap.md` only if
  implementation status needs recording

**Interfaces:**

- **Consumes:** The final CLI and renderer behavior from Task 5.
- **Produces:** User-facing documentation for installation, source roles,
  baseline files, version changes, and availability diffs.

**Steps:**

1. Document that PATH is availability, not proof of installation.
2. Document that `--save` maintains separate installed and availability
   baselines.
3. Document `--diff` and `--diff --available` separately.
4. Document updated version output and the JSON `updated` shape.
5. Explain the sibling availability registry path and custom registry-path
   behavior.
6. Keep Homebrew installation guidance consistent with the existing custom-tap
   instructions.

## Dependency Self-Review

- Task 1 consumes only existing model identity behavior, so it is Wave 1.
- Task 2 consumes only existing registry persistence, so it is Wave 1.
- Task 3 consumes only existing scanner roles and registrations, so it is Wave
  1.
- Task 4 consumes outputs from Tasks 1 and 3, so it is Wave 2.
- Task 5 consumes outputs from Tasks 1, 2, 3, and 4, so it is Wave 3.
- Task 6 documents the final behavior produced by Task 5, so it is Wave 3 and
  does not create an implementation dependency for earlier tasks.

No task consumes an interface produced in the same wave or a later wave.
