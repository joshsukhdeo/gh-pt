package heuristics

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createMockTree(t *testing.T, files []string) string {
	t.Helper()
	dir := t.TempDir()
	for _, f := range files {
		fullPath := filepath.Join(dir, f)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(fullPath, []byte("mock content"), 0644)
		require.NoError(t, err)
	}
	return dir
}

func TestDetectEcosystem_SingleIndicators(t *testing.T) {
	tests := []struct {
		name     string
		files    []string
		expected string
	}{
		// Rust
		{"Rust Cargo.toml", []string{"Cargo.toml"}, PriorityRust},
		{"Rust rust-toolchain.toml", []string{"rust-toolchain.toml"}, PriorityRust},
		{"Rust rust-toolchain", []string{"rust-toolchain"}, PriorityRust},

		// Python
		{"Python pyproject.toml", []string{"pyproject.toml"}, PriorityPython},
		{"Python requirements.txt", []string{"requirements.txt"}, PriorityPython},
		{"Python setup.py", []string{"setup.py"}, PriorityPython},
		{"Python setup.cfg", []string{"setup.cfg"}, PriorityPython},
		{"Python Pipfile", []string{"Pipfile"}, PriorityPython},
		{"Python poetry.lock", []string{"poetry.lock"}, PriorityPython},
		{"Python tox.ini", []string{"tox.ini"}, PriorityPython},

		// C/C++
		{"C/C++ CMakeLists.txt", []string{"CMakeLists.txt"}, PriorityCCpp},
		{"C/C++ configure.ac", []string{"configure.ac"}, PriorityCCpp},
		{"C/C++ configure", []string{"configure"}, PriorityCCpp},
		{"C/C++ meson.build", []string{"meson.build"}, PriorityCCpp},
		{"C/C++ build.ninja", []string{"build.ninja"}, PriorityCCpp},
		{"C/C++ conanfile.txt", []string{"conanfile.txt"}, PriorityCCpp},
		{"C/C++ conanfile.py", []string{"conanfile.py"}, PriorityCCpp},
		{"C/C++ vcpkg.json", []string{"vcpkg.json"}, PriorityCCpp},
		{"C/C++ standalone sln", []string{"solution.sln"}, PriorityCCpp},

		// Go
		{"Go go.mod", []string{"go.mod"}, PriorityGo},
		{"Go Gopkg.toml", []string{"Gopkg.toml"}, PriorityGo},
		{"Go glide.yaml", []string{"glide.yaml"}, PriorityGo},

		// Node
		{"Node package.json", []string{"package.json"}, PriorityNode},
		{"Node yarn.lock", []string{"yarn.lock"}, PriorityNode},
		{"Node pnpm-workspace.yaml", []string{"pnpm-workspace.yaml"}, PriorityNode},
		{"Node pnpm-lock.yaml", []string{"pnpm-lock.yaml"}, PriorityNode},
		{"Node bun.lockb", []string{"bun.lockb"}, PriorityNode},
		{"Node package-lock.json", []string{"package-lock.json"}, PriorityNode},

		// Java
		{"Java pom.xml", []string{"pom.xml"}, PriorityJava},
		{"Java build.gradle", []string{"build.gradle"}, PriorityJava},
		{"Java build.gradle.kts", []string{"build.gradle.kts"}, PriorityJava},
		{"Java build.sbt", []string{"build.sbt"}, PriorityJava},
		{"Java gradlew", []string{"gradlew"}, PriorityJava},
		{"Java gradlew.bat", []string{"gradlew.bat"}, PriorityJava},

		// Ruby
		{"Ruby Gemfile", []string{"Gemfile"}, PriorityRuby},
		{"Ruby gemspec", []string{"app.gemspec"}, PriorityRuby},
		{"Ruby Rakefile", []string{"Rakefile"}, PriorityRuby},

		// PHP
		{"PHP composer.json", []string{"composer.json"}, PriorityPHP},
		{"PHP composer.lock", []string{"composer.lock"}, PriorityPHP},

		// .NET
		{"Dotnet csproj", []string{"app.csproj"}, PriorityDotnet},
		{"Dotnet fsproj", []string{"app.fsproj"}, PriorityDotnet},
		{"Dotnet global.json", []string{"global.json"}, PriorityDotnet},
		{"Dotnet sln adjacent to csproj", []string{"app.sln", "app.csproj"}, PriorityDotnet},

		// Swift
		{"Swift Package.swift", []string{"Package.swift"}, PrioritySwift},
		{"Swift Podfile", []string{"Podfile"}, PrioritySwift},
		{"Swift Cartfile", []string{"Cartfile"}, PrioritySwift},

		// Dart
		{"Dart pubspec.yaml", []string{"pubspec.yaml"}, PriorityDart},
		{"Dart pubspec.lock", []string{"pubspec.lock"}, PriorityDart},

		// Elixir
		{"Elixir mix.exs", []string{"mix.exs"}, PriorityElixir},
		{"Elixir rebar.config", []string{"rebar.config"}, PriorityElixir},

		// Haskell
		{"Haskell stack.yaml", []string{"stack.yaml"}, PriorityHaskell},
		{"Haskell cabal", []string{"app.cabal"}, PriorityHaskell},
		{"Haskell cabal.project", []string{"cabal.project"}, PriorityHaskell},

		// Zig
		{"Zig build.zig", []string{"build.zig"}, PriorityZig},
		{"Zig build.zig.zon", []string{"build.zig.zon"}, PriorityZig},

		// Nim
		{"Nim nimble", []string{"app.nimble"}, PriorityNim},

		// Crystal
		{"Crystal shard.yml", []string{"shard.yml"}, PriorityCrystal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := createMockTree(t, tt.files)
			res, err := DetectEcosystem(dir)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, res)
		})
	}
}

func TestDetectEcosystem_MakefileHeuristics(t *testing.T) {
	t.Run("Makefile alone yields c_cpp", func(t *testing.T) {
		dir := createMockTree(t, []string{"Makefile", "src/main.c"})
		res, err := DetectEcosystem(dir)
		require.NoError(t, err)
		assert.Equal(t, PriorityCCpp, res)
	})

	t.Run("Makefile with Go yields go", func(t *testing.T) {
		dir := createMockTree(t, []string{"Makefile", "go.mod", "main.go"})
		res, err := DetectEcosystem(dir)
		require.NoError(t, err)
		assert.Equal(t, PriorityGo, res)
	})

	t.Run("Makefile with Rust yields rust", func(t *testing.T) {
		dir := createMockTree(t, []string{"Makefile", "Cargo.toml", "src/main.rs"})
		res, err := DetectEcosystem(dir)
		require.NoError(t, err)
		assert.Equal(t, PriorityRust, res)
	})

	t.Run("Makefile with Node yields node", func(t *testing.T) {
		dir := createMockTree(t, []string{"Makefile", "package.json"})
		res, err := DetectEcosystem(dir)
		require.NoError(t, err)
		assert.Equal(t, PriorityNode, res)
	})

	t.Run("Makefile at root with nested package.json yields node", func(t *testing.T) {
		dir := createMockTree(t, []string{"Makefile", "frontend/package.json"})
		res, err := DetectEcosystem(dir)
		require.NoError(t, err)
		assert.Equal(t, PriorityNode, res)
	})
}

func TestDetectEcosystem_RootDominance(t *testing.T) {
	t.Run("Root Cargo.toml dominates nested package.json", func(t *testing.T) {
		dir := createMockTree(t, []string{
			"Cargo.toml",
			"src/main.rs",
			"frontend/package.json",
			"frontend/src/index.js",
		})
		res, err := DetectEcosystem(dir)
		require.NoError(t, err)
		assert.Equal(t, PriorityRust, res)
	})

	t.Run("Root package.json dominates nested Cargo.toml", func(t *testing.T) {
		dir := createMockTree(t, []string{
			"package.json",
			"crates/backend/Cargo.toml",
		})
		res, err := DetectEcosystem(dir)
		require.NoError(t, err)
		assert.Equal(t, PriorityNode, res)
	})

	t.Run("Root CMakeLists.txt dominates nested pyproject.toml", func(t *testing.T) {
		dir := createMockTree(t, []string{
			"CMakeLists.txt",
			"bindings/python/pyproject.toml",
		})
		res, err := DetectEcosystem(dir)
		require.NoError(t, err)
		assert.Equal(t, PriorityCCpp, res)
	})
}

func TestDetectEcosystem_NestedDetectionWhenRootEmpty(t *testing.T) {
	t.Run("Nested package.json detected when root has no indicators", func(t *testing.T) {
		dir := createMockTree(t, []string{
			"frontend/package.json",
			"frontend/index.ts",
		})
		res, err := DetectEcosystem(dir)
		require.NoError(t, err)
		assert.Equal(t, PriorityNode, res)
	})

	t.Run("Nested Cargo.toml detected when root has no indicators", func(t *testing.T) {
		dir := createMockTree(t, []string{
			"backend/Cargo.toml",
			"backend/src/main.rs",
		})
		res, err := DetectEcosystem(dir)
		require.NoError(t, err)
		assert.Equal(t, PriorityRust, res)
	})
}

func TestDetectEcosystem_FallbacksAndErrors(t *testing.T) {
	t.Run("Repository with only build.sh defaults safely without error", func(t *testing.T) {
		dir := createMockTree(t, []string{"build.sh", "README.md", "LICENSE"})
		res, err := DetectEcosystem(dir)
		require.NoError(t, err)
		assert.Equal(t, PriorityDefault, res)
	})

	t.Run("Empty repository defaults safely without error", func(t *testing.T) {
		dir := t.TempDir()
		res, err := DetectEcosystem(dir)
		require.NoError(t, err)
		assert.Equal(t, PriorityDefault, res)
	})

	t.Run("Non-existent directory returns error", func(t *testing.T) {
		_, err := DetectEcosystem("/non/existent/path/for/sure/test")
		require.Error(t, err)
	})

	t.Run("Ignored directories do not trigger indicators", func(t *testing.T) {
		dir := createMockTree(t, []string{
			".git/config",
			"node_modules/foo/package.json",
			"target/debug/build.ninja",
			"vendor/go.mod",
		})
		res, err := DetectEcosystem(dir)
		require.NoError(t, err)
		assert.Equal(t, PriorityDefault, res)
	})
}

func TestDetectAllEcosystems_MultipleIndicatorsAtRoot(t *testing.T) {
	t.Run("Multiple distinct indicators at root returned for AI deferral", func(t *testing.T) {
		dir := createMockTree(t, []string{
			"Cargo.toml",
			"package.json",
		})
		all, err := DetectAllEcosystems(dir)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{PriorityRust, PriorityNode}, all)

		primary, err := DetectEcosystem(dir)
		require.NoError(t, err)
		// Rust has higher spec precedence than Node
		assert.Equal(t, PriorityRust, primary)
	})

	t.Run("Root indicators exclude nested indicators in DetectAllEcosystems", func(t *testing.T) {
		dir := createMockTree(t, []string{
			"go.mod",
			"sub/Cargo.toml",
		})
		all, err := DetectAllEcosystems(dir)
		require.NoError(t, err)
		assert.Equal(t, []string{PriorityGo}, all)
	})
}

func TestMapEcosystemToPriorityKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"rust", PriorityRust},
		{"python", PriorityPython},
		{"py", PriorityPython},
		{"c_cpp", PriorityCCpp},
		{"c++", PriorityCCpp},
		{"cpp", PriorityCCpp},
		{"c", PriorityCCpp},
		{"go", PriorityGo},
		{"golang", PriorityGo},
		{"node", PriorityNode},
		{"nodejs", PriorityNode},
		{"javascript", PriorityNode},
		{"typescript", PriorityNode},
		{"js", PriorityNode},
		{"ts", PriorityNode},
		{"java", PriorityJava},
		{"kotlin", PriorityJava},
		{"scala", PriorityJava},
		{"ruby", PriorityRuby},
		{"php", PriorityPHP},
		{"dotnet", PriorityDotnet},
		{"c#", PriorityDotnet},
		{"csharp", PriorityDotnet},
		{"f#", PriorityDotnet},
		{"fsharp", PriorityDotnet},
		{".net", PriorityDotnet},
		{"swift", PrioritySwift},
		{"dart", PriorityDart},
		{"flutter", PriorityDart},
		{"elixir", PriorityElixir},
		{"erlang", PriorityElixir},
		{"haskell", PriorityHaskell},
		{"zig", PriorityZig},
		{"nim", PriorityNim},
		{"crystal", PriorityCrystal},
		{"unknown-lang", PriorityDefault},
		{"", PriorityDefault},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, MapEcosystemToPriorityKey(tt.input))
		})
	}
}

func TestResolvePriorityChain(t *testing.T) {
	customPriorities := map[string][]string{
		PriorityRust:   {"cargo", "custom_pm"},
		PriorityPython: {"uv", "pip"},
	}

	assert.Equal(t, []string{"cargo", "custom_pm"}, ResolvePriorityChain(customPriorities, PriorityRust))
	assert.Equal(t, []string{"uv", "pip"}, ResolvePriorityChain(customPriorities, PriorityPython))
	// Unconfigured key falls back to default priorities
	assert.Equal(t, []string{"go", "mise", "native_os"}, ResolvePriorityChain(customPriorities, PriorityGo))
	// Unknown ecosystem falls back to PriorityDefault
	assert.Equal(t, []string{"mise", "ghpt", "native_os", "flatpak"}, ResolvePriorityChain(customPriorities, "unknown"))
}

func TestDetectEcosystem_SelfRepo(t *testing.T) {
	// Scanning current repo should identify go.mod
	res, err := DetectEcosystem("..")
	require.NoError(t, err)
	assert.Equal(t, PriorityGo, res)
}
