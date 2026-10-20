## Question

Define the three-part compilation model: header (gh-pt generated), body.sh (AI generated), footer (gh-pt generated). The compile.sh is reconstructed at runtime from header + body.sh + footer, never written to disk.

## Decision

**Decision**: Use three-part model with runtime reconstruction.

**Rationale**:
- body.sh is the sole AI artifact (no shebang, no header/footer)
- gh-pt reconstructs compile.sh at runtime: header + body.sh + footer
- compile.sh never written to disk (reconstructed in memory)
- Validation checks body.sh (what AI created), not the full script
- Header/footer can evolve without AI changes

**Implementation**:
- Header: shebang, minimal env vars (GHPT_TARGET_BASE_DIR, GHPT_REPO_PATH), logging
- Body: AI-generated build logic, calls `ghpt helper --install` directly
- Footer: cleanup, final logging, exit handling
- Runtime: gh-pt reconstructs in memory, executes in container