# Task Breakdown: Driver, Plugin, and Source Lifecycle Parity

## Module 1: `release-driver-upgrade` (`agent-002`)

- [x] Task 1.1: Persist `DriverMap` and `SymlinkDir` on release install (TDD Seam S1)
  - Acceptance: `state.InstalledApp` includes `DriverMap` (`json:"driver_map,omitempty"`), and `(*release.GithubRelease).Install()` persists `Driver`, `Plugin`, `DriverMap`, `SymlinkDir`, and `InstalledSidecars` into `state.json` and records output fields even when `NoSaveState = true`.
  - Verify: `rtk go test -v ./state/... ./release/... -run TestDeployDriverAndPluginManifests\|TestInstallSavesDriverAndSymlinkState`
  - Files: `state/state.go`, `params/params.go`, `release/release.go`, `release/driver_test.go`

- [x] Task 1.2: Restore driver/plugin params and reconcile sidecars on `gh-pt upgrade` (TDD Seam S2)
  - Acceptance: `processSingleUpdate` copies `Driver`, `Plugin`, `DriverMap`, and enables `Symlink` when driver/plugin/symlinkDir is set; captures updated sidecars/binaries from `Install()` even when `NoSaveState = true`; preserves active manifests and prunes only obsolete sidecars; persists `Driver`, `Plugin`, `DriverMap`, `SymlinkDir`, `InstalledSidecars`, `InstalledBinaries` via `state.Mutate()`.
  - Verify: `rtk go test -v ./cmd/... -run TestUpdate`
  - Files: `cmd/update.go`, `cmd/update_test.go`

## Module 2: `source-driver-lifecycle` (`agent-003`)

- [x] Task 2.1: Deploy driver/plugin manifests and persist fields in source builds (TDD Seam S3a)
  - Acceptance: `handleCompileFromSource` deploys driver/plugin manifests into the staging/symlink directory and saves `Driver`, `Plugin`, `DriverMap`, `IncludeSidecars`, and `SidecarMode` in `state.InstalledApp`; `compile.SaveCompileState` persists the same fields.
  - Verify: `rtk go test -v ./compile/... ./cmd/... -run TestSource\|TestCompile`
  - Files: `cmd/root.go`, `compile/compile.go`, `cmd/root_test.go`

- [x] Task 2.2: Modernize `CompileScript` upgrade path in `DoUpdate` (TDD Seam S3b)
  - Acceptance: `DoUpdate` on `app.CompileScript != ""` syncs the local git clone if present, respects container sandboxing when configured, refreshes symlinks in `app.SymlinkDir`, re-deploys driver/plugin manifests, reconciles obsolete sidecars, and persists updated `Version` and `InstalledSidecars`.
  - Verify: `rtk go test -v ./... && make fmt && make lint`
  - Files: `cmd/update.go`, `cmd/update_test.go`
