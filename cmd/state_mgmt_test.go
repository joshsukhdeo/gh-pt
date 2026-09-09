package cmd

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/adrg/xdg"
	"github.com/joshsukhdeo/gh-install/state"
)

func TestListState_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	xdg.Reload()

	err := ListState(&RootCLI{})
	assert.NoError(t, err)
}

func TestListState_WithApps(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	xdg.Reload()

	st, err := state.LoadState()
	assert.NoError(t, err)
	st.AddApp(&state.InstalledApp{
		Repository: "test/repo",
		Version: "v1",
		Global: false,
		Disabled: false,
	})

	err = ListState(&RootCLI{})
	assert.NoError(t, err)
}
