#!/usr/bin/env bash
set -euo pipefail

python3 - << 'EOF'
import os
import re

def patch_file(filepath, search_pattern, replacement):
    with open(filepath, 'r', encoding='utf-8') as f:
        content = f.read()
    if search_pattern not in content and not re.search(search_pattern, content):
        print(f"[*] Skipping {filepath} (pattern not found or already patched)")
        return
    
    if isinstance(search_pattern, re.Pattern):
        patched = search_pattern.sub(replacement, content)
    else:
        patched = content.replace(search_pattern, replacement)
        
    with open(filepath, 'w', encoding='utf-8') as f:
        f.write(patched)
    print(f"[+] Successfully patched {filepath}")

# 1. Patch GPU/Hardware Acceleration Detection (cmd/root.go)
gpu_search = """		} else if _, err := os.Stat("/dev/dri"); err == nil {
			hwSpecific = "(?:gpu|cuda|rocm)"
		}"""

gpu_replace = """		}
		// Interrogate exact compute nodes, ignoring generic display interfaces
		if _, err := os.Stat("/dev/nvidia0"); err == nil {
			if hwSpecific != "" { hwSpecific += "|" }
			hwSpecific += "cuda"
		} else if _, err := os.Stat("/dev/kfd"); err == nil {
			if hwSpecific != "" { hwSpecific += "|" }
			hwSpecific += "rocm"
		}"""

patch_file('cmd/root.go', gpu_search, gpu_replace)

# 2. Patch Musl vs Glibc Prioritization (selector/selector.go)
musl_search = re.compile(r'					var nonMusl \[\*\]SelectorItem\n\s*for _, item := range currentMatches \{\n\s*if !muslRegex\.MatchString\(item\.Name\) \{\n\s*nonMusl = append\(nonMusl, item\)\n\s*\}\n\s*\}\n\s*if len\(nonMusl\) > 0 \{\n\s*currentMatches = nonMusl\n\s*\}')

musl_replace = """					// Prefer GNU for system packages, prefer Musl for standalone binaries
					var preferredLibc []*SelectorItem
					for _, item := range currentMatches {
						isPkg := strings.HasSuffix(strings.ToLower(item.Name), ".deb") || strings.HasSuffix(strings.ToLower(item.Name), ".rpm")
						isMusl := muslRegex.MatchString(item.Name)
						if isPkg && !isMusl {
							preferredLibc = append(preferredLibc, item)
						} else if !isPkg && isMusl {
							preferredLibc = append(preferredLibc, item)
						}
					}
					if len(preferredLibc) > 0 {
						currentMatches = preferredLibc
					}"""

patch_file('selector/selector.go', musl_search, musl_replace)
EOF

go fmt ./...
make build
echo "[+] Vulnerabilities patched and binary rebuilt."