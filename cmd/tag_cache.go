package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/adrg/xdg"
	"github.com/charmbracelet/log"
)

// TagCacheEntry represents a cached latest release tag.
type TagCacheEntry struct {
	Tag       string    `json:"tag"`
	FetchedAt time.Time `json:"fetched_at"`
}

// DefaultTagCacheTTL is the duration for which cached release tags remain valid.
var DefaultTagCacheTTL = 10 * time.Minute

// ReleaseTagCache manages an in-memory and disk-backed TTL cache for GitHub release tags.
type ReleaseTagCache struct {
	mu        sync.RWMutex
	entries   map[string]TagCacheEntry
	ttl       time.Duration
	enabled   bool
	cachePath string
	loaded    bool
}

// GetDefaultTagCachePath returns the file path for persistent tag cache.
func GetDefaultTagCachePath() string {
	if env := os.Getenv("XDG_CACHE_HOME"); env != "" {
		return filepath.Join(env, "gh-pt", "tags_cache.json")
	}
	// In test environments, isolate cache within test directory:
	if dataEnv := os.Getenv("XDG_DATA_HOME"); dataEnv != "" && (strings.Contains(dataEnv, "tmp") || strings.Contains(dataEnv, "Test")) {
		return filepath.Join(dataEnv, "gh-pt", "tags_cache.json")
	}
	if xdg.DataHome != "" && (strings.Contains(xdg.DataHome, "tmp") || strings.Contains(xdg.DataHome, "Test")) {
		return filepath.Join(xdg.DataHome, "gh-pt", "tags_cache.json")
	}
	if xdg.CacheHome != "" {
		return filepath.Join(xdg.CacheHome, "gh-pt", "tags_cache.json")
	}
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		return filepath.Join(home, ".cache", "gh-pt", "tags_cache.json")
	}
	return filepath.Join(os.TempDir(), "gh-pt-cache", "tags_cache.json")
}

// NewReleaseTagCache creates a new ReleaseTagCache instance.
func NewReleaseTagCache() *ReleaseTagCache {
	return &ReleaseTagCache{
		entries:   make(map[string]TagCacheEntry),
		ttl:       DefaultTagCacheTTL,
		enabled:   true,
		cachePath: GetDefaultTagCachePath(),
	}
}

var globalTagCache = NewReleaseTagCache()

// ensureLoaded loads persisted cache entries from disk if not yet loaded.
func (c *ReleaseTagCache) ensureLoaded() {
	currentPath := GetDefaultTagCachePath()
	if c.cachePath != currentPath {
		c.cachePath = currentPath
		c.entries = make(map[string]TagCacheEntry)
		c.loaded = false
	}
	if c.loaded {
		return
	}
	c.loaded = true
	if c.cachePath == "" {
		return
	}
	data, err := os.ReadFile(c.cachePath)
	if err != nil {
		return
	}
	var diskEntries map[string]TagCacheEntry
	if err := json.Unmarshal(data, &diskEntries); err != nil {
		log.Debug("Failed to unmarshal release tag cache", "error", err)
		return
	}
	now := time.Now()
	for repo, entry := range diskEntries {
		if now.Sub(entry.FetchedAt) < c.ttl {
			c.entries[repo] = entry
		}
	}
}

// saveDisk writes valid entries to disk atomically.
func (c *ReleaseTagCache) saveDisk() {
	if !c.enabled || c.cachePath == "" {
		return
	}
	dir := filepath.Dir(c.cachePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}
	data, err := json.Marshal(c.entries)
	if err != nil {
		return
	}
	tmpFile := c.cachePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return
	}
	_ = os.Rename(tmpFile, c.cachePath)
}

// Get returns the cached tag if present and unexpired.
func (c *ReleaseTagCache) Get(repo string) (string, bool) {
	c.mu.Lock()
	if !c.enabled {
		c.mu.Unlock()
		return "", false
	}
	c.ensureLoaded()
	entry, ok := c.entries[repo]
	c.mu.Unlock()

	if !ok || entry.Tag == "" {
		return "", false
	}
	if time.Since(entry.FetchedAt) >= c.ttl {
		return "", false
	}
	return entry.Tag, true
}

// Set stores a release tag in the cache.
func (c *ReleaseTagCache) Set(repo string, tag string) {
	if repo == "" || tag == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.enabled {
		return
	}
	c.ensureLoaded()
	c.entries[repo] = TagCacheEntry{
		Tag:       tag,
		FetchedAt: time.Now(),
	}
	c.saveDisk()
}

// SetBatch stores multiple release tags into the cache in one transaction.
func (c *ReleaseTagCache) SetBatch(tags map[string]string) {
	if len(tags) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.enabled {
		return
	}
	c.ensureLoaded()
	now := time.Now()
	for repo, tag := range tags {
		if repo != "" && tag != "" {
			c.entries[repo] = TagCacheEntry{
				Tag:       tag,
				FetchedAt: now,
			}
		}
	}
	c.saveDisk()
}

// Clear flushes all in-memory entries and deletes the cache file.
func (c *ReleaseTagCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]TagCacheEntry)
	c.loaded = true
	if c.cachePath != "" {
		_ = os.Remove(c.cachePath)
	}
}

// Invalidate removes a specific repository entry from the cache.
func (c *ReleaseTagCache) Invalidate(repo string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureLoaded()
	delete(c.entries, repo)
	c.saveDisk()
}

// SetTTL adjusts the cache entry TTL.
func (c *ReleaseTagCache) SetTTL(ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ttl = ttl
}

// SetEnabled enables or disables the cache.
func (c *ReleaseTagCache) SetEnabled(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = enabled
}

// Global convenience functions operating on the global cache instance:

func GetCachedReleaseTag(repo string) (string, bool) {
	return globalTagCache.Get(repo)
}

func SetCachedReleaseTag(repo, tag string) {
	globalTagCache.Set(repo, tag)
}

func SetCachedReleaseTags(tags map[string]string) {
	globalTagCache.SetBatch(tags)
}

func ClearReleaseTagCache() {
	globalTagCache.Clear()
}

func InvalidateReleaseTag(repo string) {
	globalTagCache.Invalidate(repo)
}

func SetReleaseTagCacheTTL(ttl time.Duration) {
	globalTagCache.SetTTL(ttl)
}

func SetReleaseTagCacheEnabled(enabled bool) {
	globalTagCache.SetEnabled(enabled)
}
