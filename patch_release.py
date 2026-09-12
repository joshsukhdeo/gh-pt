import sys

with open('release/release.go', 'r') as f:
    lines = f.readlines()

for i, line in enumerate(lines):
    if 'destinationPath := r.resolveDestinationPath(binaryPath)' in line:
        lines[i] = line + "\tr.InstalledBinaries = append(r.InstalledBinaries, filepath.Base(destinationPath))\n"

with open('release/release.go', 'w') as f:
    f.writelines(lines)
