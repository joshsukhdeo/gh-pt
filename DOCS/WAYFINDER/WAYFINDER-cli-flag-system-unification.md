# Wayfinder: CLI Flag System & Execution Unification (`cli-flag-system-unification`)

**File**: `WAYFINDER-cli-flag-system-unification.md`  
**Repository**: `gh-pt` (`/home/tay/projects/gh-pt`)  
**Companion Spec**: [`DOCS/SPECS/SPEC-cli-flag-system-unification.md`](file:///home/tay/projects/gh-pt/DOCS/SPECS/SPEC-cli-flag-system-unification.md)  
**Primary Research**: [`DOCS/RESEARCH/RESEARCH-cli-switches-install-vs-source-audit.md`](file:///home/tay/projects/gh-pt/DOCS/RESEARCH/RESEARCH-cli-switches-install-vs-source-audit.md)  

---

## Destination

Establish a clean, domain-partitioned CLI flag architecture across `install`, `source`, and `upgrade` (`update`/`sync`), eliminate dead and duplicate switches, and unify divergent package storage, symlink, sidecar, and replay pipelines across `gh-pt`.

---

## Notes & Core Invariants

- **Domain**: Go CLI (`gh-pt`), Kong Parser (`params/params.go`), Package Storage (XDG Base Directory / ADR-004), Container Sandboxing (`podman`/`docker`), State Locking.
- **Skills**: `spec-driven-development`, `tdd-workflow`, `ponytail`, `architect-review`.
- **Standing Invariants**:
  1. **Strict Test Isolation**: Set `XDG_DATA_HOME` via `t.TempDir()` and immediately call `xdg.Reload()` in all unit/integration tests.
  2. **File-Locked State Access**: Never mutate `state.json` directly; always route through `state.LoadState()` and `st.Save()` under `state.json.lock`.
  3. **No Unrequested Flags**: Keep parser surfaces minimal. If a flag does not alter execution for that subcommand, it must not appear in `--help`.
  4. **Backward Compatibility**: Existing installations recorded in `state.json` (such as legacy `~/src/apps` paths or stored `env_inject` fields) must parse cleanly without data corruption during upgrades or uninstalls.

---

## Decisions so far

- [ADR-004: Canonical Application Package Store](../../.memory/decisions.md): Accepted — standard extraction location is `$XDG_DATA_HOME/gh-pt/packages/{ownerID}/{repoID}`.
- [RESEARCH: CLI Switches Install vs Source Audit](../RESEARCH/RESEARCH-cli-switches-install-vs-source-audit.md): Complete audit of 65 switches identifying structural cross-contamination, dead flags, and execution engine divergences.
- [SPEC: CLI Flag System & Execution Unification](../SPECS/SPEC-cli-flag-system-unification.md): Formally defines the decomposition of `CommonInstallFlags` into composable domain structs and specifies execution unification rules.

---

## Completed Tickets

- [x] [Ticket 1: Kong Struct Hierarchy Refactor & Dead/Redundant Flag Pruning](#ticket-1-kong-struct-hierarchy-refactor--deadredundant-flag-pruning) (`wayfinder:task`, AFK)
- [x] [Ticket 2: ADR-004 Package Store Directory & Atomic Symlink/Rename Engine Unification](#ticket-2-adr-004-package-store-directory--atomic-symlinkrename-engine-unification) (`wayfinder:task`, AFK)
- [x] [Ticket 3: Sidecar Mode & Intermediate Dist Handoff Unification](#ticket-3-sidecar-mode--intermediate-dist-handoff-unification) (`wayfinder:task`, AFK)
- [x] [Ticket 4: Upgrade Replay Hygiene & Full Integration Verification](#ticket-4-upgrade-replay-hygiene--full-integration-verification) (`wayfinder:task`, AFK)

## Open Tickets

None.

---

## Out of Scope

- Modifying the AI prompt generation templates in `ai/prompt.go`.
- Reworking the internal OCI container specification or SELinux/seccomp profiles in `compile/container.go`.
- Modifying the schema structure of `manifest.json`.

---

### Ticket 1: Kong Struct Hierarchy Refactor & Dead/Redundant Flag Pruning
**Label:** `wayfinder:task` (AFK)  
**Status:** COMPLETED  
**Verification:** CommonInstallFlags decomposed into composable domain structs (SharedInstallSourceFlags, ReleaseAssetFlags, SourceBuildFlags). Dead/joke flags pruned (--env-inject, --BARBAROUS, etc.). Help outputs verified; integration tests in params/ pass.  
**Deliverable:**
1. In `params/params.go`:
   - Decompose `CommonInstallFlags` into:
     - `SharedInstallSourceFlags` (`TargetFlags`, `DependencyFlags`, `DriverPluginFlags`, `SharedRuntimeFlags`, `OutputFlags`).
     - `ReleaseAssetFlags` (`ResolutionFlags`, `VerificationFlags`, `ReleaseExtractorFlags`, `ProgressBar`).
     - `SourceBuildFlags` (`AICmd`, `TargetOS`, `TargetArch`, `NoCompileContainer`, `NoAIContainer`).
   - Rebind:
     - `InstallCmd` $\rightarrow$ `SharedInstallSourceFlags` + `ReleaseAssetFlags`.
     - `SourceCmd` $\rightarrow$ `SharedInstallSourceFlags` + `SourceBuildFlags`.
     - `UpgradeFlags` $\rightarrow$ Add `NoCompileContainer` and `NoAIContainer`.
   - Prune dead/redundant flags:
     - Remove `--env-inject` from `params/params.go`.
     - Remove redundant joke flags (`--self-inflicted-technical-debt`, `--LE-RETROGROUCH`, `--retrograde-stopgap`, `--BARBAROUS`).
     - Remove `--allow-root-user-install` (retain `--force-root`).
     - Remove `--ai-setup-sidecars`.
   - Update `ExecContext` to compose the new sub-structs for complete internal backward compatibility.
2. In `cmd/router.go`:
   - Update `RunCommand` mappings for `install`, `source`, and `upgrade` to match refactored struct hierarchies.
   - Pass `cli.Upgrade.NoCompileContainer` and `cli.Upgrade.NoAIContainer` to `r.ExecContext`.
3. In `cmd/root.go`:
   - Fix inverted logic bug in `validateTargetPath` ([`cmd/root.go:488`](file:///home/tay/projects/gh-pt/cmd/root.go#L488)): change `if !os.IsNotExist(err)` to `if os.IsNotExist(err)`.
   - Remove unreachable phantom check `if r.CompileFromSource && !r.AI` ([`cmd/root.go:419-421`](file:///home/tay/projects/gh-pt/cmd/root.go#L419-L421)).
   - Simplify downgrade logic in `RunInstall` to check strictly `r.AllowDowngrade`.
4. **TDD Gates**:
   - Add/update CLI integration tests in `params/cli_integration_test.go`:
     - Test that `gh-pt install` rejects `--no-compile-container` and `--target-os`.
     - Test that `gh-pt source` rejects `--verify-checksum`, `--format`, and `--extractor`.
     - Test that `gh-pt upgrade` accepts `--no-compile-container`.
     - Test that pruned flags (`--env-inject`, `--BARBAROUS`) return parsing errors.
   - Test that directory creation works when `TargetPathCreate = true` and target path is missing.

---

### Ticket 2: ADR-004 Package Store Directory & Atomic Symlink/Rename Engine Unification
**Label:** `wayfinder:task` (AFK)  
**Status:** COMPLETED  
**Verification:** Source builds unified onto ADR-004 canonical package store ($XDG_DATA_HOME/gh-pt/packages/{owner}/{repo}) with legacy ~/src/apps path fallback. Unified atomic symlinking and rename engine verified with tests.  
**Deliverable:**
1. In `cmd/root.go` (`handleCompileFromSource`):
   - Replace legacy `symlinkDir = filepath.Join(homeDir, "src", "apps", ownerID, repoID)` with canonical package directory resolver:
     `symlinkDir = rel.ResolvePackageDir(ownerID, repoID)` (pointing to `$XDG_DATA_HOME/gh-pt/packages/{owner}/{repo}`).
   - Preserve existing app paths if already installed in `~/src/apps` (checked via `state.LoadState()`).
2. Symlink & Rename Engine Unification:
   - Expose `DeployBinariesFromStaging(stagingDir, targetPath string) error` in `release/symlink.go` (or shared helper).
   - Replace the raw `createSymlinks` implementation in `cmd/root.go:2617` with `DeployBinariesFromStaging`.
   - Ensure source builds:
     - Use atomic symlinking via `createSymlinkAtomic` with fallback to hardlink and copy.
     - Respect binary renames specified via `-t` / `--rename`.
     - Automatically discover and register shell completion scripts.
3. **TDD Gates**:
   - Test in `cmd/root_test.go` that a source build installs into the canonical `$XDG_DATA_HOME/gh-pt/packages` directory.
   - Test that binary renaming (`--rename original=custom`) works on source builds.
   - Test symlink fallback on simulated symlink failure.

---

### Ticket 3: Sidecar Mode & Intermediate Dist Handoff Unification
**Label:** `wayfinder:task` (AFK)  
**Status:** COMPLETED  
**Verification:** Dead compile.Execute and duplicate handleDistHandoff purged (-213 lines) from compile/compile.go. Unit test suite compile/compile_test.go added and passing (12/12 tests in compile/).  
**Deliverable:**
1. In `cmd/root.go` (`moveDistWithSidecarDetection`):
   - Replace hardcoded sidecar destination `filepath.Join(xdg.DataHome, "gh-pt", "sidecars")` with destination resolved by `--sidecar-mode`:
     - `local-map`: Map standard directories into target prefix (`~/.local` or `/usr/local`).
     - `xdg_data_home`: Symlink to `$XDG_DATA_HOME/gh-pt/sidecars`.
     - `custom-path:/path`: Symlink to custom path.
2. Purge dead compile functions:
   - Delete unused `compile.Execute` ([`compile/compile.go:49-98`](file:///home/tay/projects/gh-pt/compile/compile.go#L49-L98)) and `handleDistHandoff` ([`compile/compile.go:212-235`](file:///home/tay/projects/gh-pt/compile/compile.go#L212-L235)).
   - Align `SaveCompileState` in `compile/compile.go` or deprecate in favor of `state.AddApp`.
3. **TDD Gates**:
   - Verify `moveDistWithSidecarDetection` places sidecars according to `SidecarMode` in `cmd/root_test.go`.
   - Verify all unit tests compile and pass after removing dead functions in `compile/compile.go`.

---

### Ticket 4: Upgrade Replay Hygiene & Full Integration Verification
**Label:** `wayfinder:task` (AFK)  
**Status:** COMPLETED  
**Verification:** Upgrade replay logic isolated in cmd/update.go. Replay flags sanitized for source vs release builds. EnvInject deprecated cleanly. make build, make fmt, and rtk go test ./... passing with 100% success rate (804/804 tests).  
**Deliverable:**
1. In `cmd/update.go`:
   - In `processSingleUpdate` ([`cmd/update.go:178-226`](file:///home/tay/projects/gh-pt/cmd/update.go#L178-L226)):
     - Cleanly isolate replay parameters: do not copy release-only flags into source builds.
     - For source-built applications (`app.CompileScript != ""`):
       - Pass `r.NoCompileContainer` and `r.NoAIContainer` from CLI flags to container execution config.
       - Replay `app.Rename`, `app.SymlinkDir`, `app.Driver`, `app.Plugin`, `app.DriverMap`, `app.IncludeSidecars`, `app.SidecarMode`.
       - Remove `EnvInject` copy logic.
2. In `state/state.go` and `state/history.go`:
   - Clean up `EnvInject` references (leave JSON tag on `state.InstalledApp` for backward compatibility when loading old states, but omit from active state creation).
3. **Full Verification**:
   - `make build`
   - `rtk go test -v ./...`
   - `make fmt && make lint`
