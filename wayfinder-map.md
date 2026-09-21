## Destination

Implement comprehensive UI enhancements (Pacman progress bar, state icons across all list/show commands) and robust semantic status feedback logic (Downgrades, Reinstalls, ZEROGRADE, Barbarous) while supporting an exhaustive integration test suite for CLI parameters.

## Notes

- Domain: Go CLI testing, `pterm` UI progress bars, `alecthomas/kong` parser behavior, integration/E2E testing.
- The `--test` flag has been introduced to securely verify parsed parameters without mock API requests, unlocking scaleable integration testing.
- We are using the local-markdown tracker format for this map.

## Decisions so far

- **Integration Testing & Mocking Strategy**: Added a hidden `--test` flag that outputs the parsed internal config (JSON) and safely exits. This resolves the complexity of integration testing without requiring complex GitHub API mocks.
- **Status Message Matrix**: Built `status` package for semantic feedback (`GenerateStatusMessage`). Dynamically handles `ZEROGRADE`, `UPGRADE`, `DOWNGRADE`, `ADOPTED`, and maps `--force` plus downgrade flags to the appropriate abort strings. Integrated directly into `release.go` for all commands.
- **Flag Cascading Logic**: Specialty flags (`--BARBAROUS`, `--LE-RETROGROUCH`, `--retrograde-stopgap`, `--self-inflicted-technical-debt`) correctly cascade across `cmd/root.go`, `cmd/update.go`, and `release/release.go` to manipulate verification, sandboxing, Wine, downgrade permissions, and atomic state pinning.
- **Pacman UI & Status Feedback**: Linked the install lifecycle and ghost modes (`🍒`, `👻`, `ᗣ`) into `PacmanUI` with clean status reporting upon command completion.
- **Icon Propagation**: Universal propagation of `GetStateIndicator()` across `ls`, `ll`, `install`, `upgrade`, and `source` with complete emoji (`📌🗻🧪🎯`) and text fallback (`^*!@`) parity.
- **CLI Parameter Integration Test Suite**: Implemented `params/cli_integration_test.go` exercising all subcommands, sidecar flags, parameter cascades, and JSON serialization using the `--test` harness.

## Frontier

**All tickets resolved. Map complete.**

## Blocked

*(None)*

## Not yet specified

*(None)*

## Out of scope

- Unit testing internal extraction logic (this is strictly for CLI routing, state feedback, and UI output).
