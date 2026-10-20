# Spec: Repository Context Heuristics

## Objective
To provide a ruthlessly comprehensive, deterministic mapping mechanism that `ghpt source` will use to scan a cloned repository's root directory. The detected ecosystem will dynamically select the correct package manager priority array from the user's `config.yml` `dependency_resolution` schema and inject it into the AI prompt.

## The Config Schema Map
The heuristics below map directly to the keys expected in the `config.yml` `dependency_resolution.priorities` block. If a key is missing from the user's config, or if no heuristics match, `ghpt` falls back to the `default` priority array.

```yaml
dependency_resolution:
  priorities:
    default: ["mise", "ghpt", "native_os", "flatpak"]
    rust:    ["cargo", "mise", "native_os"]
    python:  ["uv", "mise", "native_os"]
    c_cpp:   ["vcpkg", "native_os"]
    go:      ["go", "mise", "native_os"]
    node:    ["pnpm", "bun", "yarn", "npm", "native_os"]
    java:    ["gradle", "maven", "native_os"]
    ruby:    ["bundle", "mise", "native_os"]
    dotnet:  ["dotnet", "native_os"]
    swift:   ["swift", "native_os"]
    zig:     ["zig", "native_os"]
```

## Language & Toolchain Heuristics
`ghpt` will scan the repository root (and potentially 1 level deep for monorepos) for the following indicator files. The first match determines the ecosystem.

### 1. Rust (`rust`)
- `Cargo.toml`
- `rust-toolchain.toml`
- `rust-toolchain`

### 2. Python (`python`)
- `pyproject.toml`
- `requirements.txt`
- `setup.py`
- `setup.cfg`
- `Pipfile`
- `poetry.lock`
- `tox.ini`

### 3. C / C++ (`c_cpp`)
- `CMakeLists.txt`
- `Makefile` (If other specific language files are absent; often used generally)
- `configure.ac` / `configure` (Autotools)
- `meson.build`
- `build.ninja`
- `conanfile.txt` / `conanfile.py`
- `vcpkg.json`
- `*.sln` (Visual Studio C++ Solutions, if C# indicator is missing)

### 4. Go (`go`)
- `go.mod`
- `Gopkg.toml` (Legacy Dep)
- `glide.yaml` (Legacy Glide)

### 5. Node.js / JavaScript / TypeScript (`node`)
- `package.json`
- `yarn.lock`
- `pnpm-workspace.yaml` / `pnpm-lock.yaml`
- `bun.lockb`
- `package-lock.json`

### 6. Java / Kotlin / Scala (`java`)
- `pom.xml` (Maven)
- `build.gradle` / `build.gradle.kts` (Gradle)
- `build.sbt` (Scala/SBT)
- `gradlew` / `gradlew.bat`

### 7. Ruby (`ruby`)
- `Gemfile`
- `*.gemspec`
- `Rakefile`

### 8. PHP (`php`)
- `composer.json`
- `composer.lock`

### 9. C# / .NET (`dotnet`)
- `*.csproj`
- `*.fsproj`
- `global.json`
- `*.sln` (When adjacent to `.csproj`)

### 10. Swift / Objective-C (`swift`)
- `Package.swift` (Swift Package Manager)
- `Podfile` (CocoaPods)
- `Cartfile` (Carthage)

### 11. Dart / Flutter (`dart`)
- `pubspec.yaml`
- `pubspec.lock`

### 12. Elixir / Erlang (`elixir`)
- `mix.exs`
- `rebar.config`

### 13. Haskell (`haskell`)
- `stack.yaml`
- `*.cabal`
- `cabal.project`

### 14. Zig (`zig`)
- `build.zig`
- `build.zig.zon`

### 15. Nim (`nim`)
- `*.nimble`

### 16. Crystal (`crystal`)
- `shard.yml`

## Resolution Tie-Breakers (Monorepos)
In the event that a repository contains multiple indicator files (e.g., a React frontend with a `package.json` and a Rust backend with a `Cargo.toml`), the heuristic engine uses the following logic:
1. **Root Dominance:** Files in the exact root of the repository take precedence over files in subdirectories.
2. **AI Deferral:** If the root contains multiple distinct ecosystem indicators (e.g., both `package.json` and `Cargo.toml` in the root), `ghpt` will pass **both** priority arrays to the AI template. The AI will evaluate the primary compilation target (based on the user's install command or repo description) and select the appropriate array for the JSON dependency manifest.

## Success Criteria for Heuristics
1. **Deterministic Matching:** Scanning the `joshsukhdeo/gh-pt` repository correctly identifies `go.mod` and triggers the `go` priority chain.
2. **Fallback:** Scanning a repository with only a `build.sh` script and no standard package manager files safely defaults to the `default` priority chain without erroring.
3. **Monorepo Handling:** Scanning a repository with a `package.json` in `/frontend` and a `Cargo.toml` in the root correctly prioritizes `rust` due to Root Dominance.
