# Research Audit: CLI Switches & Flags — `install` vs `source` Architectures

**File**: `RESEARCH-cli-switches-install-vs-source-audit.md`  
**Repository**: `gh-pt` (`/home/tay/projects/gh-pt`)  
**Auditor**: Antigravity Research Subagent  
**Date**: November 19, 2026  
**Status**: COMPLETE / CANONICAL AUDIT  

---

## Executive Summary

An exhaustive primary-source audit of every CLI switch, flag definition, parser tag, and execution binding across `gh-pt` was conducted against the live codebase:
- [`params/params.go`](file:///home/tay/projects/gh-pt/params/params.go)
- [`cmd/router.go`](file:///home/tay/projects/gh-pt/cmd/router.go)
- [`cmd/root.go`](file:///home/tay/projects/gh-pt/cmd/root.go)
- [`cmd/update.go`](file:///home/tay/projects/gh-pt/cmd/update.go)
- [`release/release.go`](file:///home/tay/projects/gh-pt/release/release.go)
- [`release/symlink.go`](file:///home/tay/projects/gh-pt/release/symlink.go)
- [`release/driver.go`](file:///home/tay/projects/gh-pt/release/driver.go)
- [`compile/compile.go`](file:///home/tay/projects/gh-pt/compile/compile.go)
- [`compile/container.go`](file:///home/tay/projects/gh-pt/compile/container.go)

A total of **65 distinct flags/switches** were audited. The findings reveal substantial architectural drift:
1. **Pervasive Structural Over-Exposure**: `SourceCmd` embeds the entire `CommonInstallFlags` struct ([`params/params.go:503`](file:///home/tay/projects/gh-pt/params/params.go#L503)). Consequently, 23 release-asset-specific switches (checksums, VT sandboxes, asset regexes, release tags, archive formats, extractor selections, AVX heuristics) are exposed on `gh-pt source` where they have zero effect or logical validity.
2. **Reverse Misplacement**: Compilation container switches (`--no-compile-container`, `--no-ai-container`) are embedded in `CommonInstallFlags` ([`params/params.go:235-236`](file:///home/tay/projects/gh-pt/params/params.go#L235-L236)) and exposed on `gh-pt install`, despite pre-compiled release asset installations never touching compilation sandboxes.
3. **Dead and Unwired Logic**:
   - `--env-inject`: Defined, stored in state, cloned in history, replayed on upgrade, but **never wired to any runtime environment injector or wrapper script**.
   - `--ai-setup-sidecars`: Only executed if `--sidecars` is passed on release install; runs an unvetted text prompt to stdout and is completely absent from source builds.
   - `--target-path-create`: Flawed inverted logic condition at [`cmd/root.go:488`](file:///home/tay/projects/gh-pt/cmd/root.go#L488) prevents path creation when the path does not exist.
4. **Three-Way Divergence in Symlink and Sidecar Execution**:
   - Package storage location diverged between ADR-004 canonical path (`$XDG_DATA_HOME/gh-pt/packages/...` in [`release/symlink.go:248`](file:///home/tay/projects/gh-pt/release/symlink.go#L248)) vs legacy `~/src/apps/...` in [`cmd/root.go:1802`](file:///home/tay/projects/gh-pt/cmd/root.go#L1802).
   - `.ghpt/dist` handoff implemented twice divergently: [`cmd/root.go:1442`](file:///home/tay/projects/gh-pt/cmd/root.go#L1442) (`moveDistWithSidecarDetection`) vs [`compile/compile.go:212`](file:///home/tay/projects/gh-pt/compile/compile.go#L212) (`handleDistHandoff`).
   - Symlink creation implemented twice divergently: [`release/symlink.go:251`](file:///home/tay/projects/gh-pt/release/symlink.go#L251) (atomic symlinks with hardlink/copy fallbacks, driver/plugin manifests, shell completions) vs [`cmd/root.go:2617`](file:///home/tay/projects/gh-pt/cmd/root.go#L2617) (raw `os.Symlink` without fallback, crashes on Windows, ignores rename rules).
   - Entire compile workflow duplicated: `compile.Execute` ([`compile/compile.go:49`](file:///home/tay/projects/gh-pt/compile/compile.go#L49)) is completely bypassed/abandoned by [`cmd/root.go:1773`](file:///home/tay/projects/gh-pt/cmd/root.go#L1773) (`handleCompileFromSource`).

---

## Part A: Unnecessary, Dead, Redundant, or Duplicate-Logic Switches

### 1. Unused, Partially Wired, Redundant Aliases, or Obsolete Switches

| Switch / Flag | Defined In | Actual Call Sites | Status | Technical Analysis & Evidence |
| :--- | :--- | :--- | :--- | :--- |
| `--env-inject` | [`params/params.go:185`](file:///home/tay/projects/gh-pt/params/params.go#L185) | [`cmd/update.go:210`](file:///home/tay/projects/gh-pt/cmd/update.go#L210), [`state/state.go:52`](file:///home/tay/projects/gh-pt/state/state.go#L52), [`state/history.go:295`](file:///home/tay/projects/gh-pt/state/history.go#L295) | **Dead / Unwired** | Declared on `SidecarFlags` as `[]string`, persisted in `state.InstalledApp.EnvInject`, cloned in rollback history, and replayed in `cmd/update.go`. However, **no code in `release/`, `cmd/`, or `wrapper/` ever reads this field to inject environment variables, generate wrapper scripts, or mutate shell configs**. It is dead code. |
| `--ai-setup-sidecars` | [`params/params.go:187`](file:///home/tay/projects/gh-pt/params/params.go#L187) | [`release/release.go:569`](file:///home/tay/projects/gh-pt/release/release.go#L569), [`1839`](file:///home/tay/projects/gh-pt/release/release.go#L1839), [`2524`](file:///home/tay/projects/gh-pt/release/release.go#L2524) | **Partially Wired / Obsolete** | Supposed to "analyze sidecars and generate post-install setup commands". Only fires in `release/release.go:2524` if `--sidecars` (`r.CliParams.Sidecars != ""`) is also explicitly set. When run, it simply passes a prompt to `agy` or host shell to dump unstructured text to `os.Stdout` ([`release/release.go:605-609`](file:///home/tay/projects/gh-pt/release/release.go#L605-L609)). It executes nothing and is completely omitted from source builds. |
| `--target-path-create` | [`params/params.go:153`](file:///home/tay/projects/gh-pt/params/params.go#L153) | [`cmd/root.go:489`](file:///home/tay/projects/gh-pt/cmd/root.go#L489) | **Defective Logic** | In `validateTargetPath` ([`cmd/root.go:486-512`](file:///home/tay/projects/gh-pt/cmd/root.go#L486-L512)): `targetPathInfo, err := os.Stat(r.TargetPath); if err != nil { if !os.IsNotExist(err) { createPath := r.TargetPathCreate ... } }`. If the directory does NOT exist, `os.IsNotExist(err)` is `true`, making `!os.IsNotExist(err)` `false`! The creation block is skipped, immediately returning error `target installation path error` ([`cmd/root.go:511`](file:///home/tay/projects/gh-pt/cmd/root.go#L511)). |
| `--self-inflicted-technical-debt` | [`params/params.go:239`](file:///home/tay/projects/gh-pt/params/params.go#L239) | [`cmd/root.go:557`](file:///home/tay/projects/gh-pt/cmd/root.go#L557), [`cmd/update.go:452`](file:///home/tay/projects/gh-pt/cmd/update.go#L452), [`release/release.go:1899`](file:///home/tay/projects/gh-pt/release/release.go#L1899) | **Redundant Alias** | 100% duplicate alias for `--allow-downgrade`. In `cmd/root.go:557`: `if r.SelfInflictedDebt { r.AllowDowngrade = true }`. Redundant CLI bloat. |
| `--LE-RETROGROUCH` | [`params/params.go:240`](file:///home/tay/projects/gh-pt/params/params.go#L240) | [`cmd/root.go:549`](file:///home/tay/projects/gh-pt/cmd/root.go#L549), [`cmd/update.go:444`](file:///home/tay/projects/gh-pt/cmd/update.go#L444), [`release/release.go:1899`](file:///home/tay/projects/gh-pt/release/release.go#L1899), [`2086`](file:///home/tay/projects/gh-pt/release/release.go#L2086) | **Redundant Joke Flag** | Sets `AllowDowngrade = true` and `PinInstall = false` ([`cmd/root.go:550-551`](file:///home/tay/projects/gh-pt/cmd/root.go#L550-L551)). In `release/release.go:2086`, sets unpinned state. |
| `--retrograde-stopgap` | [`params/params.go:241`](file:///home/tay/projects/gh-pt/params/params.go#L241) | [`cmd/root.go:553`](file:///home/tay/projects/gh-pt/cmd/root.go#L553), [`cmd/update.go:448`](file:///home/tay/projects/gh-pt/cmd/update.go#L448), [`release/release.go:1899`](file:///home/tay/projects/gh-pt/release/release.go#L1899), [`2087`](file:///home/tay/projects/gh-pt/release/release.go#L2087) | **Redundant Joke Flag** | Sets `AllowDowngrade = true` and `PinInstall = true` ([`cmd/root.go:554-555`](file:///home/tay/projects/gh-pt/cmd/root.go#L554-L555)). Can be accomplished via `--allow-downgrade --pin-install`. |
| `--BARBAROUS` | [`params/params.go:242`](file:///home/tay/projects/gh-pt/params/params.go#L242) | [`cmd/root.go:539`](file:///home/tay/projects/gh-pt/cmd/root.go#L539), [`cmd/update.go:434`](file:///home/tay/projects/gh-pt/cmd/update.go#L434), [`release/release.go:1899`](file:///home/tay/projects/gh-pt/release/release.go#L1899) | **Redundant Composite Flag** | Composite bypass macro: sets `InsecureAllowUnsigned=true`, `VerifyChecksum=false`, `SkipVtSandbox=true`, `AllowForeignArch=true`, `AllowDowngrade=true`, and `Wine="allow"` ([`cmd/root.go:540-547`](file:///home/tay/projects/gh-pt/cmd/root.go#L540-L547)). |
| `--allow-root-user-install` vs `--force-root` | [`params/params.go:233-234`](file:///home/tay/projects/gh-pt/params/params.go#L233-L234) | [`cmd/root.go:444`](file:///home/tay/projects/gh-pt/cmd/root.go#L444), [`458`](file:///home/tay/projects/gh-pt/cmd/root.go#L458), [`473`](file:///home/tay/projects/gh-pt/cmd/root.go#L473) | **Redundant Overlap** | Both flags allow root to install to user paths (`~/.local/bin`) at [`cmd/root.go:473`](file:///home/tay/projects/gh-pt/cmd/root.go#L473) (`if !r.AllowRootUserInstall && !r.ForceRoot`). The only difference is `--force-root` also suppresses the root warning banner at line 444 (`if !inContainer && !r.ForceRoot`). These two should be unified into `--force-root`. |
| Phantom `--compile-from-source` | [`params/params.go:549`](file:///home/tay/projects/gh-pt/params/params.go#L549) (field on ExecContext only) | [`cmd/root.go:419-421`](file:///home/tay/projects/gh-pt/cmd/root.go#L419-L421) | **Phantom Flag / Dead Check** | [`cmd/root.go:419-421`](file:///home/tay/projects/gh-pt/cmd/root.go#L419-L421) checks: `if r.CompileFromSource && !r.AI { return fmt.Errorf("--compile-from-source can only be used with --ai") }`. But `--compile-from-source` is no longer a CLI flag; it is set strictly via `cmd/router.go:207` when invoking the `gh-pt source` subcommand, which also hardcodes `r.AI = true` ([`cmd/router.go:208`](file:///home/tay/projects/gh-pt/cmd/router.go#L208)). This check is unreachable unless manipulated in tests. |

---

### 2. Diverging & Duplicate Implementations Across `install` vs `source`

The audit identified five major architectural divergence points between the release asset pipeline (`gh-pt install`) and the source build pipeline (`gh-pt source`):

#### Divergence 1: Symlink Directory Allocation (`packages` vs `apps`)
* **`install` (ADR-004)**: In [`release/symlink.go:248`](file:///home/tay/projects/gh-pt/release/symlink.go#L248) (`resolvePackageDir`), release archives are installed into `$XDG_DATA_HOME/gh-pt/packages/{ownerID}/{repoID}` (e.g., `~/.local/share/gh-pt/packages/cli/cli`).
* **`source` (Legacy)**: In [`cmd/root.go:1802`](file:///home/tay/projects/gh-pt/cmd/root.go#L1802) (`handleCompileFromSource`), source builds are installed into `~/src/apps/{ownerID}/{repoID}`.
* **Impact**: Inconsistent directory layout across installation mechanisms. An app installed from source cannot be cleanly detected or managed by tools expecting the ADR-004 canonical package store.

#### Divergence 2: Intermediate Dist Handoff (`moveDistWithSidecarDetection` vs `handleDistHandoff`)
* **`cmd/root.go:1442-1600` (`moveDistWithSidecarDetection`)**:
  * Scans `.ghpt/dist`.
  * Detects Gnome bundles by inspecting `metadata.json` or `--plugin=gnome-extension` ([`cmd/root.go:1447-1458`](file:///home/tay/projects/gh-pt/cmd/root.go#L1447-L1458)).
  * Classifies files using `isSuspectedLocalSidecar` ([`cmd/root.go:1480`](file:///home/tay/projects/gh-pt/cmd/root.go#L1480)).
  * Moves suspected sidecars to hardcoded `~/.local/share/gh-pt/sidecars` ([`cmd/root.go:1545`](file:///home/tay/projects/gh-pt/cmd/root.go#L1545)), ignoring `--sidecar-mode`.
* **`compile/compile.go:212-235` (`handleDistHandoff`)**:
  * Completely separate implementation.
  * Walks `.ghpt/dist` and copies all files directly into `cfg.TargetPath` ([`compile/compile.go:228`](file:///home/tay/projects/gh-pt/compile/compile.go#L228)).
  * Does not detect sidecars; returns an empty `sidecars` slice ([`compile/compile.go:215, 234`](file:///home/tay/projects/gh-pt/compile/compile.go#L215-L234)).
  * Does not detect Gnome extensions.
* **Impact**: Total duplication of dist-moving logic with conflicting behaviors.

#### Divergence 3: Symlink Deployment (`createSymlinks` vs `executeSymlinkInstall`)
* **`cmd/root.go:2617-2665` (`createSymlinks`)**:
  * Used by `handleCompileFromSource` ([`cmd/root.go:2194`](file:///home/tay/projects/gh-pt/cmd/root.go#L2194)).
  * Only iterates `symlinkDir`, `service`, and `preferences` ([`cmd/root.go:2623`](file:///home/tay/projects/gh-pt/cmd/root.go#L2623)).
  * Uses non-atomic `os.Remove(destPath)` followed by `os.Symlink(srcPath, destPath)` ([`cmd/root.go:2651-2655`](file:///home/tay/projects/gh-pt/cmd/root.go#L2651-L2655)).
  * **Fails completely on Windows** without developer mode or elevated permissions (no fallback).
  * Completely ignores `--rename` (`r.Rename`), `--keep-suffixes`, and does not register shell completions.
* **`release/symlink.go:251-385` (`executeSymlinkInstall`)**:
  * Used by `gh-pt install`.
  * Atomic symlink creation via `createSymlinkAtomic` ([`release/symlink.go:25-63`](file:///home/tay/projects/gh-pt/release/symlink.go#L25-L63)) with temporary file rename to prevent TOCTOU races.
  * Graceful fallback: symlink $\rightarrow$ hardlink $\rightarrow$ file copy ([`release/symlink.go:51-62`](file:///home/tay/projects/gh-pt/release/symlink.go#L51-L62)).
  * Respects `Rename` mapping ([`release/symlink.go:320`](file:///home/tay/projects/gh-pt/release/symlink.go#L320), `resolveDestinationPath`).
  * Automates shell completion discovery and registration via `ScanAndInstallArchiveCompletions` ([`release/symlink.go:362`](file:///home/tay/projects/gh-pt/release/symlink.go#L362)).
  * Integrates `symlinkSidecars` ([`release/symlink.go:368`](file:///home/tay/projects/gh-pt/release/symlink.go#L368)) and `DeployDriverAndPluginManifests` ([`release/symlink.go:375`](file:///home/tay/projects/gh-pt/release/symlink.go#L375)).

#### Divergence 4: Compile Orchestration (`handleCompileFromSource` vs `compile.Execute`)
* **`cmd/root.go:1773-2317` (`handleCompileFromSource`)**:
  * 544 lines of code embedded directly in `cmd/root.go`.
  * Implements two-stage container execution (AI container stage 1, hardened compiler stage 2).
  * Handles script generation, validation, template rendering (`header.sh`, `body.sh`, `footer.sh`).
  * Persists directly to `state.Apps` and `state.InstallMap`.
* **`compile/compile.go:49-98` (`compile.Execute`) & `compile.SaveCompileState`**:
  * Standalone compile runner in `compile/compile.go`.
  * **Completely uncalled and orphaned** from runtime CLI execution! (Only referenced in documentation and a unit test in [`cmd/root_test.go:379`](file:///home/tay/projects/gh-pt/cmd/root_test.go#L379)).
  * Lacks the two-stage execution and security validation present in `handleCompileFromSource`.

#### Divergence 5: Compilation Container Flag Misplacement
* `--no-compile-container` ([`params/params.go:235`](file:///home/tay/projects/gh-pt/params/params.go#L235)) and `--no-ai-container` ([`params/params.go:236`](file:///home/tay/projects/gh-pt/params/params.go#L236)) are defined on `ExecutionFlags`, which is embedded in `CommonInstallFlags`.
* They are available on `gh-pt install`, where they do absolutely nothing.
* Conversely, on `gh-pt upgrade`, `UpgradeFlags` ([`params/params.go:366-379`](file:///home/tay/projects/gh-pt/params/params.go#L366-L379)) **omits** `NoCompileContainer`. When `gh-pt upgrade` synchronizes and rebuilds a source app ([`cmd/update.go:537`](file:///home/tay/projects/gh-pt/cmd/update.go#L537)), `r.NoCompileContainer` is always `false` because the CLI flag cannot be parsed into `UpgradeCmd`! The user is forced to use the environment variable `GHPT_NO_COMPILE_CONTAINER=1`.

---

## Part B: Complete Classification of Switches

### 1. `install`-Only Switches
These switches are currently exposed on `CommonInstallFlags` (and thus inherited by both `InstallCmd` and `SourceCmd`), but **only apply to pre-built GitHub release assets**. They have no meaning, effect, or execution in `gh-pt source`:

| Flag Name | CLI Switch & Aliases | Definition Site | Implementation Call Sites | Reason `install`-Only |
| :--- | :--- | :--- | :--- | :--- |
| `ReleaseVersion` | `-v`, `--release-version` | [`params/params.go:132`](file:///home/tay/projects/gh-pt/params/params.go#L132) | [`cmd/root.go:833`](file:///home/tay/projects/gh-pt/cmd/root.go#L833), [`release/release.go:1852`](file:///home/tay/projects/gh-pt/release/release.go#L1852) | `source` hardcodes `r.Track = "latest-commit"` ([`cmd/router.go:209`](file:///home/tay/projects/gh-pt/cmd/router.go#L209)) and clones git `HEAD` ([`cmd/root.go:1854`](file:///home/tay/projects/gh-pt/cmd/root.go#L1854)). Tag selection flag is completely ignored. |
| `ReleaseAsset` | `-a`, `--release-asset` | [`params/params.go:133`](file:///home/tay/projects/gh-pt/params/params.go#L133) | [`release/release.go:2008`](file:///home/tay/projects/gh-pt/release/release.go#L2008), [`selector/interface.go:215`](file:///home/tay/projects/gh-pt/selector/interface.go#L215) | Filters specific release asset files published on GitHub Releases. Source builds clone a git repository. |
| `ReleaseAssetRegexp` | `-A`, `--release-asset-regexp` | [`params/params.go:134`](file:///home/tay/projects/gh-pt/params/params.go#L134) | [`cmd/root.go:824-829`](file:///home/tay/projects/gh-pt/cmd/root.go#L824-L829), [`selector/interface.go:220`](file:///home/tay/projects/gh-pt/selector/interface.go#L220) | Regex matching GitHub release asset filenames. Irrelevant to git clone and local build. |
| `Type` | `-T`, `--format` | [`params/params.go:136`](file:///home/tay/projects/gh-pt/params/params.go#L136) | [`cmd/root.go:825`](file:///home/tay/projects/gh-pt/cmd/root.go#L825), [`selector/interface.go:250`](file:///home/tay/projects/gh-pt/selector/interface.go#L250) | Filters asset archive types (`tar.gz`, `zip`, `deb`, `appimage`). Source builds compile code directly. |
| `All` | `--all` | [`params/params.go:137`](file:///home/tay/projects/gh-pt/params/params.go#L137) | [`cmd/root.go:836`](file:///home/tay/projects/gh-pt/cmd/root.go#L836), [`selector/interface.go:370`](file:///home/tay/projects/gh-pt/selector/interface.go#L370) | Selects all assets/archives from release. Ignored in source builds. |
| `OnlyFirstMatch` | `--only-first-match` | [`params/params.go:138`](file:///home/tay/projects/gh-pt/params/params.go#L138) | [`release/release.go:2386`](file:///home/tay/projects/gh-pt/release/release.go#L2386), [`selector/interface.go:379`](file:///home/tay/projects/gh-pt/selector/interface.go#L379) | Halts extraction after first binary match in archive. Ignored in source builds. |
| `MaxExeInstalls` | `--max-exe-installs` | [`params/params.go:139`](file:///home/tay/projects/gh-pt/params/params.go#L139) | [`release/release.go:2389`](file:///home/tay/projects/gh-pt/release/release.go#L2389), [`selector/interface.go:392`](file:///home/tay/projects/gh-pt/selector/interface.go#L392) | Caps extracted executables from archive. Ignored in source builds. |
| `Prerelease` | `-P`, `--prerelease` | [`params/params.go:140`](file:///home/tay/projects/gh-pt/params/params.go#L140) | [`cmd/root.go:640`](file:///home/tay/projects/gh-pt/cmd/root.go#L640), [`release/release.go:1854`](file:///home/tay/projects/gh-pt/release/release.go#L1854) | Queries GitHub pre-releases API. Source builds build git branch. |
| `Stable` | `--stable` | [`params/params.go:141`](file:///home/tay/projects/gh-pt/params/params.go#L141) | [`cmd/root.go:664`](file:///home/tay/projects/gh-pt/cmd/root.go#L664), [`release/release.go:1755`](file:///home/tay/projects/gh-pt/release/release.go#L1755) | Excludes GitHub pre-releases. Ignored in source builds. |
| `FallbackReleases` | `--fallback-releases` | [`params/params.go:142`](file:///home/tay/projects/gh-pt/params/params.go#L142) | [`cmd/root.go:389`](file:///home/tay/projects/gh-pt/cmd/root.go#L389), [`release/release.go:1930`](file:///home/tay/projects/gh-pt/release/release.go#L1930) | Fallback pagination over older GitHub releases. Completely irrelevant to source. |
| `SearchForInstallInstructionsIfNoReleaseAssets` | `--search-for-install-instructions-if-no-release-assets` | [`params/params.go:143`](file:///home/tay/projects/gh-pt/params/params.go#L143) | [`release/release.go:1994`](file:///home/tay/projects/gh-pt/release/release.go#L1994) | Scrapes README when release assets are missing. |
| `AvxLevel` | `--avx-level` | [`params/params.go:144`](file:///home/tay/projects/gh-pt/params/params.go#L144) | [`cmd/root.go:651`](file:///home/tay/projects/gh-pt/cmd/root.go#L651), [`release/release.go:1957`](file:///home/tay/projects/gh-pt/release/release.go#L1957) | Matches `avx`/`avx2`/`avx512` strings in asset filenames. Compilers handle CPU flags via source scripts. |
| `KeepSuffixes` | `-k`, `--keep-suffixes` | [`params/params.go:152`](file:///home/tay/projects/gh-pt/params/params.go#L152) | [`cmd/root.go:694`](file:///home/tay/projects/gh-pt/cmd/root.go#L694), [`release/release.go:624`](file:///home/tay/projects/gh-pt/release/release.go#L624) | Retains OS/arch suffixes on extracted release binaries. Source build binaries have native names. |
| `VerifyChecksum` | `--verify-checksum` | [`params/params.go:167`](file:///home/tay/projects/gh-pt/params/params.go#L167) | [`release/release.go:1613, 2177, 2301`](file:///home/tay/projects/gh-pt/release/release.go#L1613) | Downloads checksum manifests from release. Git commit hashes provide integrity in source. |
| `InsecureAllowUnsigned` | `--insecure-allow-unsigned`, `--skip-checksums` | [`params/params.go:168`](file:///home/tay/projects/gh-pt/params/params.go#L168) | [`release/release.go:1633`](file:///home/tay/projects/gh-pt/release/release.go#L1633) | Bypasses release asset checksum verification. |
| `SkipVtSandbox` | `--skip-vt-sandbox` | [`params/params.go:169`](file:///home/tay/projects/gh-pt/params/params.go#L169) | [`cmd/root.go:628`](file:///home/tay/projects/gh-pt/cmd/root.go#L628), [`release/release.go:2184`](file:///home/tay/projects/gh-pt/release/release.go#L2184) | Skips VirusTotal upload of downloaded release binary. Source builds never upload to VT. |
| `Wine` | `--wine` | [`params/params.go:231`](file:///home/tay/projects/gh-pt/params/params.go#L231) | [`cmd/root.go:825`](file:///home/tay/projects/gh-pt/cmd/root.go#L825), [`selector/interface.go:260`](file:///home/tay/projects/gh-pt/selector/interface.go#L260) | Enables `.exe` asset matching under Wine for Linux. Source builds compile natively or cross-compile. |
| `AllowForeignArch` | `--allow-foreign-arch` | [`params/params.go:232`](file:///home/tay/projects/gh-pt/params/params.go#L232) | [`release/release.go:1964`](file:///home/tay/projects/gh-pt/release/release.go#L1964), [`selector/interface.go:272`](file:///home/tay/projects/gh-pt/selector/interface.go#L272) | Matches foreign architecture release assets. Source uses `--target-os`/`--target-arch`. |
| `Extractor` | `--extractor` | [`params/params.go:237`](file:///home/tay/projects/gh-pt/params/params.go#L237) | [`cmd/root.go:691`](file:///home/tay/projects/gh-pt/cmd/root.go#L691), [`release/release.go:2425`](file:///home/tay/projects/gh-pt/release/release.go#L2425) | Precedence of archive extractors (`ouch`, `native`, `internal`). Source builds don't extract archives. |
| `NoCompletionSetup`| `--no-completion-setup` | [`params/params.go:245`](file:///home/tay/projects/gh-pt/params/params.go#L245) | [`release/release.go:783`](file:///home/tay/projects/gh-pt/release/release.go#L783), [`release/symlink.go:360`](file:///home/tay/projects/gh-pt/release/symlink.go#L360) | Skips scanning release archives for completion scripts. Source builds never install completions. |
| `Gearlever` | `--gearlever` | [`params/params.go:246`](file:///home/tay/projects/gh-pt/params/params.go#L246) | [`cmd/root.go:560`](file:///home/tay/projects/gh-pt/cmd/root.go#L560), [`release/release.go:710, 2666`](file:///home/tay/projects/gh-pt/release/release.go#L710) | Delegates AppImage execution to Gear Lever. |
| `Extract` | `-b`, `--extract` | [`params/params.go:247`](file:///home/tay/projects/gh-pt/params/params.go#L247) | [`cmd/root.go:836`](file:///home/tay/projects/gh-pt/cmd/root.go#L836), [`release/release.go:2378`](file:///home/tay/projects/gh-pt/release/release.go#L2378) | Whitelist of binary names inside release archive to extract. |
| `ExtractRegex` | `-B`, `--extract-regex` | [`params/params.go:248`](file:///home/tay/projects/gh-pt/params/params.go#L248) | [`cmd/root.go:820`](file:///home/tay/projects/gh-pt/cmd/root.go#L820), [`selector/interface.go:340`](file:///home/tay/projects/gh-pt/selector/interface.go#L340) | Regex matching binary names inside release archive. |
| `ProgressBar` | `--progress-bar` | [`params/params.go:255`](file:///home/tay/projects/gh-pt/params/params.go#L255) | [`release/release.go:1797-1816`](file:///home/tay/projects/gh-pt/release/release.go#L1797-L1816) | Controls Pacman/Conveyor/Spinner UI for download progress. Not wired into source builds. |

---

### 2. `source`-Only Switches
These switches exclusively apply to source compilation workflows and **should not be exposed on `gh-pt install`**:

| Flag Name | CLI Switch | Definition Site | Implementation Call Sites | Reason `source`-Only |
| :--- | :--- | :--- | :--- | :--- |
| `AICmd` | `--ai-cmd` | [`params/params.go:500`](file:///home/tay/projects/gh-pt/params/params.go#L500) | [`cmd/router.go:213-224`](file:///home/tay/projects/gh-pt/cmd/router.go#L213-L224), [`cmd/root.go:1902`](file:///home/tay/projects/gh-pt/cmd/root.go#L1902) | Template command for invoking AI generator for `body.sh`/`manifest.json`. Only on `SourceCmd`. |
| `TargetOS` | `--target-os` | [`params/params.go:501`](file:///home/tay/projects/gh-pt/params/params.go#L501) | [`cmd/router.go:210`](file:///home/tay/projects/gh-pt/cmd/router.go#L210), [`compile/container.go:21, 90, 223`](file:///home/tay/projects/gh-pt/compile/container.go#L21) | Target OS for cross-compiling in container. Only on `SourceCmd`. |
| `TargetArch` | `--target-arch` | [`params/params.go:502`](file:///home/tay/projects/gh-pt/params/params.go#L502) | [`cmd/router.go:211`](file:///home/tay/projects/gh-pt/cmd/router.go#L211), [`compile/container.go:22, 91, 224`](file:///home/tay/projects/gh-pt/compile/container.go#L22) | Target CPU architecture for cross-compiling in container. Only on `SourceCmd`. |
| `NoCompileContainer` | `--no-compile-container` | [`params/params.go:235`](file:///home/tay/projects/gh-pt/params/params.go#L235) | [`cmd/root.go:2055`](file:///home/tay/projects/gh-pt/cmd/root.go#L2055), [`cmd/update.go:537`](file:///home/tay/projects/gh-pt/cmd/update.go#L537) | **Misplaced on CommonInstallFlags!** Opts out of container isolation for Stage 2 compilation. Release assets are never compiled in containers. |
| `NoAIContainer` | `--no-ai-container` | [`params/params.go:236`](file:///home/tay/projects/gh-pt/params/params.go#L236) | [`cmd/root.go:1937, 2056`](file:///home/tay/projects/gh-pt/cmd/root.go#L1937) | **Misplaced on CommonInstallFlags!** Opts out of container isolation for Stage 1 AI generation. Release asset installations do not generate build scripts. |

---

### 3. Shared Switches (`install` AND `source`)
These switches logically apply to both release asset installation and source builds. The table evaluates whether their implementation is currently **Unified** or **Split / Divergent**:

| Flag Name | CLI Switch | Definition Site | Implementation in `install` | Implementation in `source` | Implementation Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `TargetPath` | `-p`, `--target-path` | [`params/params.go:149`](file:///home/tay/projects/gh-pt/params/params.go#L149) | Sets binary install/symlink destination ([`release/symlink.go:271`](file:///home/tay/projects/gh-pt/release/symlink.go#L271)). | Sets symlink target directory ([`cmd/root.go:1777-1780, 2194`](file:///home/tay/projects/gh-pt/cmd/root.go#L1777-L1780)). | **Unified** |
| `Global` | `-g`, `--global` | [`params/params.go:150`](file:///home/tay/projects/gh-pt/params/params.go#L150) | Defaults target to `/usr/local/bin` ([`cmd/root.go:713`](file:///home/tay/projects/gh-pt/cmd/root.go#L713)). | Defaults target to `/usr/local/bin` and staging to `/usr/local/.ghpt` ([`cmd/root.go:1872`](file:///home/tay/projects/gh-pt/cmd/root.go#L1872)). | **Unified** |
| `Overwrite` | `-f`, `--force` | [`params/params.go:154`](file:///home/tay/projects/gh-pt/params/params.go#L154) | Removes existing app before reinstall ([`cmd/root.go:787`](file:///home/tay/projects/gh-pt/cmd/root.go#L787)), overwrites files ([`release/symlink.go:326`](file:///home/tay/projects/gh-pt/release/symlink.go#L326)). | Removes existing app ([`cmd/root.go:787`](file:///home/tay/projects/gh-pt/cmd/root.go#L787)), deletes existing clone directory ([`cmd/root.go:1839`](file:///home/tay/projects/gh-pt/cmd/root.go#L1839)). | **Unified** |
| `Symlink` | `--symlink` | [`params/params.go:155`](file:///home/tay/projects/gh-pt/params/params.go#L155) | Extracts to ADR-004 packages dir and creates atomic symlinks with copy fallback ([`release/symlink.go:248, 251-385`](file:///home/tay/projects/gh-pt/release/symlink.go#L248)). | Uses legacy `~/src/apps` dir and non-atomic `createSymlinks` without fallback ([`cmd/root.go:1802, 2617`](file:///home/tay/projects/gh-pt/cmd/root.go#L1802)). | **Split / Divergent** |
| `Rename` | `-t`, `--rename` | [`params/params.go:151`](file:///home/tay/projects/gh-pt/params/params.go#L151) | Renames installed binaries via `resolveDestinationPath` ([`release/release.go:620`](file:///home/tay/projects/gh-pt/release/release.go#L620)). | **Completely ignored** by `createSymlinks` ([`cmd/root.go:2617`](file:///home/tay/projects/gh-pt/cmd/root.go#L2617)). | **Split / Missing in Source** |
| `ResolveDeps` | `-y`, `--resolve-deps`| [`params/params.go:160`](file:///home/tay/projects/gh-pt/params/params.go#L160) | Auto-resolves runtime dependencies ([`release/release.go:995`](file:///home/tay/projects/gh-pt/release/release.go#L995)). | Auto-resolves build dependencies ([`cmd/root.go:2028`](file:///home/tay/projects/gh-pt/cmd/root.go#L2028)). | **Unified** |
| `PromptDeps` | `--prompt-deps` | [`params/params.go:161`](file:///home/tay/projects/gh-pt/params/params.go#L161) | Prompts before installing dependencies ([`release/release.go:1024`](file:///home/tay/projects/gh-pt/release/release.go#L1024)). | Prompts before installing dependencies ([`cmd/root.go:1657`](file:///home/tay/projects/gh-pt/cmd/root.go#L1657)). | **Unified** |
| `NoDeps` | `-n`, `--no-deps` | [`params/params.go:162`](file:///home/tay/projects/gh-pt/params/params.go#L162) | Skips dependency resolution ([`cmd/root.go:1635`](file:///home/tay/projects/gh-pt/cmd/root.go#L1635)). | Skips dependency resolution ([`cmd/root.go:1635`](file:///home/tay/projects/gh-pt/cmd/root.go#L1635)). | **Unified** |
| `Driver` | `--driver` | [`params/params.go:182`](file:///home/tay/projects/gh-pt/params/params.go#L182) | Deploys driver manifests in release package store ([`release/symlink.go:374`](file:///home/tay/projects/gh-pt/release/symlink.go#L374)). | Deploys driver manifests in source app store via `r.deploySourceDriversAndPlugins` ([`cmd/root.go:2203`](file:///home/tay/projects/gh-pt/cmd/root.go#L2203)). | **Unified** (shares `DeployDriverAndPluginManifests`) |
| `DriverMap` | `--driver-map` | [`params/params.go:183`](file:///home/tay/projects/gh-pt/params/params.go#L183) | Maps regex to driver subsystems ([`release/driver.go:810`](file:///home/tay/projects/gh-pt/release/driver.go#L810)). | Maps regex to driver subsystems ([`cmd/root.go:2203`](file:///home/tay/projects/gh-pt/cmd/root.go#L2203)). | **Unified** |
| `Plugin` | `--plugin` | [`params/params.go:184`](file:///home/tay/projects/gh-pt/params/params.go#L184) | Deploys plugin manifests in release package store ([`release/symlink.go:374`](file:///home/tay/projects/gh-pt/release/symlink.go#L374)). | Deploys plugin manifests in source app store ([`cmd/root.go:2203`](file:///home/tay/projects/gh-pt/cmd/root.go#L2203)). | **Unified** |
| `IncludeSidecars` | `-s`, `--include-sidecars` | [`params/params.go:180`](file:///home/tay/projects/gh-pt/params/params.go#L180) | Deploys companion sidecar assets ([`release/symlink.go:367`](file:///home/tay/projects/gh-pt/release/symlink.go#L367)). | Used in `moveDistWithSidecarDetection` ([`cmd/root.go:1526`](file:///home/tay/projects/gh-pt/cmd/root.go#L1526)). | **Split / Divergent** (Release respects `SidecarMode`, source dumps to `~/.local/share/gh-pt/sidecars`) |
| `SidecarMode` | `--sidecar-mode` | [`params/params.go:181`](file:///home/tay/projects/gh-pt/params/params.go#L181) | Directs sidecar placement (`auto`, `local-map`, `xdg_data_home`, `custom-path`) ([`release/symlink.go:424`](file:///home/tay/projects/gh-pt/release/symlink.go#L424)). | **Ignored in execution!** Saved to state but hardcoded to `xdg.DataHome/gh-pt/sidecars` ([`cmd/root.go:1545`](file:///home/tay/projects/gh-pt/cmd/root.go#L1545)). | **Split / Divergent** |
| `WarnUnmappedAssets`| `--warn-unmapped-assets` | [`params/params.go:186`](file:///home/tay/projects/gh-pt/params/params.go#L186) | Warns on unmapped files ([`release/release.go:2531`](file:///home/tay/projects/gh-pt/release/release.go#L2531)). | Warns on unmapped files ([`cmd/root.go:1535`](file:///home/tay/projects/gh-pt/cmd/root.go#L1535)). | **Unified** |
| `Interactive` | `-i`, `--interactive` | [`params/params.go:228`](file:///home/tay/projects/gh-pt/params/params.go#L228) | Prompts for asset selection, confirmation. | Selects interactive AI command (`cfg.AI.AIInteractiveCmd`) ([`cmd/router.go:217`](file:///home/tay/projects/gh-pt/cmd/router.go#L217)), multiselects sidecars ([`cmd/root.go:1528`](file:///home/tay/projects/gh-pt/cmd/root.go#L1528)). | **Unified** |
| `DisablePrompts` | `-D`, `--disable-prompts` | [`params/params.go:229`](file:///home/tay/projects/gh-pt/params/params.go#L229) | Suppresses interactive menus. | Suppresses confirmation prompts ([`cmd/root.go:744, 756, 768`](file:///home/tay/projects/gh-pt/cmd/root.go#L744)). | **Unified** |
| `NoSaveState` | `--no-save-state` | [`params/params.go:230`](file:///home/tay/projects/gh-pt/params/params.go#L230) | Suppresses writing to `state.json` ([`release/release.go:2859`](file:///home/tay/projects/gh-pt/release/release.go#L2859)). | Suppresses writing to `state.json` ([`cmd/root.go:2035, 2233`](file:///home/tay/projects/gh-pt/cmd/root.go#L2035)). | **Unified** |
| `ForceRoot` | `--force-root` | [`params/params.go:234`](file:///home/tay/projects/gh-pt/params/params.go#L234) | Permits execution as root ([`cmd/root.go:444, 473`](file:///home/tay/projects/gh-pt/cmd/root.go#L444)). | Permits execution as root ([`cmd/root.go:444, 473`](file:///home/tay/projects/gh-pt/cmd/root.go#L444)). | **Unified** |
| `PinInstall` | `--pin-install` | [`params/params.go:243`](file:///home/tay/projects/gh-pt/params/params.go#L243) | Pins version in `state.json` ([`release/release.go:2866`](file:///home/tay/projects/gh-pt/release/release.go#L2866)). | Pins version/commit in `state.json` ([`cmd/root.go:2260`](file:///home/tay/projects/gh-pt/cmd/root.go#L2260)). | **Unified** |
| `DryRun` | `--dry-run` | [`params/params.go:244`](file:///home/tay/projects/gh-pt/params/params.go#L244) | Simulates download and symlinking ([`release/release.go:2104`](file:///home/tay/projects/gh-pt/release/release.go#L2104), [`release/symlink.go:269`](file:///home/tay/projects/gh-pt/release/symlink.go#L269)). | Simulates git clone, build script generation, and symlink targets ([`cmd/root.go:1816-1822`](file:///home/tay/projects/gh-pt/cmd/root.go#L1816-L1822)). | **Unified** |
| `NoColor` | `--no-color` | [`params/params.go:256`](file:///home/tay/projects/gh-pt/params/params.go#L256) | Disables ANSI coloring in output ([`cmd/router.go:17`](file:///home/tay/projects/gh-pt/cmd/router.go#L17)). | Disables ANSI coloring in output ([`cmd/router.go:212`](file:///home/tay/projects/gh-pt/cmd/router.go#L212)). | **Unified** |
| `NoEmojis` | `--no-emojis` | [`params/params.go:257`](file:///home/tay/projects/gh-pt/params/params.go#L257) | Disables emojis and status icons ([`cmd/router.go:44`](file:///home/tay/projects/gh-pt/cmd/router.go#L44)). | Disables emojis and status icons ([`cmd/root.go:2283`](file:///home/tay/projects/gh-pt/cmd/root.go#L2283)). | **Unified** |

---

## Part C: Actionable Consolidation & Cleanup Plan

### 1. Prune Dead Code & Redundant Aliases
1. **Remove Unwired `--env-inject`**:
   - Delete `EnvInject` from `SidecarFlags` in [`params/params.go:185`](file:///home/tay/projects/gh-pt/params/params.go#L185).
   - Prune dead replay logic in [`cmd/update.go:210`](file:///home/tay/projects/gh-pt/cmd/update.go#L210).
2. **Consolidate Downgrade Flags**:
   - Keep only `--allow-downgrade`.
   - Deprecate/remove joke flags: `--self-inflicted-technical-debt`, `--LE-RETROGROUCH`, `--retrograde-stopgap`, and `--BARBAROUS` from [`params/params.go:239-242`](file:///home/tay/projects/gh-pt/params/params.go#L239-L242) and simplify evaluation in [`cmd/root.go:539-559`](file:///home/tay/projects/gh-pt/cmd/root.go#L539-L559).
3. **Unify Root Flags**:
   - Merge `--allow-root-user-install` into `--force-root`.
4. **Remove Unreachable Phantom Validation**:
   - Delete `if r.CompileFromSource && !r.AI` check from [`cmd/root.go:419-421`](file:///home/tay/projects/gh-pt/cmd/root.go#L419-L421).
5. **Fix `--target-path-create` Logic**:
   - Fix [`cmd/root.go:488`](file:///home/tay/projects/gh-pt/cmd/root.go#L488) from `if !os.IsNotExist(err)` to `if os.IsNotExist(err)`.

### 2. Refactor Parser Structs to Separate Concerns
Split `CommonInstallFlags` so `InstallCmd` and `SourceCmd` only expose their valid flags:
```go
// SharedFlags contains execution, targeting, output, dependency, and hardware driver switches
type SharedExecutionFlags struct {
    TargetFlags       // TargetPath, Global, Overwrite, Symlink, Rename
    DependencyFlags   // ResolveDeps, PromptDeps, NoDeps
    DriverFlags       // Driver, DriverMap, Plugin, IncludeSidecars, SidecarMode
    RuntimeFlags      // Interactive, DisablePrompts, NoSaveState, ForceRoot, PinInstall, DryRun
    OutputFlags       // NoColor, NoEmojis
}

// ReleaseAssetFlags contains switches valid strictly for GitHub pre-built releases
type ReleaseAssetFlags struct {
    ResolutionFlags   // ReleaseVersion, ReleaseAsset, Format, All, MaxExeInstalls, Prerelease, Stable
    VerificationFlags // VerifyChecksum, InsecureAllowUnsigned, SkipVtSandbox, AI
    AssetSidecarFlags // Sidecars, SidecarSymlinkTo, AISetupSidecars, WarnUnmappedAssets
    ExtractorFlags    // Extractor, Wine, AllowForeignArch, Gearlever, Extract, ExtractRegex
    ProgressBar       // ProgressBar
}

// SourceBuildFlags contains switches valid strictly for source compilation
type SourceBuildFlags struct {
    AICmd              string `help:"Command template for AI execution."`
    TargetOS           string `help:"Target operating system for cross-compilation."`
    TargetArch         string `help:"Target architecture for cross-compilation."`
    NoCompileContainer bool   `name:"no-compile-container" help:"Opt out of compile container sandbox."`
    NoAIContainer      bool   `name:"no-ai-container" help:"Opt out of AI generation container sandbox."`
}
```

### 3. Unify Symlink, Package Storage, and Dist Handoff
1. **Unify App Package Directory**: Migrate `handleCompileFromSource` ([`cmd/root.go:1802`](file:///home/tay/projects/gh-pt/cmd/root.go#L1802)) from `~/src/apps/...` to the ADR-004 canonical path:
   `xdg.DataHome/gh-pt/packages/{ownerID}/{repoID}` (via `r.resolvePackageDir`).
2. **Unify Symlink Engine**:
   - Replace `createSymlinks` in [`cmd/root.go:2617`](file:///home/tay/projects/gh-pt/cmd/root.go#L2617) with the atomic symlinking and copy/hardlink fallback engine from [`release/symlink.go:25-93`](file:///home/tay/projects/gh-pt/release/symlink.go#L25-L93).
   - Ensure `--rename` mappings are respected in source builds.
3. **Consolidate Compile Orchestration**:
   - Remove dead `compile.Execute` in [`compile/compile.go:49`](file:///home/tay/projects/gh-pt/compile/compile.go#L49) and `handleDistHandoff` in [`compile/compile.go:212`](file:///home/tay/projects/gh-pt/compile/compile.go#L212).
   - Extract `handleCompileFromSource` from [`cmd/root.go:1773`](file:///home/tay/projects/gh-pt/cmd/root.go#L1773) into a cohesive, properly tested module in the `compile/` package.
