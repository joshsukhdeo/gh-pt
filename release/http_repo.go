package release

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/joshsukhdeo/gh-pt/selector"
)

// Asset represents a release asset with download metadata.
type Asset struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	DownloadURL string `json:"download_url"`
	ContentType string `json:"content_type,omitempty"`
}

// Release represents a synthetic release structure.
type Release struct {
	TagName string  `json:"tag_name"`
	Name    string  `json:"name"`
	Assets  []Asset `json:"assets"`
}

// HTTPReleaseIndex holds the parsed directory index assets and metadata.
type HTTPReleaseIndex struct {
	BaseURL    string  `json:"base_url"`
	CleanSlug  string  `json:"clean_slug"`
	VersionTag string  `json:"version_tag"`
	Assets     []Asset `json:"assets"`
}

// ToRelease converts the HTTPReleaseIndex to a synthetic *Release.
func (idx *HTTPReleaseIndex) ToRelease() *Release {
	return &Release{
		TagName: idx.VersionTag,
		Name:    idx.VersionTag,
		Assets:  idx.Assets,
	}
}

// ToSelectorItems converts the HTTPReleaseIndex assets to selector.SelectorItem slice.
func (idx *HTTPReleaseIndex) ToSelectorItems() []*selector.SelectorItem {
	items := make([]*selector.SelectorItem, len(idx.Assets))
	for i, a := range idx.Assets {
		items[i] = &selector.SelectorItem{
			Id:   i,
			Name: a.Name,
			Size: a.Size,
			URL:  a.DownloadURL,
		}
	}
	return items
}

// CleanHTTPSlug validates an HTTP/HTTPS URL and produces a deterministic
// owner/repo-compatible slug: <host>/<sanitized-path-tail>.
// The resulting string contains exactly one slash dividing ownerID and repoID.
func CleanHTTPSlug(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL %q: %w", rawURL, err)
	}

	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("invalid scheme %q: only http and https are supported", u.Scheme)
	}

	host := u.Host
	if host == "" {
		return "", fmt.Errorf("empty host in URL %q", rawURL)
	}

	// Sanitize host (replace ':' with '-' to prevent invalid filesystem path characters)
	cleanHost := strings.ReplaceAll(host, ":", "-")

	// Sanitize path segments
	trimmedPath := strings.Trim(u.Path, "/")
	var parts []string
	if trimmedPath != "" {
		parts = strings.Split(trimmedPath, "/")
	}

	var cleanParts []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || p == "repositories" {
			continue
		}
		// If "packages" appears after an existing prefix (like openvino_model_server), omit it
		if p == "packages" && len(cleanParts) > 0 {
			continue
		}
		cleanParts = append(cleanParts, p)
	}

	var pathSlug string
	if len(cleanParts) == 0 {
		pathSlug = "root"
	} else {
		pathSlug = strings.Join(cleanParts, "-")
	}

	return fmt.Sprintf("%s/%s", cleanHost, pathSlug), nil
}

var (
	reFiletreeURL = regexp.MustCompile(`(?i)(?:var|const|let)?\s*FILETREE_URL\s*=\s*['"]([^'"]+)['"]`)
	reBucketURL   = regexp.MustCompile(`(?i)(?:var|const|let)?\s*BUCKET_URL\s*=\s*['"]([^'"]+)['"]`)
	reHTMLHref    = regexp.MustCompile(`(?i)<a\s+(?:[^>]*?\s+)?href=["']([^"']+)["'][^>]*>(.*?)</a>`)
)

type fileTreeNode struct {
	Name     string         `json:"name"`
	Type     string         `json:"type"`
	Size     int64          `json:"size"`
	Children []fileTreeNode `json:"children"`
}

type listBucketResult struct {
	XMLName  xml.Name `xml:"ListBucketResult"`
	Name     string   `xml:"Name"`
	Prefix   string   `xml:"Prefix"`
	Contents []struct {
		Key          string `xml:"Key"`
		Size         int64  `xml:"Size"`
		LastModified string `xml:"LastModified"`
	} `xml:"Contents"`
}

// FetchHTTPReleaseIndex fetches and parses directory listings, S3/GCS XML buckets,
// or OpenVINO SPA filetree indices into an HTTPReleaseIndex.
func FetchHTTPReleaseIndex(targetURL string) (*HTTPReleaseIndex, error) {
	cleanSlug, err := CleanHTTPSlug(targetURL)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build request for %s: %w", targetURL, err)
	}
	req.Header.Set("User-Agent", "gh-pt/2.0.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch target URL %s: %w", targetURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("target URL %s returned HTTP error %d", targetURL, resp.StatusCode)
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	parsedBase, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	// Cascade 1: OpenVINO / SPA filetree.json
	if match := reFiletreeURL.FindSubmatch(bodyBytes); len(match) > 1 {
		ftRaw := string(match[1])
		ftURL := resolveReferenceURL(parsedBase, ftRaw)

		bucketURL := ""
		if bMatch := reBucketURL.FindSubmatch(bodyBytes); len(bMatch) > 1 {
			bucketURL = strings.TrimSpace(string(bMatch[1]))
		}
		if bucketURL != "" {
			bucketURL = resolveReferenceURL(parsedBase, bucketURL)
		} else {
			bucketURL = fmt.Sprintf("%s://%s", parsedBase.Scheme, parsedBase.Host)
		}

		index, err := parseOpenVINOFiletree(client, targetURL, cleanSlug, ftURL, bucketURL)
		if err == nil && len(index.Assets) > 0 {
			return index, nil
		}
	}

	bodyStr := string(bodyBytes)

	// Cascade 2: S3 / GCS XML Bucket Listing (<ListBucketResult>)
	if strings.Contains(bodyStr, "<ListBucketResult") || strings.Contains(contentType, "xml") {
		var xmlResult listBucketResult
		if err := xml.Unmarshal(bodyBytes, &xmlResult); err == nil && len(xmlResult.Contents) > 0 {
			index := parseS3BucketListing(targetURL, cleanSlug, parsedBase, xmlResult, client)
			if len(index.Assets) > 0 {
				return index, nil
			}
		}
	}

	// Cascade 3: Direct File Download vs Standard HTML Directory Autoindex
	isArchive := isArchiveURL(parsedBase.Path)
	if !strings.Contains(contentType, "text/html") && (isArchive || !strings.Contains(contentType, "xml")) {
		// Single direct file
		filename := path.Base(parsedBase.Path)
		asset := Asset{
			Name:        filename,
			Size:        resp.ContentLength,
			DownloadURL: targetURL,
			ContentType: contentType,
		}
		return &HTTPReleaseIndex{
			BaseURL:    targetURL,
			CleanSlug:  cleanSlug,
			VersionTag: extractVersionFromFilename(filename),
			Assets:     []Asset{asset},
		}, nil
	}

	// HTML autoindex (<a href="...">)
	return parseHTMLAutoindex(targetURL, cleanSlug, parsedBase, bodyBytes, client)
}

func parseOpenVINOFiletree(client *http.Client, targetURL, cleanSlug, filetreeURL, bucketURL string) (*HTTPReleaseIndex, error) {
	req, err := http.NewRequest(http.MethodGet, filetreeURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "gh-pt/2.0.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("filetree URL returned %d", resp.StatusCode)
	}

	var root fileTreeNode
	if err := json.NewDecoder(resp.Body).Decode(&root); err != nil {
		return nil, err
	}

	targetParsed, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	pathSegments := strings.Split(strings.Trim(targetParsed.Path, "/"), "/")
	current := &root
	var traversed []string

	for _, seg := range pathSegments {
		if seg == "" {
			continue
		}
		var found *fileTreeNode
		for i := range current.Children {
			child := &current.Children[i]
			if strings.EqualFold(child.Name, seg) && child.Type == "directory" {
				found = child
				break
			}
		}
		if found != nil {
			current = found
			traversed = append(traversed, found.Name)
		} else {
			break
		}
	}

	var assets []Asset
	baseDownloadDir := strings.TrimRight(bucketURL, "/") + "/" + strings.Join(traversed, "/")

	for _, child := range current.Children {
		if child.Type == "file" || len(child.Children) == 0 {
			assets = append(assets, Asset{
				Name:        child.Name,
				Size:        child.Size,
				DownloadURL: strings.TrimRight(baseDownloadDir, "/") + "/" + child.Name,
			})
		}
	}

	versionTag := extractVersionTag(assets, targetURL, client)

	return &HTTPReleaseIndex{
		BaseURL:    targetURL,
		CleanSlug:  cleanSlug,
		VersionTag: versionTag,
		Assets:     assets,
	}, nil
}

func parseS3BucketListing(targetURL, cleanSlug string, parsedBase *url.URL, xmlResult listBucketResult, client *http.Client) *HTTPReleaseIndex {
	prefix := xmlResult.Prefix
	if prefix == "" {
		prefix = strings.TrimPrefix(parsedBase.Path, "/")
	}

	bucketBase := fmt.Sprintf("%s://%s", parsedBase.Scheme, parsedBase.Host)
	var assets []Asset

	for _, c := range xmlResult.Contents {
		key := c.Key
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		rel := strings.TrimPrefix(key, prefix)
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" || strings.HasSuffix(rel, "/") || strings.Contains(rel, "/") {
			// Subdirectory or self
			continue
		}

		downloadURL := strings.TrimRight(bucketBase, "/") + "/" + strings.TrimLeft(key, "/")
		assets = append(assets, Asset{
			Name:        rel,
			Size:        c.Size,
			DownloadURL: downloadURL,
		})
	}

	versionTag := extractVersionTag(assets, targetURL, client)

	return &HTTPReleaseIndex{
		BaseURL:    targetURL,
		CleanSlug:  cleanSlug,
		VersionTag: versionTag,
		Assets:     assets,
	}
}

func parseHTMLAutoindex(targetURL, cleanSlug string, parsedBase *url.URL, bodyBytes []byte, client *http.Client) (*HTTPReleaseIndex, error) {
	matches := reHTMLHref.FindAllSubmatch(bodyBytes, -1)
	var assets []Asset
	seen := make(map[string]bool)

	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		href := strings.TrimSpace(string(m[1]))
		if href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(href, "javascript:") {
			continue
		}
		// Ignore query sort links
		if strings.HasPrefix(href, "?") || strings.Contains(href, "?C=") || strings.Contains(href, "?sort") {
			continue
		}
		// Ignore parent links and directories
		if href == ".." || href == "../" || href == "/" || href == "./" || strings.HasSuffix(href, "/") {
			continue
		}

		resolved := resolveReferenceURL(parsedBase, href)
		resolvedURL, err := url.Parse(resolved)
		if err != nil {
			continue
		}

		// Ensure link is on the same host and not navigating upwards
		if resolvedURL.Host != parsedBase.Host {
			continue
		}

		filename := path.Base(resolvedURL.Path)
		if filename == "." || filename == "/" || seen[filename] {
			continue
		}
		seen[filename] = true

		assets = append(assets, Asset{
			Name:        filename,
			Size:        0,
			DownloadURL: resolved,
		})
	}

	versionTag := extractVersionTag(assets, targetURL, client)

	return &HTTPReleaseIndex{
		BaseURL:    targetURL,
		CleanSlug:  cleanSlug,
		VersionTag: versionTag,
		Assets:     assets,
	}, nil
}

func resolveReferenceURL(base *url.URL, ref string) string {
	u, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return base.ResolveReference(u).String()
}

func extractVersionTag(assets []Asset, targetURL string, client *http.Client) string {
	for _, a := range assets {
		if strings.EqualFold(a.Name, "version.txt") || strings.EqualFold(a.Name, "version") {
			req, err := http.NewRequest(http.MethodGet, a.DownloadURL, nil)
			if err == nil {
				req.Header.Set("User-Agent", "gh-pt/2.0.0")
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				req = req.WithContext(ctx)
				resp, err := client.Do(req)
				if err == nil && resp.StatusCode == http.StatusOK {
					body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
					_ = resp.Body.Close()
					lines := strings.Split(strings.TrimSpace(string(body)), "\n")
					if len(lines) > 0 && strings.TrimSpace(lines[0]) != "" {
						firstLine := strings.TrimSpace(lines[0])
						fields := strings.Fields(firstLine)
						if len(fields) > 0 {
							return fields[len(fields)-1]
						}
						return firstLine
					}
				}
			}
		}
	}

	u, err := url.Parse(targetURL)
	if err == nil {
		clean := strings.Trim(u.Path, "/")
		parts := strings.Split(clean, "/")
		if len(parts) > 0 && parts[len(parts)-1] != "" {
			return parts[len(parts)-1]
		}
	}

	return "latest"
}

func isArchiveURL(p string) bool {
	ext := strings.ToLower(filepath.Ext(p))
	switch ext {
	case ".gz", ".tgz", ".zip", ".xz", ".bz2", ".7z", ".zst", ".deb", ".rpm", ".appimage":
		return true
	default:
		return false
	}
}

func extractVersionFromFilename(name string) string {
	lower := strings.ToLower(name)
	for _, ext := range []string{".tar.gz", ".tar.xz", ".tar.bz2", ".tar.zst", ".zip", ".tgz", ".deb", ".rpm", ".appimage"} {
		lower = strings.TrimSuffix(lower, ext)
	}
	parts := strings.Split(lower, "-")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return "latest"
}

// HTTPReleaseClient satisfies selector.GithubClient for an HTTP-based release.
type HTTPReleaseClient struct {
	Index *HTTPReleaseIndex
}

// Get populates the response interface with asset metadata or releases.
func (c *HTTPReleaseClient) Get(path string, response interface{}) error {
	if c.Index == nil {
		return fmt.Errorf("HTTP release index is nil")
	}
	type assetDTO struct {
		ID                 int64  `json:"id"`
		Name               string `json:"name"`
		URL                string `json:"url"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
	}
	type releaseDTO struct {
		ID         int64      `json:"id"`
		TagName    string     `json:"tag_name"`
		Name       string     `json:"name"`
		Prerelease bool       `json:"prerelease"`
		Draft      bool       `json:"draft"`
		Assets     []assetDTO `json:"assets"`
	}

	dtos := make([]assetDTO, len(c.Index.Assets))
	for i, a := range c.Index.Assets {
		dtos[i] = assetDTO{
			ID:                 int64(i + 1),
			Name:               a.Name,
			URL:                a.DownloadURL,
			BrowserDownloadURL: a.DownloadURL,
			Size:               a.Size,
		}
	}

	if strings.Contains(path, "releases") && !strings.Contains(path, "assets") {
		rel := releaseDTO{
			ID:         1,
			TagName:    c.Index.VersionTag,
			Name:       c.Index.VersionTag,
			Prerelease: false,
			Draft:      false,
			Assets:     dtos,
		}
		if strings.Contains(path, "/tags/") {
			b, err := json.Marshal(rel)
			if err != nil {
				return err
			}
			return json.Unmarshal(b, response)
		}
		releasesList := []releaseDTO{rel}
		b, err := json.Marshal(releasesList)
		if err != nil {
			return err
		}
		return json.Unmarshal(b, response)
	}

	if strings.Contains(path, "assets") {
		b, err := json.Marshal(dtos)
		if err != nil {
			return err
		}
		return json.Unmarshal(b, response)
	}
	return nil
}

// Request returns an HTTP response containing JSON release assets.
func (c *HTTPReleaseClient) Request(method string, path string, body io.Reader) (*http.Response, error) {
	if c.Index == nil {
		return nil, fmt.Errorf("HTTP release index is nil")
	}
	type assetDTO struct {
		Name               string `json:"name"`
		Size               int64  `json:"size"`
		BrowserDownloadUrl string `json:"browser_download_url"`
		URL                string `json:"url"`
	}
	dtos := make([]assetDTO, len(c.Index.Assets))
	for i, a := range c.Index.Assets {
		dtos[i] = assetDTO{
			Name:               a.Name,
			Size:               a.Size,
			BrowserDownloadUrl: a.DownloadURL,
			URL:                a.DownloadURL,
		}
	}
	b, err := json.Marshal(dtos)
	if err != nil {
		return nil, err
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(string(b))),
		Header:     make(http.Header),
	}
	resp.Header.Set("Content-Type", "application/json")
	return resp, nil
}

// FindChecksumFileForAsset finds a checksum file that matches targetAsset or any general checksum file.
func (r *GithubRelease) FindChecksumFileForAsset(targetAsset string, assets []*selector.SelectorItem) string {
	for _, a := range assets {
		if strings.EqualFold(a.Name, targetAsset+".sha256") ||
			strings.EqualFold(a.Name, targetAsset+".sha256sum") ||
			strings.EqualFold(a.Name, targetAsset+".sha512") {
			return a.Name
		}
	}
	return r.findChecksumFile(assets)
}
