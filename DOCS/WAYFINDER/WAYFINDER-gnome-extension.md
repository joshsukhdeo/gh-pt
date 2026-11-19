## Destination

Deliver native GNOME Shell Extension lifecycle management in `gh-pt` (`--plugin=gnome-extension`), immediately install GSConnect `v74` via Method A (`gh-pt install`), and then execute Method C (`gh-pt source`) with cherry-picked upstream PRs and a strict 5-step troubleshooting cap backed by automatic **PRR / PRRR ("Walmart Rollback")** to Method A `v74`.

## Notes

- **Domain**: Go CLI (`gh-pt`), GNOME Shell Extensions (`gsconnect@andyholmes.github.io`), Meson/Ninja source builds, XDG desktop portals.
- **Skills**: `spec-driven-development`, `subagent-orchestrator`, `ponytail`, `tdd`, `wayfinder`, `architecture-decision-records`, `architecture-patterns`, `architect-review`.
- **Standing Preferences**:
  - Purge `/usr/share/gnome-shell/extensions/gsconnect@andyholmes.github.io` (`apt` package `gnome-shell-extension-gsconnect` v71) before activating user install so `Config.IS_USER` is `true` and no path shadowing occurs.
  - Keep `~/.local/bin/daemon.js`, `~/.local/bin/gsconnect-daemon`, `~/.local/bin/gsconnect`, and `~/.local/bin/gsconnect-preferences` symlinked to the active `gh-pt` package directory.
  - **Hard Cap on Method C Troubleshooting**: Max 5 steps after compile+install. If imminently solvable, give step/time estimate and ask user. If heavy/intensive, immediately rollback to Method A (`v74`), state time/step estimate, and print the exact PRR/PRRR notice.

## Decisions so far

- [ADR-005: Native GNOME Shell Extension Plugin Subsystem (`PluginGnome`)](../../.memory/decisions.md): Accepted — extend `Selector`, `AssetClassifier`, and `DeployDriverAndPluginManifests` to treat `*@*.zip` GNOME extensions as first-class Linux packages with automatic `glib-compile-schemas` and `$XDG_DATA_HOME/gnome-shell/extensions/<uuid>` symlinking.
- [Capability Map: GNOME Extension Release & Source Cherry-Pick](CAPABILITY-MAP-gnome-extension.md): Defined two vertical modules (`gnome-extension-release` -> `gnome-extension-source-cherrypick`).
- **Cherry-Pick Set**: PRs `#2211`, `#2210`, `#2209`, and `#2206` cherry-picked cleanly on top of `v74`. PR `#2208` (`clipboard-files`) excluded within 1 step per spec contract due to base conflict with `#2211` in `src/wl_clipboard.js`. Built via `meson setup _build . && ninja -C _build make-pkgdir` in 3.5s, and passed verification within 4 steps.

## Completed Tickets

- [x] [Ticket 1: TDD Seam Tests & Implementation for Method A (`gnome-extension-release`)](#ticket-1-tdd-seam-tests--implementation-for-method-a-gnome-extension-release) (`wayfinder:task`)
- [x] [Ticket 2: Purge Apt GSConnect v71 & Live-Install GSConnect v74 via Method A](#ticket-2-purge-apt-gsconnect-v71--live-install-gsconnect-v74-via-method-a) (`wayfinder:task`)
- [x] [Ticket 3: TDD Seam Tests & Implementation for Method C (`gnome-extension-source-cherrypick`)](#ticket-3-tdd-seam-tests--implementation-for-method-c-gnome-extension-source-cherrypick) (`wayfinder:task`)
- [x] [Ticket 4: Execute Method C Cherry-Pick Source Build, 5-Step Triage, and PRR/PRRR Gate](#ticket-4-execute-method-c-cherry-pick-source-build-5-step-triage-and-prrprrr-gate) (`wayfinder:task`)

## Open Tickets

None.

## Not yet specified

- None (Resolved: PRs `#2211`, `#2210`, `#2209`, and `#2206` merged cleanly; PR `#2208` excluded due to merge conflict with `#2211` in `src/wl_clipboard.js`).

## Out of scope

- Supporting legacy GNOME Shell < 45 non-ESM extensions.
- Modifying GNOME Shell's C/JS `libshell-18.so` binary on disk (unnecessary since GSConnect v73+ PR #2037 solved shortcut assignment without `GrabAccelerator`).

---

### Ticket 1: TDD Seam Tests & Implementation for Method A (`gnome-extension-release`)
**Label:** `wayfinder:task` (AFK — Subagents 1 & 2)
**Status:** COMPLETED
**Verification:** Seams 1–3 implemented with passing tests in `selector/classifier_test.go`, `release/driver_test.go`, and `selector/selector_test.go`. `PluginGnome` successfully handles `*@*.zip` and `.shell-extension.zip` with `glib-compile-schemas` post-hook.
**Question / Deliverable:**
Write failing unit tests at Seams 1–3 (`selector/classifier_test.go`, `release/driver_test.go`), implement `OSLinux`/`SubcatNative` classification for `*@*.zip` & `.shell-extension.zip`, allow `service/` and `preferences/` in `IsAllowedBinaryDir`, and implement `PluginGnome` (`--plugin=gnome-extension` / `--plugin=gnome` / `--plugin=auto`) in `params/params.go` and `release/driver.go` with `glib-compile-schemas` post-hook. Build `gh-pt` via `make build`.

### Ticket 2: Purge Apt GSConnect v71 & Live-Install GSConnect v74 via Method A
**Label:** `wayfinder:task` (AFK — Orchestrator)
**Status:** COMPLETED
**Verification:** Apt package `gnome-shell-extension-gsconnect` purged. GSConnect `v74` installed cleanly via `gh-pt install --plugin=gnome-extension`. Symlinks and schemas verified; `/home/tay/projects/runshit/run.sh` updated.
**Question / Deliverable:**
Purge `gnome-shell-extension-gsconnect` (`v71-1ubuntu1`) via `sudo apt-get purge -y gnome-shell-extension-gsconnect`, then run:
```bash
gh-pt install GSConnect/gnome-shell-extension-gsconnect --plugin=gnome-extension -f -y
```
Verify `~/.local/share/gnome-shell/extensions/gsconnect@andyholmes.github.io` symlink, `schemas/gschemas.compiled`, `~/.local/bin/daemon.js` & `~/.local/bin/gsconnect-daemon` symlinks, and update `/home/tay/projects/runshit/run.sh`.

### Ticket 3: TDD Seam Tests & Implementation for Method C (`gnome-extension-source-cherrypick`)
**Label:** `wayfinder:task` (AFK — Subagent 3)
**Status:** COMPLETED
**Verification:** Seam 4 unit test `TestMoveDistAndCreateSymlinks_GnomeExtensionBundle` passes. Preserves GNOME bundle files and directory layout in `symlinkDir` and links executables from root, `service/`, and `preferences/` subdirectories.
**Question / Deliverable:**
Ensure `handleCompileFromSource` / `deploySourceDriversAndPlugins` in `cmd/root.go` and `compile/compile.go` preserve extension directory structure (`metadata.json`, `schemas/`, `service/`, `preferences/`, `shell/`, `*.gresource`) in `symlinkDir` and execute `DeployDriverAndPluginManifests(symlinkDir)` so `gh-pt source --plugin=gnome-extension` links the compiled extension into `~/.local/share/gnome-shell/extensions/<uuid>`.

### Ticket 4: Execute Method C Cherry-Pick Source Build, 5-Step Triage, and PRR/PRRR Gate
**Label:** `wayfinder:task` (HITL/AFK — Orchestrator)
**Status:** COMPLETED
**Verification:** Build script cherry-picked PRs `#2211`, `#2210`, `#2209`, `#2206` (dropping conflicting `#2208` within 1 step). Meson/Ninja compiled package in 3.5s, staged to `symlinkDir`, linked via `PluginGnome`, and passed post-install verification in 4 diagnostic steps without requiring PRR rollback.
**Question / Deliverable:**
 Stage `~/builds/scripts/GSConnect_gnome-shell-extension-gsconnect_compile.sh` to cherry-pick non-conflicting high-value open PRs (#2211, #2210, #2209, #2206, #2208) on top of `v74`, compile with `meson` & `ninja -C _build make-pkgdir`, and install via `gh-pt source GSConnect/gnome-shell-extension-gsconnect --plugin=gnome-extension --no-compile-container -f -y`. Run at most 5 troubleshooting/verification steps. Apply the PRR/PRRR rollback gate if heavy issues occur.
