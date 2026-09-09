package tests

import (
	"testing"
	"os"
	"path/filepath"
	"github.com/stretchr/testify/assert"
	"github.com/adrg/xdg"
	"github.com/joshsukhdeo/gh-install/state"
	"github.com/joshsukhdeo/gh-install/cmd"
)

func TestPinAppState(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	xdg.Reload()

	st, _ := state.LoadState()
	st.AddApp(&state.InstalledApp{
		Repository: "test/repo1",
		Version: "v1.0",
	})

	err := cmd.PinAppState("test/repo1")
	assert.NoError(t, err)

	st, _ = state.LoadState()
	assert.True(t, st.Apps["test/repo1"].Pinned)
}

func TestRmStateOnly(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	xdg.Reload()

	st, _ := state.LoadState()
	st.AddApp(&state.InstalledApp{
		Repository: "test/repo1",
		Version: "v1.0",
	})

	err := cmd.RmStateOnly("test/repo1")
	assert.NoError(t, err)

	st, _ = state.LoadState()
	assert.Nil(t, st.Apps["test/repo1"])
}

func TestRemoveApp(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	xdg.Reload()

	st, _ := state.LoadState()




	binDir := filepath.Join(tmpDir, "bin1")
	os.WriteFile(binDir, []byte(""), 0755)

	st.AddApp(&state.InstalledApp{
		Repository: "test/repo1",
		Version: "v1.0",
		TargetPath: tmpDir,
		AssetBinaries: []string{"bin1"},
	})

	err := cmd.RemoveApp("test/repo1", false)
	assert.NoError(t, err)

	st, _ = state.LoadState()
	assert.Nil(t, st.Apps["test/repo1"])
	assert.NoFileExists(t, binDir)
}

func TestPurgeApp(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	xdg.Reload()

	st, _ := state.LoadState()

	scriptPath := filepath.Join(tmpDir, "script.sh")
	os.WriteFile(scriptPath, []byte(""), 0755)

	st.AddApp(&state.InstalledApp{
		Repository: "test/repo2",
		CompileScript: scriptPath,
	})

	err := cmd.RemoveApp("test/repo2", true)
	assert.NoError(t, err)

	st, _ = state.LoadState()
	assert.Nil(t, st.Apps["test/repo2"])
	assert.NoFileExists(t, scriptPath)
}
