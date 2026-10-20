## Question

Define the runtime reconstruction process: gh-pt reconstructs compile.sh in memory from header + body.sh + footer at execution time, never writing compile.sh to disk.

## Decision

**Decision**: Reconstruct compile.sh in memory at execution time.

**Rationale**:
- compile.sh never written to disk (prevents tampering)
- Header/footer can be updated without AI changes
- Validation checks body.sh (what AI created), not full script
- Execution uses reconstructed script in container

**Implementation**:
1. gh-pt reads body.sh from .ghpt/body.sh
2. Constructs compile.sh = header + body.sh + footer in memory
3. Writes to temporary file or pipes to container
4. Executes in hardened container
5. Cleans up temp file after execution

**Validation**:
- Validates body.sh (what AI created) before reconstruction
- Validates at execution time, not just before
- Never writes compile.sh to disk