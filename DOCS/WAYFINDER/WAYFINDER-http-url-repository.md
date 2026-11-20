## Destination

Implement native support for `--repo` (named flag alternative to positional `<repository>`) and `--http-url` / `--http` (generic HTTP/HTTPS directory index, S3/GCS bucket, and OpenVINO SPA `filetree.json` ingestion) across `gh-pt install`, `gh-pt show`, `gh-pt upgrade`, and `gh-pt rm`, verified live by installing the OpenVINO Model Server weekly latest release on Ubuntu 26.04.

---

## Notes

- **Domain**: Go CLI (`gh-pt`), HTTP directory parsers, OpenVINO Model Server, S3 bucket XML, XDG state management.
- **Specification**: [`DOCS/SPECS/SPEC-http-url-repository.md`](../SPECS/SPEC-http-url-repository.md)
- **Skills**: `spec-driven-development`, `subagent-orchestrator`, `ponytail`, `tdd`, `wayfinder`.
- **Target Verification URL**: `https://storage.openvinotoolkit.org/repositories/openvino_model_server/packages/weekly/latest/`
- **Host Target**: Ubuntu 26.04.1 LTS (Resolute Raccoon), x86_64 -> selects `ovms_ubuntu26.tar.gz` with sidecars and userspace dynamic linker driver.

---

## Decisions so far

- **[ADR-006: HTTP/HTTPS Directory & Storage Bucket Repository Subsystem]**: Accepted — allow users to specify `--http-url` (`aliases:"http"`) as an alternative to GitHub repositories, bypassing GitHub API authentication, synthesizing virtual releases, and storing `HTTPURL` in `state.InstalledApp`.
- **OpenVINO SPA Detection**: Standardize on inspecting HTML body for `var FILETREE_URL = '...'` or `var BUCKET_URL = '...'` to seamlessly handle JavaScript-rendered storage buckets like `storage.openvinotoolkit.org`.

---

## Open Tickets

*(All wave tickets complete!)*

---

## Completed Tickets

- [x] **Ticket 1: CLI Flags & Mutual Exclusion (Seam 1)**
  - Made `<repository>` optional when `--repo` or `--http-url` provided.
  - Implemented `CommonRepoFlags` (`Repo`, `HTTPURL`) across `InstallCmd`, `Show`, `UpgradeCmd`, `ExecContext`.
  - Added mutual exclusion and GitHub API bypass in `cmd/root.go` (`needsGitHubAPI`).
  - Unit tests verified in `params/cli_integration_test.go` and `cmd/root_test.go`.

- [x] **Ticket 2: Multi-Format HTTP Directory & Bucket Parser (Seam 2)**
  - Implemented `release/http_repo.go` with `CleanHTTPSlug`, `HTTPReleaseIndex`, `FetchHTTPReleaseIndex`, and `HTTPReleaseClient`.
  - Implemented 3-stage cascade: OpenVINO SPA `filetree.json`, S3 XML `ListBucketResult`, Apache/Nginx HTML `<a href="...">` directory index.
  - Added unit test suite in `release/http_repo_test.go` with 12 mock scenarios covering all parsers and edge cases.

- [x] **Ticket 3: Virtual Release Integration & Live OpenVINO Verification (Seam 3)**
  - Implemented direct HTTP download streaming in `release/download.go` (`DownloadHTTPAsset`, `DownloadAsset`).
  - Connected `release/release.go` to invoke `FetchHTTPReleaseIndex` on `r.CliParams.HTTPURL != ""`.
  - Updated `state/state.go` and `release/release.go` to record `HTTPURL` in `state.InstalledApp`.
  - Integrated `cmd/show.go` with `HTTPReleaseClient` for viewing HTTP repo release details and assets.
  - Updated `cmd/update.go` to isolate HTTP-tracked apps from GitHub GraphQL batch queries.
  - Created integration test `release/release_http_test.go` with mock server and verified installation.
  - Built binary (`make build`) and verified live installation:
    `./gh-pt install --http-url https://storage.openvinotoolkit.org/repositories/openvino_model_server/packages/weekly/latest/ -s --driver=auto -u -y`
  - Verified `~/.local/bin/ovms --version` outputs `OpenVINO Model Server 2026.5.0.dbe11fa27`.
  - Verified OpenCL driver manifest registered in `~/.config/OpenCL/vendors/`.
  - Verified `state.json` persistence and `gh-pt ls openvino` display.

---

### Ticket 1: CLI Flags & Mutual Exclusion (Seam 1)
**Label:** `wayfinder:task` (AFK — Subagent)
**Question / Deliverable:**
1. In `params/params.go`:
   - Make positional `<repository>` argument `optional:""` in `InstallCmd` and `Show`.
   - Add `Repo` (`name:"repo" aliases:"repository"`) to `CommonRepoFlags`.
   - Add `HTTPURL` (`name:"http-url" aliases:"http"`) to `CommonRepoFlags`.
   - Embed `CommonRepoFlags` into `InstallCmd`, `Show`, `UpgradeCmd`, and `ExecContext`.
2. In `cmd/root.go`:
   - In `validateInputs`: enforce mutual exclusion between `--http-url` and GitHub repository. Assign `r.Repository = r.Repo` when positional is omitted.
   - In `needsGitHubAPI`: return `false` if `r.HTTPURL != ""`.
3. Write unit tests in `params/params_test.go` and `cmd/root_test.go` verifying:
   - Positional repo omitted when `--repo` or `--http-url` is passed.
   - Mutual exclusion triggers error when both are provided.
   - `needsGitHubAPI` evaluates to `false` for `--http-url`.

---

### Ticket 2: Multi-Format HTTP Directory & Bucket Parser (Seam 2)
**Label:** `wayfinder:task` (AFK — Subagent)
**Question / Deliverable:**
1. Create `release/http_repo.go`:
   - Implement `FetchHTTPReleaseIndex(targetURL string) (*HTTPReleaseIndex, error)`.
   - Implement cascading format detection:
     - Check 1: OpenVINO / SPA `filetree.json` pattern in HTML body (`var FILETREE_URL = '...'`).
     - Check 2: S3 / GCS bucket XML (`<ListBucketResult>`).
     - Check 3: Standard HTML `<a href="...">` directory index (Apache, Nginx, Python http.server).
   - Resolve relative URLs, prune parent directory links (`../`, `/`) and sort query links (`?C=M`).
   - Construct synthetic `SelectorItem`s with populated `DownloadURL`, `Name`, `Size`.
2. Create unit tests in `release/http_repo_test.go` using `httptest.Server`:
   - Test HTML directory listing parsing.
   - Test OpenVINO SPA landing HTML + `filetree.json` resolution.
   - Test S3 `ListBucketResult` XML parsing.
   - Test invalid URL and error handling.

---

### Ticket 3: Virtual Release Integration & Live OpenVINO Verification (Seam 3)
**Label:** `wayfinder:task` (AFK — Subagent & Orchestrator)
**Question / Deliverable:**
1. In `release/release.go`:
   - In `Install()`, if `r.CliParams.HTTPURL != ""`, invoke `FetchHTTPReleaseIndex()` instead of `selector.ReleaseSelector`.
   - Feed synthesized asset names directly into `selector.NewAssetClassifier()`.
   - In `DownloadAsset()`, download directly from `asset.DownloadURL` via standard HTTP client when set.
   - Store `HTTPURL` in `state.InstalledApp`.
2. In `state/state.go`:
   - Add `HTTPURL string` json field to `InstalledApp`.
3. In `cmd/router.go` and `cmd/update.go`:
   - Support `gh-pt show --http-url <url>`.
   - Support `gh-pt upgrade` re-checking `app.HTTPURL`.
4. Run live verification test:
   ```bash
   gh-pt install --http-url https://storage.openvinotoolkit.org/repositories/openvino_model_server/packages/weekly/latest/ -s --driver=auto -u -y
   ```
   Assert `ovms` and `convert_tokenizer` are installed in `~/.local/bin`, sidecars in `~/.local/share/gh-pt/packages/...`, and `gh-pt ls` shows the installed application.
