## Destination

Implement comprehensive UI enhancements (Pacman progress bar, state icons across all list/show commands) and robust semantic status feedback logic (Downgrades, Reinstalls, ZEROGRADE, Barbarous) while supporting an exhaustive integration test suite for CLI parameters.

## Notes

- Domain: Go CLI testing, `pterm` UI progress bars, `alecthomas/kong` parser behavior, integration/E2E testing.
- The `--test` flag has been introduced to securely verify parsed parameters without mock API requests, unlocking scaleable integration testing.
- We are using the local-markdown tracker format for this map.

## Decisions so far

- **Integration Testing & Mocking Strategy**: Added a hidden `--test` flag that outputs the parsed internal config (JSON) and safely exits. This resolves the complexity of integration testing without requiring complex GitHub API mocks.
- **Status Message Matrix**: Built `status` package for semantic feedback (`GenerateStatusMessage`). Dynamically handles `ZEROGRADE`, `UPGRADE`, `DOWNGRADE`, `ADOPTED`, and maps `--force` plus downgrade flags to the appropriate abort strings. Integrated directly into `release.go` for all commands.

## Not yet specified

- **Flag Cascading Logic**: Map the new specialty flags (`---LE-RETROGROUCH`, `---retrograde-stopgap`, `---BARBAROUS`, etc.) into internal properties (e.g. `Barbarous` turning off checksums, allowing wine, skipping VT).
- **Pacman UI Animation**: Build an animated progress bar in `pterm` mapping the install lifecycle onto the `{pacman icon} o o o {repo} ...` graphic format.
- **Icon Propagation**: Basic icons added to `ls` and `ll` via `indicators.go`. Needs extension to `install`, `upgrade`, and `source` alongside Pacman UI.

## Out of scope

- Unit testing internal extraction logic (this is strictly for CLI routing, state feedback, and UI output).
