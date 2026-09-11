import os
import glob
import re

directory = os.path.expanduser("~/builds/app-build-scripts-ubuntu26")

for filepath in glob.glob(os.path.join(directory, "**/*.sh"), recursive=True):
    with open(filepath, "r") as f:
        content = f.read()

    original_content = content
    
    # Simple replacements
    content = content.replace("gh-install ", "gh-pt install ")
    content = content.replace("gh-install\n", "gh-pt install\n")
    content = content.replace("gh-install\"", "gh-pt install\"")
    content = content.replace("command -v gh-install", "command -v gh-pt")
    content = content.replace("https://github.com/joshsukhdeo/gh-install.git", "https://github.com/joshsukhdeo/gh-pt.git")
    content = content.replace("~/projects/gh-install", "~/projects/gh-pt")

    # Syntax specific replacements
    # gh-pt install "owner/repo" --clone -> gh-pt repo clone "owner/repo"
    # we can use regex
    # e.g., gh-pt install intel/ScalableVectorSearch --clone -p "${CLONE_PATH}" -g -y 2>/dev/null || true
    content = re.sub(r'gh-pt install\s+([^ ]+)\s+--clone', r'gh-pt repo clone \1', content)
    content = re.sub(r'gh-pt install\s+"([^"]+)"\s+--clone', r'gh-pt repo clone "\1"', content)

    # gh-pt install owner/repo --compile-from-source -> gh-pt source owner/repo
    content = re.sub(r'gh-pt install\s+([^ ]+)\s+--compile-from-source', r'gh-pt source \1', content)
    content = re.sub(r'gh-pt install\s+"([^"]+)"\s+--compile-from-source', r'gh-pt source "\1"', content)

    if content != original_content:
        print(f"Updated {filepath}")
        with open(filepath, "w") as f:
            f.write(content)
