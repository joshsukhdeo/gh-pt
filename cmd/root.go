package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/adrg/xdg"
	"github.com/alecthomas/kong"
	"github.com/cli/go-gh/v2"
	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/joshsukhdeo/gh-pt/config"
	"github.com/joshsukhdeo/gh-pt/params"
	"github.com/joshsukhdeo/gh-pt/release"
	"github.com/joshsukhdeo/gh-pt/state"
	"github.com/pterm/pterm"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

type RootCLI struct {
	params.ExecContext
}

const (
	GH_INSTALL_PREFIX_ENV           = "GH_INSTALL_ENV_PREFIX"
	GH_INSTALL_DEFAULT_PREFIX       = "GH_INSTALL"
	GH_INSTALL_CHECKSUM_ASSET_REGEX = ".*(?:checksum|txt)+.*$"
)

func (r *RootCLI) Validate() error {
	if runtime.GOOS == "windows" && r.ExecContext.CommonInstallFlags.Wine != "off" && r.ExecContext.CommonInstallFlags.Wine != "" {
		pterm.Warning.Println("Wine is not supported on Windows. Continuing with wine disabled.")
		r.ExecContext.CommonInstallFlags.Wine = "off"
	}

	if !r.ExecContext.Update && !r.ExecContext.UpdateAll && r.ExecContext.Ls == "" && r.ExecContext.Ll == "" && !r.ExecContext.EditSavedState && r.ExecContext.RmSavedState == "" && r.ExecContext.Rm == "" && r.ExecContext.Purge == "" && r.ExecContext.Pin == "" {
		match, _ := regexp.MatchString(`.+/.+`, r.ExecContext.Repository)
		if !match {
			return fmt.Errorf("repository must be in 'user/repository' format (provided: '%s')", r.ExecContext.Repository)
		}
	}

	if r.ExecContext.CompileFromSource && !r.ExecContext.AI {
		return fmt.Errorf("--compile-from-source can only be used with --ai")
	}

	if r.ExecContext.Clone || r.ExecContext.Fork || r.ExecContext.CompileFromSource || r.ExecContext.Show || r.ExecContext.ShowAssets > -1 || r.ExecContext.ShowVersions > -1 || r.ExecContext.ShowDescription > -1 || r.ExecContext.ShowReadme > -1 {
		return nil
	}

	// Detect root user and handle global install path
	if os.Getuid() == 0 {
		if r.ExecContext.CommonInstallFlags.Global {
			// Root + global: ensure we're using /usr/local/bin
			if r.ExecContext.CommonInstallFlags.TargetPath == GetDefaultTargetPath() {
				r.ExecContext.CommonInstallFlags.TargetPath = "/usr/local/bin"
			}
		} else if !r.ExecContext.CommonInstallFlags.AllowRootUserInstall {
			// Root without global flag and without explicit permission
			err := fmt.Errorf("running as root without --global flag. Use --global for system-wide install or --allow-root-user-install to install to user-local paths")
			log.Error().
				Err(err).
				Msg("init error")
			return err
		}
	}

	if r.ExecContext.CommonInstallFlags.TargetPath == "" {
		err := fmt.Errorf("could not determine default install path, use '--install-path' flag")
		log.Error().
			Err(err).
			Msg("init error")
		return err
	}

	targetPathInfo, err := os.Stat(r.ExecContext.CommonInstallFlags.TargetPath)
	if err != nil {
		if !os.IsNotExist(err) {
			createPath := r.ExecContext.CommonInstallFlags.TargetPathCreate
			if r.ExecContext.CommonInstallFlags.Interactive {
				createPath, _ = pterm.DefaultInteractiveConfirm.
					WithDefaultValue(true).
					Show(fmt.Sprintf("'%s' does not exist. Create?", r.ExecContext.CommonInstallFlags.TargetPath))
			}

			if createPath {
				err := os.MkdirAll(r.ExecContext.CommonInstallFlags.TargetPath, os.ModePerm)
				if err != nil {
					log.Error().
						Err(err).
						Msgf("target installation path '%s' error", r.ExecContext.CommonInstallFlags.TargetPath)
					return err
				}
				return nil
			} else {
				log.Error().
					Err(err).
					Msgf("target installation path '%s' error", r.ExecContext.CommonInstallFlags.TargetPath)
				return err
			}

		}
		log.Error().
			Err(err).
			Msgf("target installation path '%s' error", r.ExecContext.CommonInstallFlags.TargetPath)
		return err
	}

	if !targetPathInfo.Mode().IsDir() {
		err = errors.New("not a directory")
		log.Error().
			Err(err).
			Msgf("target installation path '%s' error", r.ExecContext.CommonInstallFlags.TargetPath)
	}

	return nil
}

func PostBuild(k *kong.Kong) error {
	k.Model.Positional[0].Tag.Envs = []string{fmt.Sprintf("%s_REPOSITORY", GetEnvPrefix())}
	return nil
}

func (r *RootCLI) RunInstall() error {
	if r.ExecContext.CommonInstallFlags.Barbarous {
		r.ExecContext.CommonInstallFlags.VerifyChecksum = false
		r.ExecContext.CommonInstallFlags.SkipVtSandbox = true
		r.ExecContext.CommonInstallFlags.AllowForeignArch = true
		r.ExecContext.CommonInstallFlags.AllowDowngrade = true
	}
	if r.ExecContext.CommonInstallFlags.LeRetrogrouch {
		r.ExecContext.CommonInstallFlags.SkipVtSandbox = true
		r.ExecContext.CommonInstallFlags.AllowDowngrade = true
	}
	if r.ExecContext.CommonInstallFlags.RetrogradeStopgap || r.ExecContext.CommonInstallFlags.SelfInflictedDebt {
		r.ExecContext.CommonInstallFlags.AllowDowngrade = true
	}

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	logLevel, _ := zerolog.ParseLevel(r.ExecContext.LogLevel)
	if r.ExecContext.Verbose {
		logLevel = zerolog.DebugLevel
	}
	if r.ExecContext.LogQuietInteractive && r.ExecContext.CommonInstallFlags.Interactive {
		logLevel = zerolog.Disabled
	}
	zerolog.SetGlobalLevel(logLevel)

	cfg := loadConfig()
	if cfg != nil && cfg.Core.LogToFile {
		fileLogger := &lumberjack.Logger{
			Filename:   filepath.Join(xdg.DataHome, "gh-pt", "gh-pt.log"),
			MaxSize:    10, // megabytes
			MaxBackups: 5,
			MaxAge:     30, // days
			Compress:   true,
		}
		if r.ExecContext.LogFormat == "console" {
			log.Logger = log.Output(zerolog.ConsoleWriter{Out: io.MultiWriter(os.Stdout, fileLogger)})
		} else {
			log.Logger = log.Output(io.MultiWriter(os.Stdout, fileLogger))
		}
	} else if r.ExecContext.LogFormat == "console" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	}

	if cfg != nil {
		if r.ExecContext.VTApiKey == "" {
			r.ExecContext.VTApiKey = cfg.Core.VTApiKey
		}
		if cfg.Core.AllowPrerelease {
			r.ExecContext.CommonInstallFlags.Prerelease = true
		}
	}
	if r.ExecContext.CommonInstallFlags.Stable {
		r.ExecContext.CommonInstallFlags.Prerelease = false
	}

	if r.ExecContext.CommonInstallFlags.Global || r.ExecContext.UpdateAll || (r.ExecContext.Update && r.ExecContext.CommonInstallFlags.Global) {
		// Always run sudo -v to refresh/establish the credential cache.
		// sudo -n true only checks without extending the timestamp, which can
		// expire mid-download before installBinary needs it.
		cmd := exec.Command("sudo", "-v")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Warn().Msg("sudo authentication failed or cancelled; elevated operations may fail")
		}
	}

	if r.ExecContext.CommonInstallFlags.AddDeps && r.ExecContext.CommonInstallFlags.NoDeps {
		r.ExecContext.CommonInstallFlags.AddDeps = false
		r.ExecContext.CommonInstallFlags.NoDeps = false
	} else if !r.ExecContext.CommonInstallFlags.AddDeps && !r.ExecContext.CommonInstallFlags.NoDeps {
		envDeps := strings.ToUpper(os.Getenv("GH_INSTALL_ADD_DEPS"))
		switch envDeps {
		case "TRUE":
			r.ExecContext.CommonInstallFlags.AddDeps = true
		case "FALSE":
			r.ExecContext.CommonInstallFlags.NoDeps = true
		default:
			if cfg != nil {
				r.ExecContext.CommonInstallFlags.AddDeps = cfg.Core.AddDeps
				r.ExecContext.CommonInstallFlags.NoDeps = cfg.Core.NoDeps
				if !r.ExecContext.CommonInstallFlags.DisablePrompts {
					r.ExecContext.CommonInstallFlags.DisablePrompts = cfg.Core.DisablePrompts
				}
				if !r.ExecContext.CommonInstallFlags.NoSaveState {
					r.ExecContext.CommonInstallFlags.NoSaveState = cfg.Core.NoSaveState
				}
				if r.ExecContext.CommonInstallFlags.Wine == "off" && cfg.Core.Wine != "" && cfg.Core.Wine != "off" {
				}
				if r.ExecContext.CommonInstallFlags.Extractor == "" || r.ExecContext.CommonInstallFlags.Extractor == "default" {
					r.ExecContext.CommonInstallFlags.Extractor = cfg.Core.Extractor
				}
				if !r.ExecContext.CommonInstallFlags.KeepSuffixes {
					r.ExecContext.CommonInstallFlags.KeepSuffixes = cfg.Core.KeepSuffixes
				}

			}
		}
	}

	if r.ExecContext.CommonInstallFlags.Global && r.ExecContext.CommonInstallFlags.TargetPath == GetDefaultTargetPath() {
		switch runtime.GOOS {
		case "windows":
			r.ExecContext.CommonInstallFlags.TargetPath = os.Getenv("ProgramFiles")
			if r.ExecContext.CommonInstallFlags.TargetPath == "" {
				r.ExecContext.CommonInstallFlags.TargetPath = "C:\\Program Files"
			}
		default:
			r.ExecContext.CommonInstallFlags.TargetPath = "/usr/local/bin"
		}
	}

	ghClient, err := api.DefaultRESTClient()
	if err != nil {
		log.Error().
			Err(err).
			Msg("could not init Gihub REST client")
		return err
	}

	if r.ExecContext.Ls != "" || r.ExecContext.Ll != "" {
		return ListState(r)
	}
	if r.ExecContext.EditSavedState {
		return EditState()
	}
	if r.ExecContext.RmSavedState != "" {
		if !r.ExecContext.CommonInstallFlags.DisablePrompts && !r.ExecContext.CommonInstallFlags.Overwrite {
			confirmed, err := pterm.DefaultInteractiveConfirm.WithDefaultValue(false).Show(fmt.Sprintf("Remove %q from saved state only? This does not uninstall the app.", r.ExecContext.RmSavedState))
			if err != nil {
				return err
			}
			if !confirmed {
				return nil
			}
		}
		return RmStateOnly(r.ExecContext.RmSavedState)
	}
	if r.ExecContext.Rm != "" {
		if !r.ExecContext.CommonInstallFlags.DisablePrompts && !r.ExecContext.CommonInstallFlags.Overwrite {
			confirmed, err := pterm.DefaultInteractiveConfirm.WithDefaultValue(false).Show(fmt.Sprintf("Uninstall %q and remove it from saved state? This removes the tracked binary(s) and any package managed by the OS package manager.", r.ExecContext.Rm))
			if err != nil {
				return err
			}
			if !confirmed {
				return nil
			}
		}
		return RemoveApp(r.ExecContext.Rm, false)
	}
	if r.ExecContext.Purge != "" {
		if !r.ExecContext.CommonInstallFlags.DisablePrompts && !r.ExecContext.CommonInstallFlags.Overwrite {
			confirmed, err := pterm.DefaultInteractiveConfirm.WithDefaultValue(false).Show(fmt.Sprintf("Purge %q and remove it from saved state? This removes the tracked binary(s) and purges the package if applicable.", r.ExecContext.Purge))
			if err != nil {
				return err
			}
			if !confirmed {
				return nil
			}
		}
		return RemoveApp(r.ExecContext.Purge, true)
	}
	if r.ExecContext.Pin != "" {
		return PinAppState(r.ExecContext.Pin)
	}


	if r.ExecContext.Update || r.ExecContext.UpdateAll {
		return DoUpdate(r, ghClient)
	}

	if r.ExecContext.CommonInstallFlags.Overwrite {
		// If overwrite/force is used, attempt to purge any existing installation first
		_ = RemoveApp(r.ExecContext.Repository, true)
	}

	if r.ExecContext.Repository == "" {
		return fmt.Errorf("repository argument is required for installation")
	}

	if r.ExecContext.AI && r.ExecContext.AISafetyScan {
		if err := r.handleAISafetyScan(cfg); err != nil {
			return err
		}
	}

	if r.ExecContext.Clone || r.ExecContext.Fork {
		if cfg == nil {
			cfg, _ = config.LoadConfig()
		}
		return r.handleRepoCloneOrFork(cfg)
	}

	if r.ExecContext.CompileFromSource {
		if cfg == nil {
			cfg, _ = config.LoadConfig()
		}
		return r.handleCompileFromSource(cfg)
	}

	if r.ExecContext.CommonInstallFlags.AssetBinariesRegexp == "" {
		r.ExecContext.CommonInstallFlags.AssetBinariesRegexp = "(?i).*"
	}

	if r.ExecContext.CommonInstallFlags.ReleaseAssetRegexp == "" {
		r.ExecContext.CommonInstallFlags.ReleaseAssetRegexps = buildRegexFromTypes(r.ExecContext.CommonInstallFlags.Type, r.ExecContext.CommonInstallFlags.Wine)
		r.ExecContext.CommonInstallFlags.ReleaseAssetRegexp = strings.Join(r.ExecContext.CommonInstallFlags.ReleaseAssetRegexps, " | ")
	} else {
		r.ExecContext.CommonInstallFlags.ReleaseAssetRegexps = []string{r.ExecContext.CommonInstallFlags.ReleaseAssetRegexp}
	}

	log.Debug().
		Str("repository", r.ExecContext.Repository).
		Str("release version", r.ExecContext.CommonInstallFlags.ReleaseVersion).
		Str("release asset name", r.ExecContext.CommonInstallFlags.ReleaseAsset).
		Strs("release asset binary names", r.ExecContext.CommonInstallFlags.AssetBinaries).
		Str("release asset binary name regexp", r.ExecContext.CommonInstallFlags.AssetBinariesRegexp).
		Str("target path", r.ExecContext.CommonInstallFlags.TargetPath).
		Dict("renaming binaries", func() *zerolog.Event {
			d := zerolog.Dict()
			for k, v := range r.ExecContext.CommonInstallFlags.Rename {
				d = d.Str(k, v)
			}
			return d
		}()).
		Msg("installing with values")

	response := struct{ Name string }{}
	err = ghClient.Get(fmt.Sprintf("repos/%s", r.ExecContext.Repository), &response)
	if err != nil {
		log.Error().
			Err(err).
			Msgf("repository %s doesn't exist", r.ExecContext.Repository)
	}

	installRelease := release.MakeGithubRelease(
		&r.ExecContext,
		ghClient)
	return installRelease.Install()
}

func resolveRepoPath(repo string, isClone, isFork bool, clonePath, forkPath string) string {
	parts := strings.Split(repo, "/")
	repoName := parts[len(parts)-1]

	expandHome := func(p string) string {
		if strings.HasPrefix(p, "~/") || p == "~" {
			homeDir, err := os.UserHomeDir()
			if err == nil {
				if p == "~" {
					return homeDir
				}
				return filepath.Join(homeDir, p[2:])
			}
		}
		return p
	}

	if isFork {
		base := forkPath
		if base == "" {
			base = GetDefaultForkPath()
		} else {
			base = expandHome(base)
		}
		return filepath.Join(base, repoName)
	}

	if isClone {
		base := clonePath
		if base == "" {
			base = GetDefaultClonePath()
		} else {
			base = expandHome(base)
		}
		return filepath.Join(base, repoName)
	}

	return ""
}

func buildCloneOrForkArgs(repo string, isFork bool, targetDir string, maxDepth int) []string {
	var args []string
	if isFork {
		args = []string{"repo", "fork", repo, "--clone", targetDir}
		if maxDepth > 0 {
			args = append(args, "--", "--depth", fmt.Sprintf("%d", maxDepth))
		}
	} else {
		cloneArgs := []string{"repo", "clone", repo, targetDir}
		if maxDepth > 0 {
			cloneArgs = append(cloneArgs, "--", "--depth", fmt.Sprintf("%d", maxDepth))
		}
		args = cloneArgs
	}
	return args
}

func (r *RootCLI) handleRepoCloneOrFork(cfg *config.Config) error {
	cloneBase := GetDefaultClonePath()
	forkBase := GetDefaultForkPath()
	if cfg != nil {
		cloneBase = cfg.Paths.ClonePath
		forkBase = cfg.Paths.ForkPath
	}

	targetDir := resolveRepoPath(r.ExecContext.Repository, r.ExecContext.Clone, r.ExecContext.Fork, cloneBase, forkBase)
	if r.ExecContext.CommonInstallFlags.TargetPath != "" && r.ExecContext.CommonInstallFlags.TargetPath != GetDefaultTargetPath() {
		targetDir = r.ExecContext.CommonInstallFlags.TargetPath
	}

	log.Info().
		Str("repository", r.ExecContext.Repository).
		Str("target_directory", targetDir).
		Bool("clone", r.ExecContext.Clone).
		Bool("fork", r.ExecContext.Fork).
		Msg("handling repository clone/fork")

	if r.ExecContext.CommonInstallFlags.DryRun {
		if r.ExecContext.Fork {
			log.Info().Msgf("[dry-run] Would fork and clone %s to %s", r.ExecContext.Repository, targetDir)
		} else {
			log.Info().Msgf("[dry-run] Would clone %s to %s", r.ExecContext.Repository, targetDir)
		}
		return nil
	}

	if _, err := os.Stat(targetDir); err == nil {
		if !r.ExecContext.CommonInstallFlags.Overwrite {
			return fmt.Errorf("target path %s already exists; use force to overwrite", targetDir)
		}
		_ = os.RemoveAll(targetDir)
	}

	if err := os.MkdirAll(filepath.Dir(targetDir), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	args := buildCloneOrForkArgs(r.ExecContext.Repository, r.ExecContext.Fork, targetDir, r.ExecContext.MaxDepth)

	stdOut, stdErr, err := gh.Exec(args...)
	if err != nil {
		return fmt.Errorf("failed to execute gh %s: %s (%w)", strings.Join(args, " "), stdErr.String(), err)
	}
	log.Info().Str("output", stdOut.String()).Msg("repository cloned successfully")

	if !r.ExecContext.CommonInstallFlags.NoSaveState {
		st, err := state.LoadState()
		if err == nil {
			err = st.AddApp(&state.InstalledApp{
				Repository: r.ExecContext.Repository,
				TargetPath: targetDir,
				Global:     r.ExecContext.CommonInstallFlags.Global,
				Clone:      r.ExecContext.Clone,
				Fork:       r.ExecContext.Fork,
				Pinned:     r.ExecContext.CommonInstallFlags.PinInstall,
				MaxDepth:   r.ExecContext.MaxDepth,
			})
			if err != nil {
				log.Warn().Err(err).Msg("could not save repository state")
			} else {
				log.Info().Msgf("Saved %s to state tracking.", r.ExecContext.Repository)
			}
		}
	}

	return nil
}

func getCompileScriptPath(repo string) string {
	parts := strings.Split(repo, "/")
	pkgName := parts[len(parts)-1]

	ext := ".sh"
	if runtime.GOOS == "windows" {
		ext = ".ps1"
	}

	configDir := filepath.Dir(config.GetConfigPath())
	return filepath.Join(configDir, "scripts", fmt.Sprintf("compile-%s%s", pkgName, ext))
}

func buildCompilePrompt(repo, buildDir, scriptPath, targetPath string) string {
	ext := ".sh"
	if runtime.GOOS == "windows" {
		ext = ".ps1"
	}

	return fmt.Sprintf("Please inspect the repository '%s' (cloned at '%s') and generate an automated compilation/build script at '%s'. The script should follow all build instructions for '%s', compile the application/binaries, install or copy them to '%s', and purge any temporary build artifacts. Format the output as an executable %s script. Please test and then attempt to run the compile script and it is only done when script runs successfully.", repo, buildDir, scriptPath, repo, targetPath, ext)
}

func buildCompileFixPrompt(repo, buildDir, scriptPath, targetPath, errorOutput string, attempt int) string {
	ext := ".sh"
	if runtime.GOOS == "windows" {
		ext = ".ps1"
	}

	return fmt.Sprintf("The automated compilation script at '%s' for repository '%s' (cloned at '%s') failed to run with the following error output (attempt %d of 2):\n\n%s\n\nPlease fix the script at '%s' so that it successfully compiles and installs the binaries into '%s'. Format the output as an executable %s script. Please fix and then attempt to run the compile script and it is only done when script runs successfully.", scriptPath, repo, buildDir, attempt, errorOutput, scriptPath, targetPath, ext)
}

func runAIAgent(aiCmdTemplate, prompt, dir string) error {
	var cmd *exec.Cmd
	if strings.Contains(aiCmdTemplate, "%s") {
		formattedCmd := fmt.Sprintf(aiCmdTemplate, prompt)
		if runtime.GOOS == "windows" {
			cmd = exec.Command("cmd", "/C", formattedCmd)
		} else {
			cmd = exec.Command("sh", "-c", formattedCmd)
		}
	} else {
		cmd = exec.Command(aiCmdTemplate, prompt)
	}
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (r *RootCLI) handleAISafetyScan(cfg *config.Config) error {
	aiCmdTemplate := r.ExecContext.AICmd
	if cfg != nil && cfg.AI.AICmd != "" && (r.ExecContext.AICmd == "" || r.ExecContext.AICmd == "agy -p \"%s\"") {
		aiCmdTemplate = cfg.AI.AICmd
	}
	if aiCmdTemplate == "" {
		aiCmdTemplate = "agy -p \"%s\""
	}

	prompt := fmt.Sprintf("Analyze the GitHub repository %s for safety concerns, malicious code, suspicious recent commits, or backdoors. Report your findings concisely and explicitly state if it appears safe or compromised.", r.ExecContext.Repository)

	log.Info().Msgf("Initiating AI safety scan for %s...", r.ExecContext.Repository)
	if err := runAIAgent(aiCmdTemplate, prompt, ""); err != nil {
		return fmt.Errorf("AI safety scan failed to execute: %w", err)
	}

	if !r.ExecContext.CommonInstallFlags.DisablePrompts {
		var confirm string
		fmt.Printf("\nSafety scan complete. Do you want to proceed with the installation of %s? [y/N]: ", r.ExecContext.Repository)
		_, _ = fmt.Scanln(&confirm)
		if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
			return fmt.Errorf("installation aborted by user after AI safety scan")
		}
	}

	return nil
}

func (r *RootCLI) handleCompileFromSource(cfg *config.Config) error {
	scriptPath := getCompileScriptPath(r.ExecContext.Repository)
	targetPath := r.ExecContext.CommonInstallFlags.TargetPath
	if targetPath == "" {
		targetPath = GetDefaultTargetPath()
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not determine home directory: %w", err)
	}
	parts := strings.Split(r.ExecContext.Repository, "/")
	repoName := parts[len(parts)-1]
	repoDir := filepath.Join(homeDir, "builds", repoName)

	if err := os.MkdirAll(filepath.Dir(repoDir), 0755); err != nil {
		return fmt.Errorf("failed to create builds directory: %w", err)
	}

	// Ensure fresh clone
	_ = os.RemoveAll(repoDir)

	log.Info().
		Str("repository", r.ExecContext.Repository).
		Str("build_dir", repoDir).
		Str("script_path", scriptPath).
		Str("target_path", targetPath).
		Msg("handling compile-from-source with AI")

	if r.ExecContext.CommonInstallFlags.DryRun {
		log.Info().Msgf("[dry-run] Would clone %s to %s, generate build script at %s, and execute compilation", r.ExecContext.Repository, repoDir, scriptPath)
		return nil
	}

	// 1. Clone repo into builds directory
	cloneArgs := []string{"repo", "clone", r.ExecContext.Repository, repoDir}
	if r.ExecContext.MaxDepth > 0 {
		cloneArgs = append(cloneArgs, "--", "--depth", fmt.Sprintf("%d", r.ExecContext.MaxDepth))
	}
	stdOut, stdErr, err := gh.Exec(cloneArgs...)
	if err != nil {
		return fmt.Errorf("failed to clone repository to builds dir: %s (%w)", stdErr.String(), err)
	}
	log.Info().Str("output", stdOut.String()).Msg("cloned repository to builds directory")

	// 0. Initial check: try existing compile script if it exists
	if _, err := os.Stat(scriptPath); err == nil {
		log.Info().Str("script", scriptPath).Msg("found existing compile script, attempting to run it first")

		var preExecCmd *exec.Cmd
		if runtime.GOOS == "windows" {
			preExecCmd = exec.Command("powershell", "-ExecutionPolicy", "Bypass", "-File", scriptPath)
		} else {
			preExecCmd = exec.Command("sh", scriptPath)
		}
		preExecCmd.Dir = repoDir

		outputBytes, runErr := preExecCmd.CombinedOutput()
		if len(outputBytes) > 0 {
			_, _ = os.Stdout.Write(outputBytes)
		}

		if runErr == nil {
			log.Info().Msgf("Existing compile script succeeded for %s", r.ExecContext.Repository)
			if !r.ExecContext.CommonInstallFlags.NoSaveState {
				st, err := state.LoadState()
				if err == nil {
					err = st.AddApp(&state.InstalledApp{
						Repository:    r.ExecContext.Repository,
						TargetPath:    targetPath,
						Global:        r.ExecContext.CommonInstallFlags.Global,
						CompileScript: scriptPath,
						Pinned:        r.ExecContext.CommonInstallFlags.PinInstall,
						MaxDepth:      r.ExecContext.MaxDepth,
					})
					if err != nil {
						log.Warn().Err(err).Msg("could not save repository state")
					} else {
						log.Info().Msgf("Saved %s with compileScript to state tracking.", r.ExecContext.Repository)
					}
				}
			}
			return nil
		}
		log.Warn().Err(runErr).Msg("existing compile script failed, will regenerate with AI")
	}

	// Get the commit hash we cloned
	commitHash := "unknown"
	revParseCmd := exec.Command("git", "rev-parse", "HEAD")
	revParseCmd.Dir = repoDir
	if out, err := revParseCmd.Output(); err == nil {
		commitHash = strings.TrimSpace(string(out))
	}
	log.Info().Str("commit", commitHash).Msg("compile-from-source using commit")

	// 2. Ensure scripts directory exists
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0755); err != nil {
		return fmt.Errorf("failed to create scripts directory: %w", err)
	}

	// 3. Resolve AI command template
	aiCmdTemplate := r.ExecContext.AICmd
	if cfg != nil && cfg.AI.AICmd != "" && (r.ExecContext.AICmd == "" || r.ExecContext.AICmd == `agy -p "%s"`) {
		aiCmdTemplate = cfg.AI.AICmd
	}
	if aiCmdTemplate == "" {
		aiCmdTemplate = `agy -p "%s"`
	}

	prompt := buildCompilePrompt(r.ExecContext.Repository, repoDir, scriptPath, targetPath)
	log.Info().Msgf("Generating AI compilation script using: %s", aiCmdTemplate)
	if err := runAIAgent(aiCmdTemplate, prompt, repoDir); err != nil {
		return fmt.Errorf("AI agent failed to generate/test compilation script: %w", err)
	}

	if _, err := os.Stat(scriptPath); err != nil {
		return fmt.Errorf("expected compile script '%s' was not created: %w", scriptPath, err)
	}
	_ = os.Chmod(scriptPath, 0755)

	// 4. Run the generated compile script with up to 2 retry fix loops
	maxRetries := 2
	var lastErr error
	var lastOutput string

	for attempt := 0; attempt <= maxRetries; attempt++ {
		log.Info().Str("script", scriptPath).Int("attempt", attempt+1).Msg("executing generated compile script")
		var execScriptCmd *exec.Cmd
		if runtime.GOOS == "windows" {
			execScriptCmd = exec.Command("powershell", "-ExecutionPolicy", "Bypass", "-File", scriptPath)
		} else {
			execScriptCmd = exec.Command("sh", scriptPath)
		}
		execScriptCmd.Dir = repoDir

		outputBytes, runErr := execScriptCmd.CombinedOutput()
		if len(outputBytes) > 0 {
			_, _ = os.Stdout.Write(outputBytes)
		}

		if runErr == nil {
			lastErr = nil
			break
		}

		lastErr = runErr
		lastOutput = string(outputBytes)
		log.Warn().Err(runErr).Int("attempt", attempt+1).Msg("compile script failed")

		if attempt < maxRetries {
			log.Info().Msgf("Prompting AI to fix compile script (retry %d of %d)...", attempt+1, maxRetries)
			fixPrompt := buildCompileFixPrompt(r.ExecContext.Repository, repoDir, scriptPath, targetPath, lastOutput, attempt+1)
			if fixErr := runAIAgent(aiCmdTemplate, fixPrompt, repoDir); fixErr != nil {
				log.Warn().Err(fixErr).Msg("AI repair command execution failed")
			}
			_ = os.Chmod(scriptPath, 0755)
		}
	}

	if lastErr != nil {
		return fmt.Errorf("compile script execution failed after %d retries: %w (output: %s)", maxRetries, lastErr, lastOutput)
	}

	log.Info().Msgf("Successfully compiled and installed %s from source!", r.ExecContext.Repository)

	// 5. Save compileScript to state
	if !r.ExecContext.CommonInstallFlags.NoSaveState {
		st, err := state.LoadState()
		if err == nil {
			err = st.AddApp(&state.InstalledApp{
				Repository:    r.ExecContext.Repository,
				TargetPath:    targetPath,
				Global:        r.ExecContext.CommonInstallFlags.Global,
				Version:       commitHash,
				CompileScript: scriptPath,
				Pinned:        r.ExecContext.CommonInstallFlags.PinInstall,
				MaxDepth:      r.ExecContext.MaxDepth,
			})
			if err != nil {
				log.Warn().Err(err).Msg("could not save repository state")
			} else {
				log.Info().Msgf("Saved %s with compileScript to state tracking.", r.ExecContext.Repository)
			}
		}
	}

	return nil
}

func GetDefaultTargetPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(homeDir, ".local", "bin")
}

func GetDefaultClonePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(homeDir, "src", "repos")
}

func GetDefaultForkPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(homeDir, "projects")
}

func GetDefaultInstallTypes() string {
	tarballRgx := `t(ar\\.)?([gxl]z|bz2?|zst),tar(\\.lzma)?`
	switch runtime.GOOS {
	case "windows":
		return fmt.Sprintf("exe,msi,7z,%s,zip,py,ts,js", tarballRgx)
	case "darwin":
		return fmt.Sprintf("dmg,7z,%s,zip,py,ts,js,none", tarballRgx)
	case "linux":
		hasDpkg := false
		hasRpm := false
		if _, err := exec.LookPath("dpkg"); err == nil {
			hasDpkg = true
		}
		if _, err := exec.LookPath("rpm"); err == nil {
			hasRpm = true
		}

		if hasDpkg && !hasRpm {
			return fmt.Sprintf("deb,snap,flatpak,appimage,7z,%s,zip,py,ts,js,none", tarballRgx)
		} else if hasRpm && !hasDpkg {
			return fmt.Sprintf("rpm,snap,flatpak,appimage,7z,%s,zip,py,ts,js,none", tarballRgx)
		} else if hasRpm && hasDpkg {
			// If both exist (e.g. alien installed), try to check os-release
			osRelease, err := os.ReadFile("/etc/os-release")
			if err == nil {
				content := strings.ToLower(string(osRelease))
				if strings.Contains(content, "id=fedora") || strings.Contains(content, "id=rhel") || strings.Contains(content, "id=centos") {
					return fmt.Sprintf("rpm,deb,snap,flatpak,appimage,7z,%s,zip,py,ts,js,none", tarballRgx)
				}
			}
			return fmt.Sprintf("deb,rpm,snap,flatpak,appimage,7z,%s,zip,py,ts,js,none", tarballRgx)
		}

		// Arch or others without rpm/deb natively
		return fmt.Sprintf("appimage,flatpak,snap,7z,%s,zip,py,ts,js,none", tarballRgx)
	case "freebsd":
		return fmt.Sprintf("pkg,txz,7z,%s,zip,py,ts,js,none", tarballRgx)
	default:
		return fmt.Sprintf("7z,%s,zip,none", tarballRgx)
	}
}
func buildRegexFromTypes(types []string, wine string) []string {
	archRegex := runtime.GOARCH
	switch runtime.GOARCH {
	case "amd64":
		archRegex = "(?:amd64|x86_64|x64)"
	case "arm64":
		archRegex = "(?:arm64|aarch64)"
	}

	var osRegexList []string
	switch runtime.GOOS {
	case "darwin":
		osRegexList = append(osRegexList, "(?:darwin|macos|apple)")
	case "windows":
		osRegexList = append(osRegexList, "(?:windows|win)")
	case "freebsd":
		osRegexList = append(osRegexList, "(?:freebsd)")
	case "linux":
		osRelease, err := os.ReadFile("/etc/os-release")
		if err == nil {
			content := string(osRelease)
			lines := strings.Split(content, "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "ID=") {
					distro := strings.ToLower(strings.Trim(strings.TrimPrefix(line, "ID="), "\""))
					if distro != "" && distro != "linux" {
						osRegexList = append(osRegexList, distro)
					}
				} else if strings.HasPrefix(line, "ID_LIKE=") {
					idLike := strings.ToLower(strings.Trim(strings.TrimPrefix(line, "ID_LIKE="), "\""))
					if idLike != "" {
						for _, likeDistro := range strings.Fields(idLike) {
							if likeDistro != "linux" {
								osRegexList = append(osRegexList, likeDistro)
							}
						}
					}
				}
			}
		}
		osRegexList = append(osRegexList, "linux")
	}

	hwSpecific := ""
	if runtime.GOOS == "linux" {
		if _, err := os.Stat("/sys/class/accel"); err == nil {
			hwSpecific = "npu"
		} else if _, err := os.Stat("/dev/dri"); err == nil {
			hwSpecific = "(?:gpu|cuda|rocm)"
		}
	}

	buildFinal := func(baseRegex string, t string) string {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "none" {
			return fmt.Sprintf(`^%s$`, baseRegex)
		} else if t != "" {
			return fmt.Sprintf(`^%s\.(?i:%s)$`, baseRegex, t)
		}
		return fmt.Sprintf(`^%s$`, baseRegex)
	}

	var matchers []string

	for _, t := range types {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}

		lowerT := strings.ToLower(t)
		isOsFormatPkg := lowerT == "deb" || lowerT == "rpm" || lowerT == "pkg" || lowerT == "txz" || lowerT == "dmg"
		// AppImage, Flatpak, and Snap are Linux-only universal packages — no OS token needed in filename.
		isUniversalLinux := lowerT == "appimage" || lowerT == "flatpak" || lowerT == "snap"

		for _, osRegex := range osRegexList {
			if hwSpecific != "" {
				hwBaseRegex := fmt.Sprintf(`.*(?:%s.+%s.+%s|%s.+%s.+%s|%s.+%s|%s.+%s).*`, archRegex, osRegex, hwSpecific, osRegex, archRegex, hwSpecific, hwSpecific, archRegex, archRegex, hwSpecific)
				matchers = append(matchers, buildFinal(hwBaseRegex, t))
			}

			baseRegex := fmt.Sprintf(`.*(?:%s.+%s|%s.+%s).*`, archRegex, osRegex, osRegex, archRegex)
			matchers = append(matchers, buildFinal(baseRegex, t))
		}

		// Format-specific fallback: format already implies OS (e.g. .deb implies debian/ubuntu), match arch
		if isOsFormatPkg {
			archOnlyRegex := fmt.Sprintf(`.*%s.*`, archRegex)
			matchers = append(matchers, buildFinal(archOnlyRegex, t))
		}

		// Universal Linux packages: only need arch match, no OS token required
		if isUniversalLinux {
			archOnlyRegex := fmt.Sprintf(`.*%s.*`, archRegex)
			matchers = append(matchers, buildFinal(archOnlyRegex, t))
			// Also allow bare format matches (no arch specified)
			matchers = append(matchers, buildFinal(`.*`, t))
		}

		// Fallback: OS only
		for _, osRegex := range osRegexList {
			fallbackRegex := fmt.Sprintf(`.*%s.*`, osRegex)
			matchers = append(matchers, buildFinal(fallbackRegex, t))
		}
	}

	for _, t := range types {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		matchers = append(matchers, buildFinal(`.*`, t))
	}

	var normalMatchers []string
	// Copy current matchers to normalMatchers
	normalMatchers = append(normalMatchers, matchers...)
	matchers = []string{} // Reset matchers

	var winMatchers []string
	if (wine == "allow" || wine == "priority" || wine == "force") && runtime.GOOS != "windows" {
		winOsRegex := "(?:windows|win)"
		winBaseRegex := fmt.Sprintf(`.*(?:%s.+%s|%s.+%s).*`, archRegex, winOsRegex, winOsRegex, archRegex)
		for _, t := range append([]string{"exe", "msi"}, types...) {
			winMatchers = append(winMatchers, buildFinal(winBaseRegex, t))
		}
	}

	if wine == "force" && runtime.GOOS != "windows" {
		matchers = append(matchers, winMatchers...)
	} else if wine == "priority" && runtime.GOOS != "windows" {
		matchers = append(matchers, winMatchers...)
		matchers = append(matchers, normalMatchers...)
	} else {
		matchers = append(matchers, normalMatchers...)
		matchers = append(matchers, winMatchers...)
	}

	return matchers
}

func GetEnvPrefix() string {
	envPrefix := os.Getenv(GH_INSTALL_PREFIX_ENV)
	if envPrefix == "" {
		envPrefix = GH_INSTALL_DEFAULT_PREFIX
	}

	return strings.ToUpper(envPrefix)
}

func loadConfig() *config.Config {
	cfg, _ := config.LoadConfig()
	return cfg
}
