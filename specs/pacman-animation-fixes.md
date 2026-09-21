# Spec: Progress Bar Implementation

## Objective
Implement a configurable progress bar system with modes: pacman (animated), standard (traditional progress bar), spinner variants, and none. Replace the "animation" concept with "progress-bar" for clarity.

## Background
The pacman animation was partially fixed but still has issues. Rather than just fixing bugs, we're redesigning the progress indication system to be more flexible and user-friendly.

## Amendment (2026-09-19): Dependency Facts & Full Interface

This amendment supersedes stale material in the original sections below. Where they conflict, this amendment wins.

### A. Dependencies — bubbletea v2 ecosystem (verified)
- The project runs on bubbletea v2 and must use the matching v2 ecosystem. `go.mod` already carries:
  - `charm.land/bubbletea/v2 v2.0.9` (direct)
  - `charm.land/bubbles/v2 v2.0.0` (verbatim: `charm.land/bubbles/v2`, NOT `github.com/charmbracelet/bubbles`)
- Verified contents of `charm.land/bubbles/v2@v2.0.0` module cache: `progress`, `spinner`, `cursor`, `filepicker`, `help`, `key`, `list`, `paginator`, `stopwatch`, `table`, `textarea`, `textinput`, `timer`, `viewport`.
- There is **no `gauge` package** in `charm.land/bubbles/v2` nor in `github.com/charmbracelet/bubbles`. The original `gauge` mode is **dropped** from this spec.
- The v2 `progress` API: `progress.New(opts ...Option) Model`, `Model.ViewAs(percent float64) string`, `Model.View() string`, `Model.Init()/Update(msg)`, `progress.WithoutPercentage()`, `progress.WithWidth(w)`.
- The v2 `spinner` API: `spinner.New(opts ...Option) Model`, `spinner.WithSpinner(spinner.Spinner)`, `spinner.Model.View() string`, `spinner.Model.Tick() tea.Msg`. Styles (Line, Dot, MiniDot, Jump, Pulse, Points) all exist under the same names used today.

### B. Full ProgressBar interface
The interface must cover **every method the release pipeline drives today**, so `PacmanUI` (stage-driven) and the bar-driven modes (standard/spinner/null) all satisfy it without call-site rewrites. Verified call surface (release/release.go + release/symlink.go, `r.UI.*` and `pUI.*`):

- `Start()` (no-arg), `Stop()`, `Pause()`, `Resume()`, `WaitForAnimation()`
- `Update(stage int, version, archive, asset, target, ghostType string)` — **6 args**
- `UpdateSymlink(symlink string)`, `SetCurrentAsset(index int)`, `SetAssetInstallCmd(installCmd string)`
- `AddAsset(name, fullName, symlink, installCmd string)`
- `DisableIcons` — **field**, not a method (set once at construction in release.go:1284)

This is the interface:

```go
type ProgressBar interface {
	// Lifecycle
	Start()
	Stop()
	Finish()
	Pause()
	Resume()
	WaitForAnimation()

	// Stage-driven: multi-phase install reporting.
	Update(stage int, version, archive, asset, target, ghostType string)
	UpdateSymlink(symlink string)

	// Asset lifecycle (used by multi-asset installs).
	AddAsset(name, fullName, symlink, installCmd string)
	SetCurrentAsset(index int)
	SetAssetInstallCmd(installCmd string)

	// Renders the current state ("" for modes that never render via View).
	View() string
}
```

- The old numeric trio `Start(total)/Update(current)/Finish()` is **replaced** by the stage-driven `Start()/Update(stage,...)` — the numeric names clash Go-signature-wise with PacmanUI's existing methods, and no call site actually drives a numeric progress loop. StandardProgressBar maps `stage` (0..6 install phases) → percent for its bar rendering.
- `DisableIcons` stays a field on PacmanUI. `NewProgressBar(mode, repo string, disableIcons bool)` takes it as a parameter and sets it on the pacman adapter; the interface exposes **no** icon method (standard/spinner/null don't need icons).
- `ui.GlobalPacman` assignment (release.go:1287) moves **inside** the pacman construction path (only set when mode == pacman); release.go no longer references it directly.
- `View()` returns `""` for `NullProgressBar` and the pacman adapter (pacman renders to stdout via its own goroutine).

### C. Modes
Final mode set: `pacman`, `standard`, `spinner:<style>` (dots, line, jump, pulse, points, miniDot, step), `conveyor`, `none`. **`gauge` is removed**:
- Remove `ProgressBarGauge` constant and its `IsValid()` branch (`params/params.go:17`, `:36`).
- Remove `gauge` from the `--progress-bar` flag help (`params/params.go:124`) and from the CLI/env/config `help` text.
- Drop the Scenario 4 (TTY with Gauge) test scenario.
- Delete `ui/gauge_progress.go` if it exists (it does not yet).

**`conveyor` is added** (new, per request): a package riding a conveyor belt.
- **Rendering**: a single line, ~40 chars wide (terminal width when TTY, else fixed 40):
  `▐█▌ ─────────────────────────────────────`
- The package `▐█▌` sits on top of the belt `───` and slides left→right as phase progress advances (stage 0..6 → 0..100%). An internal frame counter (incremented per `View()` call) slightly bobs the package and rotates the belt's roller glyphs so the belt reads as moving even at 0% progress.
- **Renderer**: `ui/conveyor_progress.go` — custom, pure `fmt`/`strings` (mirrors `ui/pacman.go` approach; **no new dependency**, no bubbles component).
- **Progress mapping**: `Update(stage,...)` sets `percent = stage/6`; the package position = percent across the belt. `AddAsset`/`SetCurrentAsset`/`SetAssetInstallCmd`/`UpdateSymlink` are no-ops.
- **Non-TTY + explicit `conveyor`**: behaves like `standard` (Phase 5) — print the rendered line on each newline at `Finish()`.
- `ConveyorProgressBar` implements the full interface (Amendment §B); `View()` returns the rendered belt line.

### D. Import path migration
`ui/standard_progress.go` and `ui/spinner_progress.go` currently import `github.com/charmbracelet/bubbles/progress` and `.../spinner` (bubbletea v1). Rewrite to `charm.land/bubbles/v2/progress` and `charm.land/bubbles/v2/spinner`; promote `charm.land/bubbles/v2` from `// indirect` to direct in `go.mod` (requires `make tidy` / `go mod tidy`).
- v2 drops the `bubbles`-specific `bubbles.KeyMap` concern; keep all behavior identical, only change the import path.
- `StandardProgressBar` keeps `progress.WithDefaultGradient()` and `progress.WithoutPercentage()` (verified present in v2).

### E. Resolution & factory (still missing)
- `resolveProgressBarMode(cliFlag, envVar, configVal string, isTTY bool) ProgressBarMode` — implement per original §Phase 2 step 5 (precedence: CLI > env > config > auto-detect TTY→pacman, non-TTY→none).
- `NewProgressBar(mode ProgressBarMode, repo string) ProgressBar` — pacman returns the existing `PacmanUI` wrapper; standard/spinner/none construct fresh; unknown modes fall back to `none`.
- Integration: `release.GithubRelease` currently hard-codes `ui.NewPacmanUI` (release/release.go:1282). Replace with the factory, resolving mode from `r.CliParams`/env/config.
- Keep `WaitForAnimation()` post-install ordering (release/release.go:1973) per Issue 4.

## Design

### Progress Bar Modes
We'll leverage charmbracelet's `bubbles` library for multiple out-of-box styles:

1. **pacman** - Custom animated pacman eating dots (existing behavior)
2. **standard** - Traditional progress bar `[=====>    ] 50%` (from `bubbles/progress`)
3. **spinner** - Spinning indicator with variants (from `bubbles/spinner`):
   - **dots** - Classic "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏" loading dots
   - **line** - Spinning line `|/-\`
   - **jump** - Jumping dots
   - **pulse** - Pulsing indicator
   - **points** - Animated points
   - **miniDot** - Small dots
   - **step** - Step indicator
4. **none** - No progress indication (silent mode)

### Configuration Hierarchy
**Precedence**: CLI flag > env var > config > auto-detect

1. **CLI flag**: `--progress-bar pacman|standard|spinner:dots|spinner:line|none`
2. **Environment variable**: `GH_PT_PROGRESS_BAR=pacman|standard|spinner:dots|none`
3. **Config file**: `progress_bar: pacman|standard|spinner:dots|none`
4. **Auto-detect**: 
   - TTY mode → `pacman` (default)
   - Non-TTY mode → `none` (default)

### Go Naming Conventions
- **Config field (YAML)**: `progress_bar` (snake_case)
- **Config field (Go struct)**: `ProgressBar` (PascalCase)
- **CLI flag**: `--progress-bar` (kebab-case)
- **Environment variable**: `GH_PT_PROGRESS_BAR` (SCREAMING_SNAKE_CASE)
- **Go type**: `ProgressBarMode` (string enum)

## Current Issues (from Handoff)

### Issue 1: Animation Accumulation
**Problem**: Animation lines accumulate instead of overwriting
**Root Cause**: 
- `render()` clears entire screen with `\033[2J` but then reprints header + assets
- In non-TTY mode, `Stop()` prints assets with newlines instead of overwriting
- Script runs with `sudo` and pipes output to file, so `term.IsTerminal()` returns false

### Issue 2: No Progress Bar Configuration
**Problem**: Users can't control progress bar behavior
**Requirements**:
- Config setting: `progress_bar` field in CoreConfig (default: auto-detect)
- Environment variable: `GH_PT_PROGRESS_BAR`
- CLI flag: `--progress-bar pacman|standard|none`
- Default: Auto-detect based on TTY

### Issue 3: Multiple Progress Bar Styles
**Problem**: Only pacman animation exists, no standard progress bar option
**Requirement**: Implement three modes: pacman, standard, none

### Issue 4: Progress Completion
**Problem**: Progress indication doesn't finish before showing final messages
**Requirement**: Progress bar must FINISH before showing final success/error messages

## Implementation Plan

### Phase 1: Type Definition
1. Define `ProgressBarMode` type in `params/params.go`
   ```go
   type ProgressBarMode string
   
   const (
       ProgressBarPacman   ProgressBarMode = "pacman"
       ProgressBarStandard ProgressBarMode = "standard"
       ProgressBarNone     ProgressBarMode = "none"
       // Spinner variants use "spinner:" prefix
       // e.g., "spinner:dots", "spinner:line", "spinner:jump", etc.
   )
   
   // Valid spinner styles
   var validSpinnerStyles = map[string]bool{
       "dots":    true,
       "line":    true,
       "jump":    true,
       "pulse":   true,
       "points":  true,
       "miniDot": true,
       "step":    true,
   }
   ```

2. Add validation method
   ```go
   func (m ProgressBarMode) IsValid() bool {
// Check base modes
        switch m {
        case ProgressBarPacman, ProgressBarStandard, ProgressBarNone:
            return true
        }
       
       // Check spinner variants
       if strings.HasPrefix(string(m), "spinner:") {
           style := strings.TrimPrefix(string(m), "spinner:")
           return validSpinnerStyles[style]
       }
       
       return false
   }
   
   func (m ProgressBarMode) String() string {
       return string(m)
   }
   ```

### Phase 2: Configuration
1. Add `progress_bar` field to `CoreConfig` in `config/config.go`
   ```go
   type CoreConfig struct {
       // ... existing fields ...
       ProgressBar string `yaml:"progress_bar" json:"progress_bar"`
   }
   ```

2. Set default value in config initialization
   ```go
   if cfg.Core.ProgressBar == "" {
       cfg.Core.ProgressBar = "auto" // will be resolved to pacman/none based on TTY
   }
   ```

3. Add `--progress-bar` flag to `CommonInstallFlags` in `params/params.go`
   ```go
ProgressBar string `kong:"name='progress-bar',help='Progress bar style: pacman, standard, spinner:<style>, or none. Spinner styles: dots, line, jump, pulse, points, miniDot, step'"`
    ```

4. Add environment variable support
   - Name: `GH_PT_PROGRESS_BAR`
   - Precedence: CLI flag > env var > config > auto-detect

5. Implement resolution logic in `cmd/root.go`
   ```go
   func resolveProgressBarMode(cliFlag, envVar, configVal string, isTTY bool) ProgressBarMode {
       // CLI flag takes precedence
       if cliFlag != "" {
           mode := ProgressBarMode(cliFlag)
           if mode.IsValid() {
               return mode
           }
           log.Warn("invalid progress bar mode, using default", "mode", cliFlag)
       }
       
       // Then env var
       if envVar != "" {
           mode := ProgressBarMode(envVar)
           if mode.IsValid() {
               return mode
           }
           log.Warn("invalid progress bar mode in env var, using default", "mode", envVar)
       }
       
       // Then config
       if configVal != "" && configVal != "auto" {
           mode := ProgressBarMode(configVal)
           if mode.IsValid() {
               return mode
           }
           log.Warn("invalid progress bar mode in config, using default", "mode", configVal)
       }
       
       // Auto-detect
       if isTTY {
           return ProgressBarPacman
       }
       return ProgressBarNone
   }
   ```

### Phase 3: Progress Bar Implementations
1. Create `ui/progressbar.go` with the full interface (see Amendment §B)
   ```go
   type ProgressBar interface {
       Start(total int)
       Update(current int)
       Finish()
       UpdateStage(stage int, version, archive, asset, target string)
       SetAsset(name, fullName, symlink, installCmd string)
       SetCurrentAsset(index int)
       SetAssetInstallCmd(installCmd string)
       UpdateSymlink(symlink string)
       View() string
   }
   ```

2. Implement `PacmanProgressBar` (adapter around existing `PacmanUI`)
   - Wrap existing pacman logic
   - Implement ProgressBar interface; `View()` returns `""`
   - Map `Start(total)`, `Update(current)`, `Finish()` to the stage-driven API (no-ops / `UpdateStage(1, ...)`) — **do not rename PacmanUI methods**

3. Implement `StandardProgressBar` using `charm.land/bubbles/v2/progress`
   ```go
   import "charm.land/bubbles/v2/progress"
   
   type StandardProgressBar struct {
       model progress.Model
       total int
   }
   
   func NewStandardProgressBar() *StandardProgressBar {
       return &StandardProgressBar{
           model: progress.New(progress.WithDefaultGradient(), progress.WithoutPercentage()),
       }
   }
   ```

4. Implement `SpinnerProgressBar` using `charm.land/bubbles/v2/spinner`
   ```go
   import "charm.land/bubbles/v2/spinner"
   
   type SpinnerProgressBar struct {
       model  spinner.Model
       style  string // dots, line, jump, pulse, points, miniDot, step
   }
   
   func NewSpinnerProgressBar(style string) *SpinnerProgressBar {
       var s spinner.Spinner
       switch style {
       case "line":
           s = spinner.Line
       case "jump":
           s = spinner.Jump
       case "pulse":
           s = spinner.Pulse
       case "points":
           s = spinner.Points
       case "miniDot":
           s = spinner.MiniDot
       case "step":
           s = spinner.Step
       default:
           s = spinner.Dot // default
       }
       return &SpinnerProgressBar{
           model: spinner.New(spinner.WithSpinner(s)),
           style: style,
       }
   }
   ```

5. (removed — `gauge` is dropped per Amendment §C; no `bubbles/gauge` exists in any version)

6. Implement `NullProgressBar` (no-op)
   ```go
   type NullProgressBar struct{}
   func (p *NullProgressBar) Start(total int) {}
   func (p *NullProgressBar) Update(current int) {}
   func (p *NullProgressBar) Finish() {}
   ```

7. Factory function
   ```go
   func NewProgressBar(mode ProgressBarMode, repo string) ProgressBar {
       switch mode {
       case "pacman":
           return NewPacmanProgressBar(repo) // adapter around PacmanUI
       case "standard":
           return NewStandardProgressBar()
       case "none":
           return &NullProgressBar{}
       default:
           // Handle spinner variants
           if strings.HasPrefix(string(mode), "spinner:") {
               style := strings.TrimPrefix(string(mode), "spinner:")
               return NewSpinnerProgressBar(style)
           }
           return &NullProgressBar{}
       }
   }
   ```

### Phase 4: Integration
1. Update `release/release.go` to use the factory
   - Replace direct `ui.NewPacmanUI(repo)` (release/release.go:1282) with `ui.NewProgressBar(mode, repo)`
   - Resolve `mode` via `resolveProgressBarMode` (CLI > env > config > auto-detect)
   - Call `UpdateStage()`, `SetAsset()`, `SetCurrentAsset()`, `SetAssetInstallCmd()`, `UpdateSymlink()` as today (unchanged call sites)
   - The `r.UI *ui.PacmanUI` field (release/release.go:56) becomes `*ui.ProgressBar` (or the factory type)

2. Ensure progress completion
   - Call `Finish()` before showing final messages
   - Wait for completion if needed

### Phase 5: Non-TTY Mode Handling
1. Detect non-TTY mode early
   - Check before creating progress bar
   - Force `none` mode if not TTY (unless explicitly set)

2. Handle each mode in non-TTY
   - `pacman`: Fall back to `none` or show simplified output
   - `standard`: Print progress updates on new lines
   - `none`: Silent

## Test Scenarios

### Scenario 1: TTY with Pacman Mode (Default)
```bash
gh-pt install owner/repo
# Expected: Pacman animation plays, completes, then shows success message
```

### Scenario 2: TTY with Standard Mode
```bash
gh-pt install owner/repo --progress-bar standard
# Expected: Shows [=====>    ] 50% style progress bar
```

### Scenario 3: TTY with Spinner Variants
```bash
# Default spinner (dots)
gh-pt install owner/repo --progress-bar spinner
# Expected: Shows ⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏ spinning dots

# Line spinner
gh-pt install owner/repo --progress-bar spinner:line
# Expected: Shows |/-\ spinning line

# Jump spinner
gh-pt install owner/repo --progress-bar spinner:jump
# Expected: Shows jumping dots animation

# Pulse spinner
gh-pt install owner/repo --progress-bar spinner:pulse
# Expected: Shows pulsing indicator

# Points spinner
gh-pt install owner/repo --progress-bar spinner:points
# Expected: Shows animated points

# MiniDot spinner
gh-pt install owner/repo --progress-bar spinner:miniDot
# Expected: Shows small dots animation

# Step spinner
gh-pt install owner/repo --progress-bar spinner:step
# Expected: Shows step indicator
```

### Scenario 4: TTY with None Mode
```bash
gh-pt install owner/repo --progress-bar none
# Expected: No progress indication, just final success message
```

### Scenario 5: Non-TTY (Piped Output) - Default
```bash
gh-pt install owner/repo | tee output.log
# Expected: No progress bar (auto-detected as none)
```

### Scenario 6: Non-TTY with Explicit Standard
```bash
GH_PT_PROGRESS_BAR=standard gh-pt install owner/repo | tee output.log
# Expected: Shows progress updates on new lines
```

### Scenario 7: Config File
```yaml
# ~/.config/gh-pt/config.yml
progress_bar: spinner:dots
```
```bash
gh-pt install owner/repo
# Expected: Uses dots spinner
```

### Scenario 8: Invalid Mode
```bash
gh-pt install owner/repo --progress-bar invalid
# Expected: Error message with valid options
```

### Scenario 9: Invalid Spinner Variant
```bash
gh-pt install owner/repo --progress-bar spinner:invalid
# Expected: Falls back to default spinner (dots)
```

### Scenario 10: Precedence Test
```yaml
# config.yml
progress_bar: pacman
```
```bash
GH_PT_PROGRESS_BAR=standard gh-pt install owner/repo --progress-bar spinner:line
# Expected: Uses spinner:line (CLI flag wins)
```

## Files to Modify

1. `params/params.go` - Add `ProgressBarMode` type and constants, add `--progress-bar` flag
2. `config/config.go` - Add `progress_bar` field to CoreConfig
3. `cmd/root.go` - Implement progress bar mode resolution logic
4. `ui/progressbar.go` - **EXISTS** - ProgressBar interface and factory function (extend interface per Amendment §B; add factory)
5. `ui/pacman_progress.go` - **NEW FILE** - PacmanProgressBar implementation (refactor from pacman.go)
6. `ui/standard_progress.go` - **EXISTS** - StandardProgressBar using charm.land/bubbles/v2/progress (migrate import from github.com/charmbracelet/bubbles/progress)
7. `ui/spinner_progress.go` - **EXISTS** - SpinnerProgressBar using charm.land/bubbles/v2/spinner (migrate import from github.com/charmbracelet/bubbles/spinner)
8. `ui/null_progress.go` - **NEW FILE** - NullProgressBar (no-op)
9. `release/release.go` - Update to use ProgressBar interface instead of direct PacmanUI
10. `ui/pacman.go` - Keep for now, refactor logic into pacman_progress.go later

## Success Criteria

- [ ] All progress bar modes work correctly (pacman, standard, spinner variants, none)
- [ ] Progress bar can be configured via config/env/flag
- [ ] Auto-detect works correctly (TTY = pacman, non-TTY = none)
- [ ] Progress bar completes before final messages
- [ ] Non-TTY mode doesn't accumulate lines
- [ ] Spinner variants work correctly (dots, line, jump, pulse, points, miniDot, step)
- [ ] Invalid mode handling with clear error messages
- [ ] All existing tests pass
- [ ] New tests added for progress bar modes
- [ ] Documentation updated

## Risk Assessment

### High Risk
- **Breaking existing behavior**: Users might rely on current pacman animation
  - **Mitigation**: Default to auto-detect (TTY = pacman, non-TTY = none)
  - **Mitigation**: Preserve visual appearance of pacman mode

### Medium Risk
- **Performance overhead**: Multiple progress bar implementations
  - **Mitigation**: Profile and optimize each implementation
  - **Mitigation**: NullProgressBar is zero-cost for none mode

### Low Risk
- **Complexity increase**: Multiple implementations
  - **Mitigation**: Shared interface keeps code clean
  - **Mitigation**: Factory pattern hides complexity
- **Config precedence confusion**: Users might not understand priority
  - **Mitigation**: Clear documentation and help text
- **Performance impact**: Waiting for animation might slow down scripts
  - **Mitigation**: Add timeout, allow disabling in scripts

### Low Risk
- **Config precedence confusion**: Users might not understand priority
  - **Mitigation**: Clear documentation and help text

## Timeline
- Phase 1 (Type Definition & Configuration): 2 hours
- Phase 2 (Resolution Logic): 1 hour
- Phase 3 (Progress Bar Implementations): 3 hours
  - PacmanProgressBar: 1 hour
  - StandardProgressBar (bubbles/v2/progress): 1 hour
  - SpinnerProgressBar (bubbles/v2/spinner variants): 1 hour
  - NullProgressBar + Factory: 1 hour
- Phase 4 (Integration & Non-TTY Fix): 2 hours
- Phase 5 (Testing): 2 hours
- **Total**: 10 hours (1.25 days)
