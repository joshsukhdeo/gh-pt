package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adrg/xdg"
	"github.com/joshsukhdeo/gh-pt/params"
	"github.com/joshsukhdeo/gh-pt/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockGhClient struct {
	releases []Release
	assets   map[int64][]ReleaseAsset
	tags     map[string]Release
	err      error
}

func (m *mockGhClient) Get(path string, response interface{}) error {
	if m.err != nil {
		return m.err
	}

	if strings.HasSuffix(path, "/releases") {
		data, err := json.Marshal(m.releases)
		if err != nil {
			return err
		}
		return json.Unmarshal(data, response)
	}

	if strings.Contains(path, "/assets") {
		var id int64
		_, err := fmt.Sscanf(path[strings.Index(path, "/releases/")+len("/releases/"):strings.Index(path, "/assets")], "%d", &id)
		if err == nil {
			if a, ok := m.assets[id]; ok {
				data, err := json.Marshal(a)
				if err != nil {
					return err
				}
				return json.Unmarshal(data, response)
			}
		}
		return nil
	}

	if strings.Contains(path, "/releases/tags/") {
		tag := path[strings.LastIndex(path, "/")+1:]
		if rel, ok := m.tags[tag]; ok {
			data, err := json.Marshal(rel)
			if err != nil {
				return err
			}
			return json.Unmarshal(data, response)
		}
		return fmt.Errorf("tag not found")
	}

	return nil
}

func captureOutput(f func()) string {
	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	_ = w.Close()
	os.Stdout = orig

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestShowInfo_ShowVersions(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	xdg.ConfigHome = tmpDir
	defer func() { xdg.ConfigHome = "" }()
	xdg.ConfigHome = tmpDir
		defer func() { xdg.ConfigHome = "" }()
	// xdg.Reload()

	st, err := state.LoadState()
	require.NoError(t, err)
	require.NoError(t, st.AddApp(&state.InstalledApp{
		Repository: "test/repo",
		Version:    "v1.1.0",
		Pinned:     true,
	}))

	mock := &mockGhClient{
		releases: []Release{
			{ID: 103, TagName: "v1.2.0-rc1", Prerelease: true},
			{ID: 102, TagName: "v1.1.0", Prerelease: false},
			{ID: 101, TagName: "v1.0.0", Prerelease: false},
		},
	}

	r := &RootCLI{
		ExecContext: params.ExecContext{
			Repository:   "test/repo",
			ShowVersions: 10,
		},
	}

	out := captureOutput(func() {
		err := showInfoWithClient(r, mock)
		assert.NoError(t, err)
	})

	// v1.2.0-rc1 is latest prerelease (🗻🧪)
	// v1.1.0 is latest stable, installed, and pinned (📌🗻🎯)
	// v1.0.0 is older uninstalled stable ("")
	assert.Contains(t, out, "🗻🧪 v1.2.0-rc1")
	assert.Contains(t, out, "📌🗻🎯 v1.1.0")
	assert.Contains(t, out, "v1.0.0")
}

func TestShowInfo_ShowVersions_DisableIcons(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	xdg.ConfigHome = tmpDir
	defer func() { xdg.ConfigHome = "" }()
	xdg.ConfigHome = tmpDir
		defer func() { xdg.ConfigHome = "" }()
	// xdg.Reload()

	cfgPath := filepath.Join(tmpDir, "gh-pt", "config.yml")
	_ = os.MkdirAll(filepath.Dir(cfgPath), 0755)
	_ = os.WriteFile(cfgPath, []byte("disable_icons: true\n"), 0644)

	st, err := state.LoadState()
	require.NoError(t, err)
	require.NoError(t, st.AddApp(&state.InstalledApp{
		Repository: "test/repo",
		Version:    "v1.1.0",
		Pinned:     true,
	}))

	mock := &mockGhClient{
		releases: []Release{
			{ID: 103, TagName: "v1.2.0-rc1", Prerelease: true},
			{ID: 102, TagName: "v1.1.0", Prerelease: false},
		},
	}

	r := &RootCLI{
		ExecContext: params.ExecContext{
			Repository:   "test/repo",
			ShowVersions: 10,
		},
	}

	out := captureOutput(func() {
		err := showInfoWithClient(r, mock)
		assert.NoError(t, err)
	})

	// v1.2.0-rc1 with text fallback (*!)
	// v1.1.0 with text fallback (^*@)
	assert.Contains(t, out, "*! v1.2.0-rc1")
	assert.Contains(t, out, "^*@ v1.1.0")
	assert.Contains(t, out, "--- VERSIONS ---")
}

func TestShowInfo_Show(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	xdg.ConfigHome = tmpDir
	defer func() { xdg.ConfigHome = "" }()
	xdg.ConfigHome = tmpDir
		defer func() { xdg.ConfigHome = "" }()
	// xdg.Reload()

	var releases []Release
	for i := 15; i >= 1; i-- {
		releases = append(releases, Release{
			ID:         int64(100 + i),
			TagName:    fmt.Sprintf("v1.%d.0", i),
			Prerelease: i == 15,
		})
	}

	mock := &mockGhClient{
		releases: releases,
		assets: map[int64][]ReleaseAsset{
			114: { // v1.14.0 is latest stable
				{ID: 1, Name: "app-linux-amd64.tar.gz"},
				{ID: 2, Name: "app-darwin-arm64.tar.gz"},
			},
			115: { // v1.15.0 is latest prerelease
				{ID: 3, Name: "app-prerelease.tar.gz"},
			},
		},
	}

	// Default (stable): should pick v1.14.0 and display max 10 versions, then v1.14.0 assets
	r := &RootCLI{
		ExecContext: params.ExecContext{
			Repository: "test/repo",
			Show: true, ShowVersions: -1, ShowAssets: -1, ShowDescription: -1, ShowReadme: -1,
			CommonInstallFlags: params.CommonInstallFlags{
				ReleaseVersion: "latest",
			},
		},
	}

	out := captureOutput(func() {
		err := showInfoWithClient(r, mock)
		assert.NoError(t, err)
	})

	
	// 10 versions + 2 assets = 12 lines
	
	assert.Contains(t, out, "v1.15.0")
	assert.Contains(t, out, "v1.6.0")
	assert.NotContains(t, out, "v1.5.0") // truncated after 10 versions
	assert.Contains(t, out, "app-linux-amd64.tar.gz")
	assert.Contains(t, out, "app-darwin-arm64.tar.gz")

	// With Prerelease: should pick v1.15.0
	rPrerelease := &RootCLI{
		ExecContext: params.ExecContext{
			Repository: "test/repo",
			Show: true, ShowVersions: -1, ShowAssets: -1, ShowDescription: -1, ShowReadme: -1,
			CommonInstallFlags: params.CommonInstallFlags{
				Prerelease:     true,
				ReleaseVersion: "latest",
			},
		},
	}
	outPre := captureOutput(func() {
		err := showInfoWithClient(rPrerelease, mock)
		assert.NoError(t, err)
	})
	assert.Contains(t, outPre, "app-prerelease.tar.gz")
}

func TestShowInfo_ShowAssets(t *testing.T) {
	mock := &mockGhClient{
		releases: []Release{
			{ID: 102, TagName: "v2.0.0", Prerelease: false},
			{ID: 101, TagName: "v1.0.0", Prerelease: false},
		},
		assets: map[int64][]ReleaseAsset{
			102: {
				{ID: 1, Name: "asset-2.0-a"},
				{ID: 2, Name: "asset-2.0-b"},
			},
			101: {
				{ID: 3, Name: "asset-1.0-a"},
			},
		},
	}

	// Target specific version v1.0.0
	r := &RootCLI{
		ExecContext: params.ExecContext{
			Repository: "test/repo",
			ShowAssets: 50,
			CommonInstallFlags: params.CommonInstallFlags{
				ReleaseVersion: "v1.0.0",
			},
		},
	}

	out := captureOutput(func() {
		err := showInfoWithClient(r, mock)
		assert.NoError(t, err)
	})

	assert.Contains(t, out, "asset-1.0-a")
	assert.NotContains(t, out, "asset-2.0-a")
}

func TestShowInfo_Errors(t *testing.T) {
	// Empty repository
	r := &RootCLI{
		ExecContext: params.ExecContext{
			Show: true, ShowVersions: -1, ShowAssets: -1, ShowDescription: -1, ShowReadme: -1,
		},
	}
	err := showInfoWithClient(r, &mockGhClient{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "repository must be provided")

	// Releases fetch failure
	mockErr := &mockGhClient{err: fmt.Errorf("api network error")}
	r.ExecContext.Repository = "test/repo"
	err = showInfoWithClient(r, mockErr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch releases")

	// Target version not found
	mock := &mockGhClient{
		releases: []Release{
			{ID: 1, TagName: "v1.0.0"},
		},
	}
	r.ExecContext.ReleaseVersion = "v9.9.9"
	err = showInfoWithClient(r, mock)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no release found")
}

func TestShowInfo_RoutingInRun(t *testing.T) {
	origClient := defaultRestClient
	defer func() { defaultRestClient = origClient }()

	mock := &mockGhClient{
		releases: []Release{
			{ID: 1, TagName: "v1.0.0"},
		},
		assets: map[int64][]ReleaseAsset{
			1: {{ID: 1, Name: "my-asset.deb"}},
		},
	}
	defaultRestClient = func() (ghRestClient, error) {
		return mock, nil
	}

	cli := &params.CLI{
		Show: params.ShowCmd{
			Repository: "test/repo",
			Assets:     50,
		},
	}

	// Ensure RunCommand routes to ShowInfo and returns without error
	out := captureOutput(func() {
		err := RunCommand("show", cli)
		assert.NoError(t, err)
	})
	assert.Contains(t, out, "my-asset.deb")
}

func TestShowInfo_HeadersWithIcons(t *testing.T) {
	origClient := defaultRestClient
	defer func() { defaultRestClient = origClient }()

	mock := &mockGhClient{
		releases: []Release{{ID: 1, TagName: "v1.0.0"}},
		assets: map[int64][]ReleaseAsset{1: {{ID: 1, Name: "asset.deb"}}},
	}
	defaultRestClient = func() (ghRestClient, error) {
		return mock, nil
	}

	r := &RootCLI{
		ExecContext: params.ExecContext{
			Repository: "test/repo",
			Show:       true, ShowVersions: 1, ShowAssets: 1, ShowDescription: -1, ShowReadme: -1,
		},
	}

	out := captureOutput(func() {
		err := showInfoWithClient(r, mock)
		assert.NoError(t, err)
	})

	assert.Contains(t, out, "--- 📦 VERSIONS 📦 ---")
	assert.Contains(t, out, "--- 📂 ASSETS 📂 ---")

	// Now with DisableIcons = true
	// We need to write a config file to set disableIcons, or we can just mock loadConfig?
	// loadConfig() reads from XDG_CONFIG_HOME
	tmpDir := t.TempDir()
	xdg.ConfigHome = tmpDir
		defer func() { xdg.ConfigHome = "" }()
	cfgPath := filepath.Join(tmpDir, "gh-pt", "config.yml")
	os.MkdirAll(filepath.Dir(cfgPath), 0755)
	os.WriteFile(cfgPath, []byte("disable_icons: true\n"), 0644)

	outDisabled := captureOutput(func() {
		err := showInfoWithClient(r, mock)
		assert.NoError(t, err)
	})

	assert.NotContains(t, outDisabled, "📦")
	assert.NotContains(t, outDisabled, "📂")
	assert.Contains(t, outDisabled, "--- VERSIONS ---")
	assert.Contains(t, outDisabled, "--- ASSETS ---")
}
