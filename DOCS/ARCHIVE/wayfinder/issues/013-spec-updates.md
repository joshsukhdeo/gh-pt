## Question

Update SPEC-SOURCE-REPOS.md with three-part model, runtime reconstruction, process tree detection, minimal env vars, container hardening, AI-in-container architecture.

## Decision

**Decision**: Update SPEC-SOURCE-REPOS.md with all new architecture decisions.

**Sections Updated**:
- Section 2.2: Compile artifacts (body.sh vs compile.sh)
- Section 4.3: Helper command (removed --target-base-dir, --global)
- Section 5.1: Three-part compilation model
- Section 5.2: Header template (no _ghpt_install)
- Section 5.3: Body template (no shebang, helper --install)
- Section 5.5: Validation rules (body.sh, new forbidden patterns)
- Section 5.6: NEW - Execution flow (8-step lifecycle)
- Section 6.1-6.3: Process tree detection (not wrapper binary)
- Section 6.4: AI prompt integration (helper-only workflow)

**Files Modified**:
- docs/SPEC-SOURCE-REPOS.md: All sections updated
- cmd/root.go: buildCompilePrompt, buildCompileFixPrompt
- ai/parser.go: Updated templates