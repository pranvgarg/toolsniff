# Future: Developer Artifacts

Do not add full Purge-style cleanup to the observation model. toolsniff should
finish the inventory model first, then add a separate read-only artifact domain.

Future artifact discovery may cover:

- Xcode DerivedData and archives.
- npm, pnpm, Yarn, Cargo, and Docker caches.
- `node_modules`, Rust `target`, Python virtual environments, and Flutter build
  output.
- Stale developer projects and large files.

Each artifact should include its path, size, modified time, category, evidence,
and an explanation. The first feature should report reclaimable space only:

```bash
toolsniff --artifacts
```

Do not add automatic deletion, scheduling, menu-bar cleanup, or permanent file
removal until a separate safety design exists. Any future cleanup should move
selected items to Trash and remain reversible.
