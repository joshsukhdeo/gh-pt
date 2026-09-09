# Map

## Destination
Add informational display flags (`--show`, `--show-assets`, `--show-versions`) with visual state indicators, and introduce `--prerelease` / `--stable` modes that persist in state and govern fetching, updating, and list filtering.

## Notes
- **Domain**: Go CLI (`alecthomas/kong`), GitHub REST API (`cli/go-gh`), JSON state management.
- **Rules**: Kong mutual exclusivity groups for `--show*` flags. Emoji icons (`📌🗻⚡🎯`) or text fallbacks (`^*!@`) based on config.
- **Skills**: ponytail, tdd, golang-cli, golang-spf13-cobra (actually we use kong here, so standard struct tags).

## Decisions so far
- [Ticket 1: Flag and Schema Definitions](#ticket-1): Added `--show*`, `--prerelease`, `--stable` to params, config, and state schema.

## Open Tickets
- [Ticket 2: State Tracking and List Filtering](#ticket-2) (Frontier)
- [Ticket 3: Visual Indicators and Config](#ticket-3) (Blocked by 2)
- [Ticket 4: Implementation of Show Commands](#ticket-4) (Blocked by 3)
- [Ticket 5: Install and Update Fetch Logic](#ticket-5) (Blocked by 2)

## Not yet specified
- How exactly the TUI/output formatting code will layout the aligned columns when emojis vs text are used (since emojis have different cell widths).
- Handling the case where a pinned app is updated while `--prerelease` is set - does it break the pin, or fail?

## Out of scope
- Changing the default interactive selector's behavior (it already shows all releases).

---

### Ticket 1: Flag and Schema Definitions (Task)
**Question**: Add `--show`, `--show-assets`, `--show-versions`, `--prerelease`, and `--stable` to `params.go` using Kong's mutex groups so they can't be combined with install commands. Also add `IsPrerelease` to the `StateEntry` struct.

### Ticket 2: State Tracking and List Filtering (Task)
**Question**: Update `state.go` to save the `IsPrerelease` boolean during installation. Modify the `ls` and `ll` commands to accept and apply the `--prerelease` and `--stable` filters.

### Ticket 3: Visual Indicators and Config (Task)
**Question**: Add `allow_prerelease` and `disable_icons` (or similar) to `config.go`. Create a helper function `GetStateIndicator(repo, version)` that returns the `📌🗻⚡🎯` or `^*!@` prefix for a given version string.

### Ticket 4: Implementation of Show Commands (Task)
**Question**: Implement the API fetch logic for `--show-versions` (all versions), `--show` (max 10 versions, max 50 assets of latest/selected), and `--show-assets` (all assets of latest/selected). Format the output using the indicator helper.

### Ticket 5: Install and Update Fetch Logic (Task)
**Question**: Update `release.go` to respect `--prerelease` and `--stable`. If `--prerelease` is active, fetch from `/releases` instead of `/releases/latest` and pick the newest prerelease. Ensure `update` replays the `IsPrerelease` state correctly.
