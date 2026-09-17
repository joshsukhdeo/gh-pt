package release

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyHashWithVirusTotal_Unknown_SkipSandbox(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintln(w, `{"error":{"code": "NotFoundError"}}`)
	}))
	defer server.Close()
	vtBaseURL = server.URL

	err := VerifyHashWithVirusTotal("unknownhash", "dummy.txt", "test-api-key", false, true)
	assert.NoError(t, err)
}

func TestVerifyHashWithVirusTotal_Unknown_Upload(t *testing.T) {
	// Create a dummy file
	f, _ := os.CreateTemp("", "vt-test")
	_, _ = f.WriteString("dummy payload")
	_ = f.Close()
	defer func() { _ = os.Remove(f.Name()) }()

	vtPollDelay = 10 * time.Millisecond
	reqCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount++
		switch reqCount {
		case 1:
			// First request is to /files/hash -> 404
			w.WriteHeader(http.StatusNotFound)
		case 2:
			// Second request is POST to /files -> returns analysis ID
			assert.Equal(t, http.MethodPost, r.Method)
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintln(w, `{"data":{"id": "analysis-123"}}`)
		case 3:
			// Third request is GET to /analyses/analysis-123 -> return completed
			assert.Equal(t, http.MethodGet, r.Method)
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintln(w, `{"data":{"attributes":{"status": "completed", "stats":{"malicious": 1}}}}`)
		}
	}))
	defer server.Close()
	vtBaseURL = server.URL

	err := VerifyHashWithVirusTotal("unknownhash", f.Name(), "test-api-key", false, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "malicious")
}

func TestVerifyHashWithVirusTotal_Unknown_UploadLarge(t *testing.T) {
	vtPollDelay = 10 * time.Millisecond
	f, _ := os.CreateTemp("", "vt-test-large")
	// Make it larger than 32MB so it triggers the large upload logic
	_ = f.Truncate(33 * 1024 * 1024)
	_ = f.Close()
	defer func() { _ = os.Remove(f.Name()) }()

	reqCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount++
		switch reqCount {
		case 1:
			// First request is to /files/hash -> 404
			assert.Equal(t, http.MethodGet, r.Method)
			w.WriteHeader(http.StatusNotFound)
		case 2:
			// Second request should be to /files/upload_url
			assert.Equal(t, "/files/upload_url", r.URL.Path)
			w.WriteHeader(http.StatusOK)
			// Returning the mock server URL to simulate the dynamic upload URL
			_, _ = fmt.Fprintf(w, "{\"data\": \"http://%s/upload_target\"}\n", r.Host)
		case 3:
			// Third request is POST to /upload_target
			assert.Equal(t, "/upload_target", r.URL.Path)
			assert.Equal(t, http.MethodPost, r.Method)
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintln(w, `{"data":{"id": "analysis-large"}}`)
		case 4:
			// Fourth request is GET to /analyses/analysis-large
			assert.Equal(t, "/analyses/analysis-large", r.URL.Path)
			assert.Equal(t, http.MethodGet, r.Method)
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintln(w, `{"data":{"attributes":{"status": "completed", "stats":{"malicious": 0}}}}`)
		}
	}))
	defer server.Close()
	vtBaseURL = server.URL

	err := VerifyHashWithVirusTotal("unknownhash_large", f.Name(), "test-api-key", false, false)
	assert.NoError(t, err)
}

func TestCalculateSHA256(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("hello world"), 0644)
	assert.NoError(t, err)

	hash, err := CalculateSHA256(testFile)
	assert.NoError(t, err)
	assert.Equal(t, "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9", hash)

	_, err = CalculateSHA256(filepath.Join(tmpDir, "missing.txt"))
	assert.Error(t, err)
}

func TestVerifyHashWithVirusTotal_Malicious(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"attributes": map[string]interface{}{
					"last_analysis_stats": map[string]interface{}{
						"malicious": 2,
					},
				},
			},
		})
	}))
	defer server.Close()
	vtBaseURL = server.URL + "/"

	err := VerifyHashWithVirusTotal("hash", "file", "key", false, true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "blocked installation:")
}

func TestVerifyHashWithVirusTotal_Clean(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"attributes": map[string]interface{}{
					"last_analysis_stats": map[string]interface{}{
						"malicious": 0,
					},
				},
			},
		})
	}))
	defer server.Close()
	vtBaseURL = server.URL + "/"

	err := VerifyHashWithVirusTotal("hash", "file", "key", false, true)
	assert.NoError(t, err)
}

func TestDoVTRequestWithRetry_401(t *testing.T) {
	origSleep := vtSleep
	vtSleep = func(d time.Duration) {}
	defer func() { vtSleep = origSleep }()

	vtSleep = func(d time.Duration) {}

	vtSleep = func(d time.Duration) {}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	req, _ := http.NewRequest("GET", server.URL, nil)
	client := &http.Client{}
	resp, err := doVTRequestWithRetry(client, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestDoVTRequestWithRetry_Retry(t *testing.T) {
	origSleep := vtSleep
	vtSleep = func(d time.Duration) {}
	defer func() { vtSleep = origSleep }()

	vtSleep = func(d time.Duration) {}

	vtSleep = func(d time.Duration) {}
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req, _ := http.NewRequest("GET", server.URL, nil)
	client := &http.Client{}
	resp, err := doVTRequestWithRetry(client, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 2, attempts)
}

func TestVerifyHashWithVirusTotal_Errors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{})
	}))
	defer server.Close()
	vtBaseURL = server.URL

	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "file")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0644))

	err := VerifyHashWithVirusTotal("hash", file, "key", false, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "virustotal upload failed with status")

	server500 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server500.Close()
	vtBaseURL = server500.URL

	err = VerifyHashWithVirusTotal("hash", file, "key", false, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "virustotal api returned status")

	serverBadJSON := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{bad json"))
	}))
	defer serverBadJSON.Close()
	vtBaseURL = serverBadJSON.URL

	err = VerifyHashWithVirusTotal("hash", file, "key", false, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode")
}

func TestPollVirusTotalAnalysis(t *testing.T) {
	origSleep := vtSleep
	vtSleep = func(d time.Duration) {}
	defer func() { vtSleep = origSleep }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"attributes": map[string]interface{}{
					"status": "completed",
					"stats": map[string]interface{}{
						"malicious": 1,
					},
				},
			},
		})
	}))
	defer server.Close()
	vtBaseURL = server.URL

	err := pollVirusTotalAnalysis("id", "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "blocked installation")

	callCount := 0
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if callCount == 0 {
			callCount++
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"attributes": map[string]interface{}{
						"status": "queued",
					},
				},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"attributes": map[string]interface{}{
					"status": "completed",
					"stats": map[string]interface{}{
						"malicious": 0,
					},
				},
			},
		})
	}))
	defer server2.Close()
	vtBaseURL = server2.URL

	err = pollVirusTotalAnalysis("id", "key")
	assert.NoError(t, err)
}

func TestVerifyHashWithVirusTotal_Interactive(t *testing.T) {
	r, w, err := os.Pipe()
	assert.NoError(t, err)
	origStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	go func() {
		_, _ = w.Write([]byte("N\ny\n"))
		_ = w.Close()
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	vtBaseURL = server.URL

	err = VerifyHashWithVirusTotal("hash", "file", "key", true, false)
	assert.NoError(t, err)
}
