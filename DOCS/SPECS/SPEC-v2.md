# Spec: gh-pt Package Manager Evolution (v2 Architecture)

## Objective
Evolve `gh-pt` into an intelligent, AI-assisted local package manager handling both source compilation and pre-compiled binary deployment with advanced, environment-isolated dependency resolution. 

Key objectives from the consolidated blueprint:
1. **Two-Stage AI Source Compilation:** `ghpt source` will enforce a JSON Dependency Manifest + Markdown Bash Script payload from the AI. Dependencies are resolved natively by `ghpt` before the bash script executes.
2. **Static Dependency Resolution:** `ghpt install` will perform `ldd` sweeps on extracted binaries (with `LD_LIBRARY_PATH` injected) to natively map and install missing `.so` files via OS package managers.
3. **Environment Isolation:** Contextual resolution chains will route dependencies to user-space tools (`uv`, `mise`, `cargo`) before falling back to native OS managers. This introduces a new `dependency_resolution` schema addition to the `config.yml` user config:
```yaml
dependency_resolution:
  priorities:
    default: ["mise", "ghpt", "native_os", "flatpak"]
    rust:    ["cargo", "mise", "native_os"]
    python:  ["uv", "mise", "native_os"]
    c_cpp:   ["vcpkg", "native_os"]
```
4. **CLI Flag Refactor:** Replace `--add-deps` with explicit `--resolve-deps` (silent automation) and `--prompt-deps` (safety brake).
5. **Architectural Fixes:** Abstract hardcoded package managers into a `resolver/` module, restructure `state.json` (adding hooks/system-packages), and ensure robust `state.json.lock` concurrency.

## Tech Stack
- **Language:** Go 1.21+
- **CLI Framework:** `alecthomas/kong`
- **UI & Logging:** `pterm` (interactive), `rs/zerolog` (headless)
- **Archive Extraction:** `mholt/archiver/v4`

## Commands
- **Build:** `make build` (or `go build -v -o gh-pt .`)
- **Test:** `go test -v ./...`
- **Lint:** `make lint` / `make fmt`

## Project Structure
- `cmd/` → Kong CLI routing, hook management (`ghpt hook`), repo context scanning for toolchain priorities, and refactored AI prompts (`buildCompilePrompt`).
- `release/` → Extraction logic (`.ghpt/dist/` handoff scanning), post-extraction `ldd` execution (before symlinking).
- `config/` → AI template embed files, default configurations, and the new `dependency_resolution` YAML schema.
- `state/` → `state.json` V2 schemas (`hooks`, `system-packages`), migrations, and `flock` lock verification.
- `resolver/` **[NEW]** → Generic `PackageManager` interface and OS/User-space specific abstractions (`apt`, `dnf`, `pacman`, `mise`, `uv`, `cargo`).
- `ai/` **[NEW]** → Payload parsing (extracting the JSON block and Markdown bash block from AI responses).

## Code Style
```go
// Example: Strict interface abstraction for package managers
type PackageManager interface {
    MapLibrary(libName string) ([]string, error) // e.g., mapping an .so to a package
    Install(packages []string, prompt bool) error
}

// Example: Defensive lock wrapping per AGENTS.md
st, err := state.LoadState()
if err != nil {
    return err
}
defer st.Save() // Must strictly adhere to flock underlying mechanics
```

## Testing Strategy
- **Framework:** Standard Go `testing` library.
- **Coverage Requirements:**
  - `resolver/` package: Mock OS calls to verify `.so` mapping heuristics (prefer `-base`, ignore `-dev`).
  - `ai/` package: Parse simulated Two-Stage AI payloads successfully.
  - `release/` package: Verify `LD_LIBRARY_PATH` is correctly mutated when executing `ldd` on a test binary.
  - `config/` package: Verify repo context scanning (e.g., finding `Cargo.toml` returns `["cargo", "mise", "native_os"]`).

## Boundaries
- **Always do:** 
  - Execute dependency manifests via the native Go `resolver/` module.
  - Fail-fast (exit code 1) if `--prompt-deps` is used but `isatty` is false.
  - Load/Save strictly through the `state` package to respect `state.json.lock`.
- **Ask first:** 
  - Restructuring the core Kong `CliParams` structs deeply.
  - Adding heavy 3rd-party dependencies for AST/manifest parsing.
- **Never do:** 
  - Hardcode `sudo apt-get` in business logic (must use `resolver/`).
  - Execute AI-generated bash scripts as `root`.

## Success Criteria
1. **AI JSON Manifest Parsing:** `ghpt source` correctly parses the AI's JSON block, intercepts execution, resolves packages via `resolver/`, and halts for stdin if `--prompt-deps` is active, before executing the bash script.
2. **Static Dependency Resolution:** `ghpt install --resolve-deps` extracts a `.tar.gz`, sets `LD_LIBRARY_PATH` to the extraction directory, runs `ldd`, successfully maps missing libraries (e.g., `apt-file`), applies tie-breaker heuristics, and installs them natively.
3. **Contextual Toolchain Priority:** `ghpt source` running in a Python repository reads `pyproject.toml`, injects the `["uv", "mise", "native_os"]` priority array into the AI prompt, and the resulting JSON manifest respects this.
4. **Flag Enforcement:** `--add-deps` is successfully removed. `--prompt-deps` pauses for user input. `--resolve-deps` runs silently. `--no-deps` bypasses all logic.
5. **State/Hook Integrity:** `ghpt hook ls` functions correctly, and V1 state is safely migrated to V2 (segregating apps, repos, system-packages, hooks).
6. **Final Output Output:** Any `ghpt install` or `ghpt source` command prints a finalized list of every installed/symlinked file to stdout at the conclusion of the animation.
