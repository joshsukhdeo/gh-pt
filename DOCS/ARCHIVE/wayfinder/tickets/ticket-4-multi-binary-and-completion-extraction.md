# Ticket 4: Multi-Binary Extraction & Completion Management

- **Type**: `wayfinder:prototype`
- **Status**: Closed
- **Assignee**: Completed
- **Blocked By**: None

## Resolution
1. Added `IsAllowedBinaryDir` in `selector` package to restrict archive binary scanning strictly to root, `bin/`, `bin32/`, and `bin64/` (including 1-level root folders such as `app-1.0.0/bin`). Excluded vendor and fixture paths.
2. Implemented `ScanAndInstallArchiveCompletions` and `SetupBinaryCompletions` in `release/completions.go` with probes for dynamic completion commands (`completion bash/zsh`) and archive static files (`complete.bash`, `*completion*.bash/zsh`, `_*`).
3. Managed master sourcing loaders `completions.bash` and `completions.zsh` beside `state.json` in `$XDG_DATA_HOME/gh-pt/`.
4. Supported flags `--no-completion-setup`, `--only-first-match`, and `--max-exe-installs` in `params`, `selector`, and `release`. All 509 tests passing with hermetic `state.json` preservation.
