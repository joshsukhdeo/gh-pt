package release

import (
	"bufio"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"github.com/cli/go-gh/v2"
	"github.com/joshsukhdeo/gh-pt/config"
	"github.com/joshsukhdeo/gh-pt/params"
	"github.com/joshsukhdeo/gh-pt/resolver"
	"github.com/joshsukhdeo/gh-pt/selector"
	"github.com/joshsukhdeo/gh-pt/state"
	"github.com/joshsukhdeo/gh-pt/status"
	"github.com/joshsukhdeo/gh-pt/ui"
	"github.com/pterm/pterm"
	"github.com/rs/zerolog/log"
	"golang.org/x/term"
)

var (
	execCommand      = exec.Command
	ghExec           = gh.Exec
	getNativeManager = resolver.GetNativeManager
)

type GithubRelease struct {
	CliParams                *params.ExecContext
	Client                   selector.GithubClient
	ResolvedVersion          string
	InstalledPackageNames    []string
	InstalledBinaries        []string
	InstalledAssetFullNames  []string
	InstalledAssetCleanNames []string
	ContainingArchive        string
	PendingDebs              []string
	PendingRpms              []string
	Prompter                 Prompter
	StatusMessage            string
	UI                       *ui.PacmanUI
	Sidecars                 []string
	SidecarTargetPath        string
	SidecarSymlinkTo         []string
	InstalledSidecars        []string
	IsTTYFunc                func() bool
	WarnUnmappedAssets       *bool
}

type Prompter interface {
	Confirm(message string) bool
	Input(prompt string, defaultValue string) string
	Multiselect(prompt string, options []string) ([]string, error)
}

type PtermPrompter struct{}

func (p PtermPrompter) Confirm(message string) bool {
	result, _ := pterm.DefaultInteractiveConfirm.WithDefaultValue(true).Show(message)
	return result
}

func (p PtermPrompter) Input(prompt string, defaultValue string) string {
	result, _ := pterm.DefaultInteractiveTextInput.WithDefaultValue(defaultValue).Show(prompt)
	return result
}

func (p PtermPrompter) Multiselect(prompt string, options []string) ([]string, error) {
	return pterm.DefaultInteractiveMultiselect.WithOptions(options).Show(prompt)
}

func MakeGithubRelease(cliParams *params.ExecContext, cli selector.GithubClient) *GithubRelease {

	return &GithubRelease{
		CliParams: cliParams,
		Client:    cli,
		Prompter:  PtermPrompter{},
	}
}

func (r *GithubRelease) interactiveConfirm(prompt string) bool {
	if r.CliParams.DisablePrompts {
		return true
	}
	if r.Prompter == nil {
		r.Prompter = PtermPrompter{}
	}
	if r.UI != nil {
		r.UI.Pause()
		defer r.UI.Resume()
	}
	return r.Prompter.Confirm(prompt)
}

func (r *GithubRelease) interactiveInput(prompt string, defaultValue string) string {
	if r.CliParams.DisablePrompts {
		return defaultValue
	}
	if r.Prompter == nil {
		r.Prompter = PtermPrompter{}
	}
	if r.UI != nil {
		r.UI.Pause()
		defer r.UI.Resume()
	}
	return r.Prompter.Input(prompt, defaultValue)
}

func (r *GithubRelease) isTTY() bool {
	if r.IsTTYFunc != nil {
		return r.IsTTYFunc()
	}
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

func (r *GithubRelease) isInteractive() bool {
	if r.CliParams == nil {
		return false
	}
	if r.CliParams.DisablePrompts {
		return false
	}
	if !r.CliParams.Interactive {
		return false
	}
	return r.isTTY()
}

func (r *GithubRelease) interactiveMultiselect(prompt string, options []string) ([]string, error) {
	if r.CliParams != nil && r.CliParams.DisablePrompts {
		return nil, nil
	}
	if r.Prompter == nil {
		r.Prompter = PtermPrompter{}
	}
	if r.UI != nil {
		r.UI.Pause()
		defer r.UI.Resume()
	}
	return r.Prompter.Multiselect(prompt, options)
}

func (r *GithubRelease) shouldWarnUnmappedAssets() bool {
	if r.CliParams != nil {
		return r.CliParams.WarnUnmappedAssets
	}
	if r.WarnUnmappedAssets != nil {
		return *r.WarnUnmappedAssets
	}
	cfg, err := config.LoadConfig()
	if err != nil || cfg == nil {
		return true
	}
	return cfg.Core.WarnUnmappedAssets
}

func (r *GithubRelease) resolveSidecarTargetPath() string {
	if r.CliParams != nil && r.CliParams.SidecarTargetPath != "" {
		return r.CliParams.SidecarTargetPath
	}
	if r.SidecarTargetPath != "" {
		return r.SidecarTargetPath
	}
	cfg, _ := config.LoadConfig()
	if cfg != nil && cfg.Paths.SidecarPath != "" {
		if r.CliParams != nil {
			return filepath.Join(cfg.Paths.SidecarPath, r.CliParams.Repository)
		}
		return cfg.Paths.SidecarPath
	}
	if r.CliParams != nil {
		return filepath.Join(xdg.DataHome, "gh-pt", "sidecars", r.CliParams.Repository)
	}
	return filepath.Join(xdg.DataHome, "gh-pt", "sidecars")
}

func isSuspectedRemoteSidecar(name string) bool {
	lower := strings.ToLower(name)

	// Filter out source code bundles
	if strings.Contains(lower, "source") && (strings.HasSuffix(lower, ".zip") || strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz")) {
		return false
	}
	if strings.HasSuffix(lower, "-src.zip") || strings.HasSuffix(lower, "-src.tar.gz") || strings.HasSuffix(lower, ".src.tar.gz") {
		return false
	}

	// Filter out checksums
	if isChecksumFileName(lower) {
		return false
	}

	// Filter out foreign OS installers (.exe, .dmg, .pkg, .msi, .apk, .deb, .rpm)
	installerExts := []string{".exe", ".dmg", ".pkg", ".msi", ".apk", ".deb", ".rpm"}
	for _, ext := range installerExts {
		if strings.HasSuffix(lower, ext) {
			return false
		}
	}

	// Flag assets containing keywords (plugin, data, model, asset) or extensions (.pak, .bin, .red, .so)
	keywords := []string{"plugin", "data", "model", "asset"}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}

	sidecarExts := []string{".pak", ".bin", ".red", ".so"}
	for _, ext := range sidecarExts {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}

	return false
}

func isChecksumFileName(lower string) bool {
	checksumPatterns := []string{
		`(?i)checksums?\.txt$`,
		`(?i)sha256sums?\.txt$`,
		`(?i)sha512sums?\.txt$`,
		`(?i)checksums?$`,
		`(?i)\.sha256$`,
		`(?i)\.sha512$`,
		`(?i)\.md5$`,
		`(?i)\.sig$`,
		`(?i)\.asc$`,
	}
	for _, pat := range checksumPatterns {
		if matched, _ := regexp.MatchString(pat, lower); matched {
			return true
		}
	}
	return false
}

func isSuspectedLocalSidecar(relPath string, fileName string) bool {
	lowerPath := strings.ToLower(relPath)
	lowerName := strings.ToLower(fileName)

	// Filter out standard bloat: README*, LICENSE*, .md, doc/, src/
	if strings.HasPrefix(lowerName, "readme") ||
		strings.HasPrefix(lowerName, "license") ||
		strings.HasPrefix(lowerName, "licence") ||
		strings.HasPrefix(lowerName, "copying") {
		return false
	}

	if strings.HasSuffix(lowerName, ".md") ||
		strings.HasSuffix(lowerName, ".txt") ||
		strings.HasSuffix(lowerName, ".rst") ||
		strings.HasSuffix(lowerName, ".rtf") ||
		strings.HasSuffix(lowerName, ".html") {
		return false
	}

	cleanPath := filepath.ToSlash(lowerPath)
	if strings.HasPrefix(cleanPath, "doc/") || strings.Contains(cleanPath, "/doc/") ||
		strings.HasPrefix(cleanPath, "docs/") || strings.Contains(cleanPath, "/docs/") ||
		strings.HasPrefix(cleanPath, "src/") || strings.Contains(cleanPath, "/src/") {
		return false
	}

	if strings.HasPrefix(lowerName, ".") {
		return false
	}

	// Flag shared libraries (.so, .dll, .dylib)
	if strings.HasSuffix(lowerName, ".so") ||
		strings.Contains(lowerName, ".so.") ||
		strings.HasSuffix(lowerName, ".dll") ||
		strings.HasSuffix(lowerName, ".dylib") {
		return true
	}

	// Flag config templates (.json, .yaml, .yml)
	if strings.HasSuffix(lowerName, ".json") ||
		strings.HasSuffix(lowerName, ".yaml") ||
		strings.HasSuffix(lowerName, ".yml") {
		return true
	}

	// Flag domain-specific plugins (.pak, .bin, .red, .dat or keyword "plugin")
	if strings.HasSuffix(lowerName, ".pak") ||
		strings.HasSuffix(lowerName, ".bin") ||
		strings.HasSuffix(lowerName, ".red") ||
		strings.HasSuffix(lowerName, ".dat") ||
		strings.Contains(lowerName, "plugin") {
		return true
	}

	return false
}

func (r *GithubRelease) handleSuspectedSidecars(suspected []string, extractDir string, fsObj fs.FS, releaseID int) ([]string, error) {
	if len(suspected) == 0 {
		return nil, nil
	}
	if !r.shouldWarnUnmappedAssets() {
		return nil, nil
	}

	if !r.isInteractive() {
		pterm.Warning.Printf("Release contains unmapped sidecar assets (%s). Pass --sidecars to capture them on future installs.\n", strings.Join(suspected, ", "))
		return nil, nil
	}

	selected, err := r.interactiveMultiselect("Suspected sidecar assets detected. Select items to deploy:", suspected)
	if err != nil || len(selected) == 0 {
		return nil, err
	}

	targetDir := r.resolveSidecarTargetPath()
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		log.Warn().Err(err).Msg("could not create sidecar target directory")
	}
	r.SidecarTargetPath = targetDir

	for _, item := range selected {
		r.Sidecars = append(r.Sidecars, item)

		// Check if local file in extractDir
		if extractDir != "" {
			src := filepath.Join(extractDir, item)
			if fi, err := os.Stat(src); err == nil && !fi.IsDir() {
				dst := filepath.Join(targetDir, filepath.Base(item))
				if err := copyFile(src, dst, 0755); err == nil {
					r.InstalledSidecars = append(r.InstalledSidecars, dst)
					continue
				}
			}
		}

		// Check if in fsObj
		if fsObj != nil {
			if sf, err := fsObj.Open(item); err == nil {
				dst := filepath.Join(targetDir, filepath.Base(item))
				if df, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755); err == nil {
					_, _ = io.Copy(df, sf)
					_ = df.Close()
					r.InstalledSidecars = append(r.InstalledSidecars, dst)
				}
				_ = sf.Close()
				continue
			}
		}

		// Otherwise, remote asset
		version := r.ResolvedVersion
		if version == "" && r.CliParams != nil {
			version = r.CliParams.ReleaseAsset
		}
		if r.CliParams != nil && version != "" {
			_, _, err := ghExec("release", "download", version, "--repo", r.CliParams.Repository, "--pattern", item, "--dir", targetDir)
			if err == nil {
				r.InstalledSidecars = append(r.InstalledSidecars, filepath.Join(targetDir, item))
			} else {
				log.Warn().Err(err).Str("asset", item).Msg("failed to download remote sidecar asset")
			}
		}
	}

	return selected, nil
}

func (r *GithubRelease) extractExplicitSidecars(extractDir string, fsObj fs.FS) error {
	if len(r.CliParams.Sidecars) == 0 {
		return nil
	}

	targetDir := r.resolveSidecarTargetPath()
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create sidecar target directory: %w", err)
	}
	r.SidecarTargetPath = targetDir

	var matched []string

	if extractDir != "" {
		err := filepath.Walk(extractDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(extractDir, path)
			for _, pattern := range r.CliParams.Sidecars {
				if ok, _ := filepath.Match(pattern, rel); ok {
					matched = append(matched, rel)
					break
				}
			}
			return nil
		})
		if err != nil {
			log.Warn().Err(err).Msg("error walking extraction directory for sidecars")
		}
	} else if fsObj != nil {
		err := fs.WalkDir(fsObj, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			for _, pattern := range r.CliParams.Sidecars {
				if ok, _ := filepath.Match(pattern, path); ok {
					matched = append(matched, path)
					break
				}
			}
			return nil
		})
		if err != nil {
			log.Warn().Err(err).Msg("error walking fs.FS for sidecars")
		}
	}

	for _, rel := range matched {
		var src io.ReadCloser
		if extractDir != "" {
			f, err := os.Open(filepath.Join(extractDir, rel))
			if err != nil {
				log.Warn().Err(err).Str("sidecar", rel).Msg("failed to open sidecar file")
				continue
			}
			src = f
		} else if fsObj != nil {
			f, err := fsObj.Open(rel)
			if err != nil {
				log.Warn().Err(err).Str("sidecar", rel).Msg("failed to open sidecar from fs.FS")
				continue
			}
			src = f
		}
		if src == nil {
			continue
		}

		dst := filepath.Join(targetDir, filepath.Base(rel))
		df, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			_ = src.Close()
			log.Warn().Err(err).Str("sidecar", rel).Msg("failed to create sidecar destination")
			continue
		}
		_, _ = io.Copy(df, src)
		_ = df.Close()
		_ = src.Close()
		r.InstalledSidecars = append(r.InstalledSidecars, dst)
		log.Info().Str("sidecar", rel).Str("destination", dst).Msg("deployed sidecar asset")
	}

	if len(matched) == 0 {
		log.Warn().Strs("patterns", r.CliParams.Sidecars).Msg("no sidecar assets matched the specified patterns")
	} else {
		log.Info().Int("count", len(matched)).Msg("deployed sidecar assets")
	}

	// Create symlinks if requested
	if len(r.CliParams.SidecarSymlinkTo) > 0 {
		r.createSidecarSymlinks()
	}

	return nil
}

func (r *GithubRelease) createSidecarSymlinks() {
	if r.SidecarTargetPath == "" || len(r.InstalledSidecars) == 0 {
		return
	}

	for _, targetDir := range r.CliParams.SidecarSymlinkTo {
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			log.Warn().Err(err).Str("dir", targetDir).Msg("failed to create symlink target directory")
			continue
		}

		for _, sidecarPath := range r.InstalledSidecars {
			sidecarName := filepath.Base(sidecarPath)
			linkPath := filepath.Join(targetDir, sidecarName)

			// Remove existing symlink if it exists
			if _, err := os.Lstat(linkPath); err == nil {
				if err := os.Remove(linkPath); err != nil {
					log.Warn().Err(err).Str("link", linkPath).Msg("failed to remove existing symlink")
					continue
				}
			}

			if err := os.Symlink(sidecarPath, linkPath); err != nil {
				log.Warn().Err(err).Str("sidecar", sidecarPath).Str("link", linkPath).Msg("failed to create symlink")
			} else {
				log.Info().Str("sidecar", sidecarName).Str("link", linkPath).Msg("created symlink")
			}
		}
	}
}

func (r *GithubRelease) runAISidecarSetup() error {
	if !r.CliParams.AISetupSidecars || len(r.InstalledSidecars) == 0 {
		return nil
	}

	// Get AI command template from config or use default
	cfg, _ := config.LoadConfig()
	aiCmdTemplate := "agy -p \"%s\""
	if cfg != nil && cfg.AI.AICmd != "" {
		aiCmdTemplate = cfg.AI.AICmd
	}

	// Build prompt with sidecar information
	sidecarList := strings.Join(r.InstalledSidecars, "\n")
	prompt := fmt.Sprintf(`Analyze the following sidecar files that were installed for %s and provide setup instructions:

%s

These files are located in: %s

Please provide:
1. What these files appear to be (plugins, libraries, data files, etc.)
2. Recommended environment variables to set (e.g., PLUGIN_DIR, DATA_PATH)
3. Suggested symlinks or configuration file modifications needed
4. Any additional setup steps required for the application to find these files

Format your response as actionable shell commands where possible.`, r.CliParams.Repository, sidecarList, r.SidecarTargetPath)

	log.Info().Msg("Initiating AI sidecar setup analysis...")
	
	// Execute AI agent
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell", "-Command", aiCmdTemplate, prompt)
	} else {
		cmd = exec.Command("sh", "-c", fmt.Sprintf(aiCmdTemplate, prompt))
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("AI sidecar setup failed: %w", err)
	}

	return nil
}

func (r *GithubRelease) resolveDestinationPath(binaryPath string) string {
	binaryName := filepath.Base(binaryPath)
	destinationPath := filepath.Join(r.CliParams.TargetPath, binaryName)

	if targetBinaryName, exists := r.CliParams.Rename[strings.ToLower(binaryName)]; exists {
		return filepath.Join(r.CliParams.TargetPath, targetBinaryName)
	}

	if !r.CliParams.KeepSuffixes {
		repoParts := strings.Split(r.CliParams.Repository, "/")
		repoName := repoParts[len(repoParts)-1]

		proposedName := GenerateCleanName(binaryName, repoName, r.ResolvedVersion)

		if extMatch := regexp.MustCompile(`(?i)(\.[a-z][a-z0-9]*)$`).FindStringSubmatch(binaryName); len(extMatch) > 0 {
			ext := extMatch[1]
			if !strings.HasSuffix(proposedName, ext) {
				proposedName += ext
			}
		}

		if len(binaryName) > len(proposedName) {
			if r.CliParams.Rename == nil {
				r.CliParams.Rename = make(map[string]string)
			}
			r.CliParams.Rename[strings.ToLower(binaryName)] = proposedName
			destinationPath = filepath.Join(r.CliParams.TargetPath, proposedName)
		}
	}

	if r.CliParams.Interactive && !r.CliParams.DisablePrompts {
		return r.interactiveInput(fmt.Sprintf("Install %s to", binaryPath), destinationPath)
	}

	return destinationPath
}

func (r *GithubRelease) installArchivedBinary(fileSystem fs.FS, binaryPath string) error {
	if r.CliParams.DryRun {
		log.Info().Msgf("[dry-run] Would extract and install: %s", binaryPath)
		return nil
	}

	tempExtractDir, err := os.MkdirTemp("", "gh-pt-extract-*")
	if err != nil {
		return err
	}
	defer func() {
		_ = os.RemoveAll(tempExtractDir)
	}()

	if err := copyFS(fileSystem, tempExtractDir); err != nil {
		return fmt.Errorf("failed to extract archive: %w", err)
	}

	extractedBinaryPath := filepath.Join(tempExtractDir, binaryPath)
	if _, err := os.Stat(extractedBinaryPath); err != nil {
		foundPath := ""
		_ = filepath.WalkDir(tempExtractDir, func(p string, d fs.DirEntry, e error) error {
			if e == nil && !d.IsDir() && (d.Name() == binaryPath || d.Name() == filepath.Base(binaryPath)) {
				foundPath = p
				return fs.SkipAll
			}
			return nil
		})
		if foundPath != "" {
			extractedBinaryPath = foundPath
		}
	}

	if err := r.resolveBinaryDependencies(extractedBinaryPath, tempExtractDir); err != nil {
		log.Warn().Err(err).Msg("dependency resolution encountered an error")
	}

	destinationPath := r.resolveDestinationPath(binaryPath)
	r.InstalledBinaries = append(r.InstalledBinaries, filepath.Base(destinationPath))

	log.Info().
		Msgf("will install %s to %s", binaryPath, destinationPath)

	_, err = os.Stat(destinationPath)
	if err == nil {
		if r.CliParams.Interactive {
			if !r.interactiveConfirm(fmt.Sprintf("'%s' already exists. Overwrite?", destinationPath)) {
				return fmt.Errorf("%s already exists and user did not want to overwrite", destinationPath)
			}
		} else {
			if !r.CliParams.Overwrite {
				return fmt.Errorf("%s already exists and -f/--force is not set", destinationPath)
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	sourceFile, err := os.Open(extractedBinaryPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = sourceFile.Close()
	}()

	destinationFile, err := os.Create(destinationPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = destinationFile.Close()
	}()

	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		return err
	}

	err = os.Chmod(destinationPath, 0755)
	if err != nil {
		return err
	}

	return nil
}

// findBundledSharedObjects recursively sweeps dir for shared object files (.so, .so.*).
// It returns all matching file paths and the unique parent directories containing .so files.
func findBundledSharedObjects(dir string) ([]string, []string, error) {
	var soFiles []string
	var soDirs []string
	seenDirs := make(map[string]bool)

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if strings.HasSuffix(name, ".so") || strings.Contains(name, ".so.") {
			soFiles = append(soFiles, path)
			dirPath := filepath.Dir(path)
			if !seenDirs[dirPath] {
				seenDirs[dirPath] = true
				soDirs = append(soDirs, dirPath)
			}
		}
		return nil
	})
	return soFiles, soDirs, err
}

// parseMissingLibraries parses the output of `ldd` and captures all library names marked "=> not found".
func parseMissingLibraries(lddOutput string) []string {
	var missing []string
	seen := make(map[string]bool)

	scanner := bufio.NewScanner(strings.NewReader(lddOutput))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.Contains(line, "=> not found") {
			parts := strings.Split(line, "=>")
			libName := strings.TrimSpace(parts[0])
			if libName != "" && !seen[libName] {
				seen[libName] = true
				missing = append(missing, libName)
			}
		}
	}
	return missing
}

// mapSharedObjectToPackage maps a missing shared library requirement to candidate package name(s)
// for the given package manager ("apt", "dnf", "pacman", etc.).
func mapSharedObjectToPackage(mgrName string, soName string) string {
	switch {
	case strings.HasPrefix(soName, "libz.so"):
		if mgrName == "apt" {
			return "zlib1g"
		}
		return "zlib"
	case strings.HasPrefix(soName, "libssl.so.3"), strings.HasPrefix(soName, "libcrypto.so.3"):
		if mgrName == "apt" {
			return "libssl3"
		}
		return "openssl-libs"
	case strings.HasPrefix(soName, "libssl.so.1.1"), strings.HasPrefix(soName, "libcrypto.so.1.1"):
		if mgrName == "apt" {
			return "libssl1.1"
		}
		return "openssl1.1"
	case strings.HasPrefix(soName, "libfuse.so.2"):
		return "libfuse2"
	case strings.HasPrefix(soName, "libfuse3.so"):
		if mgrName == "apt" {
			return "libfuse3-3"
		}
		return "fuse3-libs"
	case strings.HasPrefix(soName, "libcurl.so.4"):
		if mgrName == "apt" {
			return "libcurl4"
		}
		return "libcurl"
	case strings.HasPrefix(soName, "libstdc++.so.6"):
		if mgrName == "apt" {
			return "libstdc++6"
		}
		return "libstdc++"
	}

	if mgrName == "dnf" {
		return soName
	}

	re := regexp.MustCompile(`^(lib[a-zA-Z0-9_\-+]+)\.so(?:\.([0-9]+))?`)
	matches := re.FindStringSubmatch(soName)
	if len(matches) > 1 {
		prefix := matches[1]
		ver := ""
		if len(matches) > 2 {
			ver = matches[2]
		}
		if ver != "" {
			return prefix + ver
		}
		return prefix
	}

	return soName
}

// scanMissingDependencies executes `ldd <binary>` with LD_LIBRARY_PATH temporarily containing extractDir
// and any bundled .so directories, capturing missing shared object dependencies ("=> not found").
func scanMissingDependencies(binaryPath string, extractDir string) ([]string, error) {
	if runtime.GOOS != "linux" {
		return nil, nil
	}

	if _, err := exec.LookPath("ldd"); err != nil {
		return nil, nil
	}

	_, soDirs, err := findBundledSharedObjects(extractDir)
	if err != nil {
		log.Warn().Err(err).Str("extractDir", extractDir).Msg("failed to sweep for bundled .so files")
	}

	ldPaths := []string{extractDir}
	ldPaths = append(ldPaths, soDirs...)
	if currentLd := os.Getenv("LD_LIBRARY_PATH"); currentLd != "" {
		ldPaths = append(ldPaths, currentLd)
	}
	injectedLdPath := strings.Join(ldPaths, string(os.PathListSeparator))

	origLd, hasOrigLd := os.LookupEnv("LD_LIBRARY_PATH")
	_ = os.Setenv("LD_LIBRARY_PATH", injectedLdPath)
	defer func() {
		if hasOrigLd {
			_ = os.Setenv("LD_LIBRARY_PATH", origLd)
		} else {
			_ = os.Unsetenv("LD_LIBRARY_PATH")
		}
	}()

	cmd := execCommand("ldd", binaryPath)
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	cmd.Env = append(cmd.Env, "LD_LIBRARY_PATH="+injectedLdPath)

	out, err := cmd.CombinedOutput()
	if err != nil {
		outStr := string(out)
		if strings.Contains(outStr, "not a dynamic executable") || strings.Contains(outStr, "statically linked") {
			return nil, nil
		}
		log.Debug().Err(err).Str("binary", binaryPath).Str("output", outStr).Msg("ldd scan returned non-zero")
	}

	return parseMissingLibraries(string(out)), nil
}

// resolveBinaryDependencies scans the binary for missing shared library dependencies using ldd
// and installs them via the detected package manager when ResolveDeps is enabled.
func (r *GithubRelease) resolveBinaryDependencies(binaryPath string, extractDir string) error {
	if r.CliParams.NoDeps {
		return nil
	}

	missingLibs, err := scanMissingDependencies(binaryPath, extractDir)
	if err != nil {
		log.Warn().Err(err).Msg("failed to scan binary dependencies via ldd")
		return nil
	}

	if len(missingLibs) == 0 {
		return nil
	}

	log.Info().
		Strs("missing_libraries", missingLibs).
		Msg("detected missing shared object dependencies")

	if !r.CliParams.ResolveDeps {
		log.Warn().
			Strs("missing_libraries", missingLibs).
			Msg("missing shared library dependencies detected; run with --resolve-deps to install them automatically")
		return nil
	}

	mgr, err := getNativeManager()
	if err != nil {
		log.Warn().Err(err).Msg("could not detect native package manager to resolve dependencies")
		return err
	}

	var packagesToInstall []string
	seen := make(map[string]bool)
	for _, lib := range missingLibs {
		pkg := mapSharedObjectToPackage(mgr.Name(), lib)
		if pkg != "" && !seen[pkg] {
			seen[pkg] = true
			packagesToInstall = append(packagesToInstall, pkg)
		}
	}

	if len(packagesToInstall) == 0 {
		return nil
	}

	log.Info().
		Str("manager", mgr.Name()).
		Strs("packages", packagesToInstall).
		Msg("installing missing dependencies via package manager")

	if r.CliParams.PromptDeps && !r.CliParams.DisablePrompts {
		if !r.interactiveConfirm(fmt.Sprintf("Install missing dependencies: %s?", strings.Join(packagesToInstall, ", "))) {
			log.Info().Msg("skipping dependency installation per user choice")
			return nil
		}
	}

	if err := mgr.Install(packagesToInstall); err != nil {
		log.Error().Err(err).Strs("packages", packagesToInstall).Msg("failed to install dependencies")
		return err
	}

	r.InstalledPackageNames = append(r.InstalledPackageNames, packagesToInstall...)
	return nil
}

func (r *GithubRelease) installBinary(binaryPath string) error {
	if r.CliParams.DryRun {
		destinationPath := r.resolveDestinationPath(binaryPath)
		r.InstalledBinaries = append(r.InstalledBinaries, filepath.Base(destinationPath))
		log.Info().Msgf("[dry-run] Would install binary: %s to %s", binaryPath, destinationPath)
		return nil
	}
	sourceStat, err := os.Stat(binaryPath)
	if err != nil {
		return err
	}

	if !sourceStat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", binaryPath)
	}

	if err := r.resolveBinaryDependencies(binaryPath, filepath.Dir(binaryPath)); err != nil {
		log.Warn().Err(err).Msg("dependency resolution encountered an error")
	}

	source, err := os.Open(binaryPath)
	if err != nil {
		return err
	}

	defer func() {
		err = errors.Join(err, source.Close())
	}()

	destinationPath := r.resolveDestinationPath(binaryPath)
	r.InstalledBinaries = append(r.InstalledBinaries, filepath.Base(destinationPath))

	log.Info().
		Msgf("will install %s to %s", binaryPath, destinationPath)

	_, err = os.Stat(destinationPath)
	if err == nil {
		if r.CliParams.Interactive {
			if !r.interactiveConfirm(fmt.Sprintf("'%s' already exists. Overwrite?", destinationPath)) {
				return fmt.Errorf("%s already exists and user did not want to overwrite", destinationPath)
			}
		} else {
			if !r.CliParams.Overwrite {
				return fmt.Errorf("%s already exists and -f/--force is not set", destinationPath)
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	destination, err := os.Create(destinationPath)
	if err != nil {
		if os.IsPermission(err) {
			log.Info().Msg("permission denied, attempting to install with sudo")
			if err := r.ensureSudo(); err != nil {
				return err
			}
			if r.CliParams.Interactive {
				if !r.interactiveConfirm(fmt.Sprintf("Run 'sudo install -m 755 %s %s'?", binaryPath, destinationPath)) {
					return fmt.Errorf("permission denied and user aborted sudo installation")
				}
			}
			cmd := execCommand("sudo", "install", "-m", "755", binaryPath, destinationPath)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("sudo install failed: %w", err)
			}
			return nil
		}
		return err
	}

	defer func() {
		err = errors.Join(err, destination.Close())
	}()

	_, err = io.Copy(destination, source)
	if err != nil {
		return err
	}

	err = os.Chmod(destinationPath, 0755)
	if err != nil {
		return err
	}

	return nil
}

func (r *GithubRelease) ensureSudo() error {
	check := execCommand("sudo", "-n", "true")
	if err := check.Run(); err != nil {
		if !r.CliParams.Interactive {
			return fmt.Errorf("sudo session expired or unavailable; cannot prompt for password in headless mode (-D). Run 'sudo -v' beforehand or use an interactive session")
		}
		log.Warn().Msg("sudo session not cached; you may be prompted for your password")
	}
	return nil
}

// extractPackageName queries the package name from a local package file.
func extractPackageName(binaryPath string, pkgType string) string {
	var cmd *exec.Cmd
	switch pkgType {
	case "deb":
		cmd = exec.Command("dpkg-deb", "--field", binaryPath, "Package")
	case "rpm":
		cmd = exec.Command("rpm", "-qp", binaryPath, "--queryformat", "%{NAME}")
	case "pacman":
		cmd = exec.Command("pacman", "-Qp", "--noconfirm", binaryPath)
	default:
		return ""
	}
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	name := strings.TrimSpace(string(out))
	// pacman outputs "name version", take just the name
	if pkgType == "pacman" {
		parts := strings.Fields(name)
		if len(parts) > 0 {
			return parts[0]
		}
	}
	return name
}

func (r *GithubRelease) installDeb(binaryPath string) error {
	if r.CliParams.DryRun {
		log.Info().Msgf("[dry-run] Would queue deb for batch install: %s", filepath.Base(binaryPath))
		return nil
	}
	log.Info().Msgf("Queuing %s for batch installation...", filepath.Base(binaryPath))
	r.PendingDebs = append(r.PendingDebs, binaryPath)
	return nil
}

func (r *GithubRelease) installRpm(binaryPath string) error {
	if r.CliParams.DryRun {
		log.Info().Msgf("[dry-run] Would queue rpm for batch install: %s", filepath.Base(binaryPath))
		return nil
	}
	log.Info().Msgf("Queuing %s for batch installation...", filepath.Base(binaryPath))
	r.PendingRpms = append(r.PendingRpms, binaryPath)
	return nil
}

func getScore(name string, types []string) int {
	name = strings.ToLower(name)
	score := -1
	for i, t := range types {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "none" {
			if !strings.Contains(filepath.Base(name), ".") {
				score = (len(types) - i) * 10
				break
			}
		} else if t != "" {
			matched, _ := regexp.MatchString(`(?i)\.`+t+`$`, name)
			if matched {
				score = (len(types) - i) * 10
				break
			}
		}
	}

	if score == -1 {
		for i, t := range types {
			if strings.ToLower(strings.TrimSpace(t)) == "none" {
				score = (len(types) - i) * 10
				break
			}
		}
	}

	if score != -1 && runtime.GOOS == "linux" {
		isMusl, _ := regexp.MatchString(`[-_]musl[-_.]`, name)
		if isMusl {
			score -= 5
		} else {
			isGlibc, _ := regexp.MatchString(`[-_](?:glibc|gnu)[-_.]`, name)
			if isGlibc {
				score += 2
			}
		}
	}

	return score
}

// findChecksumFile looks for a checksum file in the release assets.
// Returns the asset name if found, empty string otherwise.
func (r *GithubRelease) findChecksumFile(assets []*selector.SelectorItem) string {
	// Common checksum file patterns
	patterns := []string{
		`(?i)checksums?\.txt$`,
		`(?i)sha256sums?\.txt$`,
		`(?i)sha512sums?\.txt$`,
		`(?i)checksums?$`,
	}

	for _, asset := range assets {
		for _, pattern := range patterns {
			if matched, _ := regexp.MatchString(pattern, asset.Name); matched {
				return asset.Name
			}
		}
	}
	return ""
}

// verifyChecksum verifies the checksum of a downloaded file against a checksum file.
func (r *GithubRelease) verifyChecksum(filePath, checksumFilePath string) error {
	// Read the checksum file
	file, err := os.Open(checksumFilePath)
	if err != nil {
		return fmt.Errorf("failed to open checksum file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Parse checksum file - format is typically: <hash>  <filename> or <hash> <filename>
	scanner := bufio.NewScanner(file)
	expectedHash := ""
	targetFilename := filepath.Base(filePath)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split by whitespace - checksum files use "hash  filename" or "hash filename"
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			hashValue := parts[0]
			filename := parts[len(parts)-1]

			// Remove leading * or ./ from filename (binary mode indicator)
			filename = strings.TrimPrefix(filename, "*")
			filename = strings.TrimPrefix(filename, "./")

			if strings.EqualFold(filename, targetFilename) {
				expectedHash = hashValue
				break
			}
		}
	}

	if expectedHash == "" {
		log.Warn().
			Str("filename", targetFilename).
			Msg("no checksum entry found for file, skipping verification")
		return nil
	}

	// Determine hash algorithm based on hash length
	var hasher hash.Hash
	switch len(expectedHash) {
	case 64: // SHA-256
		hasher = sha256.New()
	case 128: // SHA-512
		hasher = sha512.New()
	default:
		log.Warn().
			Int("hash_length", len(expectedHash)).
			Msg("unknown hash algorithm, skipping verification")
		return nil
	}

	// Calculate the hash of the downloaded file
	downloadedFile, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open downloaded file: %w", err)
	}
	defer func() { _ = downloadedFile.Close() }()

	if _, err := io.Copy(hasher, downloadedFile); err != nil {
		return fmt.Errorf("failed to calculate hash: %w", err)
	}

	actualHash := hex.EncodeToString(hasher.Sum(nil))

	if !strings.EqualFold(actualHash, expectedHash) {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	log.Info().
		Str("filename", targetFilename).
		Str("hash", actualHash).
		Msg("checksum verified successfully")

	return nil
}

func (r *GithubRelease) installWindows(binaryPath string) error {
	if r.CliParams.DryRun {
		log.Info().Msgf("[dry-run] Would install windows pkg: %s", binaryPath)
		return nil
	}
	var args []string
	if strings.HasSuffix(strings.ToLower(binaryPath), ".msi") {
		args = []string{"/i", binaryPath, "/qn"}
		if runtime.GOOS != "windows" && (r.CliParams.Wine == "allow" || r.CliParams.Wine == "priority" || r.CliParams.Wine == "force") {
			args = append([]string{"msiexec"}, args...)
		} else {
			args = append([]string{"msiexec"}, args...)
		}
	} else {
		args = []string{"/S"}
		if runtime.GOOS != "windows" && (r.CliParams.Wine == "allow" || r.CliParams.Wine == "priority" || r.CliParams.Wine == "force") {
			args = append([]string{binaryPath}, args...)
			args = append([]string{"wine"}, args...)
		} else {
			args = append([]string{binaryPath}, args...)
		}
	}

	cmd := execCommand(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		log.Error().Err(err).Msgf("Failed to install Windows package")
		return err
	}
	log.Info().Msg("Successfully installed Windows package!")
	return nil
}

func (r *GithubRelease) installMac(binaryPath string) error {
	if r.CliParams.DryRun {
		log.Info().Msgf("[dry-run] Would install mac pkg: %s", binaryPath)
		return nil
	}
	if strings.HasSuffix(strings.ToLower(binaryPath), ".dmg") {
		log.Info().Msg("Mounting DMG...")
		cmd := execCommand("hdiutil", "attach", binaryPath, "-nobrowse", "-mountpoint", "/Volumes/gh-pt-dmg")
		if err := cmd.Run(); err != nil {
			return err
		}
		defer func() { _ = execCommand("hdiutil", "detach", "/Volumes/gh-pt-dmg").Run() }()

		cpCmd := execCommand("sh", "-c", fmt.Sprintf("cp -R /Volumes/gh-pt-dmg/*.app %s/", r.CliParams.TargetPath))
		if err := cpCmd.Run(); err != nil {
			return err
		}
		log.Info().Msg("Successfully copied app from DMG!")
		return nil
	} else if strings.HasSuffix(strings.ToLower(binaryPath), ".pkg") {
		if err := r.ensureSudo(); err != nil {
			return err
		}
		cmd := execCommand("sudo", "installer", "-pkg", binaryPath, "-target", "/")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
		log.Info().Msg("Successfully installed macOS pkg!")
		return nil
	}
	return fmt.Errorf("unsupported mac installer format: %s", binaryPath)
}

func (r *GithubRelease) GetLatestRelease() (*selector.SelectorItem, error) {
	var prerelease, stable bool
	if r.CliParams != nil {
		prerelease = r.CliParams.Prerelease
		stable = r.CliParams.Stable
	}
	releaseSelector, err := selector.ReleaseSelector(r.Client, r.CliParams.Repository, r.CliParams.ReleaseVersion, r.CliParams.Interactive, prerelease, stable)
	if err != nil {
		return nil, err
	}
	releases, err := releaseSelector.Run()
	if err != nil {
		return nil, err
	}
	return releases[0], nil
}

func (r *GithubRelease) Install() error {
	pUI := ui.NewPacmanUI(r.CliParams.Repository)
	if r.CliParams.DisableIcons {
		pUI.DisableIcons = true
	}
	r.UI = pUI
	ui.GlobalPacman = pUI
	pUI.Update(0, "", "", "", r.CliParams.TargetPath, "")
	pUI.Start()
	defer func() {
		if pUI != nil {
			pUI.Stop()
		}
	}()

	// Auto-enable IncludeSidecars when any sidecar param is specified
	if !r.CliParams.IncludeSidecars {
		if len(r.CliParams.Sidecars) > 0 || r.CliParams.SidecarTargetPath != "" ||
			len(r.CliParams.SidecarSymlinkTo) > 0 || r.CliParams.AISetupSidecars {
			r.CliParams.IncludeSidecars = true
			log.Debug().Msg("auto-enabled --include-sidecars due to --sidecar-* params")
		}
	}

	var prerelease, stable bool
	if r.CliParams != nil {
		prerelease = r.CliParams.Prerelease
		stable = r.CliParams.Stable
	}
	releaseSelector, err := selector.ReleaseSelector(r.Client, r.CliParams.Repository, r.CliParams.ReleaseVersion, r.CliParams.Interactive, prerelease, stable)
	if err != nil {
		log.Error().
			Str("repository", r.CliParams.Repository).
			Str("release version", r.CliParams.ReleaseVersion).
			Err(err).
			Msg("could not create release selector")
		return err
	}
	releases, err := releaseSelector.Run()
	if err != nil {
		log.Error().
			Str("repository", r.CliParams.Repository).
			Str("release version", r.CliParams.ReleaseVersion).
			Err(err).
			Msg("could not select a release")
		return err
	}

	// Try each release up to FallbackReleases times if no assets found
	var assets []*selector.SelectorItem
	var selectedRelease *selector.SelectorItem
	maxAttempts := 1
	if r.CliParams.FallbackReleases > 0 {
		maxAttempts = r.CliParams.FallbackReleases + 1
	}
	if maxAttempts > len(releases) {
		maxAttempts = len(releases)
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		release := releases[attempt]
		r.ResolvedVersion = release.Name
		if pUI != nil {
			pUI.Update(1, r.ResolvedVersion, "", "", "", "")
		}

		assetSelector, err := selector.AssetSelector(r.Client, r.CliParams.Repository, selector.AssetMatchCriteria{
			ReleaseId:        release.Id,
			Name:             r.CliParams.ReleaseAsset,
			Regexps:          r.CliParams.ReleaseAssetRegexps,
			Interactive:      r.CliParams.Interactive,
			AllowForeignArch: r.CliParams.AllowForeignArch,
		})
		if err != nil {
			log.Error().
				Str("repository", r.CliParams.Repository).
				Int("release id", release.Id).
				Str("release name", release.Name).
				Str("asset name matcher", r.CliParams.ReleaseAsset).
				Err(err).
				Msg("could not create release asset selector")
			return err
		}
		assets, err = assetSelector.Run()
		if err == nil {
			selectedRelease = release
			if attempt > 0 {
				log.Info().
					Str("repository", r.CliParams.Repository).
					Str("release", release.Name).
					Int("attempt", attempt+1).
					Msg("found assets in older release")
			}
			break
		}

		if attempt < maxAttempts-1 {
			log.Warn().
				Str("repository", r.CliParams.Repository).
				Str("release", release.Name).
				Err(err).
				Msg("no assets found, trying older release")
		}
	}

	if assets == nil || selectedRelease == nil {
		if r.CliParams.SearchForInstallInstructionsIfNoReleaseAssets {
			r.fallbackToReadmeInstructions()
		}
		log.Error().
			Str("repository", r.CliParams.Repository).
			Str("release asset name matcher", r.CliParams.ReleaseAsset).
			Int("releases_tried", maxAttempts).
			Msg("could not select release asset after trying multiple releases")
		return fmt.Errorf("no matching assets found in %d releases for %s", maxAttempts, r.CliParams.Repository)
	}

	// Update releases to use the selected release
	releases = []*selector.SelectorItem{selectedRelease}

	// --- STATUS ABORTION CHECK ---
	st, _ := state.LoadState()
	var inState bool
	var prevVersion string
	var alreadyInstalled bool
	if st != nil && st.Apps != nil {
		if app, ok := st.Apps[r.CliParams.Repository]; ok {
			inState = true
			prevVersion = app.Version
			if app.TargetPath != "" {
				// Assume already installed if target path exists and it's in state.
				// We do a fast stat on the directory or binary if we know it.
				// For simplicity, checking if the path exists:
				if _, err := os.Stat(app.TargetPath); err == nil {
					alreadyInstalled = true
				}
			}
		}
	}

	installState := status.InstallState{
		InState:           inState,
		AlreadyInstalled:  alreadyInstalled,
		PrevVersion:       prevVersion,
		NewVersion:        releases[0].Name,
		AppName:           r.CliParams.Repository,
		Type:              strings.Join(r.CliParams.Type, ","),
		Repo:              r.CliParams.Repository,
		AssetName:         assets[0].Name,
		Force:             r.CliParams.Overwrite,
		AllowDowngrade:    r.CliParams.AllowDowngrade,
		LeRetrogrouch:     r.CliParams.LeRetrogrouch,
		RetrogradeStopgap: r.CliParams.RetrogradeStopgap,
		Barbarous:         r.CliParams.Barbarous,
		SelfInflictedDebt: r.CliParams.SelfInflictedDebt,
		IsUpgradeCmd:      r.CliParams.IsUpgradeCmd,
	}

	statusMsg, abortErr := status.GenerateStatusMessage(installState)
	if abortErr != nil {
		return abortErr
	}
	r.StatusMessage = statusMsg // We need to add StatusMessage to GithubRelease struct

	// Automatically allow overwriting for version upgrades
	if installState.InState && status.CompareVersions(installState.PrevVersion, installState.NewVersion) > 0 {
		r.CliParams.Overwrite = true
	}

	if pUI != nil {
		ghostType := "🍒"
		if alreadyInstalled {
			if r.CliParams.Overwrite {
				ghostType = "\033[5m👻\033[0m" // flashing ghost
			} else {
				ghostType = "ᗣ" // solid ghost
			}
		}
		pUI.Update(2, "", assets[0].Name, "", "", ghostType)
	}
	// -----------------------------

	sort.Slice(assets, func(i, j int) bool {
		scoreI := getScore(assets[i].Name, r.CliParams.Type)
		scoreJ := getScore(assets[j].Name, r.CliParams.Type)
		return scoreI > scoreJ
	})

	// Filter r.CliParams.Type down to only the formats that actually matched our chosen assets.
	// This ensures that when the state is saved, future updates will strictly seek this exact format.
	var matchedTypes []string
	for _, asset := range assets {
		for _, t := range r.CliParams.Type {
			originalT := t
			tLower := strings.ToLower(strings.TrimSpace(t))
			if tLower == "none" {
				if !strings.Contains(filepath.Base(asset.Name), ".") {
					matchedTypes = append(matchedTypes, originalT)
					break
				}
			} else if tLower != "" {
				if matched, _ := regexp.MatchString(`(?i)\.`+tLower+`$`, asset.Name); matched {
					matchedTypes = append(matchedTypes, originalT)
					break
				}
			}
		}
	}

	if len(matchedTypes) > 0 {
		seen := make(map[string]bool)
		var finalTypes []string
		for _, t := range matchedTypes {
			if !seen[t] {
				seen[t] = true
				finalTypes = append(finalTypes, t)
			}
		}
		r.CliParams.Type = finalTypes
	}

	// Generate strict regexes for the chosen assets to lock them down for future updates
	var strictRegexes []string
	for _, asset := range assets {
		strictRegex := generateStrictAssetRegex(asset.Name, releases[0].Name)
		strictRegexes = append(strictRegexes, strictRegex)
	}
	r.CliParams.ReleaseAssetRegexp = strings.Join(strictRegexes, " | ")

	downloadDir, err := os.MkdirTemp("", "*")
	if err != nil {
		log.Error().
			Err(err).
			Msg("could not create temporary download directory")
		return err
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(downloadDir))
	}()

	// Find and download checksum file if available
	var checksumFilePath string
	if r.CliParams.VerifyChecksum {
		// Get all release assets to find checksum file
		allAssetsResponse := []struct{ Name string }{}
		if err := r.Client.Get(fmt.Sprintf("repos/%s/releases/%d/assets", r.CliParams.Repository, releases[0].Id), &allAssetsResponse); err == nil {
			var allAssets []*selector.SelectorItem
			for _, a := range allAssetsResponse {
				allAssets = append(allAssets, &selector.SelectorItem{Name: a.Name})
			}

			checksumFileName := r.findChecksumFile(allAssets)
			if checksumFileName != "" {
				log.Info().
					Str("checksum_file", checksumFileName).
					Msg("found checksum file, downloading")

				_, _, execErr := ghExec("release", "download", releases[0].Name,
					"--repo", r.CliParams.Repository, "--pattern", checksumFileName, "--dir", downloadDir)
				if execErr != nil {
					log.Warn().
						Err(execErr).
						Msg("failed to download checksum file, skipping verification")
				} else {
					checksumFilePath = filepath.Join(downloadDir, checksumFileName)
				}
			}
		}
	}

	for _, asset := range assets {
		if r.CliParams.Interactive {
			if pUI != nil {
				pUI.Update(3, "", "", "", "", "")
			}
		}

		stdOut, stdErr, execErr := ghExec("release", "download", releases[0].Name,
			"--repo", r.CliParams.Repository, "--pattern", asset.Name, "--dir", downloadDir)
		if execErr != nil {
			execErr = fmt.Errorf("failed to run gh command: %s", stdErr.String())
			log.Error().
				Str("repository", r.CliParams.Repository).
				Int("release id", releases[0].Id).
				Str("release name", releases[0].Name).
				Str("release asset name", asset.Name).
				Str("download directory", downloadDir).
				Err(execErr).
				Msg("could not download release asset")
			if r.CliParams.Interactive {
				if pUI != nil {
					pUI.Stop()
					fmt.Printf("\nFailed - '%s' failed\n", stdErr.String())
				}
			}
			return execErr
		}

		if r.CliParams.Interactive {
			if pUI != nil {
				pUI.Update(4, "", "", "", "", "")
			}
		}

		log.Info().
			Str("repository", r.CliParams.Repository).
			Int("release id", releases[0].Id).
			Str("release name", releases[0].Name).
			Str("release asset name", asset.Name).
			Str("download directory", downloadDir).
			Str("output", stdOut.String()).
			Msg("downloaded release asset")

		// Verify checksum if available
		downloadedAssetPath := filepath.Join(downloadDir, asset.Name)
		if checksumFilePath != "" && r.CliParams.VerifyChecksum {
			if err := r.verifyChecksum(downloadedAssetPath, checksumFilePath); err != nil {
				log.Error().
					Err(err).
					Str("asset", asset.Name).
					Msg("checksum verification failed")
				return fmt.Errorf("checksum verification failed for %s: %w", asset.Name, err)
			}
		}

		if r.CliParams.VTApiKey != "" {
			vtHash, hashErr := CalculateSHA256(downloadedAssetPath)
			if hashErr != nil {
				return fmt.Errorf("failed to calculate SHA-256 for VirusTotal: %w", hashErr)
			}
			err := VerifyHashWithVirusTotal(vtHash, downloadedAssetPath, r.CliParams.VTApiKey, r.CliParams.Interactive && !r.CliParams.DisablePrompts, r.CliParams.SkipVtSandbox)
			if err != nil {
				return err
			}
		}

		binarySelector, execErr := selector.BinarySelector(selector.BinaryMatchCriteria{
			DownloadPath: filepath.Join(downloadDir, asset.Name),
			Names:        r.CliParams.AssetBinaries,
			Matcher:      r.CliParams.AssetBinariesRegexp,
			Interactive:  r.CliParams.Interactive,
			Extractor:    r.CliParams.Extractor,
			Repository:   r.CliParams.Repository,
		})
		if execErr != nil {
			log.Error().
				Str("repository", r.CliParams.Repository).
				Int("release id", releases[0].Id).
				Str("release name", releases[0].Name).
				Str("release asset name", asset.Name).
				Str("downloaded asset", filepath.Join(downloadDir, asset.Name)).
				Strs("asset binary name matchers", r.CliParams.AssetBinaries).
				Str("asset binary regexp matcher", r.CliParams.AssetBinariesRegexp).
				Err(execErr).
				Msg("could not create release asset binary selector")
			return execErr
		}
		binaries, execErr := binarySelector.Run()
		if execErr != nil {
			log.Error().
				Str("repository", r.CliParams.Repository).
				Int("release id", releases[0].Id).
				Str("release name", releases[0].Name).
				Str("release asset name", asset.Name).
				Str("downloaded asset", filepath.Join(downloadDir, asset.Name)).
				Strs("asset binary name matchers", r.CliParams.AssetBinaries).
				Str("asset binary regexp matcher", r.CliParams.AssetBinariesRegexp).
				Err(execErr).
				Msg("could not select release asset binary")
			return execErr
		}

		if r.CliParams.Interactive && pUI != nil {
			var bNames []string
			for _, b := range binaries {
				bNames = append(bNames, b.Name)
			}
			pUI.Update(4, "", "", strings.Join(bNames, ", "), "", "")
		}

		// Section F: Orphaned Asset Heuristics & Interactive Prompting
		var suspectedSidecars []string
		suspectedSet := make(map[string]bool)

		// 1. Scan unselected GitHub release assets
		var allReleaseAssets []struct{ Name string }
		if err := r.Client.Get(fmt.Sprintf("repos/%s/releases/%d/assets", r.CliParams.Repository, releases[0].Id), &allReleaseAssets); err == nil {
			selectedAssetMap := make(map[string]bool)
			for _, a := range assets {
				selectedAssetMap[a.Name] = true
			}
			for _, a := range allReleaseAssets {
				if !selectedAssetMap[a.Name] && isSuspectedRemoteSidecar(a.Name) {
					if !suspectedSet[a.Name] {
						suspectedSet[a.Name] = true
						suspectedSidecars = append(suspectedSidecars, a.Name)
					}
				}
			}
		}

		// 2. Scan discarded files in local extraction directory
		var extractDir string
		var fsObj fs.FS
		if len(binaries) > 0 {
			extractDir = binaries[0].ExtractDir
			fsObj = binaries[0].Fs

			selectedBinaries := make(map[string]bool)
			for _, b := range binaries {
				selectedBinaries[filepath.Clean(b.DownloadPath)] = true
				selectedBinaries[b.Name] = true
			}

			if extractDir != "" {
				_ = filepath.Walk(extractDir, func(path string, info os.FileInfo, err error) error {
					if err != nil || info.IsDir() {
						return nil
					}
					if selectedBinaries[filepath.Clean(path)] || selectedBinaries[info.Name()] {
						return nil
					}
					rel, err := filepath.Rel(extractDir, path)
					if err != nil {
						rel = info.Name()
					}
					if isSuspectedLocalSidecar(rel, info.Name()) {
						if !suspectedSet[rel] {
							suspectedSet[rel] = true
							suspectedSidecars = append(suspectedSidecars, rel)
						}
					}
					return nil
				})
			} else if fsObj != nil {
				_ = fs.WalkDir(fsObj, ".", func(fsPath string, d fs.DirEntry, err error) error {
					if err != nil || d.IsDir() {
						return nil
					}
					if selectedBinaries[fsPath] || selectedBinaries[d.Name()] {
						return nil
					}
					if isSuspectedLocalSidecar(fsPath, d.Name()) {
						if !suspectedSet[fsPath] {
							suspectedSet[fsPath] = true
							suspectedSidecars = append(suspectedSidecars, fsPath)
						}
					}
					return nil
				})
			}
		}

		if len(suspectedSidecars) > 0 {
			// If IncludeSidecars is enabled, auto-include all suspected sidecars
			if r.CliParams.IncludeSidecars {
				// Wipe old sidecars on upgrade
				if r.CliParams.IsUpgradeCmd && r.SidecarTargetPath != "" {
					if err := os.RemoveAll(r.SidecarTargetPath); err != nil {
						log.Warn().Err(err).Msg("failed to purge old sidecars")
					}
				}
				targetDir := r.resolveSidecarTargetPath()
				if err := os.MkdirAll(targetDir, 0755); err != nil {
					log.Warn().Err(err).Msg("could not create sidecar target directory")
				}
				r.SidecarTargetPath = targetDir

				for _, item := range suspectedSidecars {
					r.Sidecars = append(r.Sidecars, item)
					// Deploy the sidecar
					if extractDir != "" {
						src := filepath.Join(extractDir, item)
						if fi, err := os.Stat(src); err == nil && !fi.IsDir() {
							dst := filepath.Join(targetDir, filepath.Base(item))
							if err := copyFile(src, dst, 0755); err == nil {
								r.InstalledSidecars = append(r.InstalledSidecars, dst)
								continue
							}
						}
					}
					if fsObj != nil {
						if sf, err := fsObj.Open(item); err == nil {
							dst := filepath.Join(targetDir, filepath.Base(item))
							if df, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755); err == nil {
								_, _ = io.Copy(df, sf)
								_ = df.Close()
								r.InstalledSidecars = append(r.InstalledSidecars, dst)
							}
							_ = sf.Close()
							continue
						}
					}
				}
				log.Info().Int("count", len(suspectedSidecars)).Msg("auto-included suspected sidecar assets")
			} else {
				// Otherwise, use the existing heuristic handling (warn or prompt)
				_, _ = r.handleSuspectedSidecars(suspectedSidecars, extractDir, fsObj, releases[0].Id)
			}
		}

		// Explicit --sidecars pattern matching
		if len(r.CliParams.Sidecars) > 0 {
			// Wipe old sidecars on upgrade
			if r.CliParams.IsUpgradeCmd && r.SidecarTargetPath != "" {
				if err := os.RemoveAll(r.SidecarTargetPath); err != nil {
					log.Warn().Err(err).Msg("failed to purge old sidecars")
				}
			}
			_ = r.extractExplicitSidecars(extractDir, fsObj)
			
			// Run AI setup if requested
			if r.CliParams.AISetupSidecars {
				if err := r.runAISidecarSetup(); err != nil {
					log.Warn().Err(err).Msg("AI sidecar setup failed")
				}
			}
		}

		binariesOutput := make(map[string]string)

		if r.CliParams.Symlink {
			symlinkDir, err := r.executeSymlinkInstall(binaries, filepath.Join(downloadDir, asset.Name))
			if err != nil {
				return err
			}
			if pUI != nil {
				pUI.UpdateSymlink(symlinkDir)
				pUI.Update(6, "", "", "", "", "")
			}
			continue
		}

		for _, binary := range binaries {
			log.Info().
				Str("repository", r.CliParams.Repository).
				Int("release id", releases[0].Id).
				Str("release name", releases[0].Name).
				Str("release asset name", asset.Name).
				Str("downloaded asset", filepath.Join(downloadDir, asset.Name)).
				Str("release asset binary", binary.Name).
				Msg("processing selected release asset binary")

			actualDownloadPath := binary.DownloadPath

			if binary.Compressed && binary.BinaryType != selector.BinaryExecutable {
				log.Debug().
					Str("release asset binary", binary.Name).
					Msg("extracting archived installer to temporary directory")

				actualDownloadPath = filepath.Join(downloadDir, binary.Name)
				sourceFile, openErr := binary.Fs.Open(binary.FsPath)
				if openErr != nil {
					return openErr
				}
				destFile, createErr := os.Create(actualDownloadPath)
				if createErr != nil {
					_ = sourceFile.Close()
					return createErr
				}
				_, copyErr := io.Copy(destFile, sourceFile)
				_ = destFile.Close()
				_ = sourceFile.Close()
				if copyErr != nil {
					return copyErr
				}
			}

			switch binary.BinaryType {
			case selector.BinaryDebInstaller:
				log.Debug().Msg("binary is a deb installer")
				binariesOutput[binary.Name] = "deb"
				execErr = r.installDeb(actualDownloadPath)
			case selector.BinaryRpmInstaller:
				log.Debug().Msg("binary is a rpm installer")
				binariesOutput[binary.Name] = "rpm"
				execErr = r.installRpm(actualDownloadPath)
			case selector.BinaryPacmanInstaller:
				log.Debug().Msg("binary is a pacman installer")
				binariesOutput[binary.Name] = "pacman"
				execErr = r.installPacman(actualDownloadPath)
			case selector.BinaryPkgInstaller:
				log.Debug().Msg("binary is a freebsd pkg/txz installer")
				binariesOutput[binary.Name] = "pkg"
				execErr = r.installPkg(actualDownloadPath)
			case selector.BinaryMacInstaller:
				log.Debug().Msg("binary is a mac installer")
				binariesOutput[binary.Name] = "mac"
				execErr = r.installMac(actualDownloadPath)
			case selector.BinaryWindowsInstaller:
				log.Debug().Msg("binary is a windows installer")
				binariesOutput[binary.Name] = "windows"
				execErr = r.installWindows(actualDownloadPath)
			default:
				log.Debug().Msg("binary is a plain executable")
				binariesOutput[binary.Name] = "binary"
				if binary.Compressed {
					execErr = r.installArchivedBinary(binary.Fs, binary.FsPath)
				} else {
					execErr = r.installBinary(actualDownloadPath)
				}
			}

			if execErr != nil {
				log.Error().
					Str("repository", r.CliParams.Repository).
					Int("release id", releases[0].Id).
					Str("release name", releases[0].Name).
					Str("release asset name", asset.Name).
					Str("downloaded asset", filepath.Join(downloadDir, asset.Name)).
					Str("release asset binary", binary.Name).
					Err(execErr).
					Msg("could not install release asset binary")
				return execErr
			}

			// Track asset information for state
			r.InstalledAssetFullNames = append(r.InstalledAssetFullNames, asset.Name)
			cleanName := GenerateCleanName(binary.Name, r.CliParams.Repository, releases[0].Name)
			r.InstalledAssetCleanNames = append(r.InstalledAssetCleanNames, cleanName)
			if binary.Compressed {
				r.ContainingArchive = asset.Name
			}
		}

		if !r.CliParams.All {
			break
		}
	}

	if len(r.PendingDebs) > 0 {
		if err := r.ensureSudo(); err != nil {
			return err
		}
		for _, p := range r.PendingDebs {
			if name := extractPackageName(p, "deb"); name != "" {
				r.InstalledPackageNames = append(r.InstalledPackageNames, name)
			}
		}
		var baseDebs []string
		for _, p := range r.PendingDebs {
			baseDebs = append(baseDebs, "./"+filepath.Base(p))
		}
		var args []string
		if r.CliParams.NoDeps {
			args = append([]string{"dpkg", "-i"}, baseDebs...)
		} else if r.CliParams.ResolveDeps {
			args = append([]string{"apt-get", "install", "-y"}, baseDebs...)
		} else {
			args = append([]string{"apt-get", "install"}, baseDebs...)
		}

		if r.CliParams.Interactive && !r.CliParams.DisablePrompts {
			if !r.interactiveConfirm(fmt.Sprintf("Run 'sudo %s'?", strings.Join(args, " "))) {
				return fmt.Errorf("user aborted batched .deb installation")
			}
		}

		cmd := execCommand("sudo", args...)
		cmd.Dir = filepath.Dir(r.PendingDebs[0])
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		log.Info().Msgf("Executing batched package install: sudo %s", strings.Join(args, " "))
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("batched .deb installation failed: %w", err)
		}
	}

	if len(r.PendingRpms) > 0 {
		if err := r.ensureSudo(); err != nil {
			return err
		}
		for _, p := range r.PendingRpms {
			if name := extractPackageName(p, "rpm"); name != "" {
				r.InstalledPackageNames = append(r.InstalledPackageNames, name)
			}
		}
		var baseRpms []string
		for _, p := range r.PendingRpms {
			baseRpms = append(baseRpms, "./"+filepath.Base(p))
		}
		var args []string
		if r.CliParams.NoDeps {
			args = append([]string{"rpm", "-i"}, baseRpms...)
		} else if r.CliParams.ResolveDeps {
			args = append([]string{"dnf", "localinstall", "-y"}, baseRpms...)
		} else {
			args = append([]string{"dnf", "localinstall"}, baseRpms...)
		}

		if r.CliParams.Interactive && !r.CliParams.DisablePrompts {
			if !r.interactiveConfirm(fmt.Sprintf("Run 'sudo %s'?", strings.Join(args, " "))) {
				return fmt.Errorf("user aborted batched .rpm installation")
			}
		}

		cmd := execCommand("sudo", args...)
		cmd.Dir = filepath.Dir(r.PendingRpms[0])
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		log.Info().Msgf("Executing batched package install: sudo %s", strings.Join(args, " "))
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("batched .rpm installation failed: %w", err)
		}
	}

	if pUI != nil {
		pUI.Update(5, "", "", "", "", "")
		time.Sleep(500 * time.Millisecond) // Let the animation finish eating dots
	}

	if !r.CliParams.NoSaveState && !r.CliParams.DryRun {
		st, err := state.LoadState()
		if err == nil {
			_ = st.AddApp(&state.InstalledApp{
				Repository:               r.CliParams.Repository,
				TargetPath:               r.CliParams.TargetPath,
				Global:                   r.CliParams.Global,
				ReleaseAsset:             r.CliParams.ReleaseAsset,
				ReleaseRegexp:            r.CliParams.ReleaseAssetRegexp,
				Version:                  releases[0].Name,
				Rename:                   r.CliParams.Rename,
				Type:                     r.CliParams.Type,
				All:                      r.CliParams.All,
				AssetBinaries:            r.CliParams.AssetBinaries,
				AssetBinariesRegexp:      r.CliParams.AssetBinariesRegexp,
				InstalledBinaries:        r.InstalledBinaries,
				InstalledAssetNames:      r.InstalledAssetCleanNames,
				InstalledAssetsFullNames: r.InstalledAssetFullNames,
				ContainingArchive:        r.ContainingArchive,
				PackageNames:             r.InstalledPackageNames,
				Pinned:                   r.CliParams.PinInstall,
				Extractor:                r.CliParams.Extractor,
				IsPrerelease:             releases[0].Prerelease,
				Sidecars:                 r.Sidecars,
				SidecarTargetPath:        r.SidecarTargetPath,
				SidecarSymlinkTo:         r.SidecarSymlinkTo,
				IncludeSidecars:          r.CliParams.IncludeSidecars,
				InstalledSidecars:        r.InstalledSidecars,
				FallbackReleases:         r.CliParams.FallbackReleases,
			})
		} else {
			log.Warn().Err(err).Msg("could not save installed app state")
		}
	}

	if r.StatusMessage != "" {
		fmt.Printf("\n\033[1;32m%s\033[0m\n", r.StatusMessage)
	} else {
		repoName := r.CliParams.Repository
		if parts := strings.Split(repoName, "/"); len(parts) == 2 {
			repoName = parts[1]
		}
		fmt.Printf("\n\033[1;32mInstalled %s successfully!\033[0m\n", repoName)
	}

	if r.CliParams == nil || (!r.CliParams.Update && !r.CliParams.UpdateAll) {
		repo := ""
		if r.CliParams != nil {
			repo = r.CliParams.Repository
		}
		state.LogHistory("install", repo, releases[0].Name)
	}

	return nil
}

func (r *GithubRelease) installPkg(binaryPath string) error {
	basePath := "./" + filepath.Base(binaryPath)
	if r.CliParams.DryRun {
		log.Info().Msgf("[dry-run] Would install freebsd pkg: %s", filepath.Base(binaryPath))
		return nil
	}
	if err := r.ensureSudo(); err != nil {
		return err
	}
	var args []string
	if r.CliParams.NoDeps {
		args = []string{"pkg", "add", basePath}
	} else if r.CliParams.ResolveDeps {
		args = []string{"pkg", "install", "-y", basePath}
	} else {
		args = []string{"pkg", "install", basePath}
	}

	if r.CliParams.Interactive {
		if !r.interactiveConfirm(fmt.Sprintf("Run 'sudo %s'?", strings.Join(args, " "))) {
			return fmt.Errorf("'%s' is a FreeBSD PKG installer and user did not want to run it", filepath.Base(binaryPath))
		}
	}

	cmd := execCommand("sudo", args...)
	cmd.Dir = filepath.Dir(binaryPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		log.Error().
			Str("installer binary", binaryPath).
			Err(err).
			Msgf("'sudo %s' failed", strings.Join(args, " "))
		if r.CliParams.Interactive {
			pterm.Error.Println("Failed to install FreeBSD package")
		}
		return err
	}
	log.Info().
		Str("installer binary", binaryPath).
		Msgf("ran 'sudo %s'", strings.Join(args, " "))
	if r.CliParams.Interactive {
		pterm.Success.Println("Successfully installed FreeBSD package!")
	}
	return nil
}

func (r *GithubRelease) installPacman(binaryPath string) error {
	if r.CliParams.DryRun {
		log.Info().Msgf("[dry-run] Would install pacman pkg: %s", filepath.Base(binaryPath))
		return nil
	}
	if err := r.ensureSudo(); err != nil {
		return err
	}
	if name := extractPackageName(binaryPath, "pacman"); name != "" {
		r.InstalledPackageNames = append(r.InstalledPackageNames, name)
	}
	log.Debug().
		Str("binaryPath", binaryPath).
		Msg("installing pacman package")

	var cmd *exec.Cmd
	basePath := "./" + filepath.Base(binaryPath)
	if r.CliParams.ResolveDeps {
		cmd = execCommand("sudo", "pacman", "-U", "--noconfirm", basePath)
	} else if r.CliParams.NoDeps {
		cmd = execCommand("sudo", "pacman", "-U", "--nodeps", "--noconfirm", basePath)
	} else {
		cmd = execCommand("sudo", "pacman", "-U", basePath)
	}
	cmd.Dir = filepath.Dir(binaryPath)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func generateStrictAssetRegex(assetName string, resolvedVersion string) string {
	if resolvedVersion == "" {
		return fmt.Sprintf("^%s$", regexp.QuoteMeta(assetName))
	}

	noVStr := regexp.QuoteMeta(strings.TrimPrefix(strings.ToLower(resolvedVersion), "v"))

	// We optionally consume up to 2 digits of a package revision (e.g. -1 to -99).
	versionRegex := regexp.MustCompile(fmt.Sprintf(`(?i)v?%s(?:-\d{1,2})?`, noVStr))
	matches := versionRegex.FindAllStringIndex(assetName, -1)

	if len(matches) == 0 {
		return fmt.Sprintf("^%s$", regexp.QuoteMeta(assetName))
	}

	// Manual lookahead to prevent partial consumption of longer digit sequences (e.g. -386 architecture).
	// If the character immediately following our match is a digit, we backtrack and strictly match the version only.
	for i, match := range matches {
		matchStr := assetName[match[0]:match[1]]
		// If we actually matched a suffix (length > basic version)
		if len(matchStr) > len(noVStr) && match[1] < len(assetName) {
			nextChar := assetName[match[1]]
			if nextChar >= '0' && nextChar <= '9' {
				// Backtrack match to exclude the -\d+ suffix
				baseRegex := regexp.MustCompile(fmt.Sprintf(`(?i)v?%s`, noVStr))
				baseMatch := baseRegex.FindStringIndex(assetName[match[0]:])
				if baseMatch != nil {
					matches[i][1] = match[0] + baseMatch[1]
				}
			}
		}
	}

	var parts []string
	lastIdx := 0
	for _, match := range matches {
		parts = append(parts, regexp.QuoteMeta(assetName[lastIdx:match[0]]))
		lastIdx = match[1]
	}
	parts = append(parts, regexp.QuoteMeta(assetName[lastIdx:]))

	return fmt.Sprintf("^%s$", strings.Join(parts, ".*"))
}

func (r *GithubRelease) fallbackToReadmeInstructions() {
	var readmeData struct {
		Content string `json:"content"`
	}
	err := r.Client.Get(fmt.Sprintf("repos/%s/readme", r.CliParams.Repository), &readmeData)
	if err != nil {
		return
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(readmeData.Content, "\n", ""))
	if err != nil {
		return
	}

	content := string(decoded)
	lines := strings.Split(content, "\n")

	headerRe := regexp.MustCompile(`(?i)^(\#{1,6})\s*(?:quick\s*start|install(?:ation)?|setup)`)
	anyHeaderRe := regexp.MustCompile(`^(\#{1,6})\s`)
	cmdRe := regexp.MustCompile(`^\s*(pipx|uv tool|pnpm|npm|snap|cargo|go install|curl)\b`)

	inInstallSection := false
	headerLevel := 0
	var capturedLines []string

	for _, line := range lines {
		if !inInstallSection {
			matches := headerRe.FindStringSubmatch(line)
			if len(matches) > 0 {
				inInstallSection = true
				headerLevel = len(matches[1])
			}
			continue
		}

		matches := anyHeaderRe.FindStringSubmatch(line)
		if len(matches) > 0 {
			level := len(matches[1])
			if level <= headerLevel {
				break
			}
		}
		capturedLines = append(capturedLines, line)
	}

	if len(capturedLines) == 0 {
		return
	}

	inCodeBlock := false
	for _, line := range capturedLines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			if cmdRe.MatchString(line) {
				if r.UI != nil {
					r.UI.Stop()
				}
				pterm.Info.Println("No compatible release assets found. Found alternative installation method in README:")
				fmt.Println(strings.TrimSpace(line))
				os.Exit(0)
			}
		}
	}
}
