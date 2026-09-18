package heuristics

import (
	"os"
	"path/filepath"
	"strings"
)

// Config priority keys matching SPEC-HEURISTICS.md and config.yml dependency_resolution.priorities schema.
const (
	PriorityDefault = "default"
	PriorityRust    = "rust"
	PriorityPython  = "python"
	PriorityCCpp    = "c_cpp"
	PriorityGo      = "go"
	PriorityNode    = "node"
	PriorityJava    = "java"
	PriorityRuby    = "ruby"
	PriorityPHP     = "php"
	PriorityDotnet  = "dotnet"
	PrioritySwift   = "swift"
	PriorityDart    = "dart"
	PriorityElixir  = "elixir"
	PriorityHaskell = "haskell"
	PriorityZig     = "zig"
	PriorityNim     = "nim"
	PriorityCrystal = "crystal"
)

// DefaultPriorities maps priority keys to their default resolution chains per SPEC-HEURISTICS.md.
var DefaultPriorities = map[string][]string{
	PriorityDefault: {"mise", "ghpt", "native_os", "flatpak"},
	PriorityRust:    {"cargo", "mise", "native_os"},
	PriorityPython:  {"uv", "mise", "native_os"},
	PriorityCCpp:    {"vcpkg", "native_os"},
	PriorityGo:      {"go", "mise", "native_os"},
	PriorityNode:    {"pnpm", "bun", "yarn", "npm", "native_os"},
	PriorityJava:    {"gradle", "maven", "native_os"},
	PriorityRuby:    {"bundle", "mise", "native_os"},
	PriorityDotnet:  {"dotnet", "native_os"},
	PrioritySwift:   {"swift", "native_os"},
	PriorityZig:     {"zig", "native_os"},
}

// orderedEcosystems defines the deterministic evaluation order per SPEC-HEURISTICS.md.
var orderedEcosystems = []string{
	PriorityRust,
	PriorityPython,
	PriorityCCpp,
	PriorityGo,
	PriorityNode,
	PriorityJava,
	PriorityRuby,
	PriorityPHP,
	PriorityDotnet,
	PrioritySwift,
	PriorityDart,
	PriorityElixir,
	PriorityHaskell,
	PriorityZig,
	PriorityNim,
	PriorityCrystal,
}

// isIgnoredDir returns true if a directory should be skipped during repository scanning.
func isIgnoredDir(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	switch strings.ToLower(name) {
	case "node_modules", "vendor", "target", "dist", "build":
		return true
	}
	return false
}

// MapEcosystemToPriorityKey normalizes an ecosystem name, language name, or alias
// to the corresponding priority key in config.yml.
// If unrecognized or empty, it returns PriorityDefault ("default").
func MapEcosystemToPriorityKey(ecosystem string) string {
	switch strings.ToLower(strings.TrimSpace(ecosystem)) {
	case "rust":
		return PriorityRust
	case "python", "py":
		return PriorityPython
	case "c_cpp", "c", "cpp", "c++":
		return PriorityCCpp
	case "go", "golang":
		return PriorityGo
	case "node", "nodejs", "javascript", "typescript", "js", "ts":
		return PriorityNode
	case "java", "kotlin", "scala":
		return PriorityJava
	case "ruby":
		return PriorityRuby
	case "php":
		return PriorityPHP
	case "dotnet", "c#", "csharp", "f#", "fsharp", ".net":
		return PriorityDotnet
	case "swift", "objective-c", "objc":
		return PrioritySwift
	case "dart", "flutter":
		return PriorityDart
	case "elixir", "erlang":
		return PriorityElixir
	case "haskell":
		return PriorityHaskell
	case "zig":
		return PriorityZig
	case "nim":
		return PriorityNim
	case "crystal":
		return PriorityCrystal
	case PriorityDefault:
		return PriorityDefault
	default:
		return PriorityDefault
	}
}

// ResolvePriorityChain returns the priority array for an ecosystem given a priority map.
// If priorities is nil or the key is missing, it falls back to default priorities.
func ResolvePriorityChain(priorities map[string][]string, ecosystem string) []string {
	key := MapEcosystemToPriorityKey(ecosystem)
	if priorities != nil {
		if chain, ok := priorities[key]; ok && len(chain) > 0 {
			return chain
		}
		if defaultChain, ok := priorities[PriorityDefault]; ok && len(defaultChain) > 0 {
			return defaultChain
		}
	}
	if chain, ok := DefaultPriorities[key]; ok {
		return chain
	}
	return DefaultPriorities[PriorityDefault]
}

type dirScanResult struct {
	filenames   []string
	hasMakefile bool
	hasSln      bool
	hasDotnet   bool
}

func scanSingleDir(dirPath string) (dirScanResult, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return dirScanResult{}, err
	}

	var res dirScanResult
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		res.filenames = append(res.filenames, name)

		if name == "Makefile" || name == "makefile" {
			res.hasMakefile = true
		}

		if strings.HasSuffix(strings.ToLower(name), ".sln") {
			res.hasSln = true
		}

		if strings.HasSuffix(strings.ToLower(name), ".csproj") ||
			strings.HasSuffix(strings.ToLower(name), ".fsproj") ||
			name == "global.json" {
			res.hasDotnet = true
		}
	}
	return res, nil
}

// matchEcosystems inspects a list of filenames and flags to find matching ecosystems.
// It excludes generic Makefile from specific matches.
func matchEcosystems(scan dirScanResult) map[string]bool {
	matched := make(map[string]bool)

	for _, f := range scan.filenames {
		lower := strings.ToLower(f)

		// 1. Rust
		switch f {
		case "Cargo.toml", "rust-toolchain.toml", "rust-toolchain":
			matched[PriorityRust] = true
		}

		// 2. Python
		switch f {
		case "pyproject.toml", "requirements.txt", "setup.py", "setup.cfg", "Pipfile", "poetry.lock", "tox.ini":
			matched[PriorityPython] = true
		}

		// 3. C / C++ (specific indicators)
		switch f {
		case "CMakeLists.txt", "configure.ac", "configure", "meson.build", "build.ninja", "conanfile.txt", "conanfile.py", "vcpkg.json":
			matched[PriorityCCpp] = true
		}

		// 4. Go
		switch f {
		case "go.mod", "Gopkg.toml", "glide.yaml":
			matched[PriorityGo] = true
		}

		// 5. Node.js / JavaScript / TypeScript
		switch f {
		case "package.json", "yarn.lock", "pnpm-workspace.yaml", "pnpm-lock.yaml", "bun.lockb", "package-lock.json":
			matched[PriorityNode] = true
		}

		// 6. Java / Kotlin / Scala
		switch f {
		case "pom.xml", "build.gradle", "build.gradle.kts", "build.sbt", "gradlew", "gradlew.bat":
			matched[PriorityJava] = true
		}

		// 7. Ruby
		if f == "Gemfile" || f == "Rakefile" || strings.HasSuffix(lower, ".gemspec") {
			matched[PriorityRuby] = true
		}

		// 8. PHP
		if f == "composer.json" || f == "composer.lock" {
			matched[PriorityPHP] = true
		}

		// 9. C# / .NET
		if strings.HasSuffix(lower, ".csproj") || strings.HasSuffix(lower, ".fsproj") || f == "global.json" {
			matched[PriorityDotnet] = true
		}

		// 10. Swift
		switch f {
		case "Package.swift", "Podfile", "Cartfile":
			matched[PrioritySwift] = true
		}

		// 11. Dart
		if f == "pubspec.yaml" || f == "pubspec.lock" {
			matched[PriorityDart] = true
		}

		// 12. Elixir
		if f == "mix.exs" || f == "rebar.config" {
			matched[PriorityElixir] = true
		}

		// 13. Haskell
		if f == "stack.yaml" || f == "cabal.project" || strings.HasSuffix(lower, ".cabal") {
			matched[PriorityHaskell] = true
		}

		// 14. Zig
		if f == "build.zig" || f == "build.zig.zon" {
			matched[PriorityZig] = true
		}

		// 15. Nim
		if strings.HasSuffix(lower, ".nimble") {
			matched[PriorityNim] = true
		}

		// 16. Crystal
		if f == "shard.yml" {
			matched[PriorityCrystal] = true
		}
	}

	// Visual Studio Solution (.sln):
	// If .csproj/.fsproj indicator is present, it's dotnet; otherwise c_cpp.
	if scan.hasSln {
		if scan.hasDotnet {
			matched[PriorityDotnet] = true
		} else {
			matched[PriorityCCpp] = true
		}
	}

	return matched
}

// DetectAllEcosystems scans repoPath and returns all unique ecosystem priority keys detected,
// adhering to the root-dominance tie-breaker logic specified in SPEC-HEURISTICS.md.
// If multiple indicators exist at the dominant level, all are returned (for AI deferral).
// If no indicators match, it returns []string{PriorityDefault}.
func DetectAllEcosystems(repoPath string) ([]string, error) {
	entries, err := os.ReadDir(repoPath)
	if err != nil {
		return nil, err
	}

	// 1. Scan root directory
	rootScan, err := scanSingleDir(repoPath)
	if err != nil {
		return nil, err
	}
	rootMatches := matchEcosystems(rootScan)

	// Collect subdirectories (1 level deep) for monorepo scanning
	var subdirs []string
	for _, entry := range entries {
		if entry.IsDir() && !isIgnoredDir(entry.Name()) {
			subdirs = append(subdirs, filepath.Join(repoPath, entry.Name()))
		}
	}

	// If root has .sln and nested dirs have .csproj, update root match for .sln
	if rootScan.hasSln && !rootScan.hasDotnet {
		for _, sub := range subdirs {
			subScan, err := scanSingleDir(sub)
			if err == nil && subScan.hasDotnet {
				delete(rootMatches, PriorityCCpp)
				rootMatches[PriorityDotnet] = true
				break
			}
		}
	}

	// Root Dominance: If specific language indicators exist in root, they dominate.
	if len(rootMatches) > 0 {
		var result []string
		for _, eco := range orderedEcosystems {
			if rootMatches[eco] {
				result = append(result, eco)
			}
		}
		return result, nil
	}

	// 2. Scan subdirectories 1 level deep
	nestedMatches := make(map[string]bool)
	nestedHasMakefile := false

	for _, sub := range subdirs {
		subScan, err := scanSingleDir(sub)
		if err != nil {
			continue
		}
		if subScan.hasMakefile {
			nestedHasMakefile = true
		}
		for eco := range matchEcosystems(subScan) {
			nestedMatches[eco] = true
		}
	}

	if len(nestedMatches) > 0 {
		var result []string
		for _, eco := range orderedEcosystems {
			if nestedMatches[eco] {
				result = append(result, eco)
			}
		}
		return result, nil
	}

	// 3. Makefile fallback heuristic:
	// "Makefile (If other specific language files are absent; often used generally)"
	if rootScan.hasMakefile || nestedHasMakefile {
		return []string{PriorityCCpp}, nil
	}

	// 4. Default fallback
	return []string{PriorityDefault}, nil
}

// DetectEcosystem scans the repository root (and 1 level deep for monorepos)
// and returns the primary detected ecosystem config priority key.
// Root dominance ensures root files take precedence over subdirectories.
// In the event of multiple matches at the dominant level, the first match
// per SPEC-HEURISTICS.md deterministically determines the ecosystem.
// If no indicators match, it defaults safely to PriorityDefault ("default") without error.
func DetectEcosystem(repoPath string) (string, error) {
	all, err := DetectAllEcosystems(repoPath)
	if err != nil {
		return "", err
	}
	if len(all) == 0 {
		return PriorityDefault, nil
	}
	return all[0], nil
}
