# Archive Extraction Lifecycle

Labels: `wayfinder:grilling`
Status: **Resolved (partial)**
Blocked By: `cli-params-config.md`, `state-schema.md`

## Question

How should sidecar extraction integrate into the archive/binary release flow?

## Resolution

### What's implemented:
1. **`resolveSidecarTargetPath()`** at `release/release.go:183-198`: Resolution order is struct field → config → XDG. Needs wiring to CLI param.
2. **`handleSuspectedSidecars()`** at `release/release.go:320-389`: Handles heuristic-based sidecar detection and deployment (interactive multiselect or headless warning).
3. **Sidecar scanning** at `release/release.go:1470-1536`: Scans remote assets and local extraction directory for suspected sidecars using `isSuspectedRemoteSidecar()` and `isSuspectedLocalSidecar()`.

### What's missing:
1. **Wire CLI params**: `resolveSidecarTargetPath()` checks `r.SidecarTargetPath` but should check `r.CliParams.SidecarTargetPath` first.
2. **Explicit `--sidecars` pattern matching**: Current implementation only handles "suspected" sidecars via heuristics. Need to add explicit glob matching for `r.CliParams.Sidecars` patterns during extraction.
3. **Pattern matcher**: Use `doublestar.Glob` or `filepath.Match` for sidecar patterns. Spec suggests glob patterns like `red_plugins/**`, `plugins/*.red`.

### Decisions:
1. **Hook point**: After binary extraction, before cleanup. Sidecar extraction runs alongside heuristic scanning.
2. **Pattern matcher**: `doublestar.Glob` for `**` support. Already in `go.mod`?
3. **Conflicts**: Binary selector takes precedence. If a file matches both binary and sidecar patterns, it's installed as binary only.

### Implementation notes:
Add to `Install()` after line 1536 (after heuristic scanning):
```go
// Explicit --sidecars pattern matching
if len(r.CliParams.Sidecars) > 0 {
    r.extractExplicitSidecars(extractDir, fsObj)
}
```

New function `extractExplicitSidecars()`:
- Iterate archive entries / extract dir
- Match against `r.CliParams.Sidecars` patterns using `doublestar.Glob`
- Copy matches to `resolveSidecarTargetPath()`
- Record in `r.InstalledSidecars`
