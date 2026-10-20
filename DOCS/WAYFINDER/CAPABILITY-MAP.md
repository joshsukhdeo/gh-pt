# Capability Map: Driver, Plugin, and Source Lifecycle Parity

## Destination

`gh-pt upgrade` and `gh-pt source` must treat `--driver`, `--driver-map`, `--plugin`, and `--symlink` identically to `gh-pt install`. This guarantees:
- Full state preservation of driver, plugin, driver mapping, and symlink directories across application upgrades.
- Deterministic redeployment of driver and plugin manifests alongside updated binary targets.
- Accurate reconciliation of `InstalledSidecars` and `InstalledBinaries`, pruning only genuinely obsolete sidecars while keeping active manifests and binaries intact.
- Seamless source-build lifecycle management, including git checkout synchronisation, container sandboxing parity, symlink refreshing, and driver/plugin manifest deployment.

---

## Architectural Scope & Invariants

All changes adhere strictly to core repository invariants:
1. **GitHub CLI Required**: `gh` CLI presence & authentication required for remote discovery and API queries.
2. **State Locking**: Always use `state.LoadState()` / `st.Save()` or `state.Mutate()`; never bypass `state.json.lock`.
3. **Test Isolation**: In all unit and integration tests, set `XDG_DATA_HOME` via `t.TempDir()` and immediately invoke `xdg.Reload()`.
4. **Terminal Hygiene**: Never run background progress animations while waiting on interactive stdin.
5. **Security**: Disallow GitHub tokens in CLI arguments; read strictly from environment variables or secure configuration.

---

## Module Capability Table

| Module | Specification | Dependencies | Description |
|---|---|---|---|
| `release-driver-upgrade` | [`DOCS/SPECS/SPEC-release-driver-upgrade.md`](../SPECS/SPEC-release-driver-upgrade.md) | — | Persist `Driver`, `Plugin`, `DriverMap`, `SymlinkDir` in `state/state.go` & `release/release.go`; restore them in `processSingleUpdate` (`cmd/update.go`); fix `NoSaveState` sidecar/binary reconciliation so upgrades re-deploy manifests and prune only removed ones. |
| `source-driver-lifecycle` | [`DOCS/SPECS/SPEC-source-driver-lifecycle.md`](../SPECS/SPEC-source-driver-lifecycle.md) | `release-driver-upgrade` | Wire driver/plugin manifest deployment and state persistence into source builds (`cmd/root.go`, `compile/compile.go`); fix `CompileScript` branch in `DoUpdate` (`cmd/update.go`) to update git checkout, use container sandbox when enabled, refresh symlinks, and deploy driver/plugin manifests. |

---

## Implementation & Build Order

```
[Phase 1: SPECIFY] (Current)
   ├─ DOCS/WAYFINDER/CAPABILITY-MAP.md
   ├─ DOCS/SPECS/SPEC-release-driver-upgrade.md
   └─ DOCS/SPECS/SPEC-source-driver-lifecycle.md
        ↓ (Requires Human Approval)
[Phase 2: PLAN & TASKS]
        ↓
[Phase 3: IMPLEMENTATION ORDER]
   1. release-driver-upgrade (Foundation: state schema, release install output, upgrade param forwarding, manifest reconciliation)
        ↓
   2. source-driver-lifecycle (Dependent: source build manifest deployment, containerized upgrade execution, git synchronization)
```

---

## Impacted Files & Components

- `state/state.go`: Add `DriverMap` field to `InstalledApp`.
- `release/release.go`: Store `SymlinkDir` and `DriverMap` during `Install()`; export execution outputs (`InstalledSidecars`, `InstalledBinaries`, `SymlinkDir`) when `NoSaveState` is enabled.
- `cmd/update.go`: Propagate `Driver`, `Plugin`, `DriverMap`, `Symlink` in `processSingleUpdate`; capture post-install sidecar/binary records without reloading stale state; reconcile and mutate state safely; modernize `CompileScript` pipeline.
- `cmd/root.go`: Deploy manifests and persist driver/plugin parameters during `handleCompileFromSource`.
- `compile/compile.go`: Persist driver/plugin parameters in `SaveCompileState` and execute manifest deployment in target symlink directory.
