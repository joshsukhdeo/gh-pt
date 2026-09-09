package release

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"encoding/json"

	"github.com/joshsukhdeo/gh-install/params"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to mock exec.Command
func helperCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	os.Exit(0)
}

func mockGhExec(args ...string) (bytes.Buffer, bytes.Buffer, error) {
	return bytes.Buffer{}, bytes.Buffer{}, nil
}


type MockPrompter struct {
	MockConfirm bool
	MockInput   string
}

func (m MockPrompter) Confirm(message string) bool {
	return m.MockConfirm
}

func (m MockPrompter) Input(prompt string, defaultValue string) string {
	return m.MockInput
}
type MockGithubClient struct {
	GetResponses map[string]interface{}
	ReqResponses map[string]*http.Response
	GetError     error
	ReqError     error
}

func (m *MockGithubClient) Get(path string, response interface{}) error {
	if m.GetError != nil {
		return m.GetError
	}
	if val, ok := m.GetResponses[path]; ok {
		b, _ := json.Marshal(val)
		json.Unmarshal(b, response)
	}
	return nil
}
func (m *MockGithubClient) Request(method string, path string, body io.Reader) (*http.Response, error) {
	if m.ReqError != nil {
		return nil, m.ReqError
	}
	if resp, ok := m.ReqResponses[path]; ok {
		return resp, nil
	}
	return &http.Response{Body: io.NopCloser(bytes.NewReader([]byte(`[]`)))}, nil
}

func TestGetScore(t *testing.T) {
	types := []string{"tar.gz", "zip", "deb"}
	assert.Equal(t, 30, getScore("app.tar.gz", types))
	assert.Equal(t, 20, getScore("app.zip", types))
	assert.Equal(t, 10, getScore("app.deb", types))
	assert.Equal(t, -1, getScore("app.rpm", types))
}

func TestGenerateStrictAssetRegex(t *testing.T) {
    assert.Equal(t, "^app\\.tar\\.gz$", generateStrictAssetRegex("app.tar.gz", ""))
    assert.Contains(t, generateStrictAssetRegex("app-v1.0.0.tar.gz", "v1.0.0"), ".*")
}

func TestGithubRelease_ResolveDestinationPath(t *testing.T) {
	gr := &GithubRelease{
		CliParams: &params.CLI{
			TargetPath: "/tmp/bin",
			Rename: map[string]string{
				"test-linux-amd64": "test",
			},
			DisablePrompts: true,
		},
	}

	dest := gr.resolveDestinationPath("test-linux-amd64")
	assert.Equal(t, filepath.Join("/tmp/bin", "test"), dest)
}

func TestGithubRelease_MakeGithubRelease(t *testing.T) {
	p := &params.CLI{}
	client := &MockGithubClient{}
	r := MakeGithubRelease(p, client)
	assert.NotNil(t, r)
}

func TestGithubRelease_InstallBinary(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source")
	err := os.WriteFile(sourceFile, []byte("content"), 0644)
	require.NoError(t, err)

	gr := &GithubRelease{
		CliParams: &params.CLI{
			TargetPath:     tmpDir,
			Rename:         map[string]string{},
			DisablePrompts: true,
			Overwrite:      true,
		},
	}

	destFile := filepath.Join(tmpDir, "dest_binary")
	gr.CliParams.Rename = map[string]string{
		"source": "dest_binary",
	}

	err = gr.installBinary(sourceFile)
	require.NoError(t, err)

	content, err := os.ReadFile(destFile)
	require.NoError(t, err)
	assert.Equal(t, "content", string(content))
}

func TestGithubRelease_InstallDebRpm(t *testing.T) {
	origExecCommand := execCommand
	execCommand = helperCommand
	defer func() { execCommand = origExecCommand }()

	gr := &GithubRelease{
		CliParams: &params.CLI{
			NoDeps: true,
		},
	}

	err := gr.installDeb("/tmp/test.deb")
	require.NoError(t, err)

	err = gr.installRpm("/tmp/test.rpm")
	require.NoError(t, err)
}

func TestGithubRelease_InstallArchivedBinary(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source_archive")
	err := os.WriteFile(sourceFile, []byte("archived content"), 0644)
	require.NoError(t, err)

	fsys := os.DirFS(tmpDir)

	gr := &GithubRelease{
		CliParams: &params.CLI{
			TargetPath: tmpDir,
			Rename: map[string]string{
				"source_archive": "dest_archive",
			},
			DisablePrompts: true,
			Overwrite:      true,
		},
	}

	err = gr.installArchivedBinary(fsys, "source_archive")
	require.NoError(t, err)

	destFile := filepath.Join(tmpDir, "dest_archive")
	content, err := os.ReadFile(destFile)
	require.NoError(t, err)
	assert.Equal(t, "archived content", string(content))
}

func TestGithubRelease_InstallPkg(t *testing.T) {
	origExecCommand := execCommand
	execCommand = helperCommand
	defer func() { execCommand = origExecCommand }()

	gr := &GithubRelease{
		CliParams: &params.CLI{
			NoDeps: true,
		},
	}

	err := gr.installPkg("/tmp/test.pkg")
	require.NoError(t, err)
}

func TestGithubRelease_InstallPacman(t *testing.T) {
	origExecCommand := execCommand
	execCommand = helperCommand
	defer func() { execCommand = origExecCommand }()

	gr := &GithubRelease{
		CliParams: &params.CLI{
			NoDeps: true,
		},
	}

	err := gr.installPacman("/tmp/test.pkg.tar.zst")
	require.NoError(t, err)
}

func TestGithubRelease_EnsureSudo(t *testing.T) {
	origExecCommand := execCommand
	execCommand = helperCommand
	defer func() { execCommand = origExecCommand }()

	gr := &GithubRelease{
		CliParams: &params.CLI{
			DisablePrompts: true,
		},
	}
	err := gr.ensureSudo()
	assert.NoError(t, err)
}

func TestGithubRelease_Install(t *testing.T) {
	origExecCommand := execCommand
	execCommand = helperCommand
	origGhExec := ghExec
	ghExec = mockGhExec
	defer func() {
		execCommand = origExecCommand
		ghExec = origGhExec
	}()

	client := &MockGithubClient{
		GetResponses: map[string]interface{}{
			"repos/owner/repo/releases": []map[string]interface{}{
				{"Tag_name": "v1.0.0", "Id": 1},
			},
		},
	}

	gr := &GithubRelease{
		CliParams: &params.CLI{
			DisablePrompts: true,
			Repository:     "owner/repo",
			ReleaseVersion: "latest",
			Type:           []string{"binary"},
		},
		Client: client,
	}

	err := gr.Install()
	assert.Error(t, err)
}

func TestGithubRelease_GetTopgradeConfigPath(t *testing.T) {
	// Not testing SetupTopgrade anymore because it was deleted in main branch!
}

func TestGithubRelease_CompileFromSource(t *testing.T) {
	origExecCommand := execCommand
	execCommand = helperCommand
	defer func() { execCommand = origExecCommand }()

	gr := &GithubRelease{
		CliParams: &params.CLI{
			CompileFromSource: true,
			AI:                true,
			DisablePrompts:    true,
            AICmd:             "true", // mock success via shell
		},
	}
    // As it uses `os.UserHomeDir()`, let's just make sure it fails safely or executes gracefully without panics
    // in our controlled stub. The actual method is `r.handleCompileFromSource` which is in `cmd/root.go`.
    // Wait, the method is in `cmd/root.go`, not `release.go`!
    // We shouldn't test `handleCompileFromSource` in `release_test.go`.
    _ = gr
}

func TestSuccessMessage(t *testing.T) {
	// The codebase prints "Successfully installed FreeBSD package!" and similar messages for pkg, pacman, and zip
	// This hits those code paths if dry run is false, or interactive mode is enabled.
	origExecCommand := execCommand
	execCommand = helperCommand
	defer func() { execCommand = origExecCommand }()

	gr := &GithubRelease{
		CliParams: &params.CLI{
			NoDeps: true,
			Interactive: true,
			DisablePrompts: false,
		},
        Prompter: MockPrompter{MockConfirm: true, MockInput: "ok"},
	}

	// Test installation which prints pterm.Success.Println internally
	err := gr.installPkg("/tmp/test.pkg")
	require.NoError(t, err)

	err = gr.installPacman("/tmp/test.pkg.tar.zst")
	require.NoError(t, err)
}
