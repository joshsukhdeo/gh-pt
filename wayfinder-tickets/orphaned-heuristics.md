# Orphaned Asset Heuristics

Labels: `wayfinder:grilling`
Status: **Resolved**
Blocked By: `archive-extraction.md`

## Question

How should orphaned/suspected sidecar assets be detected and surfaced?

## Resolution

1. **Detection timing**: **Both remote and local**. Remote catches unselected release assets (like `plugins.zip`), local catches files inside archives (like `lib/plugin.red`).

2. **False positive filtering**: **All three layers**: `README*`/`LICENSE*`/`.md`/`doc/`/`src/`, foreign-OS installers (`.exe`/`.dmg`/`.pkg`/`.msi`/`.apk` on Linux), and anything matching `BinarySelector` patterns.

3. **Interactive prompt UX**: **Pause/resume PacmanUI**. Already implemented in `handleSuspectedSidecars()` at `release/release.go:333` with `interactiveMultiselect()`.

## Implementation Notes

### Already implemented:
- `isSuspectedRemoteSidecar()` at `release/release.go:200-240`
- `isSuspectedLocalSidecar()` at `release/release.go:242-280`
- Remote asset scanning at `release/release.go:1470-1480`
- Local extraction scanning at `release/release.go:1496-1532`
- `handleSuspectedSidecars()` at `release/release.go:320-389`

### Missing:
- `WarnUnmappedAssets` config toggle is defined at `config/config.go:46` but not wired to CLI or checked in `shouldWarnUnmappedAssets()`.

### Wire config toggle:
Add to `CommonInstallFlags` in `params/params.go`:
```go
WarnUnmappedAssets bool `default:"true" help:"Warn about suspected unmapped sidecar assets."`
```

Check in `shouldWarnUnmappedAssets()`:
```go
func (r *GithubRelease) shouldWarnUnmappedAssets() bool {
    if r.WarnUnmappedAssets != nil {
        return *r.WarnUnmappedAssets
    }
    cfg, _ := config.LoadConfig()
    if cfg != nil {
        return cfg.Core.WarnUnmappedAssets
    }
    return true
}
```
