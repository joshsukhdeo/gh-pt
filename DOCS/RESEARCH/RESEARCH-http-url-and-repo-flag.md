# Comprehensive Research: Supporting `--repo` Aliases and `--http-url` Direct Ingestion in `gh-pt`

**Document Status:** Complete & Verified Against Primary Sources  
**Date:** 2026-11-19  
**Repository:** `joshsukhdeo/gh-pt`  
**Target Path:** `/home/tay/projects/gh-pt/DOCS/RESEARCH/RESEARCH-http-url-and-repo-flag.md`  

---

## 1. Executive Summary & Problem Formulation

The GitHub Package Tracker (`gh-pt`) is designed as a GitHub CLI extension for installing, tracking, updating, and uninstalling pre-compiled release binaries and assets across Linux, macOS, Windows, and FreeBSD. Its existing execution architecture assumes that packages originate exclusively from GitHub repositories in `owner/repo` format, requiring active GitHub authentication (`gh auth status`) and querying GitHub REST/GraphQL APIs for release tags and asset manifests.

### The Problem
Modern machine learning toolchains, vendor SDKs, and internal enterprise packages are frequently distributed directly via HTTP/S3/GCS bucket mirrors and web directory indices rather than GitHub releases. A prime example is the Intel OpenVINO Model Server (OVMS), distributed at:
```
https://storage.openvinotoolkit.org/repositories/openvino_model_server/packages/weekly/latest/
```
Currently, `gh-pt` cannot install such packages because:
1. Every core command (`install`, `show`, `upgrade`, `rm`, `source`) strictly expects a positional `owner/repo` string.
2. CLI flags do not support `--repo` (with alias `--repository`) or `--http-url` (with alias `--http`).
3. Core execution in `cmd/root.go` hard-fails if GitHub REST client initialization or repository existence checks fail.
4. Asset downloads in `release/release.go` shell out to `gh release download` rather than utilizing native HTTP/HTTPS streaming.

### The Mandate
This research determines the exact structural modifications required to:
1. Allow the standard positional `<repository>` argument across commands (`install`, `show`, `upgrade`, `rm`, `state add`, `rollback`, `history`, `source`) to also be specified via `--repo` (`aliases:"repository"`).
2. Allow `--http-url` (`aliases:"http"`) to be passed **instead** of the positional `<repository>` or named `--repo` flag.
3. Enable `--http-url` to discover downloadable files, synthesize an internal repository-like release structure, and pass them through `gh-pt`'s existing asset classifier, selector, extraction, sidecar, driver, plugin, and `state.json` lifecycle.
4. Fully inspect the primary target (`storage.openvinotoolkit.org`) and determine the exact flags, binary paths, companion libraries, and drivers required for installation.

---

## 2. Primary Source HTTP Investigation: `storage.openvinotoolkit.org`

An investigation was conducted directly against the live HTTP endpoint:
`https://storage.openvinotoolkit.org/repositories/openvino_model_server/packages/weekly/latest/`

### 2.1 Live HTTP Wire Capture & Header Analysis
Executing `curl -sIL "https://storage.openvinotoolkit.org/repositories/openvino_model_server/packages/weekly/latest/"` yielded:
```http
HTTP/2 200 
content-type: text/html
content-length: 1061
server: AmazonS3
last-modified: Mon, 26 Oct 2026 13:52:17 GMT
etag: "1aff3227bcf4e042b40a8044d3225383"
x-amz-server-side-encryption: AES256
accept-ranges: bytes
via: 1.1 ...cloudfront.net (CloudFront)
```

### 2.2 Client-Side Rendering Architecture (`list.js` & `filetree.json`)
Inspection of the returned HTML revealed that `storage.openvinotoolkit.org` does **not** serve server-side HTML `<a href>` directory links, nor does it expose public Amazon S3 XML bucket listings (requests with `?list-type=2` or `?prefix=...` return the same static `index.html` via CloudFront URL rewriting).

Instead, the HTML loads a client-side JavaScript directory browser:
```html
<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>storage.openvinotoolkit.org</title>
  <link rel="stylesheet" href="/assets/theme.css">
</head>
<body>
  <main id="main" class="o-content sidebar-right">
    <div id="content" class="clearfix container o-container" style="display: none;">
      <div id="navigation"></div>
      <table id="list"></table>
    </div>
  </main>
<script type="text/javascript" src="https://ajax.googleapis.com/ajax/libs/jquery/3.7.1/jquery.min.js"></script>
<script type="text/javascript">
  var BUCKET_URL = 'https://storage.openvinotoolkit.org';
  var FILETREE_URL = 'https://storage.openvinotoolkit.org/filetree.json';
  var EXCLUDE_FILE = [/^(.*\/)?index.html$/, /^(.*\/)?list.js$/, '/assets'];
</script>
<script type="text/javascript" src="/assets/list.js"></script>
</body>
</html>
```

Inspection of `/assets/list.js` (lines 74–154) confirmed the data ingestion pattern:
1. It requests `FILETREE_URL` (`https://storage.openvinotoolkit.org/filetree.json`, ~5.4 MB JSON).
2. It parses the current URL pathname into path segments:
   `CURRENT_PATH = "repositories/openvino_model_server/packages/weekly/latest"`.
3. It recursively navigates `data.children` to find the directory node whose nested `name` fields match the path segments.
4. It populates `<table id="list">` from the matching directory's `children`.

### 2.3 Live Remote Directory Inventory
Traversing `https://storage.openvinotoolkit.org/filetree.json` at path `repositories/openvino_model_server/packages/weekly/latest` uncovered the live package manifest:

| Type | Asset Name | File Size (Bytes) | Last Modified |
| :--- | :--- | :--- | :--- |
| `file` | `changelog.txt` | 2,634 | 2026-11-19 15:15 |
| `file` | `version.txt` | 152 | 2026-11-19 14:06 |
| `file` | `ovms_ubuntu24.tar.gz` | 189,024,090 | 2026-11-19 14:02 |
| `file` | `ovms_ubuntu24.tar.gz.sha256` | 87 | 2026-11-19 14:02 |
| `file` | `ovms_ubuntu26.tar.gz` | 199,122,488 | 2026-11-19 14:06 |
| `file` | `ovms_ubuntu26.tar.gz.sha256` | 87 | 2026-11-19 14:06 |
| `file` | `ovms_redhat9.tar.gz` | 122,653,515 | 2026-11-19 15:03 |
| `file` | `ovms_redhat9.tar.gz.sha256` | 86 | 2026-11-19 15:07 |
| `file` | `ovms_windows.zip` | 146,150,631 | 2026-11-19 14:04 |
| `file` | `ovms_windows.zip.sha256` | 82 | 2026-11-19 14:04 |

### 2.4 Metadata & Release Information
Direct extraction of `version.txt`:
```
OpenVINO Model Server 2026.5.0.dbe11fa27
OpenVINO backend 2026.5.0-23836-f36b4c17f38-releases/2026/5
OpenVINO GenAI backend 2026.5.0.0-3636-2de7224b803
```
Direct extraction of `ovms_ubuntu26.tar.gz.sha256`:
```
3b8fedf36836405b8ab84d79a005146c3a75248e805f2057b3663680dc192f45  ovms_ubuntu26.tar.gz
```
Direct extraction of `ovms/metadata.json` from the tarball:
```json
{
    "PROJECT_VERSION": "2026.5.0.dbe11fa27",
    "OPENVINO backend": "2026.5.0-23836-f36b4c17f38-releases/2026/5",
    "BAZEL_BUILD_FLAGS": "--strip=always --config=mp_on --//:distro=ubuntu",
    "BASE_IMAGE": "ubuntu:24.04"
}
```

### 2.5 Archive Internals (`ovms_ubuntu24.tar.gz` and `ovms_ubuntu26.tar.gz`)
Inspection of the tar stream (`tar -ztvf`) revealed the internal layout:
- **Archive Root Directory:** `ovms/`
- **Executables (`ovms/bin/`):**
  - `ovms/bin/ovms` (45,491,288 bytes, ELF 64-bit LSB pie executable, x86-64)
  - `ovms/bin/convert_tokenizer` (279 bytes, executable Python 3 wrapper script)
- **Shared Libraries & Hardware Drivers (`ovms/lib/`):**
  - OpenVINO Inference Plugins & Compute Engines:
    - `libopenvino_intel_gpu_plugin.so` (38.2 MB)
    - `libopenvino_intel_npu_vm_runtime.so` (697 KB)
    - `libopenvino_auto_plugin.so` (747 KB)
    - `libopenvino_auto_batch_plugin.so` (258 KB)
    - `libopenvino.so.2026.5.0`
    - `libopenvino_genai.so.2026.5.0.0`
  - Model Frontend Parsers:
    - `libopenvino_pytorch_frontend.so.2026.5.0`
    - `libopenvino_onnx_frontend.so.2026.5.0`
    - `libopenvino_tensorflow_frontend.so.2026.5.0`
    - `libopenvino_paddle_frontend.so.2026.5.0`
    - `libopenvino_gguf_frontend.so.2026.5.0`
  - MediaPipe & Python Runtime:
    - `libovms_shared.so` (31.2 MB)
    - `libovms_mediapipe_runtime_shared.so` (17.1 MB)
    - `libovms_opencv_calculators.so`
    - `libpython_calculators.so`
    - `libovms_llm_calculators.so`
  - Acceleration & Ecosystem Libraries:
    - OpenCV 4.14 (`libopencv_core`, `libopencv_imgproc`, etc.)
    - Intel TBB (`libtbb.so.12.13`)
    - Khronos OpenCL (`libOpenCL.so.1`)
    - Azure Storage Blobs SDK (`libazure-storage-blobs.so.12.18.0`)
- **Assets & Data (`ovms/share/`):**
  - Phonetic dictionary data: `ovms/share/espeak-ng-data/`
- **C/C++ Headers (`ovms/include/`):**
  - `ovms/include/ovms.h`

### 2.6 Dynamic Linking & `$ORIGIN` RUNPATH Verification
Executing `readelf -d` on the extracted `ovms/bin/ovms` binary revealed:
```
0x000000000000001d (RUNPATH)            Library runpath: [$ORIGIN/../lib/]
0x0000000000000001 (NEEDED)             Shared library: [libgit2.so]
0x0000000000000001 (NEEDED)             Shared library: [libazure-storage-blobs.so.12.18.0]
0x0000000000000001 (NEEDED)             Shared library: [libopenvino.so.2650]
0x0000000000000001 (NEEDED)             Shared library: [libopenvino_genai.so.2650]
0x0000000000000001 (NEEDED)             Shared library: [libOpenCL.so.1]
...
```
**Key Finding:** Because the executable specifies `RUNPATH: [$ORIGIN/../lib/]`, the Linux dynamic linker (`ld.so`) resolves libraries relative to the canonical directory of the target binary (`/proc/self/exe`). When `gh-pt` extracts the archive to its package store (e.g. `~/.local/share/gh-pt/packages/...`) and symlinks `ovms/bin/ovms` to `~/.local/bin/ovms`, `$ORIGIN/../lib/` automatically resolves to the extracted package's `lib/` directory.

---

## 3. Codebase Seams Analysis (Exact File & Line Citations)

The investigation inspected all critical files in the `gh-pt` pipeline. Below are the precise locations requiring modification.

### 3.1 `params/params.go` (CLI Flag & Struct Definitions)
In `params/params.go`, subcommands define positional parameters using the `arg:""` tag. When Kong parses CLI commands, positional argument fields do not accept `--flag` syntax unless explicit flag fields exist, and positional arguments without `optional:""` will cause parsing errors if omitted in favor of flags.

* **Lines 81–84 (`SourceCleanCmd`):**
  `Repository string `arg:"" optional:"" ...``
  *Seam:* Add `Repo string `name:"repo" aliases:"repository" optional:"" help:"Repository to clean."`` and `HttpUrl string `name:"http-url" aliases:"http" optional:"" help:"HTTP URL to clean."``.
* **Lines 271–274 (`InstallCmd`):**
  ```go
  type InstallCmd struct {
      Repository string `arg:"" env:"GH_PT_REPOSITORY" optional:"" predictor:"github_repos" predict:"github_repos" help:"Github repository in OWNER/REPOSITORY_NAME format."`
      CommonInstallFlags
  }
  ```
  *Seam:* Add:
  ```go
  Repo    string `name:"repo" aliases:"repository" optional:"" help:"GitHub repository (owner/repo)."`
  HttpUrl string `name:"http-url" aliases:"http" optional:"" help:"HTTP URL for web directory listing or package bucket."`
  ```
  Implement helper method `GetTarget() (repo string, httpUrl string)` on `InstallCmd`.
* **Lines 311–318 (`HistoryCmd`):**
  *Seam:* Add `Repo` and `HttpUrl` flags.
* **Lines 320–327 (`RollbackCmd`):**
  `Repository string `arg:"" ...`` is currently non-optional.
  *Seam:* Mark `Repository` as `optional:""`, and add `Repo` and `HttpUrl` flags.
* **Lines 336–347 (`StateAddCmd`):**
  `Repository string `arg:"" ...`` is currently non-optional.
  *Seam:* Mark `Repository` as `optional:""`, and add `Repo` and `HttpUrl` flags.
* **Lines 349–355 (`StateRmCmd`):**
  `Target string `arg:"" ...`` is currently non-optional.
  *Seam:* Mark `Target` as `optional:""`, and add `Repo string `name:"repo" aliases:"repository" optional:""`` and `HttpUrl string `name:"http-url" aliases:"http" optional:""``.
* **Lines 381–386 (`UpgradeCmd`):**
  ```go
  type UpgradeCmd struct {
      Repository string `arg:"" optional:"" predictor:"installed_apps" predict:"installed_apps" help:"Optional repository to update."`
      User       bool   `short:"u" name:"user" help:"Update only user installations."`
      Global     bool   `short:"g" name:"global" help:"Update only global installations."`
      UpgradeFlags
  }
  ```
  *Seam:* Add `Repo string `name:"repo" aliases:"repository" optional:""`` and `HttpUrl string `name:"http-url" aliases:"http" optional:""``.
* **Lines 465–477 (`Show`):**
  `Repository string `arg:"" env:"GH_PT_REPOSITORY" ...`` is currently non-optional.
  *Seam:* Change to `optional:""`, and add `Repo` and `HttpUrl` flags.
* **Lines 481–485 (`Hook`):**
  *Seam:* Add `Repo` and `HttpUrl` flags.
* **Lines 498–504 (`SourceCmd`):**
  *Seam:* Mark `Repository` as `optional:""`, and add `Repo` and `HttpUrl` flags.
* **Lines 540–583 (`ExecContext`):**
  *Seam:* Add fields to `ExecContext`:
  ```go
  HttpUrl      string `kong:"-"`
  IsHttpSource bool   `kong:"-"`
  ```

### 3.2 `cmd/router.go` (Command Dispatch & Routing)
In `cmd/router.go`, incoming commands are matched via strings emitted by Kong (`ctx.Command()`).

* **Lines 64–70 (`case "install", "install <repository>":`):**
  Currently extracts `r.Repository = cli.Install.Repository` and `r.SpecificallyTargeted = cli.Install.Repository != ""`.
  *Seam:* Route `cli.Install.GetTarget()`. If `httpUrl != ""`, set `r.HttpUrl = httpUrl`, `r.IsHttpSource = true`, and if `repo == ""`, synthesize repository name from URL path. Set `r.SpecificallyTargeted = true`.
* **Lines 79–84 (`case "rm", "rm <target>", "uninstall", "uninstall <target>":`):**
  Currently passes `cli.Rm.Target` directly.
  *Seam:* Fall back to `cli.Rm.Repo` or `cli.Rm.HttpUrl` if `cli.Rm.Target` is empty.
* **Lines 85–111 (`case "upgrade", "upgrade <repository>":`):**
  Currently sets `r.Repository = cli.Upgrade.Repository`.
  *Seam:* Extract target via `GetTarget()`, allowing upgrades targeted by `--repo` or `--http-url`.
* **Lines 149–168 (`case "show", "show <repository>":`):**
  Currently passes `cli.Show.Repository`.
  *Seam:* Allow `cli.Show.Repo` or `cli.Show.HttpUrl`. When `cli.Show.HttpUrl != ""`, set `r.HttpUrl` and bypass GitHub repository API calls in `cmd/show.go`.

### 3.3 `cmd/root.go` (Pre-Flight Checks & Execution Engine)
* **Lines 200–228 (`func needsGitHubAPI(r *RootCLI) bool`):**
  Checks whether GitHub API authentication is required.
  *Seam:* Add early return at line 203:
  ```go
  if r.HttpUrl != "" || r.IsHttpSource {
      return false
  }
  ```
  Prevents failing with `cannot reach GitHub API` or missing `gh` authentication when `--http-url` is specified.
* **Lines 724–735 (`RunInstall` GitHub client validation):**
  *Seam:* Guard GitHub REST client creation. When `r.HttpUrl != ""`, do not instantiate `newGitHubRESTClient()`.
* **Lines 792–799 (`RunInstall` repository validation):**
  ```go
  if r.Repository == "" {
      return fmt.Errorf("repository argument is required for installation")
  }
  if !strings.Contains(r.Repository, "/") && !strings.HasPrefix(r.Repository, "http") {
      return fmt.Errorf("unknown command or invalid repository format: '%s' (expected owner/repo)", r.Repository)
  }
  ```
  *Seam:* When `r.HttpUrl != ""`, allow `r.Repository` to be derived (e.g. `openvino_model_server`) or accept URL format. Note that line 796 already contains `!strings.HasPrefix(r.Repository, "http")`, indicating prior architectural intent for HTTP targets.
* **Lines 842–850 (`RunInstall` GitHub repository check):**
  ```go
  if ghClient == nil {
      return fmt.Errorf("GitHub client is not initialized")
  }
  err = ghClient.Get(fmt.Sprintf("repos/%s", r.Repository), &response)
  ```
  *Seam:* When `r.HttpUrl != ""`, bypass `ghClient.Get` repository validation.
* **Lines 862–868 (`RunInstall` release instantiation):**
  ```go
  installRelease := release.MakeGithubRelease(&r.ExecContext, ghClient)
  err = installRelease.Install()
  ```
  *Seam:* When `r.HttpUrl != ""`, construct `installRelease` with a synthetic release provider or invoke an HTTP-aware installer.

### 3.4 `selector/interface.go` & `selector/classifier.go`
* **`selector/interface.go` Lines 25–28 (`GithubClient` Interface):**
  ```go
  type GithubClient interface {
      Get(path string, response interface{}) error
      Request(method string, path string, body io.Reader) (*http.Response, error)
  }
  ```
  *Seam:* An HTTP directory provider can implement `GithubClient` (e.g. `SyntheticReleaseClient`) without changing any consumer interfaces.
  - When `Get("repos/<repo>/releases", &response)` or `Get("repos/<repo>/releases/latest", &latestResp)` is called, the synthetic client populates the release list with the synthesized release tag (e.g. `latest` or the tag extracted from `version.txt`).
  - When `Request(http.MethodGet, "repos/<repo>/releases/<id>/assets", nil)` is called, it returns a JSON stream of `[]struct{ Name string, Size int64, BrowserDownloadUrl string, URL string }` populated directly from the parsed HTTP directory items.
* **`selector/interface.go` Lines 151–167 (`IsAllowedBinaryDir`):**
  ```go
  func IsAllowedBinaryDir(relPath string) bool {
      clean := filepath.ToSlash(filepath.Clean(relPath))
      parts := strings.Split(clean, "/")
      ...
      if len(parts) == 3 {
          dir := parts[1]
          return dir == "bin" || dir == "bin32" || dir == "bin64" || ...
      }
      return false
  }
  ```
  *Observation:* For `ovms_ubuntu24.tar.gz`, the binary path inside the archive is `ovms/bin/ovms`. `parts` is `["ovms", "bin", "ovms"]`, so `len(parts) == 3` and `dir == "bin"`. This already returns `true` without modification.
* **`selector/classifier.go` Lines 335–353 (`DetectOS`):**
  Matches `strings.Contains(lower, "ubuntu")`, `strings.Contains(lower, "redhat")`, and `strings.Contains(lower, "windows")`.
  *Observation:* `ovms_ubuntu24.tar.gz` and `ovms_ubuntu26.tar.gz` are correctly detected as `OSLinux`. `ovms_windows.zip` is correctly detected as `OSWindows`.
* **`selector/classifier.go` Lines 619–630 (`DetectSubcategory`):**
  For `amd64`, assets containing `ubuntu` without foreign arch tokens fall through to lines 688–693:
  ```go
  if os == OSLinux && c.HostOS == "linux" {
      if c.isNativeFormatForOS(name) {
          return SubcatNative
      }
      return SubcatForeign
  }
  ```
  *Observation:* Tarballs on Linux are recognized as `SubcatNative`.

### 3.5 `release/release.go` (Download, Verification & Extraction)
* **Lines 1554–1571 (`findChecksumFile`):**
  Currently searches for `checksums?.txt`, `sha256sums?.txt`, etc.
  *Seam:* Add support for per-asset checksum files (e.g. `(?i)\.sha256$` or `<asset_name>\.sha256`). OpenVINO provides `ovms_ubuntu24.tar.gz.sha256` and `ovms_ubuntu26.tar.gz.sha256`.
* **Lines 2186–2197 (Checksum file download):**
  Calls `ghExec("release", "download", releases[0].Name, "--repo", r.CliParams.Repository, "--pattern", checksumFileName, "--dir", downloadDir)`.
  *Seam:* If `r.CliParams.HttpUrl != ""` or if the checksum asset has a direct HTTP URL, download via standard HTTP GET into `downloadDir`.
* **Lines 2263–2275 (Asset file download):**
  Calls `ghExec("release", "download", releases[0].Name, "--repo", r.CliParams.Repository, "--pattern", asset.Name, "--dir", downloadDir)`.
  *Seam:* If `r.CliParams.HttpUrl != ""` or if `asset.URL` starts with `http://` or `https://`:
  Stream download directly via `http.Get` (or with Range/resume support) to `filepath.Join(downloadDir, asset.Name)`, updating `pUI` progress accordingly.
* **Lines 2555 & 2581 (`executeSymlinkInstall` and binary extraction):**
  *Observation:* Once downloaded to `downloadDir`, the existing extraction engine (`executeSymlinkInstall` in `release/symlink.go` and `extractTar`/`extractZip` via `ouch` or Go `archiver`) works unchanged.

### 3.6 `cmd/update.go` & `cmd/state_mgmt.go`
* **`cmd/update.go` Lines 52–92 (`BuildBatchReleaseQuery` & GraphQL batch):**
  Currently aggregates all apps in `st.Apps` and sends a batched GitHub GraphQL query.
  *Seam:* Exclude apps where `app.HttpUrl != ""` from the GitHub GraphQL query. Instead, check for updates by querying their HTTP endpoints for updated versions or new timestamps.
* **`cmd/state_mgmt.go` Lines 252–270 (`findTargetApps`):**
  Matches target against `repo`, `app.Rename`, and slash segments.
  *Seam:* Also match target against `app.HttpUrl`.
* **`state/state.go` Lines 14–58 (`InstalledApp` Struct):**
  *Seam:* Add fields:
  ```go
  HttpUrl      string `json:"http_url,omitempty"`
  IsHttpSource bool   `json:"is_http_source,omitempty"`
  ```

---

## 4. Universal HTTP Directory Listing & Release Synthesizer Architecture

To support arbitrary HTTP URLs in addition to OpenVINO, the directory discovery engine must handle the three standard formats found on the web:

```
                            [ User Input: --http-url <URL> ]
                                           │
                                           ▼
                                 [ HTTP GET <URL> ]
                                           │
                    ┌──────────────────────┼──────────────────────┐
                    ▼                      ▼                      ▼
           [ Client Script ]        [ HTML Listing ]       [ S3/GCS XML ]
        (e.g. filetree.json)      (e.g. Apache/Nginx)    (<ListBucketResult>)
                    │                      │                      │
                    ▼                      ▼                      ▼
        Extract & walk children    Parse <a href="...">     Parse <Contents><Key>
                    └──────────────────────┼──────────────────────┘
                                           │
                                           ▼
                            [ Discovered Remote Assets ]
                            - Name, Size, URL, Checksum
                                           │
                                           ▼
                        [ Synthetic Release Structure ]
                            - Tag: from version.txt / URL
                            - Assets: mapped to SelectorItems
                                           │
                                           ▼
                       [ Standard gh-pt Pipeline ]
                       - AssetClassifier (OS / Arch)
                       - Selector & Interactive UI
                       - HTTP Download & SHA256 Check
                       - Symlink / Binary Extraction
                       - state.json Update
```

### 4.1 Parser Strategy
1. **JSON Filetree Detection (OpenVINO / Bucket Indexes):**
   If the response body contains `var FILETREE_URL = '...'` or matches JSON filetree schemas, fetch `FILETREE_URL` and resolve the directory node corresponding to the URL's path.
2. **HTML Anchor Parsing (Apache, Nginx, Caddy, Python `http.server`):**
   If `Content-Type: text/html` and no JSON filetree is referenced, parse `<a href="...">` links. Filter out sort parameters (`?C=...`), parent directories (`../`), and anchors (`#`). Resolve relative links against `targetURL`.
3. **S3/GCS XML Bucket Parsing:**
   If `Content-Type` is XML or the root contains `<ListBucketResult>`, parse `<Contents><Key>` elements whose prefix matches the URL path.

### 4.2 Version Determination Heuristics
When synthesizing a release from an HTTP directory:
1. **Metadata file check:** If `version.txt` or `metadata.json` exists in the discovered files, parse the version string (e.g. `2026.5.0.dbe11fa27`).
2. **URL path token:** If the URL path ends with a version token (e.g. `.../releases/v1.4.0/` or `.../packages/weekly/latest/`), extract `v1.4.0` or `latest`.
3. **HTTP ETag / Last-Modified:** Fall back to the server's `Last-Modified` timestamp formatted as `YYYY.MM.DD`.

---

## 5. Target Case Study: Installing OpenVINO Model Server via `gh-pt`

### 5.1 System Context
- **Host OS:** Linux (Ubuntu 26.04.1 LTS, kernel x86_64)
- **Target URL:** `https://storage.openvinotoolkit.org/repositories/openvino_model_server/packages/weekly/latest/`
- **Matching Archive:** `ovms_ubuntu26.tar.gz` (or `ovms_ubuntu24.tar.gz` on Ubuntu 24.04)

### 5.2 Required `gh-pt install` Invocations

#### Standard Installation (Recommended)
```bash
gh-pt install \
  --http-url https://storage.openvinotoolkit.org/repositories/openvino_model_server/packages/weekly/latest/ \
  --repo openvino/model_server \
  --symlink \
  --include-sidecars \
  --driver auto
```

#### Explanation of Flags
1. `--http-url https://storage.openvinotoolkit.org/...`:
   Instructs `gh-pt` to ingest the package from the HTTP bucket index rather than querying GitHub API.
2. `--repo openvino/model_server`:
   Assigns a clean, canonical name in `state.json` and determines the package store directory (`~/.local/share/gh-pt/packages/openvino/model_server`). If omitted, `gh-pt` automatically synthesizes the repository name `openvinotoolkit/openvino_model_server` from the URL hostname and path.
3. `--symlink`:
   Mandatory for OVMS. Extracts the full `ovms/` archive structure (preserving `ovms/lib/` and `ovms/share/`) into the package store, then creates a symlink from `ovms/bin/ovms` into `~/.local/bin/ovms`. Because `ovms` has `RUNPATH: [$ORIGIN/../lib/]`, the binary discovers its companion libraries without modifying `LD_LIBRARY_PATH`. Note that specifying `--driver auto` or `--include-sidecars` automatically turns on `--symlink` via line 1845 of `release/release.go`.
4. `--include-sidecars` (`-s`):
   Captures companion runtime assets (`ovms/share/espeak-ng-data/`).
5. `--driver auto`:
   Inspects `ovms/lib/` for hardware driver manifests:
   - Registers Khronos OpenCL ICD driver (`libOpenCL.so.1`)
   - Registers userspace Intel GPU / NPU runtime plugins (`libopenvino_intel_gpu_plugin.so`, `libopenvino_intel_npu_vm_runtime.so`)
6. `--verify-checksum` (Default: `true`):
   Automatically downloads `ovms_ubuntu26.tar.gz.sha256` and verifies the SHA256 checksum before extracting.

### 5.3 Verification Matrix Post-Install

| Verification Step | Command | Expected Result |
| :--- | :--- | :--- |
| **Binary Symlink** | `ls -la ~/.local/bin/ovms` | Symlink pointing to `.../packages/openvino/model_server/ovms/bin/ovms` |
| **Dynamic Linking** | `ldd ~/.local/bin/ovms` | All shared libraries resolve to `.../ovms/lib/*.so` via `$ORIGIN` |
| **Execution Test** | `ovms --version` | Prints `OpenVINO Model Server 2026.5.0` |
| **State Tracking** | `gh-pt ls` | Displays `openvino/model_server` with version and paths |
| **Removal Test** | `gh-pt rm openvino/model_server` | Removes binary symlink and cleans package directory |

---

## 6. Implementation Architecture & Migration Seams Summary

```
File                     Seam Lines               Purpose
───────────────────────────────────────────────────────────────────────────────────────────
params/params.go         81-84, 271-274,          Add --repo and --http-url flags to commands
                         311-355, 381-386,        Mark positional args as optional:""
                         465-477, 540-583         Add HttpUrl & IsHttpSource to ExecContext

cmd/router.go            64-70, 79-84,            Route --repo and --http-url to ExecContext
                         85-111, 149-168          Set SpecificallyTargeted and command targets

cmd/root.go              200-228, 724-735,        Bypass GitHub REST client when HttpUrl != ""
                         792-799, 842-868         Derive repository name from URL if omitted

release/release.go       1554-1571, 2186-2197,    Add per-asset .sha256 pattern to findChecksumFile
                         2263-2275                Stream HTTP GET download when HttpUrl != ""

selector/interface.go    25-28, 54-138,           Implement SyntheticReleaseClient satisfying
                         181-250                  GithubClient for release & asset discovery

cmd/show.go              54-57, 94-98             Render asset table from HTTP synthetic release

cmd/update.go            52-92, 200-237           Filter HTTP-sourced apps from GitHub GraphQL
                                                  Check HTTP source for new version/mtime

cmd/state_mgmt.go        252-270                  Match target against app.HttpUrl in StateRm

state/state.go           14-58                    Persist HttpUrl and IsHttpSource in State v3
───────────────────────────────────────────────────────────────────────────────────────────
```

All proposed changes strictly preserve backward compatibility with existing GitHub repositories, existing `state.json` schemas, and GitHub CLI workflows.
