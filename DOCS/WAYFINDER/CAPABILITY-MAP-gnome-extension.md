# Capability Map: GNOME Shell Extension Plugin & Source Cherry-Pick Lifecycle

## Destination

Enable `gh-pt` to natively classify, install, upgrade, rollback, and compile GNOME Shell Extensions (`--plugin=gnome` / `--plugin=gnome-extension` / `--plugin=auto`) across both Method A (`gh-pt install` release zip bundles) and Method C (`gh-pt source` Meson/Ninja compilation with multi-PR cherry-picking and automatic PRR/PRRR rollback on excessive troubleshooting).

---

## Architectural Scope & Invariants

All changes adhere strictly to core repository invariants (`AGENTS.md`):
1. **GitHub CLI Required**: `gh` CLI presence & authentication required.
2. **State Locking**: Always use `state.LoadState()` / `st.Save()` or `state.Mutate()`; never bypass `state.json.lock`.
3. **Test Isolation**: In all unit and integration tests, set `XDG_DATA_HOME` and `GH_PT_TEST_SYSDIR` via `t.TempDir()` and immediately invoke `xdg.Reload()`.
4. **Terminal Hygiene**: Never run background progress animations while waiting on interactive stdin.
5. **Security**: Disallow GitHub tokens in CLI arguments; read strictly from environment or config.

---

## Module Capability Table

| Module id | Specification | Depends on | Responsibility |
|---|---|---|---|
| `gnome-extension-release` | [`DOCS/SPECS/SPEC-gnome-extension-release.md`](../SPECS/SPEC-gnome-extension-release.md) | — | **Method A**: Classify `*@*.zip` & `.shell-extension.zip` as `OSLinux` / `SubcatNative`; relax `IsAllowedBinaryDir` for `service/` & `preferences/` executables; add `PluginGnomeExtension` (`gnome` / `gnome-extension` / `auto`) in `params` & `release/driver.go` to symlink extracted package to `~/.local/share/gnome-shell/extensions/<uuid>` (or `/usr/share/gnome-shell/extensions/<uuid>` with `-g`) and compile `schemas/` via `glib-compile-schemas`. |
| `gnome-extension-source-cherrypick` | [`DOCS/SPECS/SPEC-gnome-extension-source-cherrypick.md`](../SPECS/SPEC-gnome-extension-source-cherrypick.md) | `gnome-extension-release` | **Method C**: Support `--cherry-pick-prs` (or deterministic `.ghpt/body.sh` cherry-pick + Meson/Ninja `install-zip` / `_build/<uuid>` staging) in `gh-pt source`, bounded troubleshooting (max 5 steps), and automatic **PRR / PRRR Rollback** to the stable Method A release when cherry-picked PRs introduce deep architectural regressions. |

---

## Build Order

`gnome-extension-release` (Method A + immediate live GSConnect v74 install) → `gnome-extension-source-cherrypick` (Method C + PR cherry-pick compile, max-5-step verification, and PRR/PRRR rollback guard).
