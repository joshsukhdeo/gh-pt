# Specification: Source Driver & Plugin Lifecycle Parity (`source-driver-lifecycle`)

## 1. Objective

Provide complete feature and lifecycle parity between source-built installations (`gh-pt source <repo>`) and release binary installations (`gh-pt install <repo>`). Specifically:
- Ensure `--driver`, `--driver-map`, `--plugin`, and `--symlink` flags are fully registered, deployed, and persisted into state during `handleCompileFromSource` and `compile.Execute`.
- Fix the `CompileScript` upgrade pathway in `DoUpdate` (`cmd/update.go`) so that source-built applications:
  1. Update their underlying git checkout (`git fetch` / `git pull` / checkout latest target SHA).
  2. Execute builds inside the container sandbox (`compile.ExecuteInContainer`) when containerization is enabled, rather than unconditionally invoking raw host shell execution.
  3. Perform post-build artifact handoff, symlink recreation, and driver/plugin manifest deployment.
  4. Properly reconcile and record updated `Version`, `InstalledSidecars`, and `InstalledBinaries` in `state.json`.

---

## 2. Tech Stack & Dependencies

- **Language**: Go 1.27
- **CLI Framework**: Kong (`params/params.go`, `cmd/router.go`, `cmd/root.go`, `cmd/update.go`)
- **Compilation Engine**: `compile` package (`compile/compile.go`, `compile/container.go`)
- **Manifest Deployment**: `release.DeployDriverAndPluginManifests` (or shared driver deployment helper)
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

- `cmd/root.go`:
  - In `handleCompileFromSource` (lines 2050-2135), pass driver and plugin parameters (`Driver`, `Plugin`, `DriverMap`) into the compile execution options.
  - Invoke driver/plugin manifest deployment targeting the resolved symlink directory upon successful compilation.
- `compile/compile.go`:
  - In `SaveCompileState` (lines 253-301), persist `Driver`, `Plugin`, `DriverMap`, `IncludeSidecars`, and `SidecarMode` into `state.InstalledApp`.
  - Expose/invoke manifest deployment and capture generated manifests into `InstalledSidecars`.
- `cmd/update.go`:
  - In `DoUpdate` (lines 453-500), overhaul the `app.CompileScript != ""` branch:
    - Synchronize local repository clone (`git fetch` / `git pull` / checkout) before triggering the compile script.
    - Check if container execution is requested or configured; use `compile.ExecuteInContainer` when appropriate.
    - Trigger post-build artifact discovery, symlink refreshing, and manifest deployment.
    - Reconcile `InstalledSidecars` and update the application entry in `state.json`.

---

## 5. Exact Codebase Defects & Remediation

### Defect 1: Manifest Omission & State Truncation in Source Compiles
- **Location**: `cmd/root.go:2050-2135` & `compile/compile.go:43-92, 253-301`
- **Problem**: When installing via `gh-pt source`, flags like `--driver`, `--plugin`, and `--driver-map` are accepted on the CLI, but:
  - `handleCompileFromSource` never triggers `DeployDriverAndPluginManifests` on `symlinkDir` or the install destination.
  - `SaveCompileState` constructs a `state.InstalledApp` with only basic fields (`Repository`, `BinaryPath`, `CompileScript`, etc.), omitting `Driver`, `Plugin`, `DriverMap`, `IncludeSidecars`, and `SidecarMode`.
- **Remediation**:
  1. Pass the full driver/plugin parameter set to `compile.Execute` / `compile.SaveCompileState`.
  2. Invoke `DeployDriverAndPluginManifests` on the resolved symlink directory.
  3. Append newly deployed manifests to `InstalledSidecars` and persist `Driver`, `Plugin`, `DriverMap`, `IncludeSidecars`, `SidecarMode`, and `SymlinkDir` into `state.InstalledApp`.

### Defect 2: Out-of-Date Local Clone During Source Upgrades
- **Location**: `cmd/update.go:453-470`
- **Problem**: When updating an application with `app.CompileScript != ""`, `DoUpdate` checks the remote `HEAD` SHA using `gh api repos/{repo}/commits/HEAD`. If the SHA differs from `app.Version`, it reports that an update is available. However, it never updates the local repository clone in `~/builds/<repo>` (e.g., via `git fetch` / `git checkout <sha>` or `git pull`). As a result, compiling simply rebuilds the stale local source code.
- **Remediation**: Execute git synchronization (`git fetch` and checkout of target commit/branch) within the cloned build directory before invoking the build script.

### Defect 3: Host Execution Bypass and Missing Container Sandboxing
- **Location**: `cmd/update.go:478-485`
- **Problem**: `cmd/update.go` runs `exec.Command("sh", "-c", app.CompileScript)` directly on the host machine. If the application was originally built or configured with container sandbox execution, this configuration is completely bypassed during upgrade, potentially running untrusted or dependency-heavy build scripts directly on the host.
- **Remediation**: Inspect application configuration and execution flags. If container mode is enabled, dispatch the compilation task through `compile.ExecuteInContainer`.

### Defect 4: Missing Post-Build Artifact & Manifest Pipeline on Source Upgrades
- **Location**: `cmd/update.go:486-500`
- **Problem**: After running `app.CompileScript`, `DoUpdate` only updates `app.Version = latestSHA` and saves state. It completely skips:
  - Moving newly generated binary artifacts to destination (`moveDistWithSidecarDetection` / `processNewFilesAndSymlinks`).
  - Refreshing binary symlinks (`createSymlinks`).
  - Deploying updated driver/plugin manifests (`DeployDriverAndPluginManifests`).
  - Reconciling `InstalledSidecars` to remove obsolete files.
- **Remediation**: Wire the standard post-build pipeline into the `CompileScript` branch in `cmd/update.go`, ensuring artifacts are moved, symlinks are updated, driver manifests are deployed, and sidecars are reconciled before persisting state.

---

## 6. Code Style & Observability

- **Structured Logging (`golang-observability`)**:
  - Use `charmbracelet/log` for all lifecycle messages:
    - `log.Info("synchronizing source repository", "repo", app.Repository, "target_sha", latestSHA)`
    - `log.Debug("executing source build in container", "container", containerName)`
    - `log.Debug("re-deploying driver manifests for source build", "driver", app.Driver)`
  - Return errors cleanly or log them at the CLI entry point; never double-log and return errors.

---

## 7. Testing Strategy

### Seam S3: Source Build State Persistence, Manifest Deployment & Upgrade Parity
- **Target Files**: `cmd/root_test.go`, `cmd/update_test.go`
- **Test Flow**:
  1. Set up isolated test environment with `t.TempDir()`, `XDG_DATA_HOME`, and `xdg.Reload()`.
  2. **Install Test**:
     - Invoke `handleCompileFromSource` with mock compile script, `--driver "vulkan"`, `--driver-map ".*icd.*\\.json$:vulkan"`, and `--symlink`.
     - Verify that:
       - `state.LoadState()` records `Driver == "vulkan"`, `DriverMap == ".*icd.*\\.json$:vulkan"`, and `SymlinkDir`.
       - Manifest file exists in `SymlinkDir`.
       - `InstalledSidecars` contains the manifest.
  3. **Upgrade Test**:
     - Prepare mock git repo with initial commit and a subsequent new commit.
     - Trigger `DoUpdate` on the source-installed app.
     - Verify that:
       - Git repository is updated to the latest commit.
       - Build runner is invoked (in container if specified).
       - Binary symlink is refreshed.
       - Driver manifest is refreshed in `SymlinkDir`.
       - Updated `Version` and reconciled `InstalledSidecars` are persisted to `state.json`.

---

## 8. Boundaries & Invariants

- **State File Locking**: Maintain `state.json.lock` discipline via `state.LoadState()` / `st.Save()` or `state.Mutate()`.
- **Source Isolation**: Source builds should not taint paths outside their designated build directory and XDG paths.
- **Terminal Hygiene**: Clean up any background spinners or indicators prior to stdout/stderr generation.
- **Preserve Custom Scripts**: Do not rewrite or clobber custom `CompileScript` content provided by the user.

---

## 9. Success Criteria

- [ ] `handleCompileFromSource` registers driver and plugin manifests in the target symlink directory.
- [ ] `compile.SaveCompileState` persists `Driver`, `Plugin`, `DriverMap`, `IncludeSidecars`, and `SidecarMode` in `state.InstalledApp`.
- [ ] `DoUpdate` updates the local git clone to the latest commit before running source builds.
- [ ] `DoUpdate` uses container sandboxing when configured.
- [ ] `DoUpdate` triggers artifact placement, symlink refresh, driver manifest deployment, and sidecar reconciliation for source apps.
- [ ] Full test suite passes via `rtk go test -v ./...`.

---

## 10. Open Questions & Assumptions

- *Assumption*: Git updates use `git fetch` and checkout/pull in the local build directory (`~/builds/<repo>`).
- *Assumption*: Source builds default `SymlinkDir` to `~/src/apps/<owner>/<repo>` when `app.SymlinkDir` is empty.
