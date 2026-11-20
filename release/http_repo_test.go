package release

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTP_CleanHTTPSlug(t *testing.T) {
	tests := []struct {
		name        string
		rawURL      string
		wantErr     bool
		expected    string
		errContains string
	}{
		{
			name:        "invalid scheme ftp",
			rawURL:      "ftp://example.com/foo/bar",
			wantErr:     true,
			errContains: "only http and https are supported",
		},
		{
			name:        "invalid scheme file",
			rawURL:      "file:///path/to/archive.tar.gz",
			wantErr:     true,
			errContains: "only http and https are supported",
		},
		{
			name:        "empty host",
			rawURL:      "http:///foo/bar",
			wantErr:     true,
			errContains: "empty host",
		},
		{
			name:        "relative url",
			rawURL:      "/foo/bar",
			wantErr:     true,
			errContains: "only http and https are supported",
		},
		{
			name:     "clean openvino weekly url",
			rawURL:   "https://storage.openvinotoolkit.org/repositories/openvino_model_server/packages/weekly/latest/",
			wantErr:  false,
			expected: "storage.openvinotoolkit.org/openvino_model_server-weekly-latest",
		},
		{
			name:     "generic download url",
			rawURL:   "https://example.com/downloads/v1.0.0/",
			wantErr:  false,
			expected: "example.com/downloads-v1.0.0",
		},
		{
			name:     "url with port and single segment",
			rawURL:   "http://localhost:8080/myrepo",
			wantErr:  false,
			expected: "localhost-8080/myrepo",
		},
		{
			name:     "url with root path only",
			rawURL:   "https://mirror.example.org/",
			wantErr:  false,
			expected: "mirror.example.org/root",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slug, err := CleanHTTPSlug(tt.rawURL)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, slug)

				// Verify it splits cleanly into ownerID and repoID without nested slashes
				parts := strings.Split(slug, "/")
				assert.Len(t, parts, 2, "slug must have exactly two segments separated by a single slash")
				assert.NotEmpty(t, parts[0])
				assert.NotEmpty(t, parts[1])
				assert.False(t, strings.HasSuffix(slug, "/"), "slug must not have trailing slash")
			}
		})
	}
}

func TestHTTP_OpenVINO_SPA(t *testing.T) {
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repositories/openvino_model_server/packages/weekly/latest/":
			w.Header().Set("Content-Type", "text/html")
			html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><title>storage.openvinotoolkit.org</title></head>
<body>
<script type="text/javascript">
  var BUCKET_URL = '%s';
  var FILETREE_URL = '%s/filetree.json';
</script>
</body>
</html>`, ts.URL, ts.URL)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(html))

		case "/filetree.json":
			w.Header().Set("Content-Type", "application/json")
			jsonTree := `{
  "name": "root",
  "type": "directory",
  "children": [
    {
      "name": "repositories",
      "type": "directory",
      "children": [
        {
          "name": "openvino_model_server",
          "type": "directory",
          "children": [
            {
              "name": "packages",
              "type": "directory",
              "children": [
                {
                  "name": "weekly",
                  "type": "directory",
                  "children": [
                    {
                      "name": "latest",
                      "type": "directory",
                      "children": [
                        {
                          "name": "ovms_ubuntu26.tar.gz",
                          "type": "file",
                          "size": 199122488
                        },
                        {
                          "name": "ovms_ubuntu26.tar.gz.sha256",
                          "type": "file",
                          "size": 87
                        },
                        {
                          "name": "ovms_ubuntu24.tar.gz",
                          "type": "file",
                          "size": 189024090
                        },
                        {
                          "name": "version.txt",
                          "type": "file",
                          "size": 40
                        }
                      ]
                    }
                  ]
                }
              ]
            }
          ]
        }
      ]
    }
  ]
}`
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(jsonTree))

		case "/repositories/openvino_model_server/packages/weekly/latest/version.txt":
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OpenVINO Model Server 2026.5.0.dbe11fa27\nOpenVINO backend 2026.5.0\n"))

		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	targetURL := ts.URL + "/repositories/openvino_model_server/packages/weekly/latest/"
	index, err := FetchHTTPReleaseIndex(targetURL)
	require.NoError(t, err)
	require.NotNil(t, index)

	assert.Equal(t, "2026.5.0.dbe11fa27", index.VersionTag)
	assert.NotEmpty(t, index.CleanSlug)
	assert.Len(t, index.Assets, 4)

	var foundArchive, foundSha bool
	for _, a := range index.Assets {
		if a.Name == "ovms_ubuntu26.tar.gz" {
			foundArchive = true
			assert.Equal(t, int64(199122488), a.Size)
			assert.Equal(t, ts.URL+"/repositories/openvino_model_server/packages/weekly/latest/ovms_ubuntu26.tar.gz", a.DownloadURL)
		}
		if a.Name == "ovms_ubuntu26.tar.gz.sha256" {
			foundSha = true
			assert.Equal(t, int64(87), a.Size)
		}
	}
	assert.True(t, foundArchive, "ovms_ubuntu26.tar.gz should be in assets")
	assert.True(t, foundSha, "ovms_ubuntu26.tar.gz.sha256 should be in assets")

	rel := index.ToRelease()
	require.NotNil(t, rel)
	assert.Equal(t, "2026.5.0.dbe11fa27", rel.TagName)
	assert.Equal(t, "2026.5.0.dbe11fa27", rel.Name)
	assert.Len(t, rel.Assets, 4)
}

func TestHTTP_S3_ListBucketResult(t *testing.T) {
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases/v2.1.0/":
			w.Header().Set("Content-Type", "application/xml")
			xml := `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
    <Name>my-bucket</Name>
    <Prefix>releases/v2.1.0/</Prefix>
    <Contents>
        <Key>releases/v2.1.0/app-linux-amd64.tar.gz</Key>
        <Size>12345678</Size>
        <LastModified>2026-11-19T12:00:00.000Z</LastModified>
    </Contents>
    <Contents>
        <Key>releases/v2.1.0/app-linux-amd64.tar.gz.sha256</Key>
        <Size>65</Size>
        <LastModified>2026-11-19T12:00:00.000Z</LastModified>
    </Contents>
    <Contents>
        <Key>releases/v2.1.0/version.txt</Key>
        <Size>6</Size>
        <LastModified>2026-11-19T12:00:00.000Z</LastModified>
    </Contents>
</ListBucketResult>`
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(xml))

		case "/releases/v2.1.0/version.txt":
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("v2.1.0\n"))

		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	targetURL := ts.URL + "/releases/v2.1.0/"
	index, err := FetchHTTPReleaseIndex(targetURL)
	require.NoError(t, err)
	require.NotNil(t, index)

	assert.Equal(t, "v2.1.0", index.VersionTag)
	assert.Len(t, index.Assets, 3)

	var foundBinary bool
	for _, a := range index.Assets {
		if a.Name == "app-linux-amd64.tar.gz" {
			foundBinary = true
			assert.Equal(t, int64(12345678), a.Size)
			assert.Equal(t, ts.URL+"/releases/v2.1.0/app-linux-amd64.tar.gz", a.DownloadURL)
		}
	}
	assert.True(t, foundBinary)
}

func TestHTTP_HTML_Autoindex(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		html := `<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 3.2 Final//EN">
<html>
 <head><title>Index of /dist/v1.5.0</title></head>
 <body>
<h1>Index of /dist/v1.5.0</h1>
<table>
  <tr><th><a href="?C=N;O=D">Name</a></th><th><a href="?C=M;O=A">Last modified</a></th></tr>
  <tr><td><a href="../">Parent Directory</a></td><td>&nbsp;</td></tr>
  <tr><td><a href="tool-v1.5.0-x86_64.tar.gz">tool-v1.5.0-x86_64.tar.gz</a></td><td>2026-11-10</td></tr>
  <tr><td><a href="tool-v1.5.0-aarch64.tar.gz">tool-v1.5.0-aarch64.tar.gz</a></td><td>2026-11-10</td></tr>
  <tr><td><a href="subfolder/">subfolder/</a></td><td>2026-11-10</td></tr>
</table>
</body></html>`
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	defer ts.Close()

	targetURL := ts.URL + "/dist/v1.5.0/"
	index, err := FetchHTTPReleaseIndex(targetURL)
	require.NoError(t, err)
	require.NotNil(t, index)

	assert.Equal(t, "v1.5.0", index.VersionTag)
	assert.Len(t, index.Assets, 2)

	assetNames := []string{index.Assets[0].Name, index.Assets[1].Name}
	assert.Contains(t, assetNames, "tool-v1.5.0-x86_64.tar.gz")
	assert.Contains(t, assetNames, "tool-v1.5.0-aarch64.tar.gz")
	assert.NotContains(t, assetNames, "Parent Directory")
	assert.NotContains(t, assetNames, "../")
	assert.NotContains(t, assetNames, "subfolder/")
}
