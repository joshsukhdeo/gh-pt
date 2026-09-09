# gh-pt

## Installation

```bash
gh extension install joshsukhdeo/gh-pt
```

---

## Features & Capabilities

- **Intelligent Asset Selection:** Automatically detects your Linux distribution via `/etc/os-release` and prioritizes distro-specific assets (e.g. `ubuntu` or `fedora` tagged releases) over generic `linux` ones.
- **Native Package Manager Routing:** Detects which package managers are available on your system and routes installations accordingly:
  - **Ubuntu/Debian** (`dpkg`): `.deb` → `apt-get install` or `dpkg -i`
  - **Fedora/RHEL/CentOS** (`rpm`): `.rpm` → `dnf install` or `rpm -i`
  - **Arch Linux** (`pacman`): `.pkg.tar.zst` / `.pkg.tar.xz` → `pacman -U`
  - **FreeBSD** (`pkg`): `.pkg` / `.txz` → `pkg install` or `pkg add`
  - **Archive Embedded Installers:** Automatically extracts and routes native installers (e.g. `.deb`, `.rpm`, `.msi`) embedded within generic archives.
  - **MacOS & Windows Support:** Custom installation pipelines for `.dmg`, `.pkg`, `.msi`, and Windows `setup.exe` executables natively or via Wine.
  - **Fallback:** AppImage, Flatpak, Snap, or raw binary extraction for distros without native package managers.
- **State Management & Updates:** Tracks installed binaries in `state.json` so you can update them all later with a single command. All installation flags (`-T`, `--all`, `--asset-binaries-regexp`) are persisted so updates reproduce the exact same installation behavior.
- **State Management:** Use `state ls` or `state ll` to list saved installations, `--full` for expanded metadata, `--edit-saved-state` to manage update participation, `state rm-saved-state` to remove tracking only, `state rm` to uninstall, and `--purge` to uninstall and remove cached compile scripts.
- **Pinned Versions:** Use `--pin-install` during installation or `--pin REPOSITORY` afterward to skip an application during updates.
- **Dependency Resolution:** Automatically resolves and installs dependencies for `.deb` (via `apt`), `.rpm` (via `dnf`), and `.pkg.tar.zst` (via `pacman`) with the `-y` flag.
- **Cross-Platform:** Supports Linux, macOS, Windows, and FreeBSD.
- **Wine Support:** Can pull and install Windows `.exe`/`.msi` binaries on Linux and FreeBSD with `--wine allow`, `--wine priority`, or `--wine force`.
- **Architecture Safety:** Rejects release assets for foreign architectures by default; use `--allow-foreign-arch` when a cross-architecture asset is intentional.
- **Checksum Verification:** Verifies downloaded assets against release checksum files when available.
- **Safe Extraction and Removal:** Supports native `tar`/`7z` extraction with `--native-extract`, protects uninstall paths from traversal, and offers `--dry-run` for previewing an installation.
- **AI-Assisted Installation:** `--ai` enables repository analysis, `scan --ai` checks source before installation, and `source` generates and runs a build script for repositories without usable release binaries.
- **VirusTotal Checks:** Optional VirusTotal binary scanning with `--vt-api-key`; `--skip-vt-sandbox` bypasses sandbox uploads for unknown hashes.
- **Clean Naming:** Automatically strips messy hardware/OS tags (like `-x86_64-linux`) and redundant version strings from the final installed binary name.
- **Sudo Safety:** Before running any `sudo` command, verifies that a sudo session is cached. In headless mode (`-D`), fails fast with a clear error instead of silently hanging waiting for a password prompt.
- **Prerequisite Validation:** Checks that the GitHub CLI (`gh`) is installed and in PATH before doing anything, with a clear error and install link if missing.

---

## Usage

```bash
$ gh install --help
Usage: gh-pt [<repository>] [flags]

Install binaries for a Github repository release interactively or
non-interactively.

    Intended for quickly installing release binaries for projects that do not distribute
    using Homebrew or other package managers.

Arguments:
  [<repository>]    Github repository in OWNER/REPOSITORY_NAME format
                    ($GH_INSTALL_REPOSITORY).

Flags:
  -h, --help                    Show context-sensitive help.
  -U, --update-all              Update all installed applications (user and
                                global) ($GH_INSTALL_UPDATE_ALL).
  -u, --update                  Update user installations (add -g for global
                                only) ($GH_INSTALL_UPDATE).
  -p, --target-path="~/.local/bin"
                                Target installation directory (default:
                                ~/.local/bin or /usr/local/bin if --global)
                                ($GH_INSTALL_TARGET_PATH).
  -l, --log-level="info"        Log level ($GH_INSTALL_LOG_LEVEL).
  -f, --log-format="console"    Log output format ($GH_INSTALL_LOG_FORMAT).
      --version                 Show version ($GH_INSTALL_VERSION).

Interactive Mode
  -i, --interactive      Use interactive installation. If true, all non-log
                         related flags are ignored ($GH_INSTALL_INTERACTIVE).
      --[no-]log-quiet-interactive
                         Quiet log in interactive mode
                         ($GH_INSTALL_LOG_QUIET_INTERACTIVE)

State Management
  state ls=STRING                List saved state in short format, optionally
                             filtered by repository or asset.
  state ll=STRING                List saved state in long format, optionally
                             filtered by repository or asset.
  --full                     Show expanded installation metadata in list views.
  --edit-saved-state         Edit saved state (enable/disable updates or remove
                             apps) ($GH_INSTALL_EDIT_SAVED_STATE).
  state rm-saved-state=STRING    Remove a saved app from state tracking only
                             ($GH_INSTALL_RM_SAVED_STATE).
  state rm=STRING                Uninstall an application and remove it from state
                             ($GH_INSTALL_RM).
  --purge=STRING             Uninstall an app, remove cached compile scripts, and
                             remove it from state ($GH_INSTALL_PURGE).
  --pin=STRING               Pin a saved installation to its current version
                             ($GH_INSTALL_PIN).
      --pin-install          Pin this installation to the current version
                             ($GH_INSTALL_PIN_INSTALL).

Non-interactive Mode
  -v, --release-version="latest"
                                   Repository release tag (version) to install
                                   ($GH_INSTALL_RELEASE_VERSION).
  -a, --release-asset=STRING       Name of repository release asset to download.
                                   If not set, --release-asset-regexp is used
                                   ($GH_INSTALL_RELEASE_ASSET).
  -A, --release-asset-regexp=STRING
                                   Regular expression matching release asset to
                                   download ($GH_INSTALL_RELEASE_ASSET_REGEXP).
  -T, --format=FORMAT,...          Comma-separated list of types to match and
                                   prioritize ($GH_INSTALL_TYPE).
      --all                        Install all matched assets instead of just
                                   the first one ($GH_INSTALL_ALL).
  -b, --asset-binaries=ASSET-BINARIES,...
                                   If release asset is an archive - names
                                   of a binaries in the archive to install.
                                   If not set, --install-binary-regexp is used
                                   ($GH_INSTALL_ASSET_BINARIES).
  -B, --asset-binaries-regexp=STRING
                                   If release asset is an archive - regular
                                   expression matching binaries in the archive
                                   to install. If not set, repository name is
                                   used ($GH_INSTALL_ASSET_BINARIES_REGEXP).
  -g, --global                     Install globally (e.g. /usr/local/bin)
                                   instead of user bin ($GH_INSTALL_GLOBAL).
  -y, --add-deps                   Automatically resolve and install
                                   dependencies without prompting
                                   ($GH_INSTALL_ADD_DEPS).
  -n, --no-deps                    Do not install dependencies (use dpkg/rpm
                                   directly) ($GH_INSTALL_NO_DEPS).
  -t, --rename=KEY=VALUE;...       Rename binaries installed at target path,
                                   "<asset archive binary | asset>=<renamed
                                   binary>;..." ($GH_INSTALL_RENAME)
  -k, --keep-suffixes              Keep OS/hardware suffixes on extracted
                                   binaries instead of automatically stripping them
                                   ($GH_INSTALL_KEEP_SUFFIXES).
  -D, --disable-prompts            Disable all interactive prompts
                                   ($GH_INSTALL_DISABLE_PROMPTS).
  -S, --no-save-state              Do not save installation to state
                                   (prevents tracking for updates)
                                   ($GH_INSTALL_NO_SAVE_STATE).
      --wine=MODE                  Wine mode: force, priority, allow, or off
                                   ($GH_INSTALL_WINE).
      --allow-foreign-arch         Allow assets for a foreign architecture
                                   ($GH_INSTALL_ALLOW_FOREIGN_ARCH).
      --allow-root-user-install    Allow user-local installation when running
                                   as root ($GH_INSTALL_ALLOW_ROOT_USER_INSTALL).
      --native-extract             Use native tar/7z extraction utilities
                                   ($GH_INSTALL_NATIVE_EXTRACT).
      --[no-]target-path-create    Create target installation
                                   directory if it does not exist
                                   ($GH_INSTALL_TARGET_PATH_CREATE).
  -f, --force                      Overwrite target binaries and skip
                                   destructive-action confirmations
                                   ($GH_INSTALL_FORCE).
      --dry-run                    Show what would be installed without doing it
                                   ($GH_INSTALL_DRY_RUN).
      --verify-checksum             Verify release checksums when available
                                   ($GH_INSTALL_VERIFY_CHECKSUM).

Repository Mode
  repo clone                          Clone and track the repository in ~/src
                                   ($GH_INSTALL_CLONE).
  repo fork                           Fork, clone, and track the repository in
                                   ~/projects ($GH_INSTALL_FORK).

AI Mode
  --ai                             Enable AI-assisted installation
                                   ($GH_INSTALL_AI).
  --ai-cmd=STRING                  AI command template ($GH_INSTALL_AI_CMD).
  scan --ai                 Scan the repository before installation
                                   ($GH_INSTALL_AI_SAFETY_SCAN).
  source            Build from source with an AI-generated script
                                   ($GH_INSTALL_COMPILE_FROM_SOURCE).

Security Mode
  --vt-api-key=STRING              VirusTotal API key ($VT_API_KEY).
  --skip-vt-sandbox                Skip VirusTotal sandbox uploads
                                   ($GH_INSTALL_SKIP_VT_SANDBOX).
```

---

## Package Manager Detection

On Linux, `gh-pt` dynamically detects which package managers are available on your system using `exec.LookPath` and adjusts the default asset priority accordingly:

| Distribution | Detected via | Default priority |
|---|---|---|
| Ubuntu / Debian | `dpkg` in PATH | `deb > snap > flatpak > appimage > tar.gz` |
| Fedora / RHEL / CentOS | `rpm` in PATH | `rpm > snap > flatpak > appimage > tar.gz` |
| Arch Linux / Manjaro | Neither `dpkg` nor `rpm` | `appimage > flatpak > snap > tar.gz` |
| FreeBSD | `GOOS=freebsd` | `pkg > txz > tar.gz` |
| macOS | `GOOS=darwin` | `dmg > tar.gz > zip` |

Additionally, `.pkg.tar.zst` and `.pkg.tar.xz` files are intercepted before archive extraction and routed directly to `sudo pacman -U` on Arch-based systems.

The default priority can always be overridden with `-T` (e.g. `-T rpm,deb,tar.gz`).

---

## State Management & Update System

By default, every successful installation is saved to an internal `state.json` file inside your XDG Data directory. This tracks the repository, current version, target path, scope (User/Global), and all installation flags (format types, `--all`, `--asset-binaries-regexp`).

You can instantly update all tracked applications by running:
```bash
gh install -U
```
*(`-U` updates all global and user packages. `-u` updates only user packages. `-u -g` updates only global packages).*

To view and manage your current state:
- `gh install --edit-saved-state`: Launches an interactive terminal UI to enable/disable automatic updates for specific apps, or delete them from the tracker.
- `gh install state ls="fzf"`: Lists saved installations matching a repository or asset filter.
- `gh install state ll="fzf" --full`: Shows expanded metadata for matching saved installations.
- `gh install state rm-saved-state="fzf"`: Removes the application from state tracking without uninstalling it.
- `gh install state rm="fzf"`: Uninstalls the application and removes it from state.
- `gh install --purge="fzf"`: Uninstalls the application, removes cached compile scripts, and removes it from state.
- `gh install --pin="fzf"`: Pins the saved installation so update commands skip it.

If you are running `gh install` in a temporary script and don't want to track it for updates, pass the `-S` (`--no-save-state`) flag.

---

- **Repository Tracking (Clone / Fork):** Supports cloning (`repo clone`) or forking (`repo fork`) git repositories into configurable base directories (defaults: `~/src` and `~/projects`), tracking them in `state.json` and automatically syncing them via `gh repo sync` during `gh install -U` / `-u`.
- **AI Compilation from Source:** Supports `source` (with `--ai` and configurable `--ai-cmd 'agy -p "%s"'`) which clones the target repository to a temporary directory, invokes the AI agent to produce an automated build script saved at `~/.config/gh-pt/scripts/compile-<pkgname>.sh` (or `.ps1` on Windows), executes compilation and installation, purges the temporary workspace, and tracks `compile_script` in state for replay during `-U`/`-u` updates.
- **Pinned updates:** Pinned entries remain tracked but are skipped by `-U` and `-u` until the pin is removed.

---

## Configuration & Environment Variables

All CLI flags can be set via environment variables (prefixed with `GH_INSTALL_`) or a YAML configuration file located at `~/.config/gh-pt/config.yml`.

Example `config.yml`:
```yaml
install_types: "deb,appimage,tar.gz,zip"
clone_path: "~/src"
fork_path: "~/projects"
ai_cmd: 'agy -p "%s"'
add_deps: true
allow_wine: false
prompt_rename: true
wine: "off"
verify_checksum: true
native_extract: false
```

The configuration precedence is: `CLI Argument > Environment Variable > config.yml > Default`.

---

## Topgrade Integration

`gh-pt` can easily be integrated with [Topgrade](https://github.com/topgrade-rs/topgrade) to keep all your installed binaries up to date automatically alongside your system packages. Just add the following to your `topgrade.toml` under the `[commands]` block:

```toml
[commands]
"gh-pt" = "gh install -U"
```

*Project maintained with model switch to nemotron-3.5-lightning-free for active development.*
