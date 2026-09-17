package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/adrg/xdg"
	"github.com/joshsukhdeo/gh-pt/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to mock exec.Command
func helperCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	os.Exit(0)
}

func setupState(t *testing.T) string {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	xdg.Reload()

	st, err := state.LoadState()
	require.NoError(t, err)

	require.NoError(t, st.AddApp(&state.InstalledApp{
		Repository: "test/repo1",
		TargetPath: tmpDir,
		Rename:     map[string]string{"binary1": "bin1"},
		Pinned:     false,
	}))
	require.NoError(t, st.AddApp(&state.InstalledApp{
		Repository:    "test/repo2",
		TargetPath:    tmpDir,
		CompileScript: filepath.Join(tmpDir, "script.sh"),
	}))

	return tmpDir
}

func TestRmStateOnly(t *testing.T) {
	setupState(t)
	err := RmStateOnly("repo1")
	require.NoError(t, err)

	st, _ := state.LoadState()
	_, exists := st.Apps["test/repo1"]
	assert.False(t, exists)
}

func TestRemoveApp(t *testing.T) {
	tmpDir := setupState(t)

	binPath := filepath.Join(tmpDir, "bin1")
	require.NoError(t, os.WriteFile(binPath, []byte("data"), 0755))

	origExecCommand := execCommand
	execCommand = helperCommand
	defer func() { execCommand = origExecCommand }()

	err := RemoveApp("repo1", false)
	require.NoError(t, err)

	st, _ := state.LoadState()
	_, exists := st.Apps["test/repo1"]
	assert.False(t, exists)
	assert.NoFileExists(t, binPath)
}

func TestPurgeApp(t *testing.T) {
	tmpDir := setupState(t)

	scriptPath := filepath.Join(tmpDir, "script.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte("data"), 0755))

	err := RemoveApp("repo2", true)
	require.NoError(t, err)

	st, _ := state.LoadState()
	_, exists := st.Apps["test/repo2"]
	assert.False(t, exists)
	assert.NoFileExists(t, scriptPath)
}

func TestPinAppState(t *testing.T) {
	setupState(t)

	err := PinAppState("repo1")
	require.NoError(t, err)

	st, _ := state.LoadState()
	assert.True(t, st.Apps["test/repo1"].Pinned)
}

func TestListState(t *testing.T) {
	setupState(t)
	err := ListState()
	assert.NoError(t, err)
}
