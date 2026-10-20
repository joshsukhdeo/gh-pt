## Question

Define container hardening: network isolation (--network=none), capability dropping (--cap-drop=ALL, --cap-add=DAC_OVERRIDE), read-only filesystem (--read-only), seccomp profile, non-root user (--user=1000:1000), tmpfs for /tmp.

## Decision

**Decision**: Implement full container hardening with network isolation, capability dropping, read-only filesystem, seccomp profile, and non-root user.

**Rationale**:
- Network isolation prevents data exfiltration
- Capability dropping minimizes attack surface
- Read-only filesystem prevents tampering
- Seccomp restricts syscalls to build operations
- Non-root user prevents privilege escalation

**Implementation**:
```bash
docker run \
  --network=none \
  --cap-drop=ALL \
  --cap-add=DAC_OVERRIDE \
  --security-opt=no-new-privileges \
  --security-opt=seccomp=seccomp-profile.json \
  --read-only \
  --tmpfs=/tmp:size=100M,mode=1777 \
  --user=1000:1000 \
  compile-image
```

**Seccomp Profile**:
- Allow only build-related syscalls
- Block network syscalls (no network)
- Block privilege escalation syscalls
- Block filesystem modifications outside allowed paths

**Validation**:
- Network disabled (verified via ping test)
- Capabilities dropped (only DAC_OVERRIDE)
- Filesystem read-only (except /build, /install, /tmp)
- Non-root user (UID 1000)
- Seccomp active (verified via /proc/1/status)