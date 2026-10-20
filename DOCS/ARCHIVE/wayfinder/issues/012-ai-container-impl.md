## Question

Implement AI-in-container architecture: Two-stage execution - AI container (network) + compile container (hardened, no network).

## Decision

**Decision**: Two-stage execution - AI container (network) + compile container (hardened, no network).

**Implementation** (cmd/root.go):
```go
func handleCompileFromSource(cfg *config.Config) error {
    // Stage 1: AI Container
    aiContainer := runAIContainer(repoDir)
    bodyScript := aiContainer.GenerateBodyScript()
    
    // Stage 2: Compile Container
    compileContainer := runCompileContainer(repoDir, bodyScript)
    result := compileContainer.Execute()
    
    return result
}
```

**Implementation Details**:
- Stage 1: AI Container
  - Network: ENABLED
  - Filesystem: Writable /workspace
  - Tools: AI agent, git, curl, etc.
  - Output: body.sh, manifest.json
  
- Stage 2: Compile Container
  - Network: DISABLED (--network=none)
  - Capabilities: DROP ALL, add DAC_OVERRIDE
  - Seccomp: Restrict to build syscalls
  - User: Non-root (1000:1000)
  - Input: body.sh, manifest.json, deps
  - Output: Compiled binaries

**Files Modified**:
- cmd/root.go: Two-stage execution
- compile/container.go: Separate AI and compile containers
- docs/SPEC-SOURCE-REPOS.md: Document architecture