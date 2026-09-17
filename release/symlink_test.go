package release

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joshsukhdeo/gh-pt/params"
	"github.com/joshsukhdeo/gh-pt/selector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)


func TestExecuteSymlinkInstall(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	targetPath := filepath.Join(homeDir, ".local", "bin")
	require.NoError(t, os.MkdirAll(targetPath, 0755))

	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	assetFile := filepath.Join(tmpDir, "jq-linux-amd64")
	require.NoError(t, os.WriteFile(assetFile, []byte("dummy binary"), 0755))

	r := &GithubRelease{
		CliParams: &params.ExecContext{
			Repository: "jqlang/jq",
			CommonInstallFlags: params.CommonInstallFlags{TargetPath: targetPath},
		},
	}

	binaries := []*selector.SelectorItem{
		{
			Name: "jq",
			DownloadPath: assetFile,
		},
	}

	symlinkDir, err := r.executeSymlinkInstall(binaries, assetFile)
	assert.NoError(t, err)

	// Now using owner/repo path: ~/src/apps/jqlang/jq
	expectedAppDir := filepath.Join(homeDir, "src", "apps", "jqlang", "jq")
	assert.Equal(t, expectedAppDir, symlinkDir)

	// Verify the file was copied to the app dir
	copiedFile := filepath.Join(expectedAppDir, "jq-linux-amd64")
	assert.FileExists(t, copiedFile)

	// Verify symlink was created in target path
	symlinkTarget := filepath.Join(targetPath, "jq")
	assert.FileExists(t, symlinkTarget)

	// Verify symlink points to the copied file
	linkInfo, err := os.Readlink(symlinkTarget)
	assert.NoError(t, err)
	assert.Equal(t, copiedFile, linkInfo)
}