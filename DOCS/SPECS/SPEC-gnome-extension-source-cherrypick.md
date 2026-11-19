# Spec: GNOME Shell Extension Source Compilation & Cherry-Pick Lifecycle (`gnome-extension-source-cherrypick` — Method C)

## Objective

Extend `gh-pt source` (Method C) to compile bleeding-edge GNOME Shell extensions from source (`GSConnect/gnome-shell-extension-gsconnect`), cherry-picking the highest-value open upstream PRs on top of `main` (`v74`), installing via the `gnome-extension` plugin pipeline, executing a strict **max-5-step troubleshooting budget**, and enforcing the **PRR / PRRR ("Good ol' Walmart Rollback")** protocol if incompatibilities or heavy work arise.

### Selected Beneficial Open PRs for Cherry-Pick Evaluation
1. **PR #2211** (`fix/wl-clipboard-ownerchange`): Wayland clipboard — fix portal failing to start.
2. **PR #2210** (`fix/rediscover-on-network-changes`): Rediscover peers when network details change.
3. **PR #2209** (`fix-queue-channel-packet-writes`): Queue channel packet writes so multi-file shares all get sent.
4. **PR #2206** (`fix/rediscover-disconnected-paired-devices`): Periodically rediscover disconnected paired devices.
5. **PR #2208** (`clipboard-files`): Clipboard support for sharing images and files.
*(Note: PR #2204 `feature/gtk4-libadwaita-port` is a massive UI rewrite that conflicts with GTK3 PRs and carries high regression risk; it is isolated as an optional test or excluded first under the 5-step cap).*

## Tech Stack

- **Build Toolchain**: `meson`, `ninja`, `glib-compile-schemas`, `git`, `gh`
- **Go Packages**: `cmd/root.go`, `compile/compile.go`, `release/driver.go`, `cmd/history_cmds.go` (`rollback`)

## Commands

```bash
make build
rtk go test -v ./cmd/... ./compile/... ./release/...
gh-pt source GSConnect/gnome-shell-extension-gsconnect --plugin=gnome-extension --no-compile-container -f -y
```

## Project Structure

```
cmd/root.go                → Support source-built directories in deploySourceDriversAndPlugins (detect metadata.json inside _build/ or .ghpt/dist/ or install-dir)
compile/compile.go         → Ensure SaveCompileState & handleDistHandoff preserve directory hierarchies for extension bundles
DOCS/SPECS/                → Specification artifacts
```

## Code Style & Build Script Contract

For `GSConnect/gnome-shell-extension-gsconnect`, the pre-seeded or AI-generated compile script (`~/builds/scripts/GSConnect_gnome-shell-extension-gsconnect_compile.sh` / `.ghpt/body.sh`) must:
1. Fetch and cherry-pick target PR refs cleanly (`git fetch origin pull/<ID>/head:pr-<ID> && git cherry-pick --no-commit pr-<ID>`), aborting/skipping any PR that has merge conflicts within 1 step.
2. Run Meson & Ninja to stage the packaged extension directory into `.ghpt/dist/` (or ` symlinkDir`):
   ```bash
   meson setup --prefix="$INSTALL_DIR" _build .
   ninja -C _build make-pkgdir
   cp -r _build/gsconnect@andyholmes.github.io/* "$ SYMLINK_DIR/"
   glib-compile-schemas "$SYMLINK_DIR/schemas"
   ```
3. Trigger `deploySourceDriversAndPlugins(symlinkDir)` so `~/.local/share/gnome-shell/extensions/gsconnect@andyholmes.github.io` points to the compiled package and `service/daemon.js` + `gsconnect` + `gsconnect-preferences` are symlinked into `~/.local/bin`.

## Troubleshooting & PRR / PRRR Rollback Protocol (Hard Constraint)

After compilation and installation complete:
1. **Max 5 Troubleshooting Steps**: Run at most 5 diagnostic/repair steps (e.g., `daemon.js --version`, starting `daemon.js` service, opening `gsconnect-preferences`, checking `journalctl --user -b /usr/bin/gjs`, verifying D-Bus activation).
2. **Decision Gate after ≤ 5 Steps**:
   - **If working cleanly**: Keep the cherry-picked source build active and record state in `state.json`.
   - **If minor & imminently solvable**: Report exact estimate of steps and time required, and ask the user whether to proceed or rollback.
   - **If work is considerably intensive / heavy**: Automatically trigger rollback to the working Method A `v74` release (`gh-pt rollback GSConnect/gnome-shell-extension-gsconnect --force` or `gh-pt install GSConnect/gnome-shell-extension-gsconnect --plugin=gnome-extension -f -y`), report the estimated steps + time that would have been required, and output the mandatory notice:
     > `Excessive work required -> yepp, dem's a hard na-na-not hard nooope, not me problem, perfunctorary PR, peace, poof, *Good ol' Walmart Rolllback wh00p wh00p*, this be a PRR ==> Pragmatic Rollback Required. Maybe even a PRRR ===> Peace y'all, move in, now move out, keep Rollin' Rollin' ROLLIN-BACK`

## Testing Strategy

### Seam 4 (`RootCLI.deploySourceDriversAndPlugins` & Source Bundle Handoff)
- **What it catches**: Verifies that a source build staging an extension bundle (`metadata.json` + `schemas/` + `service/daemon.js`) in `symlinkDir` (`~/src/apps/GSConnect/gnome-shell-extension-gsconnect` or package store) registers the GNOME Shell extension symlink and compiles schemas identically to Method A.
- **What it misses**: Upstream JS runtime bugs in unmerged PRs (caught by the 5-step post-install verification gate).

## Boundaries

- **Always do**: Snapshot working Method A (`v74`) state before running Method C so rollback is instant and zero-loss.
- **Ask first**: Exceeding 5 troubleshooting steps on broken upstream PR code.
- **Never do**: Spend more than 5 steps debugging broken upstream PR branches before either asking with an estimate or triggering the PRR/PRRR rollback.

## Success Criteria

- [x] `gh-pt source GSConnect/gnome-shell-extension-gsconnect --plugin=gnome-extension` builds from source, cherry-picks clean PRs, stages the extension bundle, and links it via `PluginGnome`.
- [x] Post-install verification completes within ≤ 5 steps.
- [x] If heavy issues arise, automatic rollback to `v74` (Method A) completes and prints the exact PRR/PRRR notice with step/time estimates.
