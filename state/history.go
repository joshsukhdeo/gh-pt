package state

import (
	"encoding/json"
	"path/filepath"
	"time"

	"github.com/adrg/xdg"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

var HistoryLogger = &lumberjack.Logger{
	Filename:   filepath.Join(xdg.DataHome, "gh-install", "history.jsonl"),
	MaxSize:    5,
	MaxBackups: 2,
}

type HistoryEntry struct {
	Time    string `json:"time"`
	Action  string `json:"action"`
	Repo    string `json:"repo"`
	Version string `json:"version"`
}

func LogHistory(action, repo, version string) {
	entry := HistoryEntry{
		Time:    time.Now().Format(time.RFC3339),
		Action:  action,
		Repo:    repo,
		Version: version,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	data = append(data, '\n')
	_, _ = HistoryLogger.Write(data)
}
