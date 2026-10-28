# SPEC-history-and-rollback: 30-Day Update History Database & Rollbacks

## Status
Approved

## Context & Motivation
`gh-pt` manages binary installations, upgrades, and source builds across multiple platforms. Previously, `state/history.go` provided only a fire-and-forget append-only log file (`history.jsonl`). Users had no structured mechanism to:
1. Query update actions over the preceding 30 days.
2. Recover or roll back to a previously installed binary version when an update introduces a regression or breaks functionality.
3. Automatically retain and clean up backup binaries within a strictly enforced 30-day retention window.

## Objectives
1. Implement a structured JSON history database (`$XDG_DATA_HOME/gh-pt/history.json`) guarded by file-level locking (`history.lock`).
2. Maintain a strict 30-day rolling retention window: updates older than 30 days and their associated cached backup binaries are automatically pruned.
3. Automatically snapshot and backup existing binaries prior to an update or overwrite.
4. Provide a rollback engine that restores binaries, restores previous `InstalledApp` state configurations, and records the rollback event in the history database.
5. Provide a first-class CLI command set:
   - `gh-pt history [repository] [--days 30] [--format table|json] [--clean]`
   - `gh-pt rollback <repository> [--to <version>] [--dry-run] [--force]`
   - Aliased under `gh-pt state history` and `gh-pt state rollback`.

## Data Architecture

### History Database Schema
Stored at: `$XDG_DATA_HOME/gh-pt/history.json`
Lock file: `$XDG_DATA_HOME/gh-pt/history.lock`

```go
type HistoryRecord struct {
    ID              string            `json:"id"`
    Timestamp       time.Time         `json:"timestamp"`
    Action          string            `json:"action"` // "install", "update", "rollback", "remove"
    Repository      string            `json:"repository"`
    PreviousVersion string            `json:"previous_version,omitempty"`
    NewVersion      string            `json:"new_version,omitempty"`
    AppSnapshot     *InstalledApp     `json:"app_snapshot,omitempty"`
    BackupFiles     map[string]string `json:"backup_files,omitempty"` // originalPath -> backupPath
    Success         bool              `json:"success"`
    Message         string            `json:"message,omitempty"`
}

type HistoryDB struct {
    Version int             `json:"version"`
    Records []HistoryRecord `json:"records"`
}
```

### Backups Directory
Stored at: `$XDG_DATA_HOME/gh-pt/backups/<sanitized-repo>/<version>/<binary-name>`
Permissions: `0755` (executable)

### Retention Policy
- Constant: `DefaultHistoryRetention = 30 * 24 * time.Hour` (30 days).
- On any history mutation (record update, query with prune, explicit clean), any record with `time.Since(record.Timestamp) > retention` is pruned.
- When a record is pruned, all files referenced in `record.BackupFiles` are unlinked from disk, preventing unbounded storage growth.

## Rollback Lifecycle
1. User invokes `gh-pt rollback <repo> [--to <version>]`.
2. Ensure repository exists in `state.json`.
3. Locate matching history record:
   - If `--to` is specified, find the record matching that version.
   - If `--to` is omitted, find the most recent record providing a `PreviousVersion`.
4. Validate rollback artifacts:
   - If `BackupFiles` exist on disk, restore each binary to its target destination path.
   - If `BackupFiles` are absent on disk, indicate fallback reinstall requirement (`gh-pt <repo> -v <version> --force`).
5. Revert `state.json` via `state.Mutate`:
   - If `AppSnapshot` is recorded, restore original configuration flags, drivers, and sidecars.
   - Set `st.Apps[repo].Version = targetVersion`.
6. Record new `Action: "rollback"` record into history database.
7. Print formatted confirmation to the user.

## Core Invariants
1. **Locking**: All history DB operations MUST acquire `history.lock` via `flock.New`.
2. **State Transactionality**: Any state modifications during rollback MUST execute through `state.Mutate`.
3. **Test Isolation**: In tests, set `XDG_DATA_HOME` via `t.TempDir()` and call `xdg.Reload()`.
4. **Clean Exit**: Non-interactive commands report errors with clean non-zero exit codes; never panic or loop.
