# TASK SPEC: Implement First-Class Sidecar Asset & Plugin Support in `gh-pt`

## 1. Context & Objective
Certain release targets (e.g., `FQingLars/retransformer`, Neovim, game engines, audio plugins) require non-binary companion assets (such as `.red` shared object plugins, runtime assets, models, configuration files, or data directories) to function alongside the compiled binaries.

Currently, `gh-pt` only extracts matched executables and discards archive sidecars or repository subfolders. When installing from binary releases or compiling via `gh pt source`, companion assets are orphaned or wiped from temporary directories.

You must implement native **Sidecar Asset Support** in `gh-pt` across both release archive extraction and remote repository synchronization, ensuring sidecars are persisted to user-specified or XDG-compliant library paths, recorded in `state.json`, and cleanly updated or purged during `gh pt upgrade` and `gh pt rm`.

---

## 2. Scope & Guardrails (Check Against Current Specs)
1. **Do NOT implement items reserved for V2 (`SPEC-v2.md`):**
   - Do NOT build the two-stage AI compilation JSON manifest parser (`ai/`).
   - Do NOT implement static `ldd` / `.so` dependency resolution.
   - Do NOT rewrite package manager interfaces into `resolver/`.
   - Do NOT deprecate `--add-deps` in this task.
2. **Adhere to Codebase Standards (`AGENTS.md`):**
   - Maintain full cross-platform compatibility (Linux, macOS, Windows).
   - Use atomic `state.json` updates strictly governed by `flock`.
   - Ensure changes function non-interactively when `-D` (`--disable-prompts`) is passed.
   - Run `go test -v ./...` and `make lint` before completion.

---

## 3. Detailed Requirements & Design

### A. CLI Parameters & Flags (`params/params.go`)
Add the following fields to `CommonInstallFlags` (and ensure inheritance across `InstallCmd`, `SourceCmd`, `StateAddCmd`):
* `--sidecars, -s`: String slice (`[]string`). Specifies glob patterns or folder paths to capture as sidecar assets. Example patterns:
  - `red_plugins/**`
  - `plugins/*.red`
  - `assets/*`
* `--sidecar-target-path`: String path (`type:"path"`). Specifies where sidecars should be deployed.
  - If unset, the default must resolve to:
    - Linux/BSD/macOS: `$XDG_DATA_HOME/gh-pt/sidecars/<owner>/<repo>/` (or `~/.local/lib/gh-pt/<owner>/<repo>/` if containing shared objects/libraries).
    - Windows: `%LocalAppData%\gh-pt\sidecars\<owner>\<repo>\`.
* `--env-inject`: Map/slice (`[]string` or `map[string]string`) to associate runtime environment variables pointing to the sidecar directory (e.g., `RED_PLUGIN_DIR`).

### B. Configuration Schema (`config/config.go`)
Extend `PathsConfig` and `CoreConfig` to accommodate sidecar defaults:
```go
type PathsConfig struct {
    InstallPath     string `yaml:"install_path"`
    GlobalPath      string `yaml:"global_path"`
    ClonePath       string `yaml:"clone_path"`
    ForkPath        string `yaml:"fork_path"`
    SidecarPath     string `yaml:"sidecar_path"` // Default base path for sidecars
}
```
If `--sidecar-target-path` is not provided on the CLI, check `cfg.Paths.SidecarPath` before falling back to XDG defaults.

### C. State Tracking Schema (`state/state.go`)

Update the `InstalledApp` struct in `state/state.go` to persist sidecar state:

```go
type InstalledApp struct {
    // ... existing fields ...
    Sidecars           []string `json:"sidecars,omitempty"`             // Patterns used
    SidecarTargetPath  string   `json:"sidecar_target_path,omitempty"`  // Destination path
    InstalledSidecars  []string `json:"installed_sidecars,omitempty"`   // Absolute or relative file paths written
}
```

Ensure that:

* `state.json.lock` is respected during mutation.
* `cmd/state_cmds.go` includes `sidecars` and `sidecar_target_path` in `GetStateFields()`, `ParseStateUpdate()`, and the interactive `StateEdit()` form.

### D. Sidecar Extraction Lifecycle (`release/release.go` & `release/symlink.go`)

#### 1. Archive / Binary Releases:

In `GithubRelease.Install()`:
* When extracting an archive (`.tar.gz`, `.zip`, etc.) via `archiver/v4` or external extractors (`ouch`, `native`):
* Do NOT only extract binaries matching `BinarySelector`.
* If `--sidecars` patterns are specified, evaluate archive file entries against the patterns.
* Copy matched files/directories into `SidecarTargetPath`, preserving relative directory structures.
* Record the written files in `r.InstalledSidecars`.

#### 2. Source Compilation (`cmd/root.go` -> `handleCompileFromSource`):

* Update `buildCompilePrompt()` to explicitly instruct the AI agent to copy or stage files matching the sidecar patterns into `sidecarTargetPath` alongside binary compilation.
* For repositories where release assets don't include sidecars but the git tree does:
* If `--sidecars` is supplied, execute a post-build copy from the cloned repo tree (`repoDir`) directly into `sidecarTargetPath`.

#### 3. Remote Sparse-Checkout Fallback for Binary Releases:

* If the user downloads a standalone pre-compiled binary release (which lacks sidecars), but `--sidecars` patterns are passed:
* Fetch the corresponding sidecar subdirectories directly from the GitHub repository at the matching tag/commit using a shallow sparse-checkout or the GitHub REST tree/contents API.
* Write them to `SidecarTargetPath`.

### E. Lifecycle Operations (`cmd/update.go` & `cmd/state_mgmt.go`)

1. **Upgrade (`cmd/update.go`):**
* Replay `--sidecars` patterns and `--sidecar-target-path` from `app.Sidecars` and `app.SidecarTargetPath`.
* Overwrite or prune deprecated sidecar files using an `rsync`-like clean approach (remove stale files no longer present in the upstream release).

2. **Uninstall / Purge (`cmd/state_mgmt.go` -> `RemoveApp`):**
* If an app has a `SidecarTargetPath` tracked in `state.json`:
* Delete the tracked sidecar files or remove the app's dedicated sidecar folder (e.g. `rm -rf <SidecarTargetPath>`).
* Log deletion clearly to stdout (`Deleted sidecar assets at <path>`).

### F. Orphaned Asset Heuristics & Interactive Prompting - UPDATED
To prevent silent failures while respecting headless automation, implement a `warn_unmapped_assets` boolean in `CoreConfig` (default: `true`). This heuristic evaluates both unselected GitHub release assets and discarded files from local archive extraction.

**1. Detection Logic (Remote & Local)**
*   **Remote Assets (`release/release.go`):** After `AssetSelector` chooses the primary binary, evaluate remaining assets. Filter out source code (`.zip`/`.tar.gz` source bundles), checksums, and foreign OS installers (`.exe`, `.dmg`, `.pkg`, `.msi`, `.apk`). Flag assets containing keywords (`plugin`, `data`, `model`, `asset`) or extensions (`.pak`, `.bin`, `.red`).
*   **Local Archive Extraction:** Before purging the temporary extraction directory, evaluate discarded files. Filter out standard bloat (`README*`, `LICENSE*`, `.md`, `doc/`, `src/`). Flag shared libraries (`.so`, `.dll`, `.dylib`), config templates (`.json`, `.yaml`), or domain-specific plugins.

**2. Execution Routing (Interactive vs. Headless)**
If suspected sidecars are flagged, branch the logic based on the session state:

*   **Interactive Mode (`r.CliParams.Interactive == true` && `!r.CliParams.DisablePrompts`):**
    1. Pause the `PacmanUI`.
    2. Trigger a `pterm.DefaultInteractiveMultiselect` prompt: *"Suspected sidecar assets detected. Select items to deploy:"*
    3. List the flagged remote assets and/or local files as selectable options.
    4. If the user selects items, dynamically append them to `r.CliParams.Sidecars` and process them through the sidecar deployment lifecycle into `SidecarTargetPath`.
    5. Ensure these dynamic selections are saved to `app.Sidecars` in `state.json` so future headless upgrades remember them.
    6. Resume the `PacmanUI`.

*   **Headless Mode (`r.CliParams.DisablePrompts == true` OR non-TTY):**
    1. Do NOT halt execution or prompt for input.
    2. Emit a non-blocking `pterm.Warning` to stdout detailing the suspected items (e.g., `Warning: Release contains unmapped sidecar assets (plugins.zip). Pass --sidecars to capture them on future installs.`). 
    3. Exit with code 0 to allow bash loops to proceed safely.

### G. Explicit Discovery (`gh-pt show`)
To investigate a repository's remote tree for potential sidecars before installing, do not embed recursive remote scanning into the `install` command. Instead, augment `gh-pt show` to perform explicit discovery.

When `gh-pt show <owner>/<repo>` is executed, it should leverage the GitHub REST API (via `go-gh`) to query the git tree recursively for the target branch/tag:
```bash
# Conceptual API equivalent:
gh api "repos/$REPO/git/trees/$BRANCH?recursive=1" --jq '.tree[].path' | grep -E '\.(so|dll|dylib|red|pak|json|yaml|yml)$'
```
`gh-pt show` will present this clean list of potential remote assets to the user under a "Potential Sidecar Assets" header, allowing them to formulate their `--sidecars` flag accurately before executing an install.

---

## 4. Verification & Testing Instructions

Write hermetic unit and integration tests covering:

1. `TestSidecarExtraction_FromArchive`: Test archive extraction with sidecars, verifying both binary placement in `targetPath` and sidecars in `sidecarTargetPath`.
2. `TestState_SidecarPersistence`: Verify JSON marshaling, unmarshaling, and state update capabilities for sidecar fields.
3. `TestRemoveApp_PurgesSidecars`: Verify that running `RemoveApp(target, purge)` cleanly purges both the binary and the sidecar directory from the filesystem.
4. Run `go test -v ./...` and `make lint` across the entire workspace to ensure 100% pass rate.
