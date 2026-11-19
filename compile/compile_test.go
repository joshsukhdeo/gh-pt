package compile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adrg/xdg"
	"github.com/joshsukhdeo/gh-pt/ai"
	"github.com/joshsukhdeo/gh-pt/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveCompileState(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	xdg.Reload()

	cfg := &Config{
		Repository:        "owner/app",
		TargetPath:        filepath.Join(tmpDir, "bin"),
		SymlinkDir:        filepath.Join(tmpDir, "packages", "owner", "app"),
		ScriptPath:        filepath.Join(tmpDir, "build.sh"),
		Driver:            "vulkan",
		Plugin:            "gnome-extension",
		DriverMap:         `.*icd.*\.json$:vulkan`,
		IncludeSidecars:   true,
		SidecarMode:       "local-map",
		InstalledSidecars: []string{filepath.Join(tmpDir, "sidecars", "data.json")},
	}

	result := &CompileResult{
		Version:        "v1.2.3",
		InstalledFiles: []string{filepath.Join(cfg.SymlinkDir, "binary")},
		Sidecars:       []string{filepath.Join(cfg.SymlinkDir, "manifest.json")},
		Dependencies: []ai.Dependency{
			{Name: "meson"},
			{Name: "ninja"},
		},
		CommitHash: "abcdef123456",
	}

	err := SaveCompileState(cfg, result)
	require.NoError(t, err)

	st, err := state.LoadState()
	require.NoError(t, err)

	app, ok := st.Apps["owner/app"]
	require.True(t, ok)
	assert.Equal(t, "v1.2.3", app.Version)
	assert.Equal(t, cfg.ScriptPath, app.CompileScript)
	assert.Equal(t, cfg.SymlinkDir, app.SymlinkDir)
	assert.Equal(t, "vulkan", app.Driver)
	assert.Equal(t, "gnome-extension", app.Plugin)
	assert.Equal(t, `.*icd.*\.json$:vulkan`, app.DriverMap)
	assert.True(t, app.IncludeSidecars)
	assert.Equal(t, "local-map", app.SidecarMode)
	assert.Equal(t, result.Sidecars, app.InstalledSidecars)

	mapEntry := st.GetInstallMap("owner/app")
	require.NotNil(t, mapEntry)
	require.NotNil(t, mapEntry.Deps)
	assert.Equal(t, "meson, ninja", mapEntry.Deps.Pkg.PackageID)
	require.NotNil(t, mapEntry.Installed)
	assert.Equal(t, result.InstalledFiles, mapEntry.Installed.Files)
}

func TestParseAndWriteManifest(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "manifest.json")

	manifest := &ai.Manifest{
		Toolchain: "meson",
		Dependencies: []ai.Dependency{
			{Name: "glib-2.0"},
		},
	}

	err := WriteManifest(manifestPath, manifest)
	require.NoError(t, err)
	assert.FileExists(t, manifestPath)

	parsed, err := ParseManifest(manifestPath)
	require.NoError(t, err)
	assert.Equal(t, manifest.Toolchain, parsed.Toolchain)
	require.Len(t, parsed.Dependencies, 1)
	assert.Equal(t, "glib-2.0", parsed.Dependencies[0].Name)
}

func TestClean(t *testing.T) {
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "build_to_clean")
	require.NoError(t, os.MkdirAll(testDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(testDir, "file.txt"), []byte("data"), 0644))

	require.NoError(t, Clean(testDir))
	assert.NoDirExists(t, testDir)
}

func TestGetBuildPath(t *testing.T) {
	path := GetBuildPath("owner/repo")
	assert.NotEmpty(t, path)
	assert.True(t, filepath.IsAbs(path))
	assert.Equal(t, "repo", filepath.Base(path))

	assert.Empty(t, GetBuildPath("invalid"))
}
