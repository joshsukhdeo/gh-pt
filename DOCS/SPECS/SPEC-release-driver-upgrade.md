# Specification: Release Driver & Plugin Upgrade Lifecycle (`release-driver-upgrade`)

## 1. Objective

Ensure `gh-pt upgrade` (and batch updates) preserves and executes `--driver`, `--driver-map`, `--plugin`, and `--symlink` configurations originally recorded during `gh-pt install`. The upgrade pipeline must:
- Persist `DriverMap` and `SymlinkDir` into application state alongside `Driver` and `Plugin`.
- Restore all driver and plugin flags into `appParamsCopy` during `processSingleUpdate`.
- Re-deploy driver and plugin manifests to the symlink directory during version upgrades.
- Reliably capture post-install outputs (`InstalledSidecars`, `InstalledBinaries`, `SymlinkDir`) despite `NoSaveState = true`.
- Reconcile sidecars and binaries in `state.Mutate()`, pruning only genuinely obsolete files while preserving active manifests and current binaries.

---

## 2. Tech Stack & Dependencies

- **Language**: Go 1.27
- **CLI Framework**: Kong (`params/params.go`, `cmd/router.go`, `cmd/root.go`, `cmd/update.go`)
- **State & Storage**: File-locked state persistence (`state/state.go`, `state.json.lock`)
- **Observability**: Structured key-value logging via `github.com/charmbracelet/log`
- **Testing**: Standard Go `testing` package with temporary XDG data isolation (`xdg.Reload()`)

---

## 3. Commands

```bash
make build              # Build binary
rtk go test -v ./...    # Run test suite with rtk prefix
make fmt && make lint   # Format and lint code
```

---

## 4. Project Structure & Affected Files

- `state/state.go`: Update `InstalledApp` struct definition to include `DriverMap`.
- `release/release.go`:
  - Populate `SymlinkDir` and `DriverMap` on the `InstalledApp` instance within `GithubRelease.Install()`.
  - Expose installation artifacts (`InstalledSidecars`, `InstalledBinaries`, resolved `SymlinkDir`) on `GithubRelease` or return them from execution so callers running with `NoSaveState = true` can access the newly deployed assets.
- `cmd/update.go`:
  - In `processSingleUpdate`, map `app.Driver`, `app.Plugin`, and `app.DriverMap` into `appParamsCopy`.
  - Ensure `appParamsCopy.Symlink` is forced to `true` whenever `app.Driver != ""` or `app.Plugin != ""`.
  - Replace the flawed post-install state reload (`freshState.Apps[app.Repository]`) with direct capture of the returned sidecars and binaries from `GithubRelease`.
  - In `state.Mutate()` (around line 363), reconcile old sidecars against new ones, delete only obsolete sidecars, and record the complete updated `InstalledApp` (including `Driver`, `Plugin`, `DriverMap`, `SymlinkDir`, `InstalledSidecars`, and `InstalledBinaries`).

---

## 5. Exact Codebase Defects & Remediation

### Defect 1: Missing `DriverMap` in `state.InstalledApp`
- **Location**: `state/state.go:48-49`
- **Problem**: `state.InstalledApp` defines `Driver` and `Plugin`, but omits `DriverMap`. When an app is installed with `--driver-map "bin=driver"`, this mapping is discarded from `state.json`.
- **Remediation**: Add `DriverMap string ` + "`json:\"driver_map,omitempty\"`" to `InstalledApp`.

### Defect 2: `SymlinkDir` and `DriverMap` Omitted in `GithubRelease.Install()`
- **Location**: `release/release.go:2368, 2670-2701`
- **Problem**: `Install()` computes `symlinkDir` locally when `r.CliParams.Symlink` is true (auto-enabled when `Driver` or `Plugin` is present per `release/release.go:1657`), but does not set `app.SymlinkDir` or `app.DriverMap` when constructing `InstalledApp` for `st.AddApp()`.
- **Remediation**: Assign `app.SymlinkDir = symlinkDir` and `app.DriverMap = r.CliParams.DriverMap` before saving state.

### Defect 3: Parameter Loss in `processSingleUpdate`
- **Location**: `cmd/update.go:177-220`
- **Problem**: `processSingleUpdate` copies sidecar settings (`Sidecars`, `SidecarSymlinkTo`, `IncludeSidecars`, `SidecarMode`), but fails to copy:
  - `appParamsCopy.Driver = app.Driver`
  - `appParamsCopy.Plugin = app.Plugin`
  - `appParamsCopy.DriverMap = app.DriverMap`
  - `appParamsCopy.SymlinkDir = app.SymlinkDir`
  - Auto-enabling `appParamsCopy.Symlink = true` if `app.Driver != ""` or `app.Plugin != ""` or `app.SymlinkDir != ""`.
- **Remediation**: Propagate all driver and plugin fields to `appParamsCopy`.

### Defect 4: Broken Sidecar Reconciliation with `NoSaveState = true`
- **Location**: `cmd/update.go:215, 236-248, 363-380` & `release/release.go:2667`
- **Problem**: `processSingleUpdate` sets `appParamsCopy.NoSaveState = true` so `Install()` does not overwrite the global state during execution. However, `cmd/update.go:236-248` reloads `state.LoadState()` expecting `freshState.Apps[app.Repository]` to reflect the newly installed sidecars and binaries. Because `NoSaveState` was active, `freshState.Apps[app.Repository]` still contains the pre-upgrade state. As a result, newly deployed driver manifests are ignored, and obsolete sidecar calculation either prunes the wrong files or fails completely.
- **Remediation**:
  1. Have `GithubRelease.Install()` record or return the list of installed sidecars, binaries, and symlink destination on the struct or via return values.
  2. In `cmd/update.go`, retrieve `InstalledSidecars` and `InstalledBinaries` directly from the release execution result.
  3. In `state.Mutate()`, calculate obsolete sidecars:
     $$\text{Obsolete} = \text{OldSidecars} \setminus \text{NewSidecars}$$
     Prune only files in $\text{Obsolete}$. Preserve active sidecars and newly written driver/plugin manifests.
  4. Write back `Driver`, `Plugin`, `DriverMap`, `SymlinkDir`, `InstalledSidecars`, and `InstalledBinaries` inside `state.Mutate()`.

---

## 6. Code Style & Observability

- **Terse and Idiomatic Go**: Adhere to project Go guidelines.
- **Structured Logging (`golang-observability`)**:
  - Use `charmbracelet/log` for operational observability.
  - Log manifest deployment: `log.Debug("deployed driver manifest", "driver", app.Driver, "target", manifestPath)`.
  - Log sidecar pruning: `log.Debug("pruning obsolete sidecar", "path", path)`.
  - Adhere to the single-handling rule: Log errors at the top-level handler or return them; never log and return the same error.

---

## 7. Testing Strategy

### Seam S1: Release Install & State Verification
- **Target Files**: `release/driver_test.go` or `release/release_test.go`
- **Test Flow**:
  1. Configure temporary test isolation with `t.TempDir()`, `XDG_DATA_HOME`, and `xdg.Reload()`.
  2. Initialize `GithubRelease` with `--driver "vulkan"`, `--driver-map ".*icd.*\\.json$:vulkan"`, and `--symlink`.
  3. Execute `Install()`.
  4. Call `state.LoadState()`.
  5. **Assertions**:
     - `app.Driver == "vulkan"`
     - `app.DriverMap == ".*icd.*\\.json$:vulkan"`
     - `app.SymlinkDir` is non-empty and points to the configured package symlink directory.
     - `app.InstalledSidecars` contains the registered driver manifest.

### Seam S2: Upgrade Parameter Propagation & Sidecar Reconciliation
- **Target Files**: `cmd/update_test.go`
- **Test Flow**:
  1. Configure test environment with an existing `InstalledApp` containing a v1 binary, an active driver manifest in `InstalledSidecars`, and `DriverMap` set.
  2. Mock/invoke `processSingleUpdate` / `DoUpdate` targeting v2.
  3. **Assertions**:
     - `appParamsCopy` inherits `Driver`, `Plugin`, `DriverMap`, and `Symlink = true`.
     - Manifests for v2 are deployed to `SymlinkDir`.
     - Active driver manifest is preserved in `InstalledSidecars`.
     - Genuinely obsolete sidecars (present in v1 but absent in v2) are pruned from disk.
     - `state.LoadState()` verifies v2 record has updated version, intact driver settings, and reconciled `InstalledSidecars`.

---

## 8. Boundaries & Invariants

- **State File Locking**: Any mutation of `state.json` must be done through `state.Mutate()` or `st.Save()` with an acquired `state.json.lock`.
- **Zero Token Leakage**: GitHub tokens must not be exposed in CLI arguments or logs.
- **Terminal Cleanliness**: No long-running spinner or animation left hanging.
- **Backward Compatibility**: If an existing application in `state.json` lacks `DriverMap` or `SymlinkDir`, upgrade must handle empty defaults gracefully without panics.

---

## 9. Success Criteria

- [ ] `state.InstalledApp` has `DriverMap` field with JSON tag `driver_map,omitempty`.
- [ ] `GithubRelease.Install()` sets `SymlinkDir` and `DriverMap` in state.
- [ ] `processSingleUpdate` copies `Driver`, `Plugin`, `DriverMap`, and sets `Symlink` to true when drivers are active.
- [ ] Post-upgrade state mutation accurately records new sidecars and binaries without depending on `freshState` when `NoSaveState` is used.
- [ ] Obsolete sidecars are cleanly pruned while active driver manifests remain intact.
- [ ] All tests in `rtk go test -v ./...` pass with 0 regressions.

---

## 10. Open Questions & Assumptions

- *Assumption*: Driver and plugin manifest files are cataloged in `InstalledSidecars` so sidecar reconciliation naturally covers them.
- *Assumption*: If `SymlinkDir` is missing in older state entries, `resolvePackageDir` in `release/symlink.go` resolves the canonical package directory (`$XDG_DATA_HOME/gh-pt/packages/<owner>/<repo>`).
