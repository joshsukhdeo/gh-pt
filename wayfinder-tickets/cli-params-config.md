# CLI Params & Config Schema

Labels: `wayfinder:grilling`
Status: **Resolved**
Blocked By: None

## Question

How should the CLI params and config schema be structured for sidecar support?

## Resolution

1. **`--env-inject` type**: `[]string` (KEY=VALUE pairs). Slice is simpler; Kong parses maps awkwardly.
2. **`SidecarPath` location**: `PathsConfig`. Already exists at `config/config.go:23`.
3. **XDG fallback resolution**: Use-site in `release.go`. Resolution order: CLI flag → config → XDG default (`$XDG_DATA_HOME/gh-pt/sidecars/<owner>/<repo>/`).
4. **`WarnUnmappedAssets`**: Already exists at `config/config.go:46`.

## Implementation Notes

Add to `CommonInstallFlags` in `params/params.go`:
```go
Sidecars          []string `optional:"" short:"s" help:"Glob patterns for sidecar assets to capture."`
SidecarTargetPath string   `optional:"" type:"path" help:"Target directory for sidecar assets."`
EnvInject         []string `optional:"" help:"Environment variables pointing to sidecar directory (KEY=VALUE)."`
```

Sidecar destination resolution in `release.go`:
```go
func (r *GithubRelease) resolveSidecarTargetPath() string {
    if r.CliParams.SidecarTargetPath != "" {
        return r.CliParams.SidecarTargetPath
    }
    if r.Config.Paths.SidecarPath != "" {
        return filepath.Join(r.Config.Paths.SidecarPath, r.CliParams.Repository)
    }
    return filepath.Join(xdg.DataHome, "gh-pt", "sidecars", r.CliParams.Repository)
}
```
