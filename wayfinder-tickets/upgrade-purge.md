# Upgrade/Purge Lifecycle

Labels: `wayfinder:grilling`
Status: **Resolved**
Blocked By: `archive-extraction.md`

## Question

How should sidecars be handled during upgrade and uninstall?

## Resolution

1. **Upgrade pruning strategy**: **Wipe and re-extract**. Delete entire `SidecarTargetPath` before re-extracting sidecars fresh. Simpler than diffing, avoids stale file accumulation. User modifications are lost, but sidecars are typically auto-generated assets, not user-edited configs.

2. **Purge scope**: **Delete entire `SidecarTargetPath` directory**. Matches how binaries are purged. If user has custom files there, that's their problem.

3. **Upgrade param replay**: **Yes to all three**. `DoUpdate` must replay `app.Sidecars`, `app.SidecarTargetPath`, and `app.EnvInject` (once `EnvInject` is added to state schema).

## Implementation Notes

### Upgrade (`cmd/update.go`)
Add to `DoUpdate` after line 169 (after `appParams.Extractor = app.Extractor`):
```go
appParams.Sidecars = app.Sidecars
appParams.SidecarTargetPath = app.SidecarTargetPath
appParams.EnvInject = app.EnvInject
```

Add wipe logic in `release.Install()` before sidecar extraction:
```go
if r.CliParams.IsUpgradeCmd && r.SidecarTargetPath != "" {
    if err := os.RemoveAll(r.SidecarTargetPath); err != nil {
        log.Warn().Err(err).Msg("failed to purge old sidecars")
    }
}
```

### Purge (`cmd/state_mgmt.go`)
Add to `RemoveApp` after line 373 (after binary deletion loop):
```go
if app.SidecarTargetPath != "" {
    if err := os.RemoveAll(app.SidecarTargetPath); err != nil {
        log.Warn().Err(err).Msgf("Failed to purge sidecar directory %s", app.SidecarTargetPath)
    } else {
        log.Info().Msgf("Deleted sidecar assets at %s", app.SidecarTargetPath)
    }
}
```
