# Logger Migration: zerolog → charmbracelet/log

## Objective
Replace `github.com/rs/zerolog` with `github.com/charmbracelet/log` across the entire gh-pt codebase to improve CLI user experience and align with the charmbracelet ecosystem.

## Rationale
- **gh-pt is a CLI tool** where user experience matters
- **Already using charm ecosystem** - bubbletea v2, lipgloss v2 are direct dependencies
- **Performance is irrelevant** - CLI tools don't log at high throughput
- **Better out-of-box aesthetics** - charmbracelet/log provides styled terminal output without ConsoleWriter overhead
- **Ecosystem alignment** - seamless integration with bubbletea and other charm tools

## Current State
- **Logger**: `github.com/rs/zerolog` (v1.35.0)
- **Files affected**: 19 Go files
- **Key locations**:
  - `cmd/root.go` - Logger initialization and configuration
  - `cmd/state_mgmt.go` - State management logging
  - `cmd/update.go` - Update command logging
  - `release/release.go` - Release installation logging
  - `release/symlink.go` - Symlink creation logging
  - `release/virustotal.go` - VirusTotal scanning logging
  - `selector/interface.go` - Asset selection logging
  - `selector/magic.go` - Magic number detection logging
  - `ui/pacman.go` - Animation logging (uses slog, not zerolog)

## Migration Strategy

### Phase 1: Specification & Planning
- [x] Create migration spec (this document)
- [ ] Define API mapping (zerolog → charmbracelet/log)
- [ ] Identify test boundaries
- [ ] Map execution roadmap with wayfinder

### Phase 2: Implementation
- [ ] Update `cmd/root.go` logger initialization
- [ ] Migrate all logging calls across 18 files
- [ ] Update any zerolog-specific features (Dict, Event, etc.)
- [ ] Remove zerolog from go.mod

### Phase 3: Verification
- [ ] All existing tests pass
- [ ] Build succeeds without errors
- [ ] Manual testing of CLI output
- [ ] Verify log levels work correctly
- [ ] Verify file logging still works

## API Mapping

### zerolog → charmbracelet/log

| zerolog Pattern | charmbracelet/log Pattern |
|----------------|---------------------------|
| `log.Info().Msg("message")` | `log.Info("message")` |
| `log.Info().Str("key", "val").Msg("message")` | `log.Info("message", "key", "val")` |
| `log.Debug().Int("count", 5).Msg("counting")` | `log.Debug("counting", "count", 5)` |
| `log.Error().Err(err).Msg("failed")` | `log.Error("failed", "error", err)` |
| `log.Warn().Msg("warning")` | `log.Warn("warning")` |
| `log.Fatal().Msg("fatal")` | `log.Fatal("fatal")` |
| `log.Panic().Msg("panic")` | `log.Fatal("panic")` (charmbracelet/log has no Panic) |

### Configuration

**zerolog:**
```go
zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
zerolog.SetGlobalLevel(zerolog.DebugLevel)
log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
```

**charmbracelet/log:**
```go
log.SetLevel(log.DebugLevel)
log.SetOutput(os.Stdout)
// Styles are automatic
```

### Special Cases

**zerolog.Dict():**
```go
// zerolog
log.Info().Dict("renaming", zerolog.Dict().Str("old", old).Str("new", new)).Msg("renamed")

// charmbracelet/log - use structured key-value pairs
log.Info("renamed", "old", old, "new", new)
```

**zerolog.Event:**
```go
// zerolog
log.Info().Func(func(e *zerolog.Event) {
    e.Str("key", "value")
}).Msg("message")

// charmbracelet/log - direct key-value
log.Info("message", "key", "value")
```

## Test Boundaries

### What MUST be tested:
1. **Logger initialization** - Verify logger is created with correct level
2. **Log output** - Verify logs appear in expected format
3. **Log levels** - Verify Debug/Info/Warn/Error/Fatal work correctly
4. **File logging** - Verify logs can be written to files
5. **TTY detection** - Verify behavior differs for TTY vs non-TTY

### What does NOT need testing:
- Internal charmbracelet/log implementation
- Styling/rendering (that's the library's job)
- Performance (not a concern for CLI tools)

## Success Criteria

### Functional Requirements:
- [ ] All log statements compile without errors
- [ ] All existing tests pass
- [ ] CLI output is readable and styled
- [ ] Log levels work correctly (debug, info, warn, error, fatal)
- [ ] File logging works when configured
- [ ] Non-TTY mode produces plain output

### Non-Functional Requirements:
- [ ] No performance regression (acceptable for CLI)
- [ ] Binary size increase < 5MB
- [ ] No breaking changes to CLI interface
- [ ] All log messages preserved (no information loss)

## Risk Assessment

### High Risk:
- **Breaking existing log parsing** - If external tools parse gh-pt logs
  - **Mitigation**: Verify no external log parsing exists
  
### Medium Risk:
- **Missing log statements** - Some logs might be missed in migration
  - **Mitigation**: Grep for all zerolog usage, verify 100% coverage
  
### Low Risk:
- **Different log format** - Users might notice different output
  - **Mitigation**: This is the goal - better aesthetics
- **Performance regression** - charmbracelet/log is slower
  - **Mitigation**: Not a concern for CLI tools

## Rollback Plan

If migration fails:
1. Revert all changes
2. Restore zerolog dependency
3. Investigate failure cause
4. Re-attempt with fixes

## Dependencies

### New Dependencies:
- `github.com/charmbracelet/log` v1.0.0

### Removed Dependencies:
- `github.com/rs/zerolog` v1.35.0

### Transitive Dependencies:
- charmbracelet/log will pull in:
  - `github.com/go-logfmt/logfmt` (for logfmt output)
  - `charm.land/lipgloss/v2` (already present)
  - `github.com/muesli/termenv` (already present)

## Timeline

- **Phase 1 (Spec & Planning)**: 30 minutes
- **Phase 2 (Implementation)**: 2-3 hours
- **Phase 3 (Verification)**: 1 hour
- **Total**: 3.5-4.5 hours

## Notes

- This migration is a **one-way door** - once done, reverting is costly
- The migration should be done in a **single PR** to avoid merge conflicts
- Consider doing this **after** the bubbletea v2 migration is complete
- This is **not** a performance optimization - it's a UX improvement
