## Destination

Implement first-class sidecar asset support in `gh-pt`: CLI params, config schema, state tracking, archive/source extraction lifecycle, upgrade/purge operations, orphaned asset heuristics, and `gh-pt show` discovery. Full spec at `docs/SPEC-SIDECARS.md`.

## Notes

- Domain: Go CLI, `alecthomas/kong` parser, `pterm` UI, XDG paths, archive extraction (`archiver/v4`), GitHub REST API (`go-gh`).
- Spec is prescriptive: follow `docs/SPEC-SIDECARS.md` section order.
- Orphaned asset heuristics: hardcoded keywords/extensions with `warn_unmapped_assets` boolean toggle (no configurable patterns).
- Core flow (schema → extraction → lifecycle) is strict layer order. Heuristics and discovery can proceed in parallel once extraction is done.
- Source compilation and remote sparse-checkout are lower priority; defer if blocked.
- Local-markdown tracker (issues disabled on repo).

## Decisions so far

- [CLI Params & Config Schema](wayfinder-tickets/cli-params-config.md): `--env-inject` as `[]string`, `SidecarPath` in `PathsConfig` (already exists), XDG fallback at use-site in `release.go`.
- [State Tracking Schema](wayfinder-tickets/state-schema.md): Absolute paths for `InstalledSidecars`, all fields in `GetStateFields()`, `sidecars` and `sidecar_target_path` editable via `ParseStateUpdate()`, `installed_sidecars` display-only.
- [Archive Extraction Lifecycle](wayfinder-tickets/archive-extraction.md): Implemented `extractExplicitSidecars()` with pattern matching, `resolveSidecarTargetPath()` with XDG fallback.
- [Upgrade/Purge Lifecycle](wayfinder-tickets/upgrade-purge.md): Wipe-and-reextract for upgrade, delete entire `SidecarTargetPath` for purge, replay all sidecar params in `DoUpdate`.
- [Orphaned Asset Heuristics](wayfinder-tickets/orphaned-heuristics.md): Both remote and local detection, all three false-positive layers, pause/resume PacmanUI. Implemented `shouldWarnUnmappedAssets()` with config toggle.
- [gh-pt show Discovery](wayfinder-tickets/show-discovery.md): Flag-gated with `--discover-sidecars`, server-side sparse with client-side filter, suggest `--sidecars` patterns in output.

## Additional Features Implemented

- **Auto-detection with `--include-sidecars`**: Automatically detects and includes suspected sidecar assets without requiring explicit patterns. Implied when any `--sidecar-*` param is specified.
- **Multi-target symlinking**: `--sidecar-symlink-to` creates symlinks from installed sidecars to app-specific directories
- **AI setup integration**: `--ai-setup-sidecars` uses AI to analyze sidecars and generate setup instructions
- **State persistence**: All sidecar fields (`sidecars`, `sidecar_target_path`, `sidecar_symlink_to`, `include_sidecars`, `installed_sidecars`, `env_inject`) tracked in state.json

## Frontier

**All tickets resolved. Map complete.**

## Blocked

- [Archive Extraction Lifecycle](wayfinder-tickets/archive-extraction.md) ← blocked by CLI params, state schema
- [Upgrade/Purge Lifecycle](wayfinder-tickets/upgrade-purge.md) ← blocked by archive extraction
- [Orphaned Asset Heuristics](wayfinder-tickets/orphaned-heuristics.md) ← blocked by archive extraction
- [gh-pt show Discovery](wayfinder-tickets/show-discovery.md) ← blocked by archive extraction

## Not yet specified

- **Source compilation integration**: How `gh pt source` instructs AI agent to stage sidecars during build.
- **Remote sparse-checkout fallback**: REST tree API vs `git archive` vs shallow clone for binary-release sidecar fetch.
- **Env-inject runtime semantics**: Shell profile snippets vs runtime env vars vs both.

## Out of scope

- V2 items per spec: AI compilation manifest parser, static `ldd`/`.so` resolution, package manager resolver rewrite, `--add-deps` deprecation.
