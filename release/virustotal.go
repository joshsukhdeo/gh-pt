package release

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/joshsukhdeo/gh-pt/config"
	"github.com/joshsukhdeo/gh-pt/ui"
	"github.com/pterm/pterm"
)

var vtBaseURL = "https://www.virustotal.com/api/v3"
var vtPollDelay = 15 * time.Second
var vtSleep = time.Sleep

func doVTRequestWithRetry(client *http.Client, req *http.Request) (*http.Response, error) {
	for {
		resp, err := client.Do(req)
		if err != nil {
			return resp, err
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			_ = resp.Body.Close()
			log.Warn("VirusTotal rate limit (429) reached. Throttling for 15s...")
			time.Sleep(15 * time.Second)
			continue
		}
		return resp, nil
	}
}

func VerifyHashWithVirusTotal(hash string, filePath string, apiKey string, interactive bool, skipSandbox bool, pUI *ui.PacmanUI, vtSandboxMessage string) error {
RetryVT:
	if apiKey == "" {
		return nil
	}

	url := fmt.Sprintf("%s/files/%s", vtBaseURL, hash)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create virustotal request: %w", err)
	}

	req.Header.Set("x-apikey", apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := doVTRequestWithRetry(client, req)
	if err != nil {
		return fmt.Errorf("virustotal api request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized {
		if !interactive {
			return fmt.Errorf("invalid virustotal api key")
		}
		pterm.Warning.Println("Invalid VirusTotal API Key detected.")
		if os.Getenv("VT_API_KEY") != "" {
			pterm.Warning.Println("Note: Your VT_API_KEY environment variable is currently overriding the config. Updating the config below will NOT take effect until you unset the environment variable.")
		}

		var confirm string
		fmt.Printf("\nDo you want to update your API key in config? (y/N): ")
		_, _ = fmt.Scanln(&confirm)
		if strings.ToLower(strings.TrimSpace(confirm)) == "y" {
			fmt.Printf("Enter new VirusTotal API Key: ")
			var newKey string
			_, _ = fmt.Scanln(&newKey)
			if newKey != "" {
				apiKey = strings.TrimSpace(newKey)
				cfg, _ := config.LoadConfig()
				if cfg == nil {
					cfg = &config.Config{}
				}
				cfg.Core.VTApiKey = apiKey
				_ = config.SaveConfig(cfg)
				goto RetryVT
			}
		} else {
			fmt.Printf("Continue installation without VirusTotal? (y/N): ")
			_, _ = fmt.Scanln(&confirm)
			if strings.ToLower(strings.TrimSpace(confirm)) == "y" {
				return nil
			}
			return fmt.Errorf("installation aborted due to invalid vt api key")
		}
	}

	if resp.StatusCode == http.StatusNotFound {
		if skipSandbox {
			log.Warn("VirusTotal has no record of this hash. Bypassing sandbox (--skip-vt-sandbox).", "hash", hash)
			return nil
		}
		if interactive {
			var confirm string
			fmt.Printf("\nVirusTotal has no record of %s. Upload to sandbox and wait for analysis? [y/N]: ", filepath.Base(filePath))
			_, _ = fmt.Scanln(&confirm)
			if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
				log.Warn("User declined VT sandbox upload. Bypassing check.", "hash", hash)
				return nil
			}
		}
		log.Info("Uploading file to VirusTotal sandbox...", "file", filepath.Base(filePath))
		return uploadToVirusTotal(filePath, apiKey, pUI, vtSandboxMessage)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("virustotal api returned status %d", resp.StatusCode)
	}

	var result struct {
		Data struct {
			Attributes struct {
				LastAnalysisStats struct {
					Malicious int `json:"malicious"`
				} `json:"last_analysis_stats"`
			} `json:"attributes"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode virustotal response: %w", err)
	}

	maliciousCount := result.Data.Attributes.LastAnalysisStats.Malicious
	if maliciousCount > 0 {
		return fmt.Errorf("virustotal blocked installation: %d engines detected hash %s as malicious", maliciousCount, hash)
	}

	log.Info("VirusTotal confirms file is clean.", "hash", hash)
	return nil
}

func CalculateSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// progressReader wraps an io.Reader and calls a callback with the number of bytes read.
type progressReader struct {
	reader   io.Reader
	total    int64
	read     int64
	callback func(read, total int64)
}

func (pr *progressReader) Read(p []byte) (n int, err error) {
	n, err = pr.reader.Read(p)
	pr.read += int64(n)
	if pr.callback != nil {
		pr.callback(pr.read, pr.total)
	}
	return n, err
}

func uploadToVirusTotal(filePath string, apiKey string, pUI *ui.PacmanUI, vtSandboxMessage string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	uploadURL := fmt.Sprintf("%s/files", vtBaseURL)

	// For large files, first get the upload URL
	if info.Size() > 32*1024*1024 {
		req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/files/upload_url", vtBaseURL), nil)
		req.Header.Set("x-apikey", apiKey)
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := doVTRequestWithRetry(client, req)
		if err != nil {
			return err
		}
		defer func() { _ = resp.Body.Close() }()

		var result struct {
			Data string `json:"data"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&result)
		if result.Data == "" {
			return fmt.Errorf("failed to get upload_url for large file")
		}
		uploadURL = result.Data
	}

	// Start VT upload animation with real progress tracking
	if pUI != nil && vtSandboxMessage != "" {
		pUI.SetVTUpload(true, vtSandboxMessage)
	}

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		defer func() { _ = pw.Close() }()
		part, err := writer.CreateFormFile("file", filepath.Base(filePath))
		if err == nil {
			if pUI != nil && info.Size() > 0 {
				// Wrap file with progress tracking for UI
				progReader := &progressReader{
					reader: file,
					total:  info.Size(),
					callback: func(read, total int64) {
						if model := pUI.Model(); model != nil {
							model.ProgressTarget = float64(read) / float64(total)
						}
					},
				}
				_, _ = io.Copy(part, progReader)
			} else {
				// No UI or unknown size - use simple copy
				_, _ = io.Copy(part, file)
			}
		}
		_ = writer.Close()
	}()

	req, _ := http.NewRequest(http.MethodPost, uploadURL, pr)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("x-apikey", apiKey)

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := doVTRequestWithRetry(client, req)

	if err != nil {
		if pUI != nil {
			pUI.SetVTUpload(false, "")
		}
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		if pUI != nil {
			pUI.SetVTUpload(false, "")
		}
		return fmt.Errorf("virustotal upload failed with status %d", resp.StatusCode)
	}

	var result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result)

	log.Info("Upload complete. Waiting for sandbox analysis...", "analysis_id", result.Data.ID)
	if pUI != nil {
		pUI.SetVTUpload(false, "")
	}
	return pollVirusTotalAnalysis(result.Data.ID, apiKey, pUI, vtSandboxMessage)
}

func pollVirusTotalAnalysis(analysisID string, apiKey string, pUI *ui.PacmanUI, vtSandboxMessage string) error {
	client := &http.Client{Timeout: 10 * time.Second}
	for {
		time.Sleep(vtPollDelay)

		req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/analyses/%s", vtBaseURL, analysisID), nil)
		req.Header.Set("x-apikey", apiKey)

		resp, err := doVTRequestWithRetry(client, req)
		if err != nil {
			if pUI != nil {
				pUI.SetVTUpload(false, "")
			}
			return err
		}

		var result struct {
			Data struct {
				Attributes struct {
					Status string `json:"status"`
					Stats  struct {
						Malicious int `json:"malicious"`
					} `json:"stats"`
				} `json:"attributes"`
			} `json:"data"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&result)
		_ = resp.Body.Close()

		if result.Data.Attributes.Status == "completed" {
			malicious := result.Data.Attributes.Stats.Malicious
			if malicious > 0 {
				if pUI != nil {
					pUI.SetVTUpload(false, "")
				}
				return fmt.Errorf("virustotal sandbox blocked installation: %d engines detected file as malicious", malicious)
			}
			log.Info("VirusTotal sandbox analysis confirms file is clean.")
			if pUI != nil {
				pUI.SetVTUpload(false, "")
			}
			return nil
		}
		log.Info("Sandbox analysis still in progress...")
	}
}
