# Wayfinder Map: Library Updates & Bug Fixes

## Destination

Complete library modernization (bubbletea v2, charmbracelet/log, testing libs) then fix pacman animation bugs from handoff document.

## Notes

- **Domain**: CLI tooling, TUI animation, logging, Go
- **Skills to consult**: spec-driven-development, subagent-orchestrator, golang-how-to
- **Standing preferences**: 
  - Prefer parallel execution where possible
  - Maintain backward compatibility with CLI output
  - Preserve all log messages (no information loss)
  - Test thoroughly after each migration

## Decisions so far

- **Library Priority Strategy**: Libraries first, then bugs - `/home/tay/projects/gh-pt/specs/library-update-priority.md`
- **Logger Migration**: zerolog → charmbracelet/log - `/home/tay/projects/gh-pt/specs/logger-migration.md`
  - Status: COMPLETE (2026-09-19)
  - All 19 files migrated
  - All tests pass
  - Binary size: 23MB (unchanged)

## Current Frontier (Next Tickets)

### 1. Progress Bar Implementation (from Handoff)
- [x] Define `ProgressBarMode` type with pacman, standard, spinner variants, none (gauge dropped per spec amendment; `params/params.go:11-47`)
- [x] Add `progress_bar` config field (`config/config.go:47`), `--progress-bar` CLI flag + `GH_PT_PROGRESS_BAR` env var (`params/params.go:124`)
- [x] Add `view`/`v2` migration: re-point Standard+Spinner to `charm.land/bubbles/v2` and drop `ProgressBarGauge` + flag-help gauge (spec Annex A/C/D)
- [x] Implement resolution logic (CLI > env > config > auto-detect) — `resolveProgressBarMode` in `cmd/root.go`
- [x] Create ProgressBar interface and factory — interface in spec §B, routing in `release.go:Install()`
- [x] Implement PacmanProgressBar (routing to existing PacmanUI, not refactor)
- [x] Implement StandardProgressBar using bubbles/v2/progress (import migration only)
- [x] Implement SpinnerProgressBar using bubbles/v2/spinner (import migration only)
- [x] Implement NullProgressBar — `ui/progressbar.go`
- [x] Update release.go to use ProgressBar factory/interface
- [x] Fix non-TTY mode handling (no accumulation) — auto-detects TTY, defaults to `none`
- [x] Ensure progress completes before final messages — `WaitForAnimation()` called before final output

### 2. Bubble Tea v2 Pacman Rewrite
- [x] Rewrite pacman.go using bubbletea v2 Model
- [x] Add harmonica physics for smooth animation
- [x] Add comprehensive tests
- [x] Verify in TTY and non-TTY modes

### 3. Fallback-Release Version Regression Logging
- [x] Add warning log on version fallback: "Unable to find release assets for {platform} to install in version {version} => attempting to find {platform} assets in the prior version {lower-version}" — implemented in `release/release.go:1387-1393`

## Not yet specified

- Additional charmbracelet ecosystem integrations (huh forms, etc.)
- Performance profiling and optimization
- Documentation updates for new logging system

## Out of scope

- Adding new features beyond animation fixes
- Changing CLI interface or command structure
- Modifying state management or installation logic
