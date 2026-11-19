# Spec: GNOME Shell Extension Release Plugin (`gnome-extension-release` — Method A)

## Objective

Enable `gh-pt install`, `gh-pt show`, `gh-pt upgrade`, `gh-pt rm`, and `gh-pt rollback` to natively support GNOME Shell extension release bundles (such as `GSConnect/gnome-shell-extension-gsconnect`'s `gsconnect@andyholmes.github.io.zip`) without workarounds.

When a user runs:
```bash
gh-pt install GSConnect/gnome-shell-extension-gsconnect --plugin=gnome-extension -y
```
`gh-pt` must:
1. Classify `gsconnect@andyholmes.github.io.zip` as a native Linux installable (`OSLinux`, `SubcatNative`, `CategoryDefaultInstall`).
2. Extract the archive into `$XDG_DATA_HOME/gh-pt/packages/GSConnect/gnome-shell-extension-gsconnect` (enabled automatically via `--plugin=gnome-extension`).
3. Detect executables in root and `service/` (e.g. `gsconnect`, `gsconnect-preferences`, `service/daemon.js`) and symlink them into `~/.local/bin` (or `/usr/local/bin` with `-g`), plus alias `daemon.js` / `gsconnect-daemon`.
4. Detect `metadata.json` (containing `"uuid"` and `"shell-version"`), compile `schemas/*.gschema.xml` via `glib-compile-schemas` if `schemas/gschemas.compiled` is missing, and symlink the extension directory into `$XDG_DATA_HOME/gnome-shell/extensions/<uuid>` (or `/usr/share/gnome-shell/extensions/<uuid>` when `--global` is set).

## Tech Stack

- **Language**: Go 1.27
- **CLI Parser**: `github.com/alecthomas/kong` (`params/params.go`)
- **XDG Paths**: `github.com/adrg/xdg` (`release/driver.go`, `release/symlink.go`)
- **Archive Extraction**: `ouch` / `unzip` / `github.com/mholt/archiver/v4` (`selector/interface.go`)

## Commands

```bash
make build
rtk go test -v ./selector/... ./release/... ./params/...
make fmt && make lint
```

## Project Structure

```
params/params.go           → Add gnome,gnome-extension to SidecarFlags.Plugin enum
selector/classifier.go     → Detect *@*.zip and *.shell-extension.zip as OSLinux / SubcatNative
selector/interface.go      → Allow service/ and preferences/ in IsAllowedBinaryDir
release/driver.go          → Add PluginGnome / detectGnomeExtensionTargets + glib-compile-schemas PostHook
release/driver_test.go     → Unit tests for GNOME extension detection, schema compilation, and symlinking
selector/classifier_test.go→ Unit tests for GSConnect asset classification
```

## Code Style

Follow existing `release/driver.go` target detection idioms:

```go
func getGnomeExtensionDir(global bool) string {
	if global {
		return sysPath("usr", "share", "gnome-shell", "extensions")
	}
	return filepath.Join(xdg.DataHome, "gnome-shell", "extensions")
}

func detectGnomeExtensionTargets(owner, repo, pkgDir string, global bool) ([]DriverManifestTarget, error) {
	var targets []DriverManifestTarget
	metaFiles := findFilesByPattern(pkgDir, func(p string, info os.FileInfo) bool {
		return !info.IsDir() && filepath.Base(p) == "metadata.json"
	})
	for _, mf := range metaFiles {
		data, err := os.ReadFile(mf)
		if err != nil {
			continue
		}
		var meta struct {
			UUID         string          `json:"uuid"`
			ShellVersion json.RawMessage `json:"shell-version"`
		}
		if json.Unmarshal(data, &meta) != nil || meta.UUID == "" || len(meta.ShellVersion) == 0 {
			continue
		}
		extRoot := filepath.Dir(mf)
		destPath := filepath.Join(getGnomeExtensionDir(global), meta.UUID)
		targets = append(targets, DriverManifestTarget{
			Subsystem:    PluginGnome,
			SourceFile:   extRoot,
			DestPath:     destPath,
			IsSymlink:    true,
			RequiresSudo: global,
			PostHook:     "glib-compile-schemas:" + filepath.Join(extRoot, "schemas"),
		})
	}
	return targets, nil
}
```

## Testing Strategy

### Seam 1 (`selector.AssetClassifier.ClassifyReleaseWithContext`)
- **What it catches**: Ensures `gsconnect@andyholmes.github.io.zip` is categorized as `OSLinux`, `SubcatNative`, `CategoryDefaultInstall` instead of `OSUnknown` / `CategoryCannotInstall`.
- **What it misses**: Archive extraction and filesystem symlink creation.

### Seam 2 (`selector.IsAllowedBinaryDir` & `selector.BinarySelector`)
- **What it catches**: Ensures executables inside `service/daemon.js` and root `gsconnect` / `gsconnect-preferences` are discovered from an extension `.zip`.
- **What it misses**: Host plugin symlink placement.

### Seam 3 (`release.DetectPluginTargets` & `GithubRelease.DeployDriverAndPluginManifests`)
- **What it catches**: Verifies `metadata.json` parsing (`uuid`, `shell-version`), symlink target generation (`$XDG_DATA_HOME/gnome-shell/extensions/<uuid>` for user, `/usr/share/gnome-shell/extensions/<uuid>` for `--global`), and `glib-compile-schemas` post-hook execution.
- **What it misses**: Live GNOME Shell D-Bus session reload.

## Boundaries

- **Always do**: Isolate tests with `t.TempDir()`, `XDG_DATA_HOME`, `GH_PT_TEST_SYSDIR`, and `xdg.Reload()`. Preserve `state.json.lock` discipline.
- **Ask first**: Adding new third-party Go module dependencies (none required here; stdlib + existing packages suffice).
- **Never do**: Blindly overwrite user config files or run unisolated destructive commands outside `GH_PT_TEST_SYSDIR` during tests.

## Success Criteria

- [x] `gh-pt show GSConnect/gnome-shell-extension-gsconnect` lists `🐧 gsconnect@andyholmes.github.io.zip` under `native [NAT]` default installables.
- [x] `gh-pt install GSConnect/gnome-shell-extension-gsconnect --plugin=gnome-extension -y` extracts `v74` to `~/.local/share/gh-pt/packages/GSConnect/gnome-shell-extension-gsconnect`, compiles `schemas/gschemas.compiled`, symlinks `~/.local/share/gnome-shell/extensions/gsconnect@andyholmes.github.io`, and symlinks `gsconnect`, `gsconnect-preferences`, and `daemon.js` into `~/.local/bin`.
- [x] System `/usr/share/gnome-shell/extensions/gsconnect@andyholmes.github.io` (`apt` package `gnome-shell-extension-gsconnect` v71) is purged so `Config.IS_USER` evaluates `true` cleanly.
- [x] `rtk go test -v ./...` passes with zero regressions.

## Open Questions

1. Should `--plugin=gnome-extension` (with alias `--plugin=gnome`) also trigger automatically when `--plugin=auto` is passed and a valid GNOME Shell `metadata.json` (`uuid` + `shell-version`) is present? *(Recommended: Yes, matches `obs`, `gimp`, and `audio` in `DetectPluginTargets`.)*
