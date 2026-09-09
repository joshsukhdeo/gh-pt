package tests

import (
	"github.com/adrg/xdg"
	"github.com/joshsukhdeo/gh-install/cmd"
	"github.com/joshsukhdeo/gh-install/params"
	"github.com/joshsukhdeo/gh-install/state"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestListState_LsAndLl(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	xdg.Reload()

	st, _ := state.LoadState()
	st.AddApp(&state.InstalledApp{
		Repository:   "test/repo-pkg",
		PackageNames: []string{"pkg1", "pkg2"},
	})
	st.AddApp(&state.InstalledApp{
		Repository:    "test/repo-types-long",
		Type:          []string{"very-long-type-name-that-is-over-25-chars-abc-def-ghi"},
		AssetBinaries: []string{"bin1", "bin2"},
	})

	r1 := &cmd.RootCLI{CLI: params.CLI{Ls: "test", Full: true}}
	err := cmd.ListState(r1)
	assert.NoError(t, err)

	r2 := &cmd.RootCLI{CLI: params.CLI{Ll: "test", Full: true}}
	err = cmd.ListState(r2)
	assert.NoError(t, err)
}
