## Destination
gh-pt v2 Architecture (AI 2-stage source compilation, LDD static dependency resolution, Sidecar extraction lifecycle, and Core Optimizations).

## Notes
- domain: Go CLI, package management, GitHub API
- skills: subagent-orchestrator, tdd

## Decisions so far
- [Spec: v2 Architecture](docs/SPEC-v2.md): Defined the Dependency Resolution & Routing Blueprint
- [Spec: Heuristics](docs/SPEC-HEURISTICS.md): Defined the Toolchain Contextual Mapping
- [Spec: Sidecars](docs/SPEC-SIDECARS.md): Defined Companion Asset & Plugin Tracking
- [Spec: Optimizations](docs/SPEC-OPTIMIZATIONS.md): Defined Atomic Swaps, GraphQL Batching, Concurrent Updates, and Sudo Refresh
- Foundation & Reliability (Code): Implemented V2 State schemas, CLI flags, Sudo Keepalive, and Atomic Swaps.
- Ecosystem & Resolver (Code): Built `resolver` native OS/UV/Mise clients and `heuristics` directory scanner.

## Not yet specified
- How the `resolver` package interfaces with the `ai` JSON manifest parser.
- GraphQL query construction for the update sweep.
- Cross-platform mapping for `apt` vs `dnf` vs `pacman` within the resolver.

## Out of scope
- N/A
