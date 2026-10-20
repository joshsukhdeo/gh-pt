## Destination

Implement a secure, production-ready AI-assisted source build system for gh-pt with: three-part compilation model (header + body.sh + footer), runtime reconstruction of compile.sh, AI sandbox via process tree detection, minimal env vars, container hardening, and AI-in-container architecture with separate AI and compile containers.

## Notes

- **Skills to consult every session**: good-thinking, prompt-optimizer, antislop, spec-driven-development, wayfinder
- **Standing preferences**: Security-first, defense in depth, minimal attack surface, validate at execution time, defense in depth
- **Key docs**: SPEC-SOURCE-REPOS.md (source of truth), ai-build-security guide (design rationale)

## Decisions so far

- [Three-part compilation model](.wayfinder/issues/001-three-part-model.md): header + body.sh + footer reconstructed at runtime
- [Runtime reconstruction](.wayfinder/issues/002-runtime-reconstruction.md): compile.sh reconstructed in memory at execution time
- [AI sandbox via process tree](.wayfinder/issues/003-process-tree-sandbox.md): gh-pt binary checks for gh-pt ancestor, restricts to helper commands
- [Minimal env vars](.wayfinder/issues/004-minimal-env-vars.md): Only GHPT_TARGET_BASE_DIR and GHPT_REPO_PATH in header
- [Container hardening](.wayfinder/issues/005-container-hardening.md): network=none, cap-drop=ALL, read-only fs, seccomp, non-root user
- [AI-in-container architecture](.wayfinder/issues/006-ai-in-container.md): AI runs in container, body.sh validated, compile in hardened container
- [AST-based validation](.wayfinder/issues/007-ast-validation.md): Replace regex with mvdan.cc/sh AST parser
- [Container hardening implementation](.wayfinder/issues/008-container-hardening-impl.md): Implemented network=none, cap-drop=ALL, read-only, seccomp, non-root user
- [AI-in-container implementation](.wayfinder/issues/009-ai-container-impl.md): Two-stage: AI container (network) + compile container (hardened)
- [AST-based validation implementation](.wayfinder/issues/010-ast-validation-impl.md): Replace regex with mvdan.cc/sh parser
- [Container hardening implementation](.wayfinder/issues/011-container-hardening-impl.md): network=none, cap-drop=ALL, read-only, seccomp, non-root
- [AI-in-container implementation](.wayfinder/issues/012-ai-container-impl.md): AI container (network) + compile container (hardened)
- [Spec updates](.wayfinder/issues/013-spec-updates.md): Updated SPEC-SOURCE-REPOS.md sections 2.2, 4.3, 5.1-5.6, 6.1-6.4

## Not yet specified

- AST-based validation implementation details (mvdan.cc/sh integration)
- Container hardening: seccomp profile tuning for specific build tools
- Cross-compilation support in hardened containers
- Build caching for faster rebuilds
- Manifest.json dependency validation (allowlist)
- Build artifact auditing and verification
- Cross-compilation support in hardened containers
- Build caching for faster rebuilds
- AI prompt optimization for new workflow
- Test infrastructure for security controls
- Documentation updates for new architecture

## Out of scope

- GUI/TUI for build monitoring (separate effort)
- CI/CD pipeline integration (separate effort)
- Multi-architecture build matrix (separate effort)
- Distributed build workers (separate effort)