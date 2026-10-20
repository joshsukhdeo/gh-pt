# Implementation Plan: Driver, Plugin, and Source Lifecycle Parity

## 1. Major Components & Dependencies

1. **Module 1: `release-driver-upgrade`** ([`DOCS/SPECS/SPEC-release-driver-upgrade.md`](../DOCS/SPECS/SPEC-release-driver-upgrade.md))
   - **`state/state.go`**: Add `DriverMap string` (`json:"driver_map,omitempty"`) to `InstalledApp`.
   - **`release/release.go`**:
     - Record `SymlinkDir` on `GithubRelease` when `executeSymlinkInstall` runs (`r.SymlinkDir = symlinkDir`).
     - Populate `SymlinkDir: r.SymlinkDir` and `DriverMap: r.CliParams.DriverMap` when constructing `state.InstalledApp` in `Install()`.
     - Export `InstalledSidecars`, `InstalledBinaries`, and `SymlinkDir` on `GithubRelease` (or copy back to `r.CliParams`) so `cmd/update.go` can read them directly when `NoSaveState = true`.
   - **`cmd/update.go`**:
     - In `processSingleUpdate`, copy `app.Driver`, `app.Plugin`, `app.DriverMap` into `appParamsCopy`, and set `appParamsCopy.Symlink = true` when `app.SymlinkDir != ""` or `(app.Driver != "" && app.Driver != "none")` or `(app.Plugin != "" && app.Plugin != "none")`.
     - Capture newly installed sidecars, binaries, and `SymlinkDir` directly from `installReleaseFunc` / `appParamsCopy` (while still supporting `freshState` fallback if tests mock `installReleaseFunc` by mutating `state.json`).
     - Persist `Driver`, `Plugin`, `DriverMap`, `SymlinkDir`, `InstalledSidecars`, and `InstalledBinaries` inside `state.Mutate()`.

2. **Module 2: `source-driver-lifecycle`** ([`DOCS/SPECS/SPEC-source-driver-lifecycle.md`](../DOCS/SPECS/SPEC-source-driver-lifecycle.md)) — *Depends on Module 1*
   - **`cmd/root.go`**:
     - In `handleCompileFromSource`, deploy driver and plugin manifests on `symlinkDir` (or `targetPath` when `symlinkDir == ""`) via `release.GithubRelease{CliParams: &r.ExecContext}.DeployDriverAndPluginManifests(...)` and append deployed manifest paths to `r.InstalledSidecars`.
     - Persist `Driver`, `Plugin`, `DriverMap`, `IncludeSidecars`, and `SidecarMode` in `st.AddApp(&state.InstalledApp{...})`.
   - **`compile/compile.go`**:
     - Add `Driver`, `Plugin`, `DriverMap`, `IncludeSidecars`, and `SidecarMode` fields to `compile.Config` and persist them in `SaveCompileState`.
   - **`cmd/update.go`**:
     - In `DoUpdate` for `app.CompileScript != ""`:
       - Sync local git clone in `~/builds/<repo>` (`git fetch` + `git reset --hard <remoteCommit>` or `git pull`) before running the compile script when a local git repo exists.
       - Use container sandboxing (`compile.ExecuteInContainer`) when container execution is enabled and a container runtime is available, falling back cleanly when disabled or in host test harnesses.
       - Refresh symlinks (`createSymlinks`) when `app.SymlinkDir != ""`, deploy driver/plugin manifests when `app.Driver` or `app.Plugin` or `app.DriverMap` is set, reconcile obsolete sidecars, and persist `InstalledSidecars` to state.

## 2. Implementation Order & Parallelism
- **Sequential**: Module 1 (`release-driver-upgrade`) touches `state/state.go`, `release/release.go`, and `cmd/update.go`. Module 2 (`source-driver-lifecycle`) builds on `state/state.go` (`DriverMap`) and also edits `cmd/update.go` (`DoUpdate`). Therefore `agent-002` runs first, verified by `rtk go test ./state/... ./release/... ./cmd/...`, followed immediately by `agent-003`.

## 3. Risks & Mitigations
- **Risk**: Existing `cmd/update_test.go` tests mock `installReleaseFunc` by either mutating `state.json` directly or returning `nil` without running `GithubRelease.Install()`.
  - **Mitigation**: Have `installReleaseFunc` populate results on `appParams` (`ExecContext`) when `GithubRelease.Install()` runs, and preserve the `freshState` check in `processSingleUpdate` so existing mock tests continue to pass without modification.
- **Risk**: Unit tests for source builds (`cmd/update_test.go`) may use temporary shell scripts without a real git clone or Docker daemon.
  - **Mitigation**: Guard git fetch/pull on `~/builds/<repo>/.git` existing, and allow injectable/configurable compile execution or host fallback when `NoCompileContainer` / test hooks are active.

## 4. Verification Checkpoints
1. **Checkpoint 1 (after `agent-002`)**: `rtk go test -v ./state/... ./release/... ./cmd/...` passes Seam S1 and Seam S2 tests.
2. **Checkpoint 2 (after `agent-003`)**: `rtk go test -v ./... && make fmt && make lint` passes Seam S3 and full suite with zero regressions.
