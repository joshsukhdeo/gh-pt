## Question

Define the minimal environment variables passed to the AI sandbox: only GHPT_TARGET_BASE_DIR and GHPT_REPO_PATH in the header. No GHPT_TARGET_BIN_DIR, GHPT_TARGET_LIB_DIR, etc. AI uses ghpt helper --install for file operations.

## Decision

**Decision**: Minimal environment variables in header - only GHPT_TARGET_BASE_DIR and GHPT_REPO_PATH.

**Rationale**:
- Reduces attack surface: AI cannot manipulate paths via env vars
- Forces delegation to ghpt helper --install for all file operations
- AI cannot access GHPT_TARGET_BIN_DIR, GHPT_TARGET_LIB_DIR directly
- Forces delegation to helper functions (security boundary)

**Environment Variables in Header**:
- GHPT_TARGET_BASE_DIR: Base installation directory (install-dir)
- GHPT_REPO_PATH: Path to repository build directory

**Removed from Header**:
- GHPT_TARGET_BIN_DIR (use helper --install "bin=...")
- GHPT_TARGET_LIB_DIR (use helper --install "libs=...")
- GHPT_TARGET_SHARE_DIR (use helper --install "share=...")
- GHPT_LOG_FILE (internal to header)
- GHPT_INSTALL_LOG (internal to header)

**Rationale**: Reduces attack surface, forces delegation to helper functions