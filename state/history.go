package state

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"github.com/gofrs/flock"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

// DefaultHistoryRetention defines the rolling window for update history tracking (30 days).
const DefaultHistoryRetention = 30 * 24 * time.Hour

var HistoryLogger = &lumberjack.Logger{
	Filename:   filepath.Join(xdg.DataHome, "gh-pt", "history.jsonl"),
	MaxSize:    5,
	MaxBackups: 2,
}

// HistoryEntry is kept for backwards compatibility with legacy lumberjack jsonl logs.
type HistoryEntry struct {
	Time    string `json:"time"`
	Action  string `json:"action"`
	Repo    string `json:"repo"`
	Version string `json:"version"`
}

// HistoryRecord represents a single update, install, rollback, or removal event.
type HistoryRecord struct {
	ID              string            `json:"id"`
	Timestamp       time.Time         `json:"timestamp"`
	Action          string            `json:"action"` // "install", "update", "rollback", "remove"
	Repository      string            `json:"repository"`
	PreviousVersion string            `json:"previous_version,omitempty"`
	NewVersion      string            `json:"new_version,omitempty"`
	AppSnapshot     *InstalledApp     `json:"app_snapshot,omitempty"`
	BackupFiles     map[string]string `json:"backup_files,omitempty"` // originalPath -> backupPath
	Success         bool              `json:"success"`
	Message         string            `json:"message,omitempty"`
}

// HistoryDB contains the history database schema with rolling retention.
type HistoryDB struct {
	Version int             `json:"version"`
	Records []HistoryRecord `json:"records"`
}

// RollbackResult provides details on an executed or simulated rollback.
type RollbackResult struct {
	Repository     string   `json:"repository"`
	FromVersion    string   `json:"from_version"`
	ToVersion      string   `json:"to_version"`
	RestoredFiles  []string `json:"restored_files,omitempty"`
	NeedsReinstall bool     `json:"needs_reinstall,omitempty"`
	AppRestored    bool     `json:"app_restored"`
	RecordID       string   `json:"record_id"`
}

func sanitizeRepo(repo string) string {
	return strings.ReplaceAll(strings.TrimSpace(repo), "/", "_")
}

// GetHistoryDBPath returns the path to the history database JSON file.
func GetHistoryDBPath() string {
	return filepath.Join(GetDataHome(), "gh-pt", "history.json")
}

// GetHistoryLockPath returns the path to the history database lock file.
func GetHistoryLockPath() string {
	return GetHistoryDBPath() + ".lock"
}

// GetBackupsDir returns the directory where previous binary versions are archived.
func GetBackupsDir() string {
	return filepath.Join(GetDataHome(), "gh-pt", "backups")
}

// Prune removes records older than maxAge and unlinks associated backup files on disk.
func (db *HistoryDB) Prune(maxAge time.Duration) int {
	if maxAge <= 0 {
		maxAge = DefaultHistoryRetention
	}
	cutoff := time.Now().Add(-maxAge)
	var active []HistoryRecord
	prunedCount := 0

	for _, rec := range db.Records {
		if rec.Timestamp.Before(cutoff) {
			prunedCount++
			for _, backupPath := range rec.BackupFiles {
				if backupPath != "" {
					_ = os.Remove(backupPath)
					_ = os.Remove(filepath.Dir(backupPath)) // Remove version dir if empty
				}
			}
		} else {
			active = append(active, rec)
		}
	}
	db.Records = active
	return prunedCount
}

// LoadHistoryDB loads the history database, auto-prunes entries older than 30 days,
// and returns the database instance.
func LoadHistoryDB() (*HistoryDB, error) {
	path := GetHistoryDBPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &HistoryDB{
				Version: 1,
				Records: []HistoryRecord{},
			}, nil
		}
		return nil, err
	}

	var db HistoryDB
	if err := json.Unmarshal(data, &db); err != nil {
		return nil, fmt.Errorf("failed to parse history database: %w", err)
	}

	if db.Records == nil {
		db.Records = []HistoryRecord{}
	}

	return &db, nil
}

// SaveUnlocked saves the history database to disk without locking.
func (db *HistoryDB) SaveUnlocked() error {
	path := GetHistoryDBPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func mutateHistoryInternal(autoPrune bool, fn func(db *HistoryDB) error) error {
	path := GetHistoryDBPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	lock := flock.New(GetHistoryLockPath())
	if err := lock.Lock(); err != nil {
		return fmt.Errorf("failed to acquire history lock: %w", err)
	}
	defer func() { _ = lock.Unlock() }()

	db, err := LoadHistoryDB()
	if err != nil {
		return err
	}

	if autoPrune {
		_ = db.Prune(DefaultHistoryRetention)
	}

	if err := fn(db); err != nil {
		return err
	}

	return db.SaveUnlocked()
}

// MutateHistory executes a transactional read-modify-write against the history database
// guarded by a file-level lock.
func MutateHistory(fn func(db *HistoryDB) error) error {
	return mutateHistoryInternal(true, fn)
}

func copyFileContents(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// BackupAppBinaries copies all currently installed binaries for an app into the backup directory.
// Returns a map of originalPath -> backupPath.
func BackupAppBinaries(app *InstalledApp) (map[string]string, error) {
	if app == nil || app.TargetPath == "" || len(app.InstalledBinaries) == 0 {
		return nil, nil
	}

	version := app.Version
	if version == "" {
		version = "unknown"
	}

	backups := make(map[string]string)
	backupDir := filepath.Join(GetBackupsDir(), sanitizeRepo(app.Repository), version)
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	for _, binName := range app.InstalledBinaries {
		srcPath := filepath.Join(app.TargetPath, binName)
		fi, err := os.Stat(srcPath)
		if err != nil {
			continue // Binary might not exist on disk yet
		}
		if !fi.Mode().IsRegular() {
			continue
		}

		dstPath := filepath.Join(backupDir, binName)
		mode := fi.Mode().Perm()
		if mode == 0 {
			mode = 0755
		}
		if err := copyFileContents(srcPath, dstPath, mode); err != nil {
			return nil, fmt.Errorf("failed to backup binary %s: %w", binName, err)
		}
		backups[srcPath] = dstPath
	}

	return backups, nil
}

// CloneInstalledApp creates a deep copy of InstalledApp for history snapshots.
func CloneInstalledApp(app *InstalledApp) *InstalledApp {
	if app == nil {
		return nil
	}
	clone := *app
	if app.Rename != nil {
		clone.Rename = make(map[string]string, len(app.Rename))
		for k, v := range app.Rename {
			clone.Rename[k] = v
		}
	}
	if app.Type != nil {
		clone.Type = append([]string(nil), app.Type...)
	}
	if app.Extract != nil {
		clone.Extract = append([]string(nil), app.Extract...)
	}
	if app.InstalledBinaries != nil {
		clone.InstalledBinaries = append([]string(nil), app.InstalledBinaries...)
	}
	if app.InstalledAssetNames != nil {
		clone.InstalledAssetNames = append([]string(nil), app.InstalledAssetNames...)
	}
	if app.InstalledAssetsFullNames != nil {
		clone.InstalledAssetsFullNames = append([]string(nil), app.InstalledAssetsFullNames...)
	}
	if app.PackageNames != nil {
		clone.PackageNames = append([]string(nil), app.PackageNames...)
	}
	if app.Hooks != nil {
		clone.Hooks = make(map[string]string, len(app.Hooks))
		for k, v := range app.Hooks {
			clone.Hooks[k] = v
		}
	}
	if app.SystemPackages != nil {
		clone.SystemPackages = append([]string(nil), app.SystemPackages...)
	}
	if app.SidecarSymlinkTo != nil {
		clone.SidecarSymlinkTo = append([]string(nil), app.SidecarSymlinkTo...)
	}
	if app.InstalledSidecars != nil {
		clone.InstalledSidecars = append([]string(nil), app.InstalledSidecars...)
	}
	if app.EnvInject != nil {
		clone.EnvInject = append([]string(nil), app.EnvInject...)
	}
	return &clone
}

// RecordUpdate writes an update or install record into the history database.
func RecordUpdate(action, repo, prevVer, newVer string, prevApp *InstalledApp, backupFiles map[string]string, success bool, message string) (*HistoryRecord, error) {
	record := HistoryRecord{
		ID:              fmt.Sprintf("hist_%d_%s", time.Now().UnixNano(), sanitizeRepo(repo)),
		Timestamp:       time.Now(),
		Action:          action,
		Repository:      repo,
		PreviousVersion: prevVer,
		NewVersion:      newVer,
		AppSnapshot:     CloneInstalledApp(prevApp),
		BackupFiles:     backupFiles,
		Success:         success,
		Message:         message,
	}

	err := MutateHistory(func(db *HistoryDB) error {
		db.Records = append(db.Records, record)
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Forward to legacy jsonl logger
	legacyEntry := HistoryEntry{
		Time:    record.Timestamp.Format(time.RFC3339),
		Action:  action,
		Repo:    repo,
		Version: newVer,
	}
	if data, err := json.Marshal(legacyEntry); err == nil {
		data = append(data, '\n')
		_, _ = HistoryLogger.Write(data)
	}

	return &record, nil
}

// LogHistory maintains backward compatibility with existing callers.
func LogHistory(action, repo, version string) {
	_, _ = RecordUpdate(action, repo, "", version, nil, nil, true, "")
}

// GetHistory retrieves history records matching repo (or all if repo is empty),
// filtered by maxAge (defaulting to 30 days), sorted newest to oldest.
func GetHistory(repoFilter string, maxAge time.Duration) ([]HistoryRecord, error) {
	if maxAge <= 0 {
		maxAge = DefaultHistoryRetention
	}

	var results []HistoryRecord
	err := MutateHistory(func(db *HistoryDB) error {
		cutoff := time.Now().Add(-maxAge)
		for _, rec := range db.Records {
			if rec.Timestamp.Before(cutoff) {
				continue
			}
			if repoFilter != "" && !strings.EqualFold(rec.Repository, repoFilter) {
				continue
			}
			results = append(results, rec)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Sort newest first
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.After(results[j].Timestamp)
	})

	return results, nil
}

// PruneHistory explicitly cleans up history entries and backup files older than maxAge.
func PruneHistory(maxAge time.Duration) (int, error) {
	pruned := 0
	err := mutateHistoryInternal(false, func(db *HistoryDB) error {
		pruned = db.Prune(maxAge)
		return nil
	})
	return pruned, err
}

// RollbackApp rolls back the given repository to targetVersion (or its previous version).
func RollbackApp(repo string, targetVersion string, dryRun bool) (*RollbackResult, error) {
	st, err := LoadState()
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	currentApp, exists := st.Apps[repo]
	if !exists {
		return nil, fmt.Errorf("repository %s is not currently installed in state", repo)
	}

	records, err := GetHistory(repo, DefaultHistoryRetention)
	if err != nil {
		return nil, fmt.Errorf("failed to read history: %w", err)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("no history records found for %s in the last 30 days", repo)
	}

	var candidate *HistoryRecord
	if targetVersion != "" {
		for i := range records {
			r := &records[i]
			if r.PreviousVersion == targetVersion || r.NewVersion == targetVersion {
				candidate = r
				break
			}
		}
		if candidate == nil {
			return nil, fmt.Errorf("no history record found matching version %s for %s", targetVersion, repo)
		}
	} else {
		// Find the most recent record that provides a transition from a previous version
		for i := range records {
			r := &records[i]
			if r.PreviousVersion != "" && r.PreviousVersion != currentApp.Version {
				candidate = r
				targetVersion = r.PreviousVersion
				break
			}
			// Or an earlier version recorded as NewVersion that is different from current
			if r.NewVersion != "" && r.NewVersion != currentApp.Version {
				candidate = r
				targetVersion = r.NewVersion
				break
			}
		}
		if candidate == nil {
			return nil, fmt.Errorf("no previous version available to roll back to for %s (current: %s)", repo, currentApp.Version)
		}
	}

	result := &RollbackResult{
		Repository:  repo,
		FromVersion: currentApp.Version,
		ToVersion:   targetVersion,
		RecordID:    candidate.ID,
	}

	// Verify if backup files exist on disk
	hasBackups := len(candidate.BackupFiles) > 0
	if hasBackups {
		for _, bPath := range candidate.BackupFiles {
			if _, err := os.Stat(bPath); err != nil {
				hasBackups = false
				break
			}
		}
	}

	if !hasBackups {
		result.NeedsReinstall = true
	}

	if dryRun {
		if hasBackups {
			for origPath := range candidate.BackupFiles {
				result.RestoredFiles = append(result.RestoredFiles, origPath)
			}
		}
		result.AppRestored = candidate.AppSnapshot != nil
		return result, nil
	}

	// Execute rollback restoration
	if hasBackups {
		for origPath, backupPath := range candidate.BackupFiles {
			fi, err := os.Stat(backupPath)
			if err != nil {
				continue
			}
			mode := fi.Mode().Perm()
			if mode == 0 {
				mode = 0755
			}
			if err := copyFileContents(backupPath, origPath, mode); err != nil {
				return nil, fmt.Errorf("failed to restore file %s from backup: %w", origPath, err)
			}
			result.RestoredFiles = append(result.RestoredFiles, origPath)
		}
	}

	// Mutate state to update app configuration and version
	err = Mutate(func(s *State) error {
		if s.Apps == nil {
			s.Apps = make(map[string]*InstalledApp)
		}
		if candidate.AppSnapshot != nil {
			restored := CloneInstalledApp(candidate.AppSnapshot)
			restored.Version = targetVersion
			s.Apps[repo] = restored
		} else if app, ok := s.Apps[repo]; ok {
			app.Version = targetVersion
		}
		result.AppRestored = true
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update state during rollback: %w", err)
	}

	// Record rollback in history database
	_, _ = RecordUpdate("rollback", repo, result.FromVersion, targetVersion, currentApp, candidate.BackupFiles, true, "Rollback executed successfully")

	return result, nil
}
