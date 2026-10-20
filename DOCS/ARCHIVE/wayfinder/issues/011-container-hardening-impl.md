## Question

Implement container hardening in compile/container.go with network isolation, capability dropping, read-only filesystem, seccomp profile, non-root user, tmpfs for /tmp.

## Decision

**Decision**: Implemented container hardening in compile/container.go.

**Implementation** (compile/container.go):
- Network isolation: --network=none
- Capability dropping: --cap-drop=ALL, --cap-add=DAC_OVERRIDE
- Read-only filesystem: --read-only
- Seccomp profile: --security-opt=seccomp=seccomp-profile.json
- Non-root user: --user=1000:1000
- Tmpfs: --tmpfs=/tmp:size=100M,mode=1777
- Read-only source mount: -v /source:/build:ro
- Write install: -v /install:/install:rw

**Files Modified**:
- compile/container.go: RunContainer, BuildContainerCommand, GetContainerMounts
- compile/seccomp-profile.json: Seccomp profile
- compile/container_test.go: Security tests (6 tests passing)

**Tests Passing**:
- NetworkIsolation: Network disabled in container
- CapabilityDropping: Only DAC_OVERRIDE capability
- ReadOnlyFilesystem: Root filesystem read-only
- NonRootUser: User 1000:1000
- NoNewPrivileges: Set
- TmpfsForTmp: /tmp mounted as tmpfs