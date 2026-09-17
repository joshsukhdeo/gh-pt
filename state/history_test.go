package state

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

func TestLogHistory(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "history.jsonl")

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
	_, err = time.Parse(time.RFC3339, entries[0].Time)
	assert.NoError(t, err)

	assert.Equal(t, "update", entries[1].Action)
	assert.Equal(t, "junegunn/fzf", entries[1].Repo)
	assert.Equal(t, "v0.51.0", entries[1].Version)

	assert.Equal(t, "remove", entries[2].Action)
	assert.Equal(t, "junegunn/fzf", entries[2].Repo)
	assert.Equal(t, "", entries[2].Version)
}
