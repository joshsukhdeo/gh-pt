## Question

Define the AI-in-container architecture: AI runs in container with network access for research, body.sh created and validated, then compile container (hardened, no network) executes the build.

## Decision

**Decision**: AI runs in container with network access; compile runs in separate hardened container.

**Rationale**:
- AI needs network for research, dependency discovery
- Compile container has no network (prevents exfiltration)
- Clean separation of concerns
- AI container can have tools; compile container is hardened

**Architecture**:
```
Stage 1: AI Container (Working Environment)
- Network: ENABLED (for AI research/development)
- Capabilities: Standard user permissions
- Filesystem: Writable /workspace
- Tools: AI agent + development tools
- Output: body.sh, manifest.json

Stage 2: Compile Container (Hardened Execution)
- Network: DISABLED (--network=none)
- Capabilities: DROP ALL, add only DAC_OVERRIDE
- Filesystem: Read-only except /build, /install
- Seccomp: Restrict to build-related syscalls only
- User: Non-root (UID 1000)
- Input: body.sh + deps (pre-installed)
- Output: Compiled binaries in /install
```

**Container Images**:
- AI Container: OS-specific (DetectContainerImage())
- Compile Container: OS-specific (DetectContainerImage())
- Same image, different security config