package cmd

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReleaseTagCache_Operations(t *testing.T) {
	cache := NewReleaseTagCache()
	cache.cachePath = filepath.Join(t.TempDir(), "tags_cache.json")

	// Missing repo
	tag, ok := cache.Get("owner/missing")
	assert.False(t, ok)
	assert.Empty(t, tag)

	// Set and Get
	cache.Set("owner/repo", "v1.0.0")
	tag, ok = cache.Get("owner/repo")
	assert.True(t, ok)
	assert.Equal(t, "v1.0.0", tag)

	// Batch Set
	cache.SetBatch(map[string]string{
		"owner/a": "v2.0.0",
		"owner/b": "v3.1.4",
	})
	tagA, okA := cache.Get("owner/a")
	tagB, okB := cache.Get("owner/b")
	assert.True(t, okA)
	assert.Equal(t, "v2.0.0", tagA)
	assert.True(t, okB)
	assert.Equal(t, "v3.1.4", tagB)

	// Invalidate single repo
	cache.Invalidate("owner/a")
	_, okA = cache.Get("owner/a")
	assert.False(t, okA)
	tagB, okB = cache.Get("owner/b")
	assert.True(t, okB)
	assert.Equal(t, "v3.1.4", tagB)

	// TTL expiration
	cache.SetTTL(50 * time.Millisecond)
	cache.Set("owner/quick", "v0.0.1")
	tagQuick, okQuick := cache.Get("owner/quick")
	assert.True(t, okQuick)
	assert.Equal(t, "v0.0.1", tagQuick)
	time.Sleep(60 * time.Millisecond)
	_, okQuick = cache.Get("owner/quick")
	assert.False(t, okQuick, "Expired entry must not be returned")

	// Disable cache
	cache.SetTTL(time.Hour)
	cache.Set("owner/disabled-test", "v1.2.3")
	cache.SetEnabled(false)
	_, ok = cache.Get("owner/disabled-test")
	assert.False(t, ok, "Get on disabled cache must return false")
	cache.SetEnabled(true)
	tag, ok = cache.Get("owner/disabled-test")
	assert.True(t, ok)
	assert.Equal(t, "v1.2.3", tag)

	// Clear cache
	cache.Clear()
	_, ok = cache.Get("owner/repo")
	assert.False(t, ok)
	_, ok = cache.Get("owner/b")
	assert.False(t, ok)
}

func TestReleaseTagCache_DiskPersistence(t *testing.T) {
	cacheFile := filepath.Join(t.TempDir(), "tags_cache.json")

	cache1 := NewReleaseTagCache()
	cache1.cachePath = cacheFile
	cache1.SetBatch(map[string]string{
		"cli/cli":      "v2.40.0",
		"junegunn/fzf": "v0.52.0",
	})

	// Second cache instance pointing to same file
	cache2 := NewReleaseTagCache()
	cache2.cachePath = cacheFile

	tag, ok := cache2.Get("cli/cli")
	require.True(t, ok)
	assert.Equal(t, "v2.40.0", tag)

	tag, ok = cache2.Get("junegunn/fzf")
	require.True(t, ok)
	assert.Equal(t, "v0.52.0", tag)
}

func TestGlobalTagCache_Helpers(t *testing.T) {
	ClearReleaseTagCache()
	defer ClearReleaseTagCache()

	SetCachedReleaseTag("test/repo", "v1.5.0")
	tag, ok := GetCachedReleaseTag("test/repo")
	assert.True(t, ok)
	assert.Equal(t, "v1.5.0", tag)

	InvalidateReleaseTag("test/repo")
	_, ok = GetCachedReleaseTag("test/repo")
	assert.False(t, ok)
}
