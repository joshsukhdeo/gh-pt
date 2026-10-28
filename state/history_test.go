package state

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/adrg/xdg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

func setupTestHistoryEnv(t *testing.T) string {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	xdg.DataHome = tmpDir
	xdg.Reload()
	return tmpDir
}

func TestLogHistory_LegacyCompatibility(t *testing.T) {
	tmpDir := setupTestHistoryEnv(t)
	logFile := filepath.Join(tmpDir, "gh-pt", "history.jsonl")

	origLogger := HistoryLogger
	HistoryLogger = &lumberjack.Logger{
		Filename:   logFile,
		MaxSize:    5,
		MaxBackups: 2,
	}
	defer func() {
		_ = HistoryLogger.Close()
		HistoryLogger = origLogger
	}()

	LogHistory("install", "junegunn/fzf", "v0.50.0")
	LogHistory("update", "junegunn/fzf", "v0.51.0")
	LogHistory("remove", "junegunn/fzf", "")

	_ = HistoryLogger.Close()

	file, err := os.Open(logFile)
	require.NoError(t, err)
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	var entries []HistoryEntry
	for scanner.Scan() {
		var entry HistoryEntry
		err := json.Unmarshal(scanner.Bytes(), &entry)
		require.NoError(t, err)
		entries = append(entries, entry)
	}
	require.NoError(t, scanner.Err())

	require.Len(t, entries, 3)
	assert.Equal(t, "install", entries[0].Action)
	assert.Equal(t, "junegunn/fzf", entries[0].Repo)
	assert.Equal(t, "v0.50.0", entries[0].Version)

	assert.Equal(t, "update", entries[1].Action)
	assert.Equal(t, "junegunn/fzf", entries[1].Repo)
	assert.Equal(t, "v0.51.0", entries[1].Version)

	assert.Equal(t, "remove", entries[2].Action)
	assert.Equal(t, "junegunn/fzf", entries[2].Repo)
	assert.Equal(t, "", entries[2].Version)

	// Also verify records were written to the new history database
	records, err := GetHistory("junegunn/fzf", 30*24*time.Hour)
	require.NoError(t, err)
	require.Len(t, records, 3)
}

func TestHistoryDB_30DayPruning(t *testing.T) {
	tmpDir := setupTestHistoryEnv(t)

	// Create backup dir and dummy backup file
	backupDir := filepath.Join(tmpDir, "gh-pt", "backups", "test_app", "v1.0.0")
	require.NoError(t, os.MkdirAll(backupDir, 0755))
	oldBackupFile := filepath.Join(backupDir, "app_bin")
	require.NoError(t, os.WriteFile(oldBackupFile, []byte("binary v1"), 0755))

	// Manually write an expired record (35 days old) and a recent record (2 days old)
	db := &HistoryDB{
		Version: 1,
		Records: []HistoryRecord{
			{
				ID:          "hist_expired",
				Timestamp:   time.Now().Add(-35 * 24 * time.Hour),
				Action:      "update",
				Repository:  "test/app",
				NewVersion:  "v1.0.0",
				BackupFiles: map[string]string{"/usr/local/bin/app_bin": oldBackupFile},
			},
			{
				ID:         "hist_recent",
				Timestamp:  time.Now().Add(-2 * 24 * time.Hour),
				Action:     "update",
				Repository: "test/app",
				NewVersion: "v1.1.0",
			},
		},
	}
	require.NoError(t, db.SaveUnlocked())

	// Loading DB should auto-prune the 35-day-old record
	loaded, err := LoadHistoryDB()
	require.NoError(t, err)
	pruned := loaded.Prune(DefaultHistoryRetention)
	assert.Equal(t, 1, pruned)
	require.Len(t, loaded.Records, 1)
	assert.Equal(t, "hist_recent", loaded.Records[0].ID)

	// Verify old backup file was deleted from disk
	_, err = os.Stat(oldBackupFile)
	assert.True(t, os.IsNotExist(err), "expired backup file should be removed from disk")
}

func TestBackupAppBinaries_And_Rollback(t *testing.T) {
	tmpDir := setupTestHistoryEnv(t)

	binDir := filepath.Join(tmpDir, "bin")
	require.NoError(t, os.MkdirAll(binDir, 0755))

	binPath := filepath.Join(binDir, "fzf")
	require.NoError(t, os.WriteFile(binPath, []byte("v0.50.0 executable"), 0755))

	appV1 := &InstalledApp{
		Repository:        "junegunn/fzf",
		Version:           "v0.50.0",
		TargetPath:        binDir,
		InstalledBinaries: []string{"fzf"},
	}

	st, err := LoadState()
	require.NoError(t, err)
	require.NoError(t, st.AddApp(appV1))

	// Backup app v1 binaries
	backups, err := BackupAppBinaries(appV1)
	require.NoError(t, err)
	require.Contains(t, backups, binPath)
	backupPath := backups[binPath]

	// Verify backup exists on disk
	content, err := os.ReadFile(backupPath)
	require.NoError(t, err)
	assert.Equal(t, "v0.50.0 executable", string(content))

	// Now simulate upgrading to v0.51.0
	require.NoError(t, os.WriteFile(binPath, []byte("v0.51.0 executable"), 0755))
	appV2 := &InstalledApp{
		Repository:        "junegunn/fzf",
		Version:           "v0.51.0",
		TargetPath:        binDir,
		InstalledBinaries: []string{"fzf"},
	}
	require.NoError(t, st.AddApp(appV2))

	// Record the update in history
	_, err = RecordUpdate("update", "junegunn/fzf", "v0.50.0", "v0.51.0", appV1, backups, true, "Upgraded to v0.51.0")
	require.NoError(t, err)

	// Test Dry-Run Rollback
	dryRunRes, err := RollbackApp("junegunn/fzf", "", true)
	require.NoError(t, err)
	assert.Equal(t, "v0.51.0", dryRunRes.FromVersion)
	assert.Equal(t, "v0.50.0", dryRunRes.ToVersion)
	assert.Contains(t, dryRunRes.RestoredFiles, binPath)

	// Binary should still be v0.51.0 after dry run
	currContent, _ := os.ReadFile(binPath)
	assert.Equal(t, "v0.51.0 executable", string(currContent))

	// Test Live Rollback
	liveRes, err := RollbackApp("junegunn/fzf", "", false)
	require.NoError(t, err)
	assert.Equal(t, "v0.51.0", liveRes.FromVersion)
	assert.Equal(t, "v0.50.0", liveRes.ToVersion)

	// Verify file restored to v0.50.0
	restoredContent, err := os.ReadFile(binPath)
	require.NoError(t, err)
	assert.Equal(t, "v0.50.0 executable", string(restoredContent))

	// Verify state restored
	reloadedState, err := LoadState()
	require.NoError(t, err)
	assert.Equal(t, "v0.50.0", reloadedState.Apps["junegunn/fzf"].Version)

	// Verify history contains the rollback record
	hist, err := GetHistory("junegunn/fzf", 30*24*time.Hour)
	require.NoError(t, err)
	require.NotEmpty(t, hist)
	assert.Equal(t, "rollback", hist[0].Action)
	assert.Equal(t, "v0.51.0", hist[0].PreviousVersion)
	assert.Equal(t, "v0.50.0", hist[0].NewVersion)
}
