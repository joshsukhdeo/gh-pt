package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/adrg/xdg"
	"github.com/joshsukhdeo/gh-pt/params"
	"github.com/joshsukhdeo/gh-pt/state"
	"github.com/pterm/pterm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestCmdEnv(t *testing.T) string {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("NO_COLOR", "1")
	xdg.DataHome = tmpDir
	xdg.Reload()
	return tmpDir
}

func captureCmdOutput(t *testing.T, fn func() error) (string, error) {
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	pterm.SetDefaultOutput(w)
	defer func() {
		pterm.SetDefaultOutput(origStdout)
		os.Stdout = origStdout
	}()

	runErr := fn()

	_ = w.Close()

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()

	return buf.String(), runErr
}

func TestRunHistory_Empty(t *testing.T) {
	_ = setupTestCmdEnv(t)

	cli := &params.HistoryCmd{
		Days:     30,
		Format:   "table",
		NoColor:  true,
		NoEmojis: true,
	}

	out, err := captureCmdOutput(t, func() error {
		return RunHistory(cli)
	})
	require.NoError(t, err)
	assert.Contains(t, out, "No update history found")
}

func TestRunHistory_TableAndJSON(t *testing.T) {
	_ = setupTestCmdEnv(t)

	// Record a couple of update history events
	_, err := state.RecordUpdate("install", "junegunn/fzf", "", "v0.50.0", nil, nil, true, "Installed fzf")
	require.NoError(t, err)

	_, err = state.RecordUpdate("update", "junegunn/fzf", "v0.50.0", "v0.51.0", nil, nil, true, "Updated fzf")
	require.NoError(t, err)

	// Test Table format
	tableCli := &params.HistoryCmd{
		Days:     30,
		Format:   "table",
		NoColor:  true,
		NoEmojis: true,
	}

	tableOut, err := captureCmdOutput(t, func() error {
		return RunHistory(tableCli)
	})
	require.NoError(t, err)
	assert.Contains(t, tableOut, "junegunn/fzf")
	assert.Contains(t, tableOut, "v0.50.0")
	assert.Contains(t, tableOut, "v0.51.0")

	// Test JSON format
	jsonCli := &params.HistoryCmd{
		Days:     30,
		Format:   "json",
		NoColor:  true,
		NoEmojis: true,
	}

	jsonOut, err := captureCmdOutput(t, func() error {
		return RunHistory(jsonCli)
	})
	require.NoError(t, err)
	assert.Contains(t, jsonOut, `"repository": "junegunn/fzf"`)
	assert.Contains(t, jsonOut, `"action": "update"`)
}

func TestRunHistory_CleanPrune(t *testing.T) {
	_ = setupTestCmdEnv(t)

	// Manually inject expired record
	expiredRecord := state.HistoryRecord{
		ID:         "hist_old",
		Timestamp:  time.Now().Add(-40 * 24 * time.Hour),
		Action:     "install",
		Repository: "test/old",
		NewVersion: "v0.1.0",
		Success:    true,
	}
	require.NoError(t, state.MutateHistory(func(db *state.HistoryDB) error {
		db.Records = append(db.Records, expiredRecord)
		return nil
	}))

	cli := &params.HistoryCmd{
		Days:     30,
		Clean:    true,
		NoColor:  true,
		NoEmojis: true,
	}

	out, err := captureCmdOutput(t, func() error {
		return RunHistory(cli)
	})
	require.NoError(t, err)
	assert.Contains(t, out, "Pruned 1 history record(s)")
}

func TestRunRollback_DryRunAndLive(t *testing.T) {
	tmpDir := setupTestCmdEnv(t)

	binDir := filepath.Join(tmpDir, "bin")
	require.NoError(t, os.MkdirAll(binDir, 0755))
	binPath := filepath.Join(binDir, "bat")

	// Create original v0.24.0 binary
	require.NoError(t, os.WriteFile(binPath, []byte("bat v0.24.0 binary"), 0755))

	appV1 := &state.InstalledApp{
		Repository:        "sharkdp/bat",
		Version:           "v0.24.0",
		TargetPath:        binDir,
		InstalledBinaries: []string{"bat"},
	}

	st, err := state.LoadState()
	require.NoError(t, err)
	require.NoError(t, st.AddApp(appV1))

	// Backup app v1
	backups, err := state.BackupAppBinaries(appV1)
	require.NoError(t, err)
	require.NotEmpty(t, backups)

	// Simulate upgrade to v0.25.0
	require.NoError(t, os.WriteFile(binPath, []byte("bat v0.25.0 binary"), 0755))
	appV2 := &state.InstalledApp{
		Repository:        "sharkdp/bat",
		Version:           "v0.25.0",
		TargetPath:        binDir,
		InstalledBinaries: []string{"bat"},
	}
	require.NoError(t, st.AddApp(appV2))

	// Record update in history
	_, err = state.RecordUpdate("update", "sharkdp/bat", "v0.24.0", "v0.25.0", appV1, backups, true, "Upgraded bat")
	require.NoError(t, err)

	// 1. Dry Run Rollback
	dryCli := &params.RollbackCmd{
		Repository: "sharkdp/bat",
		DryRun:     true,
		Force:      true,
		NoColor:    true,
		NoEmojis:   true,
	}

	dryOut, err := captureCmdOutput(t, func() error {
		return RunRollback(dryCli)
	})
	require.NoError(t, err)
	assert.Contains(t, dryOut, "[dry-run] Rollback plan for sharkdp/bat")
	assert.Contains(t, dryOut, "Current version:  v0.25.0")
	assert.Contains(t, dryOut, "Target version:   v0.24.0")

	// File on disk should still be v0.25.0
	c, _ := os.ReadFile(binPath)
	assert.Equal(t, "bat v0.25.0 binary", string(c))

	// 2. Live Rollback
	liveCli := &params.RollbackCmd{
		Repository: "sharkdp/bat",
		Force:      true,
		NoColor:    true,
		NoEmojis:   true,
	}

	liveOut, err := captureCmdOutput(t, func() error {
		return RunRollback(liveCli)
	})
	require.NoError(t, err)
	assert.Contains(t, liveOut, "Successfully rolled back sharkdp/bat from v0.25.0 to v0.24.0")

	// File on disk should be restored to v0.24.0
	restoredContent, err := os.ReadFile(binPath)
	require.NoError(t, err)
	assert.Equal(t, "bat v0.24.0 binary", string(restoredContent))

	// State should be reverted to v0.24.0
	freshState, err := state.LoadState()
	require.NoError(t, err)
	assert.Equal(t, "v0.24.0", freshState.Apps["sharkdp/bat"].Version)
}

func TestRunRollback_Errors(t *testing.T) {
	_ = setupTestCmdEnv(t)

	// Missing repository
	cliEmpty := &params.RollbackCmd{}
	err := RunRollback(cliEmpty)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "repository argument is required")

	// Repository not in state
	cliMissing := &params.RollbackCmd{
		Repository: "nonexistent/tool",
		Force:      true,
	}
	err = RunRollback(cliMissing)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not currently installed in state")
}
