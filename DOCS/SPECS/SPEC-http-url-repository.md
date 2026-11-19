# Specification: HTTP/HTTPS Directory & Storage Bucket Repository Support (`http-url-repository`)

## 1. Objective

Enable `gh-pt install`, `gh-pt show`, `gh-pt upgrade`, and `gh-pt rm` to natively ingest and install software packages from generic HTTP/HTTPS directory listings, cloud storage bucket indexes (Amazon S3, Google Cloud Storage), and Single Page Application (SPA) filetree indices without requiring GitHub releases or the `gh` CLI.

Additionally, formalize named `--repo` (`aliases:"repository"`) support across all primary commands so repository arguments can be provided as explicit named flags or positional parameters interchangeably.

Example target invocation:
```bash
gh-pt install --http-url https://storage.openvinotoolkit.org/repositories/openvino_model_server/packages/weekly/latest/ -s --driver=auto -u -y
```

When executed, `gh-pt` must:
1. Bypass `validateGHCLI()` and `needsGitHubAPI()` entirely.
2. Ingest directory contents from `https://storage.openvinotoolkit.org/repositories/openvino_model_server/packages/weekly/latest/` by parsing HTML links, SPA `filetree.json` descriptors, or S3/GCS bucket XML endpoints.
3. Synthesize a virtual repository record (e.g. `storage.openvinotoolkit.org/packages/weekly/latest`) and virtual release assets list.
4. Route assets into `selector.AssetClassifier`, correctly picking `ovms_ubuntu26.tar.gz` on Ubuntu 26.04 or `ovms_ubuntu24.tar.gz` on Ubuntu 24.04 via existing host OS version distance scoring.
5. Download, extract, place binaries and sidecars (`ovms/bin/ovms`, `ovms/lib/*.so`), configure runtime driver integration, and persist `HTTPURL` in `state.json`.

---

## 2. Tech Stack & Dependencies

- **Language**: Go 1.27
- **CLI Framework**: Kong (`params/params.go`, `cmd/router.go`, `cmd/root.go`)
- **HTTP Client**: `net/http` standard library with custom redirect handler, streaming timeouts, and User-Agent `gh-pt/<version>`
- **Parsers**:
  - `encoding/json` (OpenVINO `filetree.json` and directory descriptors)
  - `encoding/xml` (S3/GCS `ListBucketResult` XML responses)
  - `net/url` and `regexp` (HTML `<a href="...">` directory index link extraction)
- **Asset Selection**: `selector/classifier.go` (`AssetClassifier`, `PickDefaultAssetsWithContext`, `ClassifyReleaseWithContext`)
- **State Management**: `state/state.go` (`InstalledApp.HTTPURL`)

---

## 3. Flag & Argument Contract

### 3.1. CLI Flags Addition
In `params/params.go`:

```go
type CommonRepoFlags struct {
	Repo    string `optional:"" name:"repo" aliases:"repository" predictor:"github_repos" predict:"github_repos" help:"Repository in OWNER/REPO format (alternative to positional argument)."`
	HTTPURL string `optional:"" name:"http-url" aliases:"http" help:"HTTP or HTTPS directory index or storage bucket URL to install from (bypasses GitHub API)."`
}
```

Embedded into `InstallCmd`, `ShowCmd`, `UpgradeCmd`, and `ExecContext`:

```go
type InstallCmd struct {
	Repository string `arg:"" env:"GH_PT_REPOSITORY" optional:"" predictor:"github_repos" predict:"github_repos" help:"Github repository in OWNER/REPOSITORY_NAME format."`
	CommonRepoFlags
	CommonInstallFlags
}

type Show struct {
	Repository string `arg:"" env:"GH_PT_REPOSITORY" optional:"" predictor:"github_repos" predict:"github_repos" help:"Github repository in OWNER/REPOSITORY_NAME format."`
	CommonRepoFlags
	// ... existing fields
}

type UpgradeCmd struct {
	Repository string `arg:"" optional:"" predictor:"installed_apps" predict:"installed_apps" help:"Optional repository to update."`
	CommonRepoFlags
	User       bool   `short:"u" name:"user" help:"Update only user installations."`
	Global     bool   `short:"g" name:"global" help:"Update only global installations."`
	UpgradeFlags
}
```

### 3.2. Validation & Mutual Exclusion Rules
In `cmd/root.go:validateInputs()`:

1. **Mutual Exclusion**:
   - Specifying both `--http-url` and a GitHub repository (either via positional `<repository>` or `--repo`) returns an error:
     `"cannot specify both --http-url and repository"`
   - Specifying both positional `<repository>` and `--repo` with conflicting values returns an error:
     `"conflicting repository specifications: positional %q vs --repo %q"`
2. **Canonical Mapping**:
   - If positional `<repository>` is empty and `--repo` is set: assign `r.Repository = r.Repo`.
   - If `--http-url` is provided:
     - Verify scheme is strictly `http` or `https` via `url.Parse()`.
     - Assign `r.HTTPURL = cleanURL`.
     - Synthesize a unique canonical repository key for `state.json` and directory stores:
       `cleanHostPath(r.HTTPURL)` (e.g. `storage.openvinotoolkit.org/packages/weekly/latest`).
     - Set `r.Repository = syntheticKey`.
3. **Bypassing GitHub API**:
   - `needsGitHubAPI(r *RootCLI)` returns `false` if `r.HTTPURL != ""`.
   - When `needsGitHubAPI` returns `false`, `validateGHCLI()` is skipped. Pure HTTP directory operations never invoke or require `gh`.

---

## 4. Webpage & Directory Index Parser Architecture (`release/http_repo.go`)

### 4.1. Supported Index Formats

The parser in `release/http_repo.go` executes a cascading multi-format probe:

```
                  ┌───────────────────────────────┐
                  │    HTTP GET target URL        │
                  └───────────────┬───────────────┘
                                  │
                  ┌───────────────▼───────────────┐
                  │ Does body contain             │
                  │ FILETREE_URL or BUCKET_URL?   │
                  └───────┬───────────────┬───────┘
                     YES  │               │ NO
                          │               │
     ┌────────────────────▼─────┐  ┌──────▼────────────────────────┐
     │ Fetch & traverse         │  │ Is content XML               │
     │ filetree.json subtree    │  │ (<ListBucketResult>)?        │
     └──────────────────────────┘  └──────┬────────────────┬───────┘
                                     YES  │                │ NO
                                          │                │
                     ┌────────────────────▼─────┐  ┌───────▼──────────────┐
                     │ Parse S3/GCS bucket XML  │  │ Parse HTML           │
                     │ Contents keys & sizes    │  │ <a href="..."> links │
                     └──────────────────────────┘  └──────────────────────┘
```

#### A. OpenVINO / SPA `filetree.json` Format
- **Trigger**: HTML body contains `var FILETREE_URL = '...'` or `var BUCKET_URL = '...'`.
- **Mechanism**:
  1. Extract `FILETREE_URL` (resolving relative paths against target URL).
  2. Fetch JSON tree.
  3. Traverse `tree.children` recursively matching path segments of the target URL path (e.g. `["repositories", "openvino_model_server", "packages", "weekly", "latest"]`).
  4. Extract all children where `type == "file"`.
  5. Parse `name`, `size`, `last modified`, and construct `downloadURL = BUCKET_URL + "/" + cleanPath + "/" + name`.

#### B. S3 / GCS XML Bucket Listing (`ListBucketResult`)
- **Trigger**: Content starts with or contains `<ListBucketResult>` or `<EnumerationResults>`.
- **Mechanism**:
  ```go
  type ListBucketResult struct {
      XMLName  xml.Name `xml:"ListBucketResult"`
      Contents []struct {
          Key          string `xml:"Key"`
          LastModified string `xml:"LastModified"`
          Size         int64  `xml:"Size"`
      } `xml:"Contents"`
  }
  ```
  Extracts keys sharing the URL path prefix, strips the directory prefix to obtain asset filenames, and creates full download URLs.

#### C. HTML Directory Autoindex (`<a href="...">`)
- **Trigger**: Fallback for Apache `mod_autoindex`, Nginx `autoindex`, Caddy, Lighttpd, Python `http.server`, etc.
- **Mechanism**:
  1. Parse all anchor tags `<a href="...">`.
  2. Resolve relative URLs using `baseURL.ResolveReference(parsedHref)`.
  3. Filter out:
     - Parent directory links (`..`, `../`, `/`).
     - Query-sort parameter links (`?C=N;O=D`, `?sort_by=`, `?v=`).
     - Cross-origin and non-subpath references.
     - Document anchors (`#...`).
  4. Deduplicate assets. If file sizes are omitted in HTML, optionally query `Content-Length` via `HEAD` or leave 0 for streaming download.

### 4.2. Virtual Release Synthesis
The parser returns:
```go
type HTTPReleaseIndex struct {
	BaseURL     string
	CleanSlug   string // e.g. "storage.openvinotoolkit.org/packages/weekly/latest"
	VersionTag  string // e.g. "latest", or date/version extracted from path or version.txt
	Assets      []*selector.SelectorItem
}
```

Each asset is instantiated as:
```go
&selector.SelectorItem{
	Name:        filename,
	Size:        fileSize,
	DownloadURL: absoluteURL,
	Kind:        selector.Asset,
	Type:        filepath.Ext(filename),
}
```

---

## 5. Pipeline Parity & Integration Points

### 5.1. Classification Parity
`HTTPReleaseIndex.Assets` names are directly passed to `selector.NewAssetClassifier(hostOS, hostArch, wineMode, avxLevel)`:
- `ClassifyReleaseWithContext()` sorts assets into `CategoryDefaultInstall`, `CategorySidecarInstall`, `CategoryInstallableAlternative`, and `CategoryCannotInstall`.
- Architecture filters (`amd64`, `arm64`) and OS version scoring (`ScoreAssetOSVersion`) run without modification.

### 5.2. Download & Extraction Parity
In `release/release.go:DownloadAsset()`:
- If `asset.DownloadURL != ""` and `strings.HasPrefix(asset.DownloadURL, "http")`:
  Download directly via `http.Get()` with progress bar and context cancellation, bypassing GitHub API endpoints.
- Verification: If companion `.sha256` or `SHA256SUMS` asset is detected in the HTTP index, download and verify checksum unless `--insecure-allow-unsigned` is set.
- Extraction: Pass downloaded archive to `ouch` / `archiver` standard unpack pipeline.

### 5.3. Sidecar, Driver, and Symlink Parity
- `-s` (`--include-sidecars`): Captures `ovms/lib/*.so`, `ovms/share/*`.
- `--driver=auto` / `--driver=ldso`: Places `ld.so.conf.d` manifest or sets environment hints.
- `--plugin`: Full support for application plugins.
- Symlinking: Binaries inside `ovms/bin/*` symlinked to `~/.local/bin` (or `/usr/local/bin` with `-g`).

### 5.4. State Persistence (`state.json`)
In `state/state.go`:
```go
type InstalledApp struct {
	// ...
	HTTPURL string `json:"http_url,omitempty"`
}
```
When recorded:
- `app.Repository = "storage.openvinotoolkit.org/packages/weekly/latest"`
- `app.HTTPURL = "https://storage.openvinotoolkit.org/repositories/openvino_model_server/packages/weekly/latest/"`
- `app.InstalledBinaries = ["ovms", "convert_tokenizer"]`
- `app.InstalledSidecars = ["lib/libovms_shared.so", ...]`
- Enables `gh-pt upgrade storage.openvinotoolkit.org/packages/weekly/latest` to re-fetch the HTTP URL and update seamlessly.

---

## 6. OpenVINO Model Server Target Verification Case

### 6.1. Target Endpoint
- **URL**: `https://storage.openvinotoolkit.org/repositories/openvino_model_server/packages/weekly/latest/`
- **Host Environment**: Ubuntu 26.04.1 LTS (Resolute Raccoon), Linux 6.8+ x86_64, AVX2 enabled.

### 6.2. Discovered Assets (Verified Live)
| Asset Name | File Size | Classification on Ubuntu 26.04 |
| :--- | :--- | :--- |
| `ovms_ubuntu26.tar.gz` | 199,122,488 B | **Primary Default Install (Score 0.0 - exact Ubuntu 26 match)** |
| `ovms_ubuntu24.tar.gz` | 189,024,090 B | Alternative Installable (Score 2.0 - older Ubuntu fallback) |
| `ovms_redhat9.tar.gz` | 122,653,515 B | Foreign (Alien distro package on dpkg) |
| `ovms_windows.zip` | 146,150,631 B | Unsupported / Wine (Disabled on non-wine) |
| `ovms_ubuntu26.tar.gz.sha256`| 87 B | Checksum companion (`SubcatChecksum`) |
| `ovms_ubuntu24.tar.gz.sha256`| 87 B | Checksum companion (`SubcatChecksum`) |
| `ovms_windows.zip.sha256` | 82 B | Checksum companion (`SubcatChecksum`) |
| `ovms_redhat9.tar.gz.sha256` | 86 B | Checksum companion (`SubcatChecksum`) |
| `version.txt` | 152 B | Metadata (`SubcatMetadata`) |
| `changelog.txt` | 2,634 B | Metadata (`SubcatMetadata`) |

### 6.3. Target Tarball Internal Structure
Inside `ovms_ubuntu26.tar.gz`:
- Binaries:
  - `ovms/bin/ovms` (main server executable, 46.4 MB)
  - `ovms/bin/convert_tokenizer` (tokenization helper, 279 B)
- Shared Libraries (`ovms/lib/*.so`):
  - `libovms_shared.so` (31.4 MB)
  - `libopenvino_intel_gpu_plugin.so` (39.3 MB)
  - `libopenvino.so.2026.5.0`
  - `libopencv_core.so.4.14.0`, `libtbb.so.12`
- Runtime Data:
  - `ovms/share/espeak-ng-data/*`
  - `ovms/thirdparty-licenses/*`

### 6.4. Optimal Execution Command
```bash
gh-pt install \
  --http-url https://storage.openvinotoolkit.org/repositories/openvino_model_server/packages/weekly/latest/ \
  -s \
  --driver=auto \
  -u \
  -y
```

Flags justification:
- `--http-url`: Directs `gh-pt` to ingest from the OpenVINO HTTP index.
- `-s` (`--include-sidecars`): Automatically captures `ovms/lib/` shared libraries and `ovms/share/` data directories.
- `--driver=auto`: Enables userspace `ldso` dynamic linking so `ovms` locates `libovms_shared.so` and OpenVINO runtime libraries at runtime.
- `-u`: User-local install to `~/.local/bin` without requiring `sudo`.
- `-y`: Auto-confirms prompts for unattended execution.

---

## 7. Testing Strategy & Seams

### 7.1. Seam 1: CLI Flags & Mutual Exclusion
- **File**: `params/params_test.go` and `cmd/root_test.go`
- **Verification**:
  - `gh-pt install --http-url https://example.com/repo/` parses without positional repository.
  - `gh-pt install --repo owner/repo` parses without positional repository.
  - Mutual exclusion errors when both `--http-url` and `owner/repo` are passed.
  - Verification that `needsGitHubAPI()` is false when `--http-url` is present.

### 7.2. Seam 2: HTTP Directory & Bucket Parser
- **File**: `release/http_repo_test.go`
- **Verification**:
  - `httptest.Server` simulating HTML directory listing (`<a href="tool-linux-amd64.tar.gz">`).
  - `httptest.Server` simulating OpenVINO HTML with embedded `FILETREE_URL` + JSON tree.
  - `httptest.Server` simulating S3 XML `ListBucketResult`.
  - Relative vs absolute URL resolution, query link pruning (`?C=M`), parent link pruning (`../`).

### 7.3. Seam 3: Virtual Release & Classifier Pipeline Integration
- **File**: `release/release_http_test.go`
- **Verification**:
  - Live/Mock integration test verifying `ovms_ubuntu26.tar.gz` is selected on Ubuntu 26 host.
  - Verifying direct HTTP asset streaming download with checksum verification.
  - Verifying `state.InstalledApp` records `HTTPURL`.
