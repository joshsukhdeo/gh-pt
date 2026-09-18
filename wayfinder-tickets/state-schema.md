# State Tracking Schema

Labels: `wayfinder:grilling`
Status: **Resolved**
Blocked By: None

## Question

How should sidecar state be persisted and managed?

## Resolution

1. **`InstalledSidecars` storage**: Absolute paths. Simpler for purge logic; no path reconstruction needed.
2. **`GetStateFields()` exposure**: Include all three sidecar fields (`sidecars`, `sidecar_target_path`, `installed_sidecars`).
3. **`ParseStateUpdate()` handling**: 
   - `sidecars` → `[]string` (comma-separated, editable)
   - `sidecar_target_path` → `string` (editable)
   - `installed_sidecars` → display only (not editable via CLI, managed by extraction lifecycle)
4. **Interactive `StateEdit()`**: Display sidecar fields read-only. Pattern editing is rare; users can `gh-pt state update` for that.

## Implementation Notes

Schema already exists at `state/state.go:42-44`:
```go
Sidecars          []string `json:"sidecars,omitempty"`
SidecarTargetPath string   `json:"sidecar_target_path,omitempty"`
InstalledSidecars []string `json:"installed_sidecars,omitempty"`
```

Add to `GetStateFields()` in `cmd/state_cmds.go`:
```go
"sidecars",
"sidecar_target_path",
"installed_sidecars",
```

Add to `ParseStateUpdate()` switch:
```go
case "sidecar_target_path":
    updates[field] = value
case "sidecars":
    updates[field] = parseCommaSeparated(value)
// installed_sidecars: not editable via ParseStateUpdate
```
