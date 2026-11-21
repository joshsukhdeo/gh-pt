package release

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/joshsukhdeo/gh-pt/selector"
)

// DownloadHTTPAsset downloads a remote asset via HTTP/HTTPS directly into destPath.
// Uses a temporary file and atomic rename on completion.
func DownloadHTTPAsset(ctx context.Context, downloadURL, destPath string) error {
	if ctx == nil {
		ctx = context.Background()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create download request for %s: %w", downloadURL, err)
	}
	req.Header.Set("User-Agent", "gh-pt/2.0.0")

	client := &http.Client{
		Timeout: 30 * time.Minute,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download from %s: %w", downloadURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download from %s returned HTTP %d", downloadURL, resp.StatusCode)
	}

	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", dir, err)
	}

	tmpFile := destPath + ".tmp"
	out, err := os.OpenFile(tmpFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create temp file %s: %w", tmpFile, err)
	}

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		_ = out.Close()
		_ = os.Remove(tmpFile)
		return fmt.Errorf("error streaming download for %s: %w", downloadURL, err)
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		_ = os.Remove(tmpFile)
		return fmt.Errorf("error syncing file %s: %w", tmpFile, err)
	}
	closeErr := out.Close()
	if closeErr != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("error closing file %s: %w", tmpFile, closeErr)
	}

	if err := os.Rename(tmpFile, destPath); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to rename temp file to %s: %w", destPath, err)
	}

	return nil
}

// DownloadAsset downloads a release asset using either direct HTTP streaming or gh release download.
func (r *GithubRelease) DownloadAsset(asset *selector.SelectorItem, downloadDir string) error {
	destPath := filepath.Join(downloadDir, asset.Name)
	if asset.URL != "" && (strings.HasPrefix(asset.URL, "http://") || strings.HasPrefix(asset.URL, "https://")) {
		log.Info("downloading asset via HTTP", "name", asset.Name, "url", asset.URL)
		return DownloadHTTPAsset(context.Background(), asset.URL, destPath)
	}

	releaseTag := r.ResolvedVersion
	if releaseTag == "" {
		releaseTag = "latest"
	}
	stdOut, stdErr, ghErr := ghExec("release", "download", releaseTag,
		"--repo", r.CliParams.Repository, "--pattern", asset.Name, "--dir", downloadDir)
	if ghErr != nil {
		return fmt.Errorf("failed to run gh command: %s", stdErr.String())
	}
	log.Debug("downloaded via gh", "output", stdOut.String())
	return nil
}
