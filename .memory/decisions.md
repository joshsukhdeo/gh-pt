# Architecture Decision Records (ADRs)

## ADR-001: State Schema V3 for Source Repos
**Date**: 2026-10-04
**Status**: Accepted
**Context**: Need to track source repositories with compilation scripts and manifests
**Decision**: Added `SourceRepo` struct with `Repository`, `RepoPath`, `IsFork`, `CurrentVersion`, `Track`, `LastUpdated`, `CompileScript`, `ManifestPath` fields. Migration auto-persists in `LoadState()`.
**Consequences**: V1→V2 and V2→V3 migrations handled automatically.

## ADR-002: Wrapper Binary for AI Sandbox
**Date**: 2026-10-04
**Status**: Accepted
**Context**: Need to restrict AI agents to only `ghpt helper` commands
**Decision**: Created `ghpt-wrapper` binary that validates `GHPT_AI_PARENT_PID`, blocks non-helper commands, installed with 0100 perms.
**Consequences**: AI sandbox cannot read wrapper source; shadows real `ghpt` in PATH.

## ADR-003: Reduced Upgrade Flags
**Date**: 2026-10-04
**Status**: Accepted
**Context**: `ghpt upgrade` should not accept install-only flags
**Decision**: Created dedicated `UpgradeFlags` struct accepting only: `-u`/`-g`, `-D`, `--extractor`, `-f`, `--dry-run`, `--verify-checksum`, `--skip-vt-sandbox`, `--fallback-releases`, `--warn-unmapped-assets`, `--progress-bar`
**Consequences**: `--target-base-dir`, `--symlink`, `--ai` etc. rejected on upgrade.

## ADR-004: Root Warning Banner
**Date**: 2026-10-04
**Status**: Accepted
**Context**: Running as root is dangerous but sometimes needed for global installs
**Decision**: Yellow banner on EUID=0, `sudo -v` cache for `--global`, non-fatal with `--global`, fatal without flags.

## ADR-005: Native GNOME Shell Extension Plugin Subsystem (`PluginGnome`)
**Date**: 2026-11-09
**Status**: Proposed
**Context**: GNOME Shell extensions (such as `GSConnect/gnome-shell-extension-gsconnect`) publish `<uuid>.zip` (`*@*.zip`) bundles whose root contains `metadata.json` (`uuid`, `shell-version`), `schemas/*.gschema.xml`, and executables inside `service/` and root (`gsconnect`, `gsconnect-preferences`, `service/daemon.js`). Furthermore, GSConnect relies on `import.meta.url` to determine `Config.PACKAGE_DATADIR` and sets `Config.IS_USER = true` only when `Config.PACKAGE_DATADIR` starts with `$XDG_DATA_HOME/gnome-shell` (`~/.local/share/gnome-shell`).
**Decision**:
1. Classify `*@*.zip` and `*.shell-extension.zip` as `OSLinux` / `SubcatNative` in `selector/classifier.go`.
2. Allow `service/` and `preferences/` subdirectories in `selector.IsAllowedBinaryDir`.
3. Add `PluginGnome` (`gnome`, `gnome-extension`, and `auto` detection in `release/driver.go`) which symlinks the extracted extension root to `$XDG_DATA_HOME/gnome-shell/extensions/<uuid>` (or `/usr/share/gnome-shell/extensions/<uuid>` when `--global` is set), compiles `schemas/` via `glib-compile-schemas`, and links wrapper scripts into `~/.local/bin`.
**Consequences**: `gh-pt install`, `gh-pt upgrade`, `gh-pt source`, `gh-pt rm`, and `gh-pt rollback` natively manage GNOME Shell extensions across both release archives (Method A) and Meson/Ninja source builds (Method C).

## ADR-006: GoReleaser Automated Cross-Platform Binary Releases
**Date**: 2026-11-21
**Status**: Accepted
**Context**: Manual single-platform `make build` cannot distribute cross-platform release artifacts (Linux, macOS, Windows, FreeBSD) with standard checksums and archives.
**Decision**: Added `.goreleaser.yaml` configuring CGO_ENABLED=0 builds across linux/darwin/windows/freebsd (amd64/arm64), tar.gz and zip archives, and sha256 checksums.
**Consequences**: Standardized release builds and archive manifests ready for CI/CD automation.

## ADR-007: Background Sudo Keepalive Goroutine
**Date**: 2026-11-21
**Status**: Accepted
**Context**: In headless pipelines and long source compilations (>15 mins), sudo timestamp expiration causes subsequent elevated commands to hang indefinitely waiting for stdin.
**Decision**: Spawn a non-interactive background goroutine executing `sudo -n -v` every 60 seconds when `(Global || ResolveDeps) && os.Geteuid() != 0`, cleanly terminated via context cancellation.
**Consequences**: Headless automation and long-running compilations complete without credential cache drops or stdin blocking.