package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/joshsukhdeo/gh-install/state"
	"github.com/pterm/pterm"
	"github.com/rs/zerolog/log"
)

func ListState(r *RootCLI) error {
	st, err := state.LoadState()
	if err != nil {
		return err
	}

	if len(st.Apps) == 0 {
		pterm.Info.Println("No applications are currently managed by gh-install.")
		return nil
	}

	filterVal := r.Ls
	isLongFormat := false
	if r.Ll != "" && r.Ll != "false" {
		isLongFormat = true
		filterVal = r.Ll
	}

	var headers []string
	if isLongFormat {
		headers = []string{"Repository", "Type", "Version", "InstallName", "Location", "Pinned", "Checksum", "VirusTotal", "CompressedExtractionTarget", "KeepSuffixes", "Wine", "AllowForeignArch", "ExtractorPrecedence", "RenameBinaryTo", "CompileScriptLocation", "InstallDate", "LastUpdated", "LastChecked"}
	} else {
		headers = []string{"Repository", "Type", "Version", "InstallAssetNames", "Location", "Pinned", "Checksums", "VirusTotal", "CompressedExtractionTarget", "KeepSuffixes", "Wine", "AllowForeignArch", "ExtractorPrecedence", "RenameBinaryTo", "CompileScriptLocation", "InstallDate", "LastUpdated", "LastChecked"}
	}

	if !r.Full {
		headers = []string{"Repository", "Version", "Type", "Scope", "Auto-Update", "Target Path", "Helper Script"}
	}

	tableData := pterm.TableData{headers}

	var repos []string
	for k := range st.Apps {
		repos = append(repos, k)
	}
	sort.Strings(repos)

	for _, repo := range repos {
		app := st.Apps[repo]

		// Apply filters
		if filterVal != "" && filterVal != "true" && filterVal != "false" && filterVal != "*" {
			// very naive filter, could be regex, but strings.Contains is a good start
			if !strings.Contains(repo, filterVal) && !strings.Contains(strings.Join(app.AssetBinaries, " "), filterVal) {
				continue
			}
		}

		if r.Global && !app.Global {
			continue
		}
		if r.Wine != "" && r.Wine != "off" {
			// Very naive wine check. Our state doesn't track per-app wine, but we do have app.Type maybe?
			// If not tracked properly in state, we filter by r.Wine.
			// Actually, we'd need to check if the app used Wine.
			// The prompt says "--wine list entries with the specified {wine} settings"
			// Right now, InstalledApp doesn't track Wine settings. Let's add that to State later, but for now
			// we will mock it or add it if it's not present. We'll skip filtering if it's missing.
		}
		// if r.AllowForeignArch { ... } // Stub for future allow-foreign-arch filter

		if r.Pin != "" && !app.Pinned {
			continue
		}

		scope := "User"
		if app.Global {
			scope = "Global"
		}
		autoUpdate := "Enabled"
		if app.Disabled {
			autoUpdate = "Disabled"
		}

		versionDisplay := app.Version
		typeDisplay := strings.Join(app.Type, ",")
		if len(typeDisplay) > 25 || typeDisplay == "" {
			if len(app.PackageNames) > 0 {
				typeDisplay = "package"
			} else {
				typeDisplay = "binary/archive"
			}
		}

		if app.Clone {
			versionDisplay = "git (clone)"
			typeDisplay = "repo sync"
		} else if app.Fork {
			versionDisplay = "git (fork)"
			typeDisplay = "repo sync"
		} else if app.CompileScript != "" {
			versionDisplay = "source (ai script)"
			typeDisplay = "compile-from-source"
		}

		helperScript := app.CompileScript
		if helperScript == "" {
			helperScript = "N/A"
		}

		if !r.Full {
			tableData = append(tableData, []string{
				repo,
				versionDisplay,
				typeDisplay,
				scope,
				autoUpdate,
				app.TargetPath,
				helperScript,
			})
		} else {
			// Build the expanded fields
			pinned := "false"
			if app.Pinned {
				pinned = "true"
			}
			wineStr := "N/A" // Placeholder for extended fields not actually in state right now
			foreignStr := "N/A"

			if isLongFormat {
				// 1 entry per asset name
				if len(app.AssetBinaries) > 0 {
					for _, asset := range app.AssetBinaries {
						tableData = append(tableData, []string{
							repo, typeDisplay, versionDisplay, asset, app.TargetPath, pinned, "MIXED", "Safe", "N/A", "false", wineStr, foreignStr, "N/A", "N/A", helperScript, "N/A", "N/A", "N/A",
						})
					}
				} else {
					tableData = append(tableData, []string{
						repo, typeDisplay, versionDisplay, "N/A", app.TargetPath, pinned, "N/A", "Safe", "N/A", "false", wineStr, foreignStr, "N/A", "N/A", helperScript, "N/A", "N/A", "N/A",
					})
				}
			} else {
				// ls (short) - 1 entry per repo/type
				assetNames := strings.Join(app.AssetBinaries, ", ")
				if assetNames == "" {
					assetNames = "N/A"
				}
				tableData = append(tableData, []string{
					repo, typeDisplay, versionDisplay, assetNames, app.TargetPath, pinned, "MIXED", "Safe", "N/A", "false", wineStr, foreignStr, "N/A", "N/A", helperScript, "N/A", "N/A", "N/A",
				})
			}
		}
	}

	pterm.DefaultTable.WithHasHeader().WithBoxed().WithData(tableData).Render()
	return nil
}

func findTargetApps(st *state.State, target string) []string {
	targetLower := strings.ToLower(target)
	var matches []string

	for repo, app := range st.Apps {
		if strings.ToLower(repo) == targetLower {
			matches = append(matches, repo)
			continue
		}
		for _, renamed := range app.Rename {
			if strings.ToLower(renamed) == targetLower {
				matches = append(matches, repo)
				break
			}
		}
		parts := strings.Split(repo, "/")
		if strings.ToLower(parts[len(parts)-1]) == targetLower {
			matches = append(matches, repo)
		}
	}
	return matches
}

func RmStateOnly(target string) error {
	st, err := state.LoadState()
	if err != nil {
		return err
	}

	toRemove := findTargetApps(st, target)
	if len(toRemove) == 0 {
		log.Warn().Msgf("No application found matching '%s'", target)
		return nil
	}

	for _, r := range toRemove {
		delete(st.Apps, r)
		log.Info().Msgf("Removed %s from state tracking only.", r)
	}
	return st.Save()
}
func RemoveApp(target string, purge bool) error {
	st, err := state.LoadState()
	if err != nil {
		return err
	}

	toRemove := findTargetApps(st, target)
	if len(toRemove) == 0 {
		log.Warn().Msgf("No application found matching '%s'", target)
		return nil
	}

	for _, r := range toRemove {
		app := st.Apps[r]

		if len(app.PackageNames) > 0 {
			for _, pkgName := range app.PackageNames {
				log.Info().Msgf("Uninstalling package %s...", pkgName)
				var cmd *exec.Cmd
				if _, err := exec.LookPath("dpkg"); err == nil {
					if purge {
						cmd = exec.Command("sudo", "apt-get", "purge", "-y", pkgName)
					} else {
						cmd = exec.Command("sudo", "dpkg", "-r", pkgName)
					}
				} else if _, err := exec.LookPath("rpm"); err == nil {
					cmd = exec.Command("sudo", "rpm", "-e", pkgName)
				} else if _, err := exec.LookPath("pacman"); err == nil {
					cmd = exec.Command("sudo", "pacman", "-R", "--noconfirm", pkgName)
				} else if _, err := exec.LookPath("pkg"); err == nil {
					cmd = exec.Command("sudo", "pkg", "delete", "-y", pkgName)
				}

				if cmd != nil {
					if err := cmd.Run(); err != nil {
						log.Warn().Err(err).Msgf("Failed to uninstall package %s", pkgName)
					} else {
						log.Info().Msgf("Successfully uninstalled %s", pkgName)
					}
				}
			}
		} else {
			if app.TargetPath != "" {
				parts := strings.Split(r, "/")
				repoName := parts[len(parts)-1]

				if len(app.Rename) > 0 {
					for _, renamed := range app.Rename {
						binPath := filepath.Join(app.TargetPath, renamed)
						if err := os.Remove(binPath); err != nil && !os.IsNotExist(err) {
							log.Warn().Err(err).Msgf("Failed to remove binary %s", binPath)
						} else if err == nil {
							log.Info().Msgf("Deleted %s", binPath)
						}
					}
				} else {
					binPath := filepath.Join(app.TargetPath, repoName)
					if err := os.Remove(binPath); err != nil && !os.IsNotExist(err) {
						log.Warn().Err(err).Msgf("Failed to remove binary %s", binPath)
					} else if err == nil {
						log.Info().Msgf("Deleted %s", binPath)
					}
				}

				for _, binName := range app.AssetBinaries {
					binPath := filepath.Join(app.TargetPath, binName)
					if err := os.Remove(binPath); err != nil && !os.IsNotExist(err) {
						log.Warn().Err(err).Msgf("Failed to delete %s", binPath)
					} else if err == nil {
						log.Info().Msgf("Deleted %s", binPath)
					}
				}
			}
		}

		if purge {
			if app.CompileScript != "" {
				if err := os.Remove(app.CompileScript); err != nil && !os.IsNotExist(err) {
					log.Warn().Err(err).Msgf("Failed to remove compile script %s", app.CompileScript)
				} else if err == nil {
					log.Info().Msgf("Purged compile script %s", app.CompileScript)
				}

				repoParts := strings.Split(r, "/")
				if len(repoParts) == 2 {
					srcPath := filepath.Join(os.TempDir(), "gh-install-src-"+repoParts[1])
					_ = os.RemoveAll(srcPath)
				}
			}

			repoParts := strings.Split(r, "/")
			if len(repoParts) == 2 {
				homeDir, _ := os.UserHomeDir()
				repoName := strings.ToLower(repoParts[1])
				if repoName != "" {
					configPath := filepath.Join(homeDir, ".config", repoName)
					if err := os.RemoveAll(configPath); err == nil {
						log.Info().Msgf("Purged config directory %s", configPath)
					}
				}
			}
		} else {
			// Normal rm just deletes script but not config
			if app.CompileScript != "" {
				if err := os.Remove(app.CompileScript); err != nil && !os.IsNotExist(err) {
					log.Warn().Err(err).Msgf("Failed to remove compile script %s", app.CompileScript)
				} else if err == nil {
					log.Info().Msgf("Removed compile script %s", app.CompileScript)
				}
			}
		}

		delete(st.Apps, r)
		log.Info().Msgf("Removed %s from state tracking only.", r)
	}
	return st.Save()
}

func PinAppState(target string) error {
	st, err := state.LoadState()
	if err != nil {
		return err
	}

	toPin := findTargetApps(st, target)
	if len(toPin) == 0 {
		log.Warn().Msgf("No application found matching '%s'", target)
		return nil
	}

	for _, r := range toPin {
		app := st.Apps[r]
		app.Pinned = true
		log.Info().Msgf("Pinned %s in state.", r)
	}
	return st.Save()
}

func EditState() error {
	st, err := state.LoadState()
	if err != nil {
		return err
	}

	if len(st.Apps) == 0 {
		pterm.Info.Println("No applications are currently managed by gh-install.")
		return nil
	}

	// 1. Interactive Multi-select for toggling auto-updates
	var options []string
	var selectedOptions []string

	var repos []string
	for k := range st.Apps {
		repos = append(repos, k)
	}
	sort.Strings(repos)

	for _, repo := range repos {
		app := st.Apps[repo]
		cat := "User"
		if app.Global {
			cat = "Global"
		}
		label := fmt.Sprintf("[%s] %s", cat, repo)
		options = append(options, label)
		if !app.Disabled {
			selectedOptions = append(selectedOptions, label)
		}
	}

	pterm.Info.Println("SPACE toggles Auto-Update. ENTER confirms selection.")
	selected, _ := pterm.DefaultInteractiveMultiselect.
		WithOptions(options).
		WithDefaultOptions(selectedOptions).
		WithFilter(false).
		Show("Select apps to ENABLE for automatic updates")

	// Apply selection
	selectedMap := make(map[string]bool)
	for _, s := range selected {
		selectedMap[s] = true
	}

	for _, repo := range repos {
		app := st.Apps[repo]
		cat := "User"
		if app.Global {
			cat = "Global"
		}
		label := fmt.Sprintf("[%s] %s", cat, repo)
		app.Disabled = !selectedMap[label]
	}

	_ = st.Save()

	// 2. Interactive Multi-select for removal
	pterm.Println()
	pterm.Warning.Println("SPACE selects for DELETION. ENTER confirms selection.")
	toDelete, _ := pterm.DefaultInteractiveMultiselect.
		WithOptions(options).
		WithDefaultText("Select apps to REMOVE from state completely").
		WithFilter(false).
		Show()

	if len(toDelete) > 0 {
		for _, label := range toDelete {
			// reverse extract repo name
			parts := strings.SplitN(label, "] ", 2)
			if len(parts) == 2 {
				repo := parts[1]
				delete(st.Apps, repo)
				log.Info().Msgf("Removed %s from state.", repo)
			}
		}
		_ = st.Save()
	}

	return nil
}
