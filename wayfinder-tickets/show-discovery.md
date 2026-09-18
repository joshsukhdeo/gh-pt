# gh-pt show Discovery

Labels: `wayfinder:grilling`
Status: **Resolved**
Blocked By: `archive-extraction.md`

## Question

How should `gh-pt show` expose potential sidecar assets from a remote repository?

## Resolution

1. **Discovery trigger**: **Flag-gated with `--discover-sidecars`**. No short flag (discovery is rare and opt-in; `-s` conflicts with `--sidecars`). Adds API calls; most users don't need it.

2. **Large repo handling**: **Server-side sparse with client-side filter**. Use GitHub tree API `repos/$REPO/git/trees/$BRANCH?recursive=1`, filter response for sidecar-like extensions. No pagination needed for most repos.

3. **Output format**: **Suggest `--sidecars` pattern for each**. Group by directory and suggest copy-pasteable flags (e.g., `plugins/*.red` → `--sidecars 'plugins/*.red'`).

## Implementation Notes

### Add to `ShowCmd` in `params/params.go`:
```go
DiscoverSidecars bool `help:"Discover potential sidecar assets in the repository."`
```

### Add to `cmd/show.go` (or wherever `ShowCmd` is handled):
```go
if cli.Show.DiscoverSidecars {
    discoverSidecars(cli.Show.Repository, cli.Show.Version)
}
```

### Discovery function:
```go
func discoverSidecars(repo, version string) {
    // Fetch git tree
    var tree struct {
        Tree []struct {
            Path string `json:"path"`
            Type string `json:"type"`
        } `json:"tree"`
    }
    
    ref := version
    if ref == "" || ref == "latest" {
        ref = "HEAD"
    }
    
    _, err := gh.Exec("api", fmt.Sprintf("repos/%s/git/trees/%s?recursive=1", repo, ref))
    // Parse response, filter for sidecar extensions
    
    sidecarExts := []string{".so", ".dll", ".dylib", ".red", ".pak", ".json", ".yaml", ".yml"}
    var sidecars []string
    for _, item := range tree.Tree {
        if item.Type != "blob" {
            continue
        }
        for _, ext := range sidecarExts {
            if strings.HasSuffix(strings.ToLower(item.Path), ext) {
                sidecars = append(sidecars, item.Path)
                break
            }
        }
    }
    
    // Group by directory and suggest patterns
    pterm.DefaultHeader.Println("Potential Sidecar Assets")
    patterns := suggestSidecarPatterns(sidecars)
    for _, p := range patterns {
        fmt.Printf("  --sidecars '%s'\n", p)
    }
}
```

### Pattern suggestion logic:
```go
func suggestSidecarPatterns(paths []string) []string {
    // Group by directory, find common extensions
    // e.g., ["plugins/foo.red", "plugins/bar.red"] → "plugins/*.red"
    // e.g., ["lib/plugin.so"] → "lib/*.so"
    // Return deduplicated list
}
```
