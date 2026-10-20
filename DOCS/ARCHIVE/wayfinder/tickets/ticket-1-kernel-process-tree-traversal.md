# Ticket 1: Kernel Process Tree Traversal for Active Threat AI Lockdown

- **Type**: `wayfinder:research`
- **Status**: Closed
- **Assignee**: Closed by Research Subagent
- **Blocked By**: None (Frontier)

## Question
How can `gh-pt` implement reliable, cross-platform process tree traversal (Linux, macOS, Windows, FreeBSD) in Go to detect ancestor `gh-pt` instances and enforce lockdown to `helper` commands with minimal external dependencies?

## Resolution
Adopt targeted platform-specific native queries with conditional build tags (`//go:build linux`, `darwin`, `windows`, `freebsd`) instead of heavy third-party libraries (`gopsutil`), saving 2-4MB binary overhead:
- **Linux**: Walk `/proc/[pid]/stat` (field 4 is PPID) and `/proc/[pid]/comm`.
- **macOS / FreeBSD**: Query `sysctl` with `KERN_PROC_PID` for `kinfo_proc`.
- **Windows**: Use `CreateToolhelp32Snapshot` via `kernel32.dll`.
- **Enforcement**: If any ancestor matches `gh-pt`, execute immediate hard-lockdown, intercepting all commands except `gh-pt helper ...`.
