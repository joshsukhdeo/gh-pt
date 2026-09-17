package cmd_test

import (
	"testing"

	"github.com/adrg/xdg"
	"github.com/joshsukhdeo/gh-pt/cmd"
	"github.com/joshsukhdeo/gh-pt/params"
	"github.com/joshsukhdeo/gh-pt/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListState_LsAndLl(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	xdg.Reload()

	st, _ := state.LoadState()
	require.NoError(t, st.AddApp(&state.InstalledApp{
		Repository:   "test/repo-pkg",
		PackageNames: []string{"pkg1", "pkg2"},
	}))
	require.NoError(t, st.AddApp(&state.InstalledApp{
		Repository:    "test/repo-types-long",
		Type:          []string{"very-long-type-name-that-is-over-25-chars-abc-def-ghi"},
		AssetBinaries: []string{"bin1", "bin2"},
	}))

	r1 := &cmd.RootCLI{ExecContext: params.ExecContext{Ls: "test", Full: true}}
	err := cmd.ListState(r1)
	assert.NoError(t, err)

	r2 := &cmd.RootCLI{ExecContext: params.ExecContext{Ll: "test", Full: true}}
	err = cmd.ListState(r2)
	assert.NoError(t, err)
}
