## Question

Define the AI sandbox mechanism: gh-pt binary checks for gh-pt ancestor in process tree at startup. If found, ONLY permits `ghpt helper` subcommands (exit code 126 for others). Built into main gh-pt binary, not a separate wrapper.

## Decision

**Decision**: Process tree detection in main gh-pt binary enforces AI sandbox.

**Rationale**:
- Built into main gh-pt binary (main.go:27-33), not a separate wrapper
- At startup: checks for gh-pt ancestor process via process tree traversal
- If gh-pt ancestor found: ONLY permits `ghpt helper` subcommands (exit 126 for others)
- If no gh-pt ancestor: forwards to real gh-pt (escape hatch)
- No separate wrapper binary needed

**Implementation**:
- main.go: CheckAncestorForGhPt() at startup
- cmd/sandbox.go: CheckAncestorForGhPt() traverses /proc
- IsHelperInvocation() checks if first non-flag arg is "helper"
- Exit code 126 for non-helper commands when restricted

**Security**:
- Process tree detection works on Linux/macOS via /proc
- Built into main binary (not separate wrapper)
- Escape hatch: if parent dead, forwards to real gh-pt