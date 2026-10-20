## Question

Implement container hardening: network isolation (--network=none), capability dropping (--cap-drop=ALL, --cap-add=DAC_OVERRIDE), read-only filesystem (--read-only), seccomp profile, non-root user (--user=1000:1000), tmpfs for /tmp.

## Decision

**Decision**: Implement full container hardening in compile/container.go.

**Implementation**:
- Network isolation: --network=none
- Capability dropping: --cap-drop=ALL, --cap-add=DAC_OVERRIDE
- Read-only filesystem: --read-only
- Seccomp profile: --security-opt=seccomp=seccomp-profile.json
- Non-root user: --user=1000:1000
- Tmpfs: --tmpfs=/tmp:size=100M,mode=1777
- Read-only source mount: -v /source:/build:ro
- Write install: -v /install:/install:rw

**Implementation** (compile/container.go):
```go
securityFlags := []string{
    "run", "--rm",
    "--network=none",
    "--cap-drop=ALL",
    "--cap-add=DAC_OVERRIDE",
    "--security-opt=no-new-privileges",
    "--read-only",
    "--tmpfs=/tmp:size=100M,mode=1777",
    "--user=1000:1000",
}
```

**Files Modified**:
- compile/container.go: RunContainer, BuildContainerCommand
- compile/seccomp-profile.json: Seccomp profile
- compile/container_test.go: Security tests