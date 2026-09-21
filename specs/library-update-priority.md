# Spec: Library Update Priority Strategy

## Objective
Establish a clear priority order for library updates and bug fixing: update libraries FIRST, then fix bugs (both persisting and new).

## Rationale
- Modern libraries provide better foundations for bug fixes
- Newer versions may already fix bugs we're tracking
- Reduces rework: fix bugs once on the latest library versions
- Improves code quality and maintainability before adding features

## Priority Order

### Phase 1: Library Updates (Priority: HIGH)
1. **Core Animation Libraries** (ALREADY DONE)
   - [x] bubbletea v2.0.9
   - [x] huh v2.0.3
   - [x] harmonica v0.2.0
   - [x] httpmock v1.4.2
   - [x] clock v1.3.5
   - [x] go-cmp v0.7.0
   - [x] mock v0.6.0

2. **Logger Migration** (IN PROGRESS)
   - [ ] Replace zerolog with charmbracelet/log
   - [ ] Update all 19 files
   - [ ] Remove zerolog dependency
   - [ ] Verify tests pass

3. **Additional Library Updates** (PENDING)
   - [ ] Review other dependencies for updates
   - [ ] Update any outdated packages
   - [ ] Run `go mod tidy` and verify

### Phase 2: Bug Fixing (Priority: HIGH - AFTER LIBRARIES)
1. **Pacman Animation Bugs** (from handoff)
   - [ ] Fix accumulation in non-TTY mode
   - [ ] Add `animation` config option (default: false)
   - [ ] Add `GH_PT_ANIMATION` env var
   - [ ] Add `--animation` CLI flag
   - [ ] Show arrows `-->` in disabled mode
   - [ ] Ensure animation finishes before final message

2. **Post-Migration Bugs**
   - [ ] Any bugs introduced by library updates
   - [ ] Any bugs discovered during testing
   - [ ] Regressions from logger migration

### Phase 3: Feature Implementation (Priority: MEDIUM)
1. **Bubble Tea v2 Pacman Rewrite**
   - [ ] Rewrite pacman.go using bubbletea v2 Model
   - [ ] Add harmonica physics for smooth animation
   - [ ] Add comprehensive tests
   - [ ] Verify in TTY and non-TTY modes

2. **Additional Features**
   - [ ] Any features blocked by library updates
   - [ ] Improvements enabled by new libraries

## Success Criteria
- All libraries updated to latest stable versions
- All existing tests pass
- No regressions introduced
- Pacman animation works correctly in all modes
- Code is cleaner and more maintainable

## Risk Mitigation
- Test after each library update
- Keep git history clean with logical commits
- Document any breaking changes
- Have rollback plan for each library

## Timeline
- Phase 1 (Libraries): 1-2 days
- Phase 2 (Bugs): 1-2 days
- Phase 3 (Features): 2-3 days
- Total: 4-7 days
