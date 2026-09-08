package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adrg/xdg"
	"github.com/joshsukhdeo/gh-install/cmd"
	"github.com/joshsukhdeo/gh-install/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupState(t *testing.T) string {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	xdg.Reload()

	st, err := state.LoadState()
	require.NoError(t, err)

	st.AddApp(&state.InstalledApp{
		Repository: "test/repo1",
		TargetPath: tmpDir,
		Rename:     map[string]string{"binary1": "bin1"},
		Pinned:     false,
	})
	st.AddApp(&state.InstalledApp{
		Repository:    "test/repo2",
		TargetPath:    tmpDir,
		CompileScript: filepath.Join(tmpDir, "script.sh"),
	})

	return tmpDir
}

func TestRmStateOnly(t *testing.T) {
	setupState(t)
	err := cmd.RmStateOnly("repo1")
	require.NoError(t, err)

	st, _ := state.LoadState()
	_, exists := st.Apps["test/repo1"]
	assert.False(t, exists)
}

func TestRemoveApp(t *testing.T) {
	tmpDir := setupState(t)

	binPath := filepath.Join(tmpDir, "bin1")
	os.WriteFile(binPath, []byte("data"), 0755)

	err := cmd.RemoveApp("repo1", false)
	require.NoError(t, err)

	st, _ := state.LoadState()
	_, exists := st.Apps["test/repo1"]
	assert.False(t, exists)
	assert.NoFileExists(t, binPath)
}

func TestPurgeApp(t *testing.T) {
	tmpDir := setupState(t)

	scriptPath := filepath.Join(tmpDir, "script.sh")
	os.WriteFile(scriptPath, []byte("data"), 0755)

	err := cmd.RemoveApp("repo2", true)
	require.NoError(t, err)

	st, _ := state.LoadState()
	_, exists := st.Apps["test/repo2"]
	assert.False(t, exists)
	assert.NoFileExists(t, scriptPath)
}

func TestPinAppState(t *testing.T) {
	setupState(t)

	err := cmd.PinAppState("repo1")
	require.NoError(t, err)

	st, _ := state.LoadState()
	assert.True(t, st.Apps["test/repo1"].Pinned)
}
