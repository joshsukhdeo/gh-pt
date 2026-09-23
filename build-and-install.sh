#!/usr/bin/env bash
set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BOLD='\033[1m'
NC='\033[0m'

# Status icons
GREEN_DOT="${GREEN}${BOLD}🟢${NC}"
RED_DOT="${RED}${BOLD}🔴${NC}"
YELLOW_DOT="${YELLOW}${BOLD}🟡${NC}"

# Colors for text
GREEN_TXT="${GREEN}${BOLD}"
RED_TXT="${RED}${BOLD}"
YELLOW_TXT="${YELLOW}${BOLD}"
NC_TXT="${NC}"

DIRECT=0
FORCE=0
GLOBAL=0

for arg in "$@"; do
    case "$arg" in
        --direct)    DIRECT=1 ;;
        --symlink)   DIRECT=0 ;;
        --force|-f)  FORCE=1 ;;
        --global|-g) GLOBAL=1 ;;
    esac
done

# --- Build ---
echo "[+] Building gh-pt..."
if ! make build; then
    echo -e "${RED_DOT} ${RED_TXT}BUILD FAILED${NC}"
    exit 1
fi
BUILT_BIN="$(pwd)/gh-pt"

# --- gh extension (always symlink) ---
EXTENSION_DIR="${HOME}/.local/share/gh/extensions/gh-pt"
if [ -L "${EXTENSION_DIR}" ]; then
    echo "[*] Symlink already exists at ${EXTENSION_DIR}."
else
    gh extension install .
fi

# Helper: check if the active binary on PATH is the newly installed one
active_is_new() {
    local active
    active="$(command -v gh-pt 2>/dev/null || true)"
    [ -z "$active" ] && return 1
    local resolved
    resolved="$(readlink -f "$active" 2>/dev/null || realpath "$active" 2>/dev/null || echo "$active")"

    # Determine expected location based on install mode
    local expected
    if [ "$DIRECT" -eq 1 ]; then
        # Direct copy: installed at BIN_TARGET
        expected="$(realpath "$BIN_TARGET" 2>/dev/null || echo "$BIN_TARGET")"
    else
        # Symlink: installed points to BUILT_BIN
        expected="$(realpath "$BUILT_BIN" 2>/dev/null || echo "$BUILT_BIN")"
    fi
    [ "$resolved" = "$expected" ]
}

# Check if user-level install exists and would take precedence
user_install_exists() {
    [ -e "${HOME}/.local/bin/gh-pt" ] || [ -L "${HOME}/.local/bin/gh-pt" ]
}

# Check if system binary exists
system_binary_exists() {
    local active
    active="$(command -v gh-pt 2>/dev/null || true)"
    [ -n "$active" ] && [[ "$active" == /usr/* ]]
}

# --- Set BIN_TARGET based on --global flag ---
if [ "$GLOBAL" -eq 1 ]; then
    BIN_TARGET="/usr/local/bin/gh-pt"
else
    BIN_TARGET="${HOME}/.local/bin/gh-pt"
fi

# --- If no --global, build status only ---
if [ "$GLOBAL" -eq 0 ]; then
    if active_is_new; then
        echo -e "${GREEN_DOT} ${GREEN_TXT}BUILD SUCCESSFUL${NC} (installed to user bin dir)"
    else
        echo -e "${RED_DOT} ${RED_TXT}BUILD SUCCESSFUL${NC} (inactive — user install overrides)"
    fi
    exit 0
fi

# Determine if we have an existing binary at target
target_exists=0
target_is_symlink=0
target_is_real=0
if [ "$GLOBAL" -eq 1 ]; then
    if sudo test -L "$BIN_TARGET" 2>/dev/null; then
        target_exists=1
        target_is_symlink=1
    elif sudo test -e "$BIN_TARGET" 2>/dev/null; then
        target_exists=1
        target_is_real=1
    fi
else
    if [ -L "$BIN_TARGET" ]; then
        target_exists=1
        target_is_symlink=1
    elif [ -e "$BIN_TARGET" ]; then
        target_exists=1
        target_is_real=1
    fi
fi

# Check precedence override
precedence_override=0
if user_install_exists && ! active_is_new; then
    precedence_override=1
fi

# ========== INSTALL LOGIC ==========
if [ "$DIRECT" -eq 1 ]; then
    # --- Direct (hard copy) mode ---
    if [ "$target_exists" -eq 1 ] && [ "$FORCE" -eq 0 ]; then
        if active_is_new; then
            # Active is already the installed version (copy or symlink)
            is_symlink=0
            if [ "$GLOBAL" -eq 1 ]; then
                sudo test -L "$BIN_TARGET" 2>/dev/null && is_symlink=1
            else
                [ -L "$BIN_TARGET" ] && is_symlink=1
            fi
            
            if [ "$is_symlink" -eq 1 ]; then
                echo -e "${GREEN_DOT} ${GREEN_TXT}BUILD SUCCESSFUL${NC} ${YELLOW_DOT} ${YELLOW_TXT}INSTALL WARNING: --direct requested but no copy made. Active binary is a symlink to the new build.${NC}"
            else
                echo -e "${GREEN_DOT} ${GREEN_TXT}BUILD SUCCESSFUL${NC} ${YELLOW_DOT} ${YELLOW_TXT}INSTALL WARNING: --direct requested but no copy made. Active binary is already the new build.${NC}"
            fi
            exit 0
        else
            # Active version is not the new one
            echo -e "${GREEN_DOT} ${GREEN_TXT}BUILD SUCCESSFUL${NC} ${RED_DOT} ${RED_TXT}INSTALL FAILED: active version on path outdated${NC}"
            exit 1
        fi
    fi

    # Force or no existing target - proceed with copy
    if [ "$GLOBAL" -eq 1 ]; then
        sudo rm -f "$BIN_TARGET"
        sudo cp "$BUILT_BIN" "$BIN_TARGET"
    else
        [ -e "$BIN_TARGET" ] || [ -L "$BIN_TARGET" ] && rm -f "$BIN_TARGET"
        cp "$BUILT_BIN" "$BIN_TARGET"
    fi

    if active_is_new; then
        echo -e "${GREEN_DOT} ${GREEN_TXT}BUILD SUCCESSFUL — INSTALLED SUCCESSFULLY (direct copy)${NC}"
    else
        echo -e "${RED_DOT} ${RED_TXT}BUILD SUCCESSFUL — INSTALL FAILED: active binary on PATH is not the new version${NC}"
        exit 1
    fi

else
    # --- Symlink mode (default) ---
    if [ "$target_exists" -eq 1 ] && [ "$FORCE" -eq 0 ]; then
        if active_is_new; then
            echo -e "${GREEN_DOT} ${GREEN_TXT}BUILD SUCCESSFUL — INSTALLED SUCCESSFULLY (symlink up-to-date)${NC}"
            exit 0
        else
            echo -e "${GREEN_DOT} ${GREEN_TXT}BUILD SUCCESSFUL${NC} ${RED_DOT} ${RED_TXT}INSTALL FAILED: active version on path outdated${NC}"
            exit 1
        fi
    fi

    if [ "$GLOBAL" -eq 1 ]; then
        sudo rm -f "$BIN_TARGET"
        sudo ln -s "$BUILT_BIN" "$BIN_TARGET"
    else
        [ -e "$BIN_TARGET" ] || [ -L "$BIN_TARGET" ] && rm -f "$BIN_TARGET"
        ln -s "$BUILT_BIN" "$BIN_TARGET"
    fi

    if active_is_new; then
        echo -e "${GREEN_DOT} ${GREEN_TXT}BUILD SUCCESSFUL — INSTALLED SUCCESSFULLY${NC}"
    else
        echo -e "${RED_DOT} ${RED_TXT}BUILD SUCCESSFUL — INSTALL FAILED: active binary on PATH is not the new version${NC}"
        exit 1
    fi
fi

# Check for precedence override warning
if [ "$precedence_override" -eq 1 ]; then
    if system_binary_exists; then
        echo -e "${YELLOW_DOT} ${YELLOW_TXT}WARNING: Inactive new binary; precedence override by system binary${NC}"
    else
        echo -e "${YELLOW_DOT} ${YELLOW_TXT}WARNING: Inactive new binary; precedence override by user-level install${NC}"
    fi
fi