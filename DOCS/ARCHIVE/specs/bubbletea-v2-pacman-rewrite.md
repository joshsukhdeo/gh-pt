# Spec: Bubble Tea v2 Pacman Animation Rewrite

## Objective
Rewrite the pacman animation using bubbletea v2's Model-Update-View pattern with harmonica physics for smooth, testable animation.

## Background
The current pacman animation in `ui/pacman.go` is 678 lines of manual terminal control with:
- Manual goroutine management
- Raw ANSI escape codes
- Complex state machine with mutex locks
- Difficult to test (requires capturing stdout)
- Fragile cursor control logic

## Why Bubble Tea v2?
1. **Structured concurrency** - Built-in event loop, no manual goroutines
2. **Testable** - Pure functions, easy to test without terminal
3. **Composable** - Can integrate with other bubbletea components
4. **Maintainable** - Clear separation of concerns (Model/Update/View)
5. **Physics** - Harmonica integration for smooth animations

## Current Architecture

```
PacmanUI struct (678 lines)
├── State fields (phase, headerEaten, horizontalPos, etc.)
├── Mutex for thread safety
├── Goroutine with ticker
├── tick() - state machine
├── render() - ANSI escape codes
├── renderPhase0/1/2() - phase-specific rendering
└── Public API (Start, Stop, Update, AddAsset, etc.)
```

## Target Architecture

```
Model (bubbletea.Model)
├── State (phase, positions, assets)
├── Physics (harmonica spring)
├── Update(msg) - pure state transitions
├── View() - pure rendering
└── Messages (TickMsg, AssetResolvedMsg, etc.)

PacmanRunner (orchestrator)
├── tea.Program wrapper
├── Public API (Start, Stop, Update, AddAsset)
├── Message sending
└── TTY detection
```

## Implementation Plan

### Phase 1: Model Definition
1. Define `Model` struct implementing `tea.Model`
   - State fields (phase, positions, assets)
   - Physics (harmonica spring for smooth movement)
   - Terminal dimensions (width, height)

2. Define message types
   - `TickMsg` - animation tick
   - `AssetResolvedMsg` - asset name resolved
   - `AssetCompletedMsg` - asset installation complete
   - `WindowSizeMsg` - terminal resize (built-in)

3. Implement `Init() tea.Cmd`
   - Return initial command (start ticker)

### Phase 2: Update Function
1. Implement `Update(msg tea.Msg) (tea.Model, tea.Cmd)`
   - Handle `TickMsg` - advance animation state
   - Handle `AssetResolvedMsg` - update asset info
   - Handle `AssetCompletedMsg` - mark asset complete
   - Handle `WindowSizeMsg` - update dimensions
   - Handle `tea.KeyMsg` - quit on 'q' or ctrl+c

2. Implement phase transitions
   - Phase 0 (Header): eating through repo/version/archive
   - Phase 1 (Horizontal): moving right, wrapping
   - Phase 2 (Assets): processing each asset

3. Add harmonica physics
   - Use spring for smooth horizontal movement
   - Update position based on velocity and target

### Phase 3: View Function
1. Implement `View() string`
   - Render header (repo, version, archive)
   - Render current phase animation
   - Render asset list with progress

2. Use lipgloss for styling
   - Colorize pacman (yellow)
   - Colorize dots (gray)
   - Colorize completed assets (green)

3. Handle terminal width
   - Truncate long lines
   - Wrap if necessary

### Phase 4: Runner/Orchestrator
1. Create `PacmanRunner` struct
   - Wraps `tea.Program`
   - Provides public API matching current `PacmanUI`
   - Handles TTY detection

2. Implement public methods
   - `Start()` - start tea.Program in alt screen
   - `Stop()` - quit program, wait for cleanup
   - `Update()` - send messages to model
   - `AddAsset()` - send AssetResolvedMsg

3. Handle non-TTY mode
   - Detect TTY before starting
   - If non-TTY, skip animation, show final state only

### Phase 5: Integration
1. Replace `PacmanUI` usage in `release/release.go`
   - Update all call sites
   - Ensure message sending works correctly
   - Verify animation completion before final messages

2. Update tests
   - Test Model.Update() with various messages
   - Test Model.View() output
   - Test phase transitions
   - Test asset completion

## API Compatibility

### Current API (must preserve)
```go
ui := NewPacmanUI(repo)
ui.Start()
ui.Update(stage, version, archive, asset, target, ghostType)
ui.AddAsset(name, fullName, symlink, installCmd)
ui.Stop()
```

### New API (internal)
```go
model := NewModel(repo)
runner := NewPacmanRunner(model)
runner.Start()
runner.SendAssetResolved(name, fullName, symlink, installCmd)
runner.SendAssetCompleted()
runner.Stop()
```

## Physics Integration

### Harmonica Spring
```go
spring := harmonica.NewSpring(harmonica.FPS(60), 6.0, 0.5)

// In Update:
pos, vel := spring.Update(pos, vel, targetPos)
```

### Animation Smoothing
- Horizontal movement: spring-based easing
- Dot consumption: discrete steps (no physics needed)
- Phase transitions: instant (no physics needed)

## Testing Strategy

### Unit Tests (Model)
1. Test phase transitions
   - Header → Horizontal → Assets
   - Verify state changes correctly

2. Test asset processing
   - Asset resolved → update model
   - Asset completed → mark complete, advance

3. Test physics
   - Spring updates position correctly
   - Wrapping at terminal edge

4. Test View output
   - Correct rendering for each phase
   - Handles terminal width

### Integration Tests (Runner)
1. Test TTY detection
2. Test message sending
3. Test start/stop lifecycle

### Manual Tests
1. Run in TTY mode
2. Run in non-TTY mode (piped)
3. Run with animation disabled
4. Test with multiple assets
5. Test with long asset names

## Performance Considerations

### Memory
- Model should be lightweight (< 1KB)
- Assets stored as slice of structs
- No unnecessary allocations in Update/View

### CPU
- Update called at 60 FPS (16.67ms per frame)
- View should render in < 5ms
- Physics calculations are O(1)

### Terminal
- Use alt screen mode (no scrollback pollution)
- Clear screen on exit
- Handle resize gracefully

## Migration Checklist

- [ ] Create Model struct with tea.Model interface
- [ ] Implement Init(), Update(), View()
- [ ] Define message types
- [ ] Add harmonica physics
- [ ] Create PacmanRunner wrapper
- [ ] Implement public API (Start, Stop, Update, AddAsset)
- [ ] Handle TTY detection
- [ ] Replace PacmanUI in release.go
- [ ] Update all call sites
- [ ] Write unit tests for Model
- [ ] Write integration tests for Runner
- [ ] Manual testing in TTY mode
- [ ] Manual testing in non-TTY mode
- [ ] Performance profiling
- [ ] Documentation

## Risk Assessment

### High Risk
- **Breaking existing behavior**: Users might notice differences
  - **Mitigation**: Preserve visual appearance as much as possible
  - **Mitigation**: Test extensively with real installations

### Medium Risk
- **Performance regression**: Bubble tea overhead
  - **Mitigation**: Profile and optimize View() function
  - **Mitigation**: Use efficient string building

### Low Risk
- **Learning curve**: Team needs to learn bubbletea
  - **Mitigation**: Code is well-documented
  - **Mitigation**: bubbletea has excellent docs

## Timeline
- Phase 1 (Model): 2 hours
- Phase 2 (Update): 3 hours
- Phase 3 (View): 2 hours
- Phase 4 (Runner): 2 hours
- Phase 5 (Integration): 3 hours
- Testing: 3 hours
- **Total**: 15 hours (2 days)

## Success Criteria

- [ ] All existing tests pass
- [ ] New tests added for Model and Runner
- [ ] Animation works in TTY mode
- [ ] Animation disabled in non-TTY mode
- [ ] Animation can be controlled via config/env/flag
- [ ] Performance is acceptable (< 5ms per frame)
- [ ] Code is cleaner and more maintainable
- [ ] No regressions in installation flow
