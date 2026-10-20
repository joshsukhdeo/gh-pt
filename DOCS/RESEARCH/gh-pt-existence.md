# Investigation: Existence of 'ghpt', 'gh-pt', and GitHub Extensions Named 'gh-pt' or 'ghpt'

**Date:** 2026-09-09  
**Status:** Completed  
**Author:** Subagent Research Unit  

---

## Executive Summary

1. **GitHub CLI Extensions (`gh-pt` / `ghpt`):**
   - **Does NOT exist.** No GitHub CLI extension named `gh-pt` or `ghpt` is registered, tagged with the standard `gh-extension` topic, or indexed anywhere across GitHub.
   - Searching GitHub's extension registry, topic metadata (`topic:gh-extension`), and code references for installation commands (`gh extension install gh-pt` or `gh extension install ghpt`) yields **zero** matches.

2. **Standalone Applications and Packages Named `gh-pt` or `ghpt`:**
   - **`App::GHPT` / `gh-pt.pl` (MaxMind)**: An established Perl CLI application and CPAN distribution that integrates GitHub and Pivotal Tracker. Its installed binary is named `gh-pt.pl`.
   - **`GHPT` (Grasshopper Plugin)**: An AI plugin for Rhino's Grasshopper CAD environment created by Sergey Pigach (`enmerk4r`) et al., distributed on Food4Rhino and Rhino's Yak package manager. It translates natural language prompts into visual programming definitions using ChatGPT.
   - **`GHPT` (NWU-VISLAB)**: An academic computer graphics implementation for Gaussian Hybrid Path Tracing (Gaussian Splatting research).
   - **General Package Registries**: No packages named `gh-pt` or `ghpt` exist on PyPI, npm, crates.io, Homebrew (`formulae.brew.sh`), Arch AUR, or Go (`pkg.go.dev`).

---

## 1. GitHub CLI Extension Investigation

### Methodology
GitHub CLI (`gh`) extensions adhere to strict conventions:
- Extension repository naming convention: `gh-<extension-name>` (e.g., `gh-dash`, `gh-copilot`).
- Community and official discovery topic: `gh-extension`.
- Installation syntax: `gh extension install <owner>/gh-<name>` or `gh extension install gh-<name>`.

### Findings
- **Topic Search (`topic:gh-extension pt in:name`)**: **0 results**.
- **Topic Search (`topic:gh-extension gh-pt in:name`)**: **0 results**.
- **Topic Search (`topic:gh-extension ghpt in:name`)**: **0 results**.
- **Code Search (`"gh extension install" "gh-pt"`)**: **0 results**.
- **Code Search (`"gh extension install" "ghpt"`)**: **0 relevant results** (only unrelated benchmark logs and dotfiles).
- **Official GitHub Status**: GitHub previously offered `gh-copilot` as an official CLI extension, which was deprecated in October 2025 in favor of the standalone GitHub Copilot CLI. GitHub has never shipped an official extension under `gh-pt` or `ghpt`.

**Conclusion**: The GitHub CLI extension namespace for `gh-pt` and `ghpt` is completely unclaimed and available.

---

## 2. Existing Applications & Packages

While no GitHub CLI extension exists, multiple distinct software projects utilize the names `ghpt` or `gh-pt`:

### A. `App::GHPT` / `gh-pt.pl` (MaxMind)
- **Primary Source / Repository**: [https://github.com/maxmind/App-GHPT](https://github.com/maxmind/App-GHPT)
- **Package Index**: [MetaCPAN - App::GHPT](https://metacpan.org/dist/App-GHPT)
- **Primary Maintainers**: Mark Fowler, Dave Rolsky, and MaxMind, Inc.
- **Latest Release**: v2.000001 (July 12, 2022); initial commits dating back to 2014.
- **Executable**: `bin/gh-pt.pl` (installed command: `gh-pt.pl`).
- **Functionality**: A command-line automation tool linking GitHub and Pivotal Tracker (PT) in agile workflows. It prompts developers for active Pivotal Tracker stories from the terminal, generates pull requests populated with story metadata, leaves comments linking the PR back on Pivotal Tracker, and transitions story statuses to "Delivered".

### B. `GHPT` (Grasshopper Prompt Tool for Rhino 3D)
- **Primary Source / Repository**: [https://github.com/enmerk4r/GHPT](https://github.com/enmerk4r/GHPT)
- **Ecosystem Distribution**:
  - [Food4Rhino App Page](https://www.food4rhino.com/en/app/ghpt)
  - Rhino Yak Package Manager (`GHPT`)
- **Primary Authors**: Sergey Pigach (`enmerk4r`), Callum Sykes, Jo Kamm, Ryan Erbert, Quoc Dang (Developed at AEC Tech Seattle Hackathon 2023).
- **Functionality**: A plugin for Rhino Grasshopper that accepts natural language prompts inside the CAD canvas (`GHPT = <prompt>`) and calls OpenAI GPT to automatically generate Grasshopper node definitions and scripts.

### C. `GHPT` (Gaussian Hybrid Path Tracing)
- **Primary Source / Repository**: [https://github.com/NWU-VISLAB/GHPT](https://github.com/NWU-VISLAB/GHPT)
- **Authors**: Northwest University VISLAB
- **Functionality**: An open-source research implementation of "Real-Time Relightable Gaussian Splatting using Hybrid Path Tracing" (released February 2026).

### D. Generic / Abandoned GitHub Repositories
- `cc943/ghpt`: Minimal repository containing a test C# solution ("Github Package Test Nuaaget testsst").
- `devemouse/ghpt`: Archived, empty repository from 2010 containing only a test stub.
- `lima-limon-inc/ghpt`: Minimal test repository from 2025 containing dummy text files.

---

## 3. Package Registry Matrix

| Ecosystem / Registry | Identifier Checked | Exists? | Details |
| :--- | :--- | :--- | :--- |
| **GitHub CLI Extensions** | `gh-pt` / `ghpt` | **No** | Zero registered extensions or repos with `gh-extension` topic |
| **CPAN (Perl)** | `App::GHPT` | **Yes** | Active distribution providing `gh-pt.pl` CLI utility |
| **Food4Rhino / Yak** | `GHPT` | **Yes** | Grasshopper ChatGPT definition generator |
| **PyPI (Python)** | `ghpt` / `gh-pt` | **No** | Neither package name exists |
| **npm (JavaScript)** | `ghpt` / `gh-pt` | **No** | Neither package name exists |
| **crates.io (Rust)** | `ghpt` / `gh-pt` | **No** | Neither crate name exists |
| **Homebrew** | `ghpt` / `gh-pt` | **No** | Neither formula nor cask exists |
| **Arch AUR** | `ghpt` / `gh-pt` | **No** | No package exists |
| **Go (`pkg.go.dev`)** | `ghpt` / `gh-pt` | **No** | No module exists |

---

## 4. Name Collision & Semantic Analysis

If a new project or GitHub CLI extension is introduced with the name `gh-pt` or `ghpt`:

1. **Namespace Availability in GitHub CLI**:
   - `gh extension install <user>/gh-pt` would face **zero collision** with existing GitHub CLI extensions.
2. **Semantic Associations**:
   - **GitHub + Pivotal Tracker**: In DevOps/Perl circles, `gh-pt` is historically associated with MaxMind's Pivotal Tracker bridge.
   - **GitHub + GPT / AI Prompting**: Due to the phonetic and orthographic overlap between "PT" and "GPT", developers might assume `gh-pt` or `ghpt` stands for "GitHub Prompt Tool" or "GitHub + Pre-trained Transformer" (analogous to how `enmerk4r/GHPT` stands for Grasshopper + GPT).

---

## Primary Sources & References

1. **MaxMind `App-GHPT`**:
   - GitHub Repository: [https://github.com/maxmind/App-GHPT](https://github.com/maxmind/App-GHPT)
   - MetaCPAN Package: [https://metacpan.org/dist/App-GHPT](https://metacpan.org/dist/App-GHPT)
   - Executable Entrypoint: `bin/gh-pt.pl`
2. **Enmerk4r `GHPT` (Grasshopper GPT Plugin)**:
   - GitHub Repository: [https://github.com/enmerk4r/GHPT](https://github.com/enmerk4r/GHPT)
   - Food4Rhino Listing: [https://www.food4rhino.com/en/app/ghpt](https://www.food4rhino.com/en/app/ghpt)
3. **NWU-VISLAB `GHPT` (Gaussian Hybrid Path Tracing)**:
   - GitHub Repository: [https://github.com/NWU-VISLAB/GHPT](https://github.com/NWU-VISLAB/GHPT)
4. **GitHub CLI Extension Directory & Topics**:
   - GitHub Topic `gh-extension`: [https://github.com/topics/gh-extension](https://github.com/topics/gh-extension)
   - GitHub Search API Query: `topic:gh-extension pt in:name` (returned 0 results).
