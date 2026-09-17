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

func TestComputeSymlinkDirName(t *testing.T) {
	tests := []struct {
		repository string
		binaryName string
		expected   string
	}{
		// 1. appname matched repoid -> repoid
		{"jqlang/jq", "jq", "jq"},
		{"jqlang/jq", "jq-linux-amd64", "jq"},
		
		// 2. appname matched ownerid -> ownerid-repoid
		{"nushell/nushell", "nu", "nu"}, // Wait, if repo is nushell/nushell and binary is nu, how does it match ownerid?
		// "if appname matched ownerid"
		{"hashicorp/terraform", "hashicorp", "hashicorp-terraform"},
		
		// 3. longest common name with version tag/numbers/id removed followed by right trim of spaces . and dashes
		{"cli/cli", "gh-1.0.0-linux-amd64", "gh"},
		{"neovim/neovim", "nvim-linux64", "nvim"},
		{"burntsushi/ripgrep", "rg-13.0.0", "rg"},
	}

	for _, tc := range tests {
		t.Run(tc.repository+"_"+tc.binaryName, func(t *testing.T) {
			actual := computeSymlinkDirName(tc.repository, tc.binaryName)
			assert.Equal(t, tc.expected, actual)
		})
	}
}



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

	expectedAppDir := filepath.Join(homeDir, "src", "apps", "jq")
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
