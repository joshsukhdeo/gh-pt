# Spec: Core Architectural Optimizations & Guardrails

## Objective
To harden the existing `gh-pt` deployment and update lifecycles against corruption, sequential bottlenecks, network rate limits, and headless timeout failures.

## 1. Atomic Swaps (The Corrupted Binary Trap)
**Context:** Currently, `installBinary` and `installArchivedBinary` in `release/release.go` write directly to the final `destinationPath` during extraction/download. If the process is interrupted (SIGINT or disk full), a corrupted half-written binary is left in `~/.local/bin`.
**Requirement:** 
- Implement atomic swaps for all binary installations.
- Write the incoming `io.Copy` stream to `destinationPath + ".tmp"`.
- Execute `os.Chmod` on the `.tmp` file.
- Finally, use `os.Rename(destinationPath + ".tmp", destinationPath)` to guarantee the binary is never exposed to the system in a broken state.

## 2. API Rate Limit Evasion (GraphQL Batching)
**Context:** When a user with dozens of tracked applications runs `gh pt upgrade`, `gh-pt` makes `N` sequential REST API calls to `repos/%s/releases` to check for updates, risking rate-limits.
**Requirement:**
- Refactor the update check logic to leverage the built-in `go-gh` GraphQL client (`github.com/cli/go-gh/v2/pkg/api`).
- Dynamically construct a single GraphQL query using repository aliases to fetch the latest release tag for *every* repository tracked in `state.json` in exactly one network round-trip.

## 3. Concurrent Update Sweeps (`cmd/update.go`)
**Context:** `DoUpdate` currently iterates over tracked apps synchronously.
**Requirement:**
- Integrate `golang.org/x/sync/errgroup` to decouple the update sweep.
- Spin up a worker pool to concurrently evaluate the GraphQL batch results and filter out apps that do not require updates.
- Concurrently download and extract updates for the remaining apps.
- Maintain strict atomic thread-safety by only locking and writing to `state.json` once during the final batch transaction.

## 4. Headless Sudo Cache Expiration
**Context:** In `cmd/root.go`, `sudo -v` is called globally to prime the credential cache. However, `sudo` timestamps typically expire after 15 minutes. Long-running source compilations or massive archive extractions will cause the cache to drop, causing headless pipelines (`-D`) to hang indefinitely when `gh-pt` eventually attempts to execute an elevated command.
**Requirement:**
- If the command lifecycle requires elevated privileges (e.g., `--global` installation or native package manager usage via `--resolve-deps`), spawn a background goroutine.
- The goroutine must quietly execute `sudo -v` every 60 seconds for the lifespan of the `gh-pt` process.
- Ensure the goroutine is cleanly closed via context cancellation when the installation is complete.

## Success Criteria
1. Interrupting `gh-pt install` via `Ctrl+C` midway through extraction leaves no corrupted binaries in the target path.
2. `gh pt upgrade` completes its version-check sweep via a single GraphQL request rather than iterative REST calls.
3. Multiple application updates download and extract concurrently using `errgroup`.
4. Long-running source compilations (>15 mins) that subsequently execute `sudo make install` do not prompt for a password or hang.
