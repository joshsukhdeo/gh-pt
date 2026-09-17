package cmd

import (
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/joshsukhdeo/gh-pt/release"
	"github.com/joshsukhdeo/gh-pt/state"
	"github.com/rs/zerolog/log"
)

func DoUpdate(r *RootCLI, ghClient *api.RESTClient) error {
	if r.Barbarous {
		r.VerifyChecksum = false
		r.SkipVtSandbox = true
		r.AllowForeignArch = true
		r.AllowDowngrade = true
	}
	if r.LeRetrogrouch {
		r.SkipVtSandbox = true
		r.AllowDowngrade = true
	}
	if r.RetrogradeStopgap || r.SelfInflictedDebt {
		r.AllowDowngrade = true
	}

	st, err := state.LoadState()
	if err != nil {
		return err
	}

	for _, app := range st.Apps {
		specificallyTargeted := (r.Repository != "" && strings.EqualFold(r.Repository, app.Repository))

		if r.Repository != "" && !specificallyTargeted {
			continue // specifically targeted another app
		}

		if app.Disabled && !specificallyTargeted {
			continue // skip disabled unless specifically targeted
		}

		if app.Pinned && !specificallyTargeted {
			log.Info().Msgf("Skipping %s (pinned at %s)", app.Repository, app.Version)
			continue
		}

		if r.Update && !r.UpdateAll {
			if r.Global && !app.Global {
				continue // wants global only, this is user
			}
			if !r.Global && app.Global {
				continue // wants user only, this is global
			}
		}

		log.Info().Msgf("Updating %s...", app.Repository)

		if app.CompileScript != "" {
			// Check remote commit to see if we need an update
			remoteCommit := ""
			checkCmd := exec.Command("gh", "api", "repos/"+app.Repository+"/commits/HEAD", "--jq", ".sha")
			if out, err := checkCmd.Output(); err == nil {
				remoteCommit = strings.TrimSpace(string(out))
				if remoteCommit != "" && remoteCommit == app.Version {
					log.Info().Msgf("Skipping %s (already at latest commit %s)", app.Repository, app.Version)
					continue
				}
				if remoteCommit != "" {
					log.Info().Msgf("Updating %s from %s to %s", app.Repository, app.Version, remoteCommit)
				}
			} else {
				log.Warn().Msgf("Could not check remote commit for %s, proceeding with update", app.Repository)
			}

			if r.DryRun {
				log.Info().Msgf("[dry-run] Would execute compile script %s for %s", app.CompileScript, app.Repository)
				continue
			}

			var cmd *exec.Cmd
			if runtime.GOOS == "windows" {
				cmd = exec.Command("powershell", "-ExecutionPolicy", "Bypass", "-File", app.CompileScript)
			} else {
				cmd = exec.Command("sh", app.CompileScript)
			}
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			if err := cmd.Run(); err != nil {
				log.Error().Err(err).Msgf("Failed to update %s via compile script", app.Repository)
			} else {
				log.Info().Msgf("Successfully updated %s via compile script", app.Repository)
				if remoteCommit != "" {
					app.Version = remoteCommit
				}

			state.LogHistory("update", app.Repository, app.Version)
			// Save the updated version to state
			if err := st.AddApp(app); err != nil {
				log.Warn().Err(err).Msgf("could not update state for %s", app.Repository)
			}
			}
			continue
		}

		if app.Clone || app.Fork {
			if r.DryRun {
				log.Info().Msgf("[dry-run] Would sync repo %s at %s", app.Repository, app.TargetPath)
				continue
			}

			var syncArgs []string
			if app.Fork {
				syncArgs = []string{"repo", "sync", "--source", app.Repository}
			} else {
				syncArgs = []string{"repo", "sync"}
			}

			// Run gh repo sync in the target repository directory
			cmd := exec.Command("gh", syncArgs...)
			cmd.Dir = app.TargetPath
			output, err := cmd.CombinedOutput()
			if err != nil {
				log.Error().Err(err).Msgf("Failed to sync %s: %s", app.Repository, string(output))
			} else {
				log.Info().Msgf("Successfully synced %s", app.Repository)

				if app.Fork && r.Overwrite {
					pullCmd := exec.Command("git", "pull", "origin", "--force")
					pullCmd.Dir = app.TargetPath
					pullOutput, pullErr := pullCmd.CombinedOutput()
					if pullErr != nil {
						log.Error().Err(pullErr).Msgf("Failed to pull from origin for %s: %s", app.Repository, string(pullOutput))
					} else {
						log.Info().Msgf("Successfully pulled from origin for %s", app.Repository)
					}
				}

				state.LogHistory("update", app.Repository, app.Version)
			}
			continue
		}

		// Create fresh params for this app based on stored state
		appParams := r.ExecContext
		appParams.Repository = app.Repository
		appParams.TargetPath = app.TargetPath
		appParams.Global = app.Global
		appParams.ReleaseAsset = app.ReleaseAsset
		appParams.ReleaseAssetRegexp = app.ReleaseRegexp
		if appParams.ReleaseAssetRegexp == "" {
			appParams.ReleaseAssetRegexps = buildRegexFromTypes(appParams.Type, appParams.Wine)
			appParams.ReleaseAssetRegexp = strings.Join(appParams.ReleaseAssetRegexps, " | ")
		} else {
			appParams.ReleaseAssetRegexps = strings.Split(app.ReleaseRegexp, " | ")
		}
		appParams.Rename = app.Rename
		if len(app.Type) > 0 {
			appParams.Type = app.Type
		}
		appParams.All = app.All
		appParams.AssetBinaries = app.AssetBinaries
		appParams.AssetBinariesRegexp = app.AssetBinariesRegexp
		appParams.Extractor = app.Extractor
		// Reset version to latest to ensure we get the latest
		appParams.ReleaseVersion = "latest"
		if app.IsPrerelease {
			appParams.Prerelease = true
		}
		if r.Stable {
			appParams.Prerelease = false
			appParams.Stable = true
		}
		appParams.IsUpgradeCmd = true

		// Check if there's a new version before updating
		installRelease := release.MakeGithubRelease(&appParams, ghClient)
		latestRelease, err := installRelease.GetLatestRelease()
		if err != nil {
			log.Error().Err(err).Msgf("Failed to check for updates for %s", app.Repository)
			continue
		}

		// Compare versions - skip if same version is already installed
		// Normalize by stripping leading 'v' to handle version strings like "v1.2.3" vs "1.2.3"
		normalizedLatest := strings.TrimPrefix(latestRelease.Name, "v")
		normalizedStored := strings.TrimPrefix(app.Version, "v")

		if normalizedLatest == normalizedStored {
			log.Info().Msgf("Skipping %s (already at latest version %s)", app.Repository, app.Version)
			continue
		}

		log.Info().Msgf("Updating %s from %s to %s", app.Repository, app.Version, latestRelease.Name)

		// We trust the app.ReleaseRegexp stored in state.json because it was
		// already stripped of version numbers and replaced with .*

		installRelease = release.MakeGithubRelease(&appParams, ghClient)
		err = installRelease.Install()
		if err != nil {
			log.Error().Err(err).Msgf("Failed to update %s", app.Repository)
		} else {
			log.Info().Msgf("Successfully updated %s", app.Repository)
			state.LogHistory("update", app.Repository, latestRelease.Name)
		}
	}

	return nil
}
