#!/usr/bin/env bash
set -euo pipefail

NO_SYMLINK=0
for arg in "$@"; do
    if [ "$arg" == "--no-symlink" ]; then
        NO_SYMLINK=1
    fi
done

echo "[+] Building gh-pt..."
make build

echo "[+] Registering extension with GitHub CLI..."
EXTENSION_DIR="${HOME}/.local/share/gh/extensions/gh-pt"

if [ -L "${EXTENSION_DIR}" ]; then
    echo "[*] Symlink already exists at ${EXTENSION_DIR}."
else
    gh extension install .
fi

echo "[+] Managing global binary in ${HOME}/.local/bin/gh-pt..."
if [ "$NO_SYMLINK" -eq 1 ]; then
    echo "[*] --no-symlink specified. Hard copying binary..."
    rm -f ${HOME}/.local/bin/gh-pt
    cp "$(pwd)/gh-pt" ${HOME}/.local/bin/gh-pt
else
    echo "[*] Symlinking binary..."
    rm -f ${HOME}/.local/bin/gh-pt
    ln -s "$(pwd)/gh-pt" ${HOME}/.local/bin/gh-pt
fi

echo "[+] Done. Test with: gh install --help or gh-pt --help"
