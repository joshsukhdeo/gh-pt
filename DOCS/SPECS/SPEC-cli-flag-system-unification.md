# Specification: CLI Flag System & Execution Unification (`cli-flag-system-unification`)

**File**: `SPEC-cli-flag-system-unification.md`  
**Repository**: `gh-pt` (`/home/tay/projects/gh-pt`)  
**Author**: Antigravity Architect / Research Subagent  
**Status**: APPROVED SPECIFICATION  
**Primary Research Basis**: [`DOCS/RESEARCH/RESEARCH-cli-switches-install-vs-source-audit.md`](file:///home/tay/projects/gh-pt/DOCS/RESEARCH/RESEARCH-cli-switches-install-vs-source-audit.md)  

---

## 1. Objective

Refactor the `gh-pt` CLI switch architecture and unify divergent execution engines across `gh-pt install`, `gh-pt source`, and `gh-pt upgrade`/`update` (`sync`).

Specifically:
1. **Kong Flag Taxonomy Refactor**: Break up monolithic `CommonInstallFlags` into modular, domain-correct structs (`SharedInstallSourceFlags`, `ReleaseAssetFlags`, `SourceBuildFlags`, `UpgradeFlags`). Prevent release asset switches from polluting `gh-pt source`, and prevent compilation container switches from polluting `gh-pt install`.
2. **Dead & Redundant Flag Pruning**: Remove dead switches (`--env-inject`), fix flawed inverted logic in `--target-path-create`, eliminate redundant joke flags (`--self-inflicted-technical-debt`, `--LE-RETROGROUCH`, `--retrograde-stopgap`, `--BARBAROUS`), unify overlapping root flags (`--allow-root-user-install` $\rightarrow$ `--force-root`), and purge unreachable phantom `--compile-from-source` validation checks.
3. **Execution Engine Unification**:
   - **ADR-004 Package Store Unification**: Shift source installations from legacy `~/src/apps/{owner}/{repo}` to canonical `$XDG_DATA_HOME/gh-pt/packages/{owner}/{repo}` (`resolvePackageDir`), retaining backward-compatibility fallbacks for existing installations.
   - **Symlink Engine Unification**: Replace fragile `createSymlinks` in `cmd/root.go` with the battle-tested, atomic symlinking, hardlink/copy fallback, `--rename`, and completion registration pipeline from `release/symlink.go`.
   - **Dist Handoff & Sidecar Mode Parity**: Update `moveDistWithSidecarDetection` to honor `--sidecar-mode` (`local-map`, `xdg_data_home`, `custom-path`) instead of hardcoding `~/.local/share/gh-pt/sidecars`. Purge orphaned and duplicate functions in `compile/compile.go` (`compile.Execute`, `handleDistHandoff`).
   - **Upgrade Replay Hygiene**: Ensure `cmd/update.go` cleanly isolates replay parameters between release-backed and source-backed applications, enabling `--no-compile-container` and `--no-ai-container` on `upgrade`/`update`.

---

## 2. Tech Stack & Dependencies

- **Language**: Go 1.27
- **CLI Parsing Framework**: Kong (`github.com/alecthomas/kong`)
- **Package Engine & File Storage**: XDG Base Directory specification (`github.com/adrg/xdg`), ADR-004 canonical package store (`$XDG_DATA_HOME/gh-pt/packages/{owner}/{repo}`)
- **Container Isolation**: `podman` / `docker` via `compile/container.go`
- **State Management**: File-locked state persistence (`state/state.go`, `state.json.lock`, `safety/managed.go`)
- **Observability**: Structured key-value logging via `github.com/charmbracelet/log` and `pterm` terminal UI
- **Testing**: Standard Go `testing`, `testify/assert`, `testify/require`, isolated with `t.TempDir()`, `XDG_DATA_HOME`, and `xdg.Reload()`

---

## 3. Commands

```bash
make build              # Build binary
rtk go test -v ./...    # Run test suite with rtk prefix
make fmt && make lint   # Format and lint code
```

---

## 4. Project Structure & Affected Files

- `params/params.go`:
  - Decompose `CommonInstallFlags` into `SharedInstallSourceFlags`, `ReleaseAssetFlags`, and `SourceBuildFlags`.
  - Rebind `InstallCmd`, `SourceCmd`, and `UpgradeCmd` to embed only their valid flag structs.
  - Remove `--env-inject`, joke downgrade aliases (`--self-inflicted-technical-debt`, `--LE-RETROGROUCH`, `--retrograde-stopgap`, `--BARBAROUS`), `--allow-root-user-install`, and `--ai-setup-sidecars`.
  - Add `NoCompileContainer` and `NoAIContainer` to `UpgradeFlags`.
  - Update `ExecContext` to compose the new sub-structs for backward compatibility across internal handlers.
- `cmd/router.go`:
  - Update command mapping cases for `install`, `source`, and `upgrade` to match refactored struct layouts.
  - Pass `NoCompileContainer` and `NoAIContainer` through to `r.ExecContext` during `upgrade` routing.
- `cmd/root.go`:
  - Fix inverted logic bug in `validateTargetPath` ([`cmd/root.go:488`](file:///home/tay/projects/gh-pt/cmd/root.go#L488)).
  - Remove unreachable phantom check `if r.CompileFromSource && !r.AI` ([`cmd/root.go:419-421`](file:///home/tay/projects/gh-pt/cmd/root.go#L419-L421)).
  - Simplify root permission check to use strictly `r.ForceRoot` ([`cmd/root.go:473`](file:///home/tay/projects/gh-pt/cmd/root.go#L473)).
  - Simplify downgrade evaluation in `RunInstall` ([`cmd/root.go:539-559`](file:///home/tay/projects/gh-pt/cmd/root.go#L539-L559)) to strictly check `r.AllowDowngrade`.
  - In `handleCompileFromSource`:
    - Point `symlinkDir` to canonical ADR-004 `resolvePackageDir(ownerID, repoID)` instead of `~/src/apps/...` ([`cmd/root.go:1802`](file:///home/tay/projects/gh-pt/cmd/root.go#L1802)).
    - Replace `createSymlinks` ([`cmd/root.go:2194, 2617-2665`](file:///home/tay/projects/gh-pt/cmd/root.go#L2194)) with call to unified atomic symlink and rename engine.
    - Update `moveDistWithSidecarDetection` ([`cmd/root.go:1442-1600`](file:///home/tay/projects/gh-pt/cmd/root.go#L1442-L1600)) to respect `r.SidecarMode` and resolved destination paths.
- `cmd/update.go`:
  - In `UpgradeFlags` mapping and `DoUpdate` ([`cmd/update.go:537`](file:///home/tay/projects/gh-pt/cmd/update.go#L537)): wire `r.NoCompileContainer` and `r.NoAIContainer` overrides.
  - In `processSingleUpdate` ([`cmd/update.go:178-226`](file:///home/tay/projects/gh-pt/cmd/update.go#L178-L226)): sanitize replay logic; remove references to `--env-inject`.
- `compile/compile.go`:
  - Delete orphaned, dead functions `compile.Execute` ([`compile/compile.go:49-98`](file:///home/tay/projects/gh-pt/compile/compile.go#L49-L98)) and `handleDistHandoff` ([`compile/compile.go:212-235`](file:///home/tay/projects/gh-pt/compile/compile.go#L212-L235)).
  - Deprecate/align `SaveCompileState` with `state.AddApp`.
- `state/state.go`:
  - Keep `EnvInject []string json:"env_inject,omitempty"` struct tag for deserializing historical `state.json` without errors, but mark it as deprecated.

---

## 5. Code Style & Struct Contracts

### 5.1 Kong Struct Refactoring (`params/params.go`)

To eliminate cross-contamination between `install` and `source`, the monolithic `CommonInstallFlags` is partitioned into five distinct domain structs:

```go
// SharedInstallSourceFlags contains flags common to both pre-built release installation
// and local source compilation.
type SharedInstallSourceFlags struct {
	TargetFlags
	DependencyFlags
	DriverPluginFlags
	SharedRuntimeFlags
	OutputFlags
}

// TargetFlags controls installation directories, collision handling, and symlinking.
type TargetFlags struct {
	TargetPath       string            `default:"${install_path}" short:"p" type:"path" help:"Target installation directory (default: ~/.local/bin or /usr/local/bin if --global)."`
	Global           bool              `short:"g" help:"Install globally (e.g. /usr/local/bin) instead of user bin."`
	Rename           map[string]string `optional:"" short:"t" help:"Rename binaries installed at target path."`
	TargetPathCreate bool              `default:"true" negatable:"" help:"Create target installation directory if it does not exist."`
	Overwrite        bool              `default:"false" short:"f" name:"force" aliases:"overwrite" env:"GH_PT_FORCE" help:"Overwrite target binaries."`
	Symlink          bool              `env:"GH_PT_SYMLINK" help:"Extract or build application in package store and symlink executables."`
}

// DependencyFlags controls automatic and prompted package dependency resolution.
type DependencyFlags struct {
	ResolveDeps bool `short:"y" name:"resolve-deps" help:"Automatically resolve and install dependencies without prompting."`
	PromptDeps  bool `name:"prompt-deps" help:"Prompt before installing dependencies."`
	NoDeps      bool `short:"n" help:"Do not install dependencies."`
}

// DriverPluginFlags controls Linux driver manifests and application plugins.
type DriverPluginFlags struct {
	IncludeSidecars    bool     `name:"include-sidecars" short:"s" negatable:"" env:"GH_PT_INCLUDE_SIDECARS" help:"Include companion sidecar assets (auto-detects placement)."`
	SidecarMode        string   `optional:"" name:"sidecar-mode" env:"GH_PT_SIDECAR_MODE" predictor:"sidecar_modes" predict:"sidecar_modes" help:"Sidecar placement mode: auto, local-map, xdg_data_home, or custom-path:/path/to/."`
	Driver             string   `optional:"" name:"driver" enum:"auto,vulkan,opencl,vaapi,udev,ldso,none" default:"none" help:"Register hardware/userspace driver manifests (Linux only)."`
	DriverMap          string   `optional:"" name:"driver-map" help:"Map asset/file regex patterns to driver subsystems."`
	Plugin             string   `optional:"" name:"plugin" aliases:"plugins" enum:"auto,obs,gimp,vst,lv2,clap,audio,gnome,gnome-extension,gearlever,none" default:"none" help:"Register host application plugin manifests or symlinks (Linux only)."`
	WarnUnmappedAssets bool     `default:"true" negatable:"" help:"Warn about suspected unmapped sidecar assets."`
}

// SharedRuntimeFlags controls non-release-specific execution preferences.
type SharedRuntimeFlags struct {
	Interactive    bool `default:"false" short:"i" help:"Use interactive mode."`
	DisablePrompts bool `short:"D" env:"GH_PT_DISABLE_PROMPTS" help:"Disable all interactive prompts."`
	NoSaveState    bool `env:"GH_PT_NO_SAVE_STATE" help:"Do not save installation to state."`
	ForceRoot      bool `name:"force-root" env:"GH_PT_FORCE_ROOT" help:"Allow operations when running as root (suppresses root restriction)."`
	AllowDowngrade bool `help:"Allow downgrades when updating or installing."`
	PinInstall     bool `name:"pin-install" default:"false" help:"Pin this installation to the current version."`
	DryRun         bool `default:"false" help:"Show what would be downloaded or built."`
}

// ReleaseAssetFlags contains switches that apply STRICTLY to pre-built release assets (gh-pt install).
type ReleaseAssetFlags struct {
	ResolutionFlags
	VerificationFlags
	ReleaseExtractorFlags
	ProgressBar string `name:"progress-bar" env:"GH_PT_PROGRESS_BAR" predictor:"progress_bars" predict:"progress_bars" help:"Progress bar style: pacman, standard, spinner:<style>, or none."`
}

// ResolutionFlags controls GitHub release selection and asset regex matching.
type ResolutionFlags struct {
	ReleaseVersion                                string   `default:"latest" short:"v" help:"Repository release tag (version) to install."`
	ReleaseAsset                                  string   `optional:"" short:"a" help:"Name of repository release asset to download."`
	ReleaseAssetRegexp                            string   `optional:"" short:"A" help:"Regular expression matching release asset to download."`
	ReleaseAssetRegexps                           []string `kong:"-"`
	Type                                          []string `default:"${install_types}" short:"T" name:"format" env:"GH_PT_TYPE" help:"Comma-separated list of types to match and prioritize."`
	All                                           AllFlag  `optional:"" help:"Install all matched assets/archives/extracted binaries."`
	OnlyFirstMatch                                bool     `name:"only-first-match" help:"Only install the first matched binary."`
	MaxExeInstalls                                int      `name:"max-exe-installs" default:"0" help:"Maximum number of executables to install (0 = all unique)."`
	Prerelease                                    bool     `short:"P" help:"Include prereleases."`
	Stable                                        bool     `help:"Include only stable releases."`
	FallbackReleases                              int      `default:"0" help:"Try this many older releases if no assets found in latest (0=disabled, max 3)."`
	SearchForInstallInstructionsIfNoReleaseAssets bool     `env:"GH_PT_README_FALLBACK" help:"Extract alternative installation instructions from README if asset matching fails."`
	AvxLevel                                      string   `default:"auto" enum:"auto,none,avx,avx2,avx512" env:"GH_PT_AVX_LEVEL" help:"AVX instruction set preference."`
}

// VerificationFlags controls hash checks and sandbox scans for release downloads.
type VerificationFlags struct {
	VerifyChecksum        bool `default:"true" help:"Verify asset checksums."`
	InsecureAllowUnsigned bool `name:"insecure-allow-unsigned" aliases:"skip-checksums" env:"GH_PT_INSECURE_ALLOW_UNSIGNED" help:"Allow unsigned release assets without cryptographic verification."`
	SkipVtSandbox         bool `default:"true" name:"skip-vt-sandbox" help:"Skip VirusTotal sandbox scan."`
	AI                    bool `help:"Use AI to scan and analyze releases."`
}

// ReleaseExtractorFlags controls archive unpacking and OS compatibility layers.
type ReleaseExtractorFlags struct {
	Extractor          string   `env:"GH_PT_EXTRACTOR" predictor:"extractors" predict:"extractors" help:"Archive extractor precedence." default:"${extractor}"`
	Wine               string   `default:"off" enum:"force,priority,allow,off" env:"GH_PT_WINE" help:"Wine mode."`
	AllowForeignArch   bool     `env:"GH_PT_ALLOW_FOREIGN_ARCH" help:"Allow installing assets with foreign architectures."`
	KeepSuffixes       bool     `short:"k" help:"Keep OS/hardware suffixes on extracted binaries."`
	NoCompletionSetup  bool     `name:"no-completion-setup" help:"Do not automatically install shell autocompletion scripts."`
	Gearlever          bool     `name:"gearlever" env:"GH_PT_GEARLEVER" help:"Install AppImage via Gear Lever."`
	Extract            []string `optional:"" short:"b" help:"Names of binaries to extract from archive."`
	ExtractRegex       string   `optional:"" short:"B" help:"Regular expression matching binaries in archive."`
	Sidecars           string   `optional:"" name:"sidecars" short:"S" help:"Regex pattern for sidecar assets to capture from archive."`
	SidecarSymlinkTo   []string `optional:"" help:"Create symlinks from sidecars to these directories."`
}

// SourceBuildFlags contains switches that apply STRICTLY to source compilation (gh-pt source).
type SourceBuildFlags struct {
	AICmd              string `help:"Command template for AI execution."`
	TargetOS           string `help:"Target operating system for cross-compilation (e.g., linux, windows, darwin)."`
	TargetArch         string `help:"Target architecture for cross-compilation (e.g., amd64, arm64, arm)."`
	NoCompileContainer bool   `name:"no-compile-container" env:"GH_PT_NO_COMPILE_CONTAINER" help:"Opt out of mandatory container isolation for compilation (UNSAFE)."`
	NoAIContainer      bool   `name:"no-ai-container" env:"GH_PT_NO_AI_CONTAINER" help:"Run AI generation directly on host without container isolation (UNSAFE)."`
}
```

### 5.2 Subcommand Bindings (`params/params.go`)

Commands bind only the specific structs they support:

```go
// InstallCmd installs release assets or clones repositories.
type InstallCmd struct {
	Repository string `arg:"" env:"GH_PT_REPOSITORY" optional:"" predictor:"github_repos" predict:"github_repos" help:"Github repository in OWNER/REPOSITORY_NAME format."`
	SharedInstallSourceFlags
	ReleaseAssetFlags
}

// SourceCmd compiles a repository from source using AI and container sandboxing.
type SourceCmd struct {
	Repository string `arg:"" env:"GH_PT_REPOSITORY" predictor:"github_repos" predict:"github_repos" help:"Github repository in OWNER/REPOSITORY_NAME format."`
	SharedInstallSourceFlags
	SourceBuildFlags
}

// UpgradeFlags governs batch and single-package upgrades for both release and source installs.
type UpgradeFlags struct {
	DisablePrompts        bool   `short:"D" env:"GH_PT_DISABLE_PROMPTS" help:"Disable all interactive prompts."`
	Extractor             string `env:"GH_PT_EXTRACTOR" predictor:"extractors" predict:"extractors" help:"Archive extractor precedence." default:"${extractor}"`
	Overwrite             bool   `default:"false" short:"f" name:"force" env:"GH_PT_FORCE" help:"Reinstall packages overwriting existing files."`
	DryRun                bool   `default:"false" help:"Show what would be upgraded."`
	VerifyChecksum        bool   `default:"true" help:"Verify asset checksums."`
	InsecureAllowUnsigned bool   `name:"insecure-allow-unsigned" aliases:"skip-checksums" env:"GH_PT_INSECURE_ALLOW_UNSIGNED" help:"Allow unsigned release assets."`
	SkipVtSandbox         bool   `default:"true" name:"skip-vt-sandbox" help:"Skip VirusTotal sandbox scan."`
	FallbackReleases      int    `default:"0" help:"Try older releases if no assets in latest."`
	WarnUnmappedAssets    bool   `default:"true" negatable:"" help:"Warn about suspected unmapped sidecar assets."`
	ProgressBar           string `name:"progress-bar" env:"GH_PT_PROGRESS_BAR" predictor:"progress_bars" predict:"progress_bars" help:"Progress bar style."`
	NoColor               bool   `name:"no-color" aliases:"no-colors" env:"GH_PT_NO_COLOR" help:"Disable color output."`
	NoEmojis              bool   `name:"no-emojis" aliases:"no-emoji" env:"GH_PT_NO_EMOJIS" help:"Disable emoji/icon output."`
	// Source-build overrides:
	NoCompileContainer    bool   `name:"no-compile-container" env:"GH_PT_NO_COMPILE_CONTAINER" help:"Opt out of container isolation during source upgrades."`
	NoAIContainer         bool   `name:"no-ai-container" env:"GH_PT_NO_AI_CONTAINER" help:"Opt out of AI container isolation during source repairs."`
}

type UpgradeCmd struct {
	Repository string `arg:"" optional:"" predictor:"installed_apps" predict:"installed_apps" help:"Optional repository to update."`
	User       bool   `short:"u" name:"user" help:"Update only user installations."`
	Global     bool   `short:"g" name:"global" help:"Update only global installations."`
	UpgradeFlags
}
```

### 5.3 Backward Compatibility in `ExecContext`

To avoid breaking existing internal APIs that accept `*params.ExecContext`, `ExecContext` composes the refactored structs:

```go
type ExecContext struct {
	UpdateAll bool
	Update    bool
	SharedInstallSourceFlags
	ReleaseAssetFlags
	SourceBuildFlags
	Repository          string
	Clone               bool
	Fork                bool
	MaxDepth            int
	CompileFromSource   bool
	AI                  bool
	Track               string
	LogLevel            string
	LogFormat           string
	LogQuietInteractive bool
	Verbose             bool
	VTApiKey            string
	Ls                  string
	Ll                  string
	Full                bool
	ListFormat          string
	EditSavedState      bool
	RmSavedState        string
	Rm                  string
	Purge               string
	Pin                 string
	Show                bool
	ShowAssets          int
	ShowVersions        int
	ShowDescription     int
	ShowReadme          int
	DiscoverSidecars    bool
	DisableIcons        bool
	HookEvent           string
	HookScriptPath      string
	InstalledSidecars   []string `kong:"-"`
	InstalledBinaries   []string `kong:"-"`
	ResolvedSymlinkDir  string   `kong:"-"`
}
```

---

## 6. Execution Engine Unification

### 6.1 Package Store Directory Unification (ADR-004)

`cmd/root.go:handleCompileFromSource` previously hardcoded:
```go
// Legacy:
symlinkDir = filepath.Join(homeDir, "src", "apps", ownerID, repoID)
```
This is migrated to use the canonical package directory resolver:
```go
// Unified:
rel := release.MakeGithubRelease(&r.ExecContext, nil)
symlinkDir = rel.ResolvePackageDir(ownerID, repoID)
```
When `resolvePackageDir` runs, it:
1. Checks if `app.SymlinkDir` already exists in `state.json` (preserving legacy `~/src/apps` paths for existing apps).
2. Otherwise, defaults to `$XDG_DATA_HOME/gh-pt/packages/{ownerID}/{repoID}`.

### 6.2 Symlink & Rename Engine Unification

Replace the rudimentary `createSymlinks` in `cmd/root.go:2617-2665` with the atomic symlinking engine:

```go
// Unified symlink deployment for source builds:
func (r *RootCLI) createUnifiedSymlinks(symlinkDir, targetPath string) error {
	rel := release.MakeGithubRelease(&r.ExecContext, nil)
	return rel.DeployBinariesFromStaging(symlinkDir, targetPath)
}
```
`DeployBinariesFromStaging`:
1. Scans `symlinkDir`, plus whitelisted executable subdirectories (`service`, `preferences`).
2. Checks file executable permissions (`mode & 0111 != 0`).
3. Applies `--rename` mapping (`r.Rename[name]`).
4. Invokes `createSymlinkAtomic` (atomic symlink creation via temporary file rename, falling back to hardlink or copy).
5. Scans for shell completion scripts and registers them automatically unless `--no-completion-setup` is active.

### 6.3 Sidecar Mode & Dist Handoff Unification

In `cmd/root.go:1442` (`moveDistWithSidecarDetection`):
1. **Remove hardcoded sidecar destination**: Replace `filepath.Join(xdg.DataHome, "gh-pt", "sidecars")` with `rel.ResolveSidecarSymlinkDest(r.SidecarMode)`.
2. **Honor SidecarMode**:
   - `local-map`: Map standard directories (`share`, `lib`, `etc`, `include`) into `--target-path` prefix equivalents (`~/.local` or `/usr/local`).
   - `xdg_data_home`: Symlink to `$XDG_DATA_HOME/gh-pt/sidecars`.
   - `custom-path:/path/to`: Symlink to the specified custom directory.
3. **Purge dead compile handlers**: Remove uncalled `compile.Execute` and `handleDistHandoff` in `compile/compile.go`.

### 6.4 Upgrade Replay Hygiene (`cmd/update.go`)

In `cmd/update.go:processSingleUpdate`:
- Release-backed applications: Replay only `ReleaseAssetFlags` and `SharedInstallSourceFlags`.
- Source-backed applications (`app.CompileScript != ""`):
  - Replay `app.TargetPath`, `app.Global`, `app.SymlinkDir`, `app.Driver`, `app.Plugin`, `app.DriverMap`, `app.IncludeSidecars`, `app.SidecarMode`, `app.Rename`.
  - Apply CLI overrides for `NoCompileContainer` and `NoAIContainer` from `r.NoCompileContainer` and `r.NoAIContainer`.
  - Discard invalid release asset parameters (`ReleaseAsset`, `ReleaseRegexp`, `Type`, `Extractor`).

---

## 7. Testing Strategy

### 7.1 Parser & Subcommand Scope Tests (`params/cli_integration_test.go`)
1. **`TestKong_InstallCmd_RejectsSourceFlags`**:
   - Verify `gh-pt install cli/cli --no-compile-container` returns unknown flag error.
   - Verify `gh-pt install cli/cli --target-os=linux` returns unknown flag error.
2. **`TestKong_SourceCmd_RejectsReleaseAssetFlags`**:
   - Verify `gh-pt source cli/cli --verify-checksum` returns unknown flag error.
   - Verify `gh-pt source cli/cli --skip-vt-sandbox` returns unknown flag error.
   - Verify `gh-pt source cli/cli --format=tar.gz` returns unknown flag error.
   - Verify `gh-pt source cli/cli --extractor=ouch` returns unknown flag error.
3. **`TestKong_UpgradeCmd_AcceptsContainerFlags`**:
   - Verify `gh-pt upgrade --no-compile-container` successfully parses `NoCompileContainer = true`.
   - Verify `gh-pt upgrade --no-ai-container` successfully parses `NoAIContainer = true`.
4. **`TestKong_PrunedFlags_FailGracefully`**:
   - Verify `--env-inject` returns unknown flag error.
   - Verify `--self-inflicted-technical-debt` returns unknown flag error.
   - Verify `--BARBAROUS` returns unknown flag error.

### 7.2 Execution Unification Tests (`cmd/root_test.go` & `cmd/update_test.go`)
1. **`TestTargetPathCreate_WhenDirMissing`**:
   - Verify `validateTargetPath` creates the directory when `TargetPathCreate = true` and directory does not exist.
2. **`TestCompileFromSource_UsesADR004PackagePath`**:
   - Run source compilation mock in isolated `XDG_DATA_HOME`.
   - Verify artifacts stage in `$XDG_DATA_HOME/gh-pt/packages/owner/repo` instead of `~/src/apps`.
3. **`TestCompileFromSource_AppliesRenameRules`**:
   - Execute source build with `-t original_name=custom_name`.
   - Verify symlink in `TargetPath` is created as `custom_name` pointing to `original_name`.
4. **`TestCompileFromSource_RespectsSidecarMode`**:
   - Execute source build with `--include-sidecars --sidecar-mode=local-map`.
   - Verify standard `share/` or `lib/` files in dist are linked to `~/.local/share` or `~/.local/lib`.
5. **`TestSourceUpgrade_HonorsNoCompileContainerFlag`**:
   - Execute `DoUpdate` with `NoCompileContainer = true`.
   - Verify container execution is bypassed and build executes via host shell.

---

## 8. Boundaries & Non-Goals

### In Scope
- Refactoring `params/params.go` struct hierarchy for `InstallCmd`, `SourceCmd`, and `UpgradeCmd`.
- Pruning dead, broken, and redundant CLI switches.
- Fixing `--target-path-create` inverted logic bug.
- Unifying package storage directories under ADR-004.
- Unifying symlink, rename, and completion generation in source builds.
- Unifying sidecar mode routing in source builds.
- Wiring container flags into `UpgradeCmd`.

### Out of Scope
- Rewriting the AI prompt generator (`ai/prompt.go`).
- Modifying how container environments are constructed inside `compile/container.go`.
- Changing the schema format of `manifest.json`.

---

## 9. Success Criteria

1. **Clean Command Help Output**:
   - `gh-pt install --help` displays zero compilation or container flags.
   - `gh-pt source --help` displays zero release asset, checksum, VT, or extractor flags.
   - `gh-pt upgrade --help` displays `--no-compile-container` and `--no-ai-container`.
2. **Zero Regressions in Test Suite**:
   - All tests in `params/`, `cmd/`, `release/`, `compile/`, `state/`, and `selector/` pass with zero failures.
3. **Strict Invariant Adherence**:
   - Zero state-lock bypasses.
   - Complete isolation under temporary XDG test environments.
