package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/adrg/xdg"
	"github.com/joshsukhdeo/gh-pt/params"
	"github.com/joshsukhdeo/gh-pt/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	for name, content := range files {
		data := []byte(content)
		hdr := &tar.Header{
			Name: name,
			Mode: 0755,
			Size: int64(len(data)),
		}
		require.NoError(t, tw.WriteHeader(hdr))
		_, err := tw.Write(data)
		require.NoError(t, err)
	}

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

func TestGithubRelease_HTTPInstall(t *testing.T) {
	tempData := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tempData)
	targetDir := filepath.Join(tempData, "bin")
	require.NoError(t, os.MkdirAll(targetDir, 0755))
	xdg.Reload()

	tarBytes := createTestTarGz(t, map[string]string{
		"mockapp": "#!/bin/sh\necho mockapp 1.2.3\n",
	})

	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html><html><body>
<a href="mockapp-linux-amd64.tar.gz">mockapp-linux-amd64.tar.gz</a>
<a href="version.txt">version.txt</a>
</body></html>`)
		case "/version.txt":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = fmt.Fprint(w, "v1.2.3\n")
		case "/mockapp-linux-amd64.tar.gz":
			w.Header().Set("Content-Type", "application/gzip")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(tarBytes)))
			_, _ = w.Write(tarBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	gr := &GithubRelease{
		CliParams: &params.ExecContext{
			HTTPURL:    ts.URL,
			Repository: "mockhost/mockapp",
			CommonInstallFlags: params.CommonInstallFlags{
				DisablePrompts: true,
				ReleaseVersion: "latest",
				Type:           []string{"binary"},
				TargetPath:     targetDir,
				Overwrite:      true,
				ProgressBar:    "none",
			},
		},
	}

	err := gr.Install()
	require.NoError(t, err)

	installedBinary := filepath.Join(targetDir, "mockapp")
	assert.FileExists(t, installedBinary)

	st, err := state.LoadState()
	require.NoError(t, err)
	app, ok := st.Apps["mockhost/mockapp"]
	require.True(t, ok)
	assert.Equal(t, ts.URL, app.HTTPURL)
	assert.Equal(t, "v1.2.3", app.Version)
	assert.Contains(t, app.InstalledBinaries, "mockapp")
}
