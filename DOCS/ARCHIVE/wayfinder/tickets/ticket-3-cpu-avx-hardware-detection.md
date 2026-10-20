# Ticket 3: CPU AVX Capability Detection & Asset Priority

- **Type**: `wayfinder:research`
- **Status**: Closed
- **Assignee**: Closed by Research Subagent
- **Blocked By**: None (Frontier)

## Question
How should `gh-pt` detect CPU instruction sets (`AVX`, `AVX2`, `AVX-512`) across supported operating systems using `golang.org/x/sys/cpu` or system heuristics, and how should asset regex matchers prioritize these tags relative to OS and architecture?

## Resolution
- **Detection**: Leverage `golang.org/x/sys/cpu` (already an indirect dependency in `go.mod`). Verifies CPUID feature bits and kernel `OSXSAVE` state (`XGETBV`) via `cpu.X86.HasAVX`, `cpu.X86.HasAVX2`, `cpu.X86.HasAVX512`.
- **Priority Chain**: `avx512` -> `avx2` -> `avx` -> generic. If host lacks AVX support or runs on non-x86, AVX-specific binaries are strictly ignored to avoid illegal instruction faults unless explicitly requested.
- **CLI Override**: Wired `--avx-level` into `params.CommonInstallFlags` (options: `auto`, `none`, `avx`, `avx2`, `avx512`) and `$XDG_CONFIG_HOME/gh-pt/config.yml`.
- **Implementation**: Implemented in `selector/avx.go`, `selector/selector.go`, `selector/interface.go`, `params/params.go`, `config/config.go`, `main.go`, and `release/release.go` with complete unit test coverage in `selector/avx_test.go`.

