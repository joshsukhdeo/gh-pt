## Destination

Rename the project and CLI from `gh-pt` to `gh-pt` (supporting `gh pt`, `ghpt`), refactor the CLI from flat flags to nested subcommands (`state ls`, `config set`, `repo clone`, etc.), split the XDG state files to separate standard binaries from source/cloned repos, build TUI editors for config/state, and implement AI/VirusTotal repository scanning.

## Notes

- **Domain**: Package management, Go CLI, `alecthomas/kong`, GitHub extensions.
- **Skills**: ponytail (minimalism, shortest path), tdd (seam testing for CLI routing), subagent-orchestrator (for running parallel refactors).
- **Standing Preferences**: Keep the CLI backwards-compatible enough so we don't break existing saved states. We need a clean migration path from `~/.local/share/gh-pt` to `~/.local/share/gh-pt`.
- The user handles the cloud repo rename via GitHub web UI or `gh repo rename`; the map handles the codebase, module, and CLI logic.

## Decisions so far

*(Empty)*

## Open Tickets

- [Ticket 1: Codebase and Namespace Renaming](#ticket-1) (Frontier)
- [Ticket 2: CLI Command Router Refactor](#ticket-2) (Blocked by 1)
- [Ticket 3: State File Splitting and Migration Architecture](#ticket-3) (Blocked by 1)
- [Ticket 4: TUI Interactive Editors for Config and State](#ticket-4) (Blocked by 2, 3)
- [Ticket 5: VirusTotal and AI Scan Engine](#ticket-5) (Blocked by 2)

## Not yet specified

- **AI Safety Scan**: What model is used for `scan --ai`? Is it an external API call, or a local heuristic? How does it report "safety" without burning massive tokens?
- **Fork Update Behavior**: The user mentioned "implement a different behavior for forked if a different behavior should occur instead". We need to define exactly what updating a fork means (e.g., syncing upstream vs pulling origin).

## Out of scope

- Renaming the physical GitHub repository remotely without user interaction (the user must run `gh repo rename gh-pt`).

---

### Ticket 1: Codebase and Namespace Renaming
**Question:** How do we safely rename the `go.mod` module path, all internal imports, the binary name, and the XDG data/config paths from `gh-pt` to `gh-pt`, while leaving a migration hook for old `gh-pt` state?

### Ticket 2: CLI Command Router Refactor
**Question:** How do we transition `alecthomas/kong` from a flat flag structure (`--ls`, `--clone`) to a nested subcommand tree (`gh-pt state ls`, `gh-pt repo clone`, `gh-pt config set`) while supporting aliases like `gh pt` and `ghpt`, and handling default args as the standard install action?

### Ticket 3: State File Splitting and Migration Architecture
**Question:** How do we split the existing `state.json` into standard installs vs cloned/forked/source items, and what does the data schema look like now that we must persist `--max-depth {int}`?

### Ticket 4: TUI Interactive Editors for Config and State
**Question:** What `pterm` interactive UI components will we use to allow the user to gracefully edit `config.yml` and `state.json` directly from the terminal without breaking the schema?

### Ticket 5: VirusTotal and AI Scan Engine
**Question:** How does `gh-pt scan --vt` hook into our existing VirusTotal logic, and how does it retroactively update the saved state for currently installed apps?
