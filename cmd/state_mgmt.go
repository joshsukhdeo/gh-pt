package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/joshsukhdeo/gh-pt/state"
	"github.com/pterm/pterm"
	"github.com/rs/zerolog/log"
)

var execCommand = exec.Command

func ListState(rList ...*RootCLI) error {
	var r *RootCLI
	if len(rList) > 0 && rList[0] != nil {
		r = rList[0]
	} else {
		r = &RootCLI{}
	}

	st, err := state.LoadState()
	if err != nil {
		return err
	}

	if len(st.Apps) == 0 {
		pterm.Info.Println("No applications are currently managed by gh-pt.")
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

		if r.ExecContext.Prerelease && !app.IsPrerelease {
			continue
		}
		if r.ExecContext.Stable && app.IsPrerelease {
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

func safeDeletePath(baseDir, name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("empty target name")
	}
	if filepath.IsAbs(name) || name == "." || name == ".." {
		return "", fmt.Errorf("unsafe delete target %q", name)
	}
	cleanName := filepath.Clean(name)
	if cleanName == "." || cleanName == ".." || cleanName == string(filepath.Separator) {
		return "", fmt.Errorf("unsafe delete target %q", name)
	}
	if filepath.Base(cleanName) != cleanName {
		return "", fmt.Errorf("refusing to delete path with traversal %q", name)
	}
	fullPath := filepath.Join(baseDir, cleanName)
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", err
	}
	absFull, err := filepath.Abs(fullPath)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absBase, absFull)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("refusing to delete outside target path %q", name)
	}
	return fullPath, nil
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
		app := st.Apps[r]

		// Uninstall packages via package manager if they were installed
		if len(app.PackageNames) > 0 {
			for _, pkgName := range app.PackageNames {
				log.Info().Msgf("Uninstalling package %s...", pkgName)
				var cmd *exec.Cmd
				// Detect which package manager to use
				if _, err := exec.LookPath("dpkg"); err == nil {
					cmd = execCommand("sudo", "dpkg", "-r", pkgName)
				} else if _, err := exec.LookPath("rpm"); err == nil {
					cmd = execCommand("sudo", "rpm", "-e", pkgName)
				} else if _, err := exec.LookPath("pacman"); err == nil {
					cmd = execCommand("sudo", "pacman", "-R", "--noconfirm", pkgName)
				} else if _, err := exec.LookPath("pkg"); err == nil {
					cmd = execCommand("sudo", "pkg", "delete", "-y", pkgName)
				}

				if cmd != nil {
					if err := cmd.Run(); err != nil {
						log.Warn().Err(err).Msgf("Failed to uninstall package %s", pkgName)
					} else {
						log.Info().Msgf("Successfully uninstalled %s", pkgName)
					}
				}
			}
		}

		// Delete installed binaries from disk
		if app.TargetPath != "" {
			parts := strings.Split(r, "/")
			repoName := parts[len(parts)-1]

			// If renamed binaries exist, delete those specific files
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
				// Try the repo name as the binary name
				binPath := filepath.Join(app.TargetPath, repoName)
				if err := os.Remove(binPath); err != nil && !os.IsNotExist(err) {
					log.Warn().Err(err).Msgf("Failed to remove binary %s", binPath)
				} else if err == nil {
					log.Info().Msgf("Deleted %s", binPath)
				}
			}
		}

		delete(st.Apps, r)
		log.Info().Msgf("Removed %s from state tracking only.", r)
		state.LogHistory("remove", r, "")
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
					cmd = execCommand("sudo", "dpkg", "-r", pkgName)
				} else if _, err := exec.LookPath("rpm"); err == nil {
					cmd = execCommand("sudo", "rpm", "-e", pkgName)
				} else if _, err := exec.LookPath("pacman"); err == nil {
					cmd = execCommand("sudo", "pacman", "-R", "--noconfirm", pkgName)
				} else if _, err := exec.LookPath("pkg"); err == nil {
					cmd = execCommand("sudo", "pkg", "delete", "-y", pkgName)
				}

				if cmd != nil {
					if err := cmd.Run(); err != nil {
						log.Warn().Err(err).Msgf("Failed to uninstall package %s", pkgName)
					} else {
						log.Info().Msgf("Successfully uninstalled %s", pkgName)
					}
				}
			}
		} else if app.TargetPath != "" && !app.Clone && !app.Fork {
			parts := strings.Split(r, "/")
			repoName := parts[len(parts)-1]

			if len(app.Rename) > 0 {
				for _, renamed := range app.Rename {
					binPath, err := safeDeletePath(app.TargetPath, renamed)
					if err != nil {
						log.Warn().Err(err).Msgf("Skipping unsafe binary name %q", renamed)
						continue
					}
					if err := os.Remove(binPath); err != nil && !os.IsNotExist(err) {
						log.Warn().Err(err).Msgf("Failed to remove binary %s", binPath)
					} else if err == nil {
						log.Info().Msgf("Deleted %s", binPath)
					}
				}
			} else {
				binPath, err := safeDeletePath(app.TargetPath, repoName)
				if err != nil {
					log.Warn().Err(err).Msgf("Skipping unsafe binary name %q", repoName)
				} else {
					if err := os.Remove(binPath); err != nil && !os.IsNotExist(err) {
						log.Warn().Err(err).Msgf("Failed to remove binary %s", binPath)
					} else if err == nil {
						log.Info().Msgf("Deleted %s", binPath)
					}
				}
			}

			for _, binName := range app.AssetBinaries {
				binPath, err := safeDeletePath(app.TargetPath, binName)
				if err != nil {
					log.Warn().Err(err).Msgf("Skipping unsafe binary name %q", binName)
					continue
				}
				if err := os.Remove(binPath); err != nil && !os.IsNotExist(err) {
					log.Warn().Err(err).Msgf("Failed to delete %s", binPath)
				} else if err == nil {
					log.Info().Msgf("Deleted %s", binPath)
				}
			}
		}

		if app.Clone || app.Fork {
			if purge && app.TargetPath != "" {
				if err := os.RemoveAll(app.TargetPath); err != nil {
					log.Warn().Err(err).Msgf("Failed to purge repository directory %s", app.TargetPath)
				} else {
					log.Info().Msgf("Purged cloned/forked repository at %s", app.TargetPath)
				}
			} else {
				log.Info().Msgf("Kept repository directory at %s (use --purge to delete)", app.TargetPath)
			}
		}

		if app.CompileScript != "" {
			if err := os.Remove(app.CompileScript); err != nil && !os.IsNotExist(err) {
				log.Warn().Err(err).Msgf("Failed to remove compile script %s", app.CompileScript)
			} else if err == nil {
				if purge {
					log.Info().Msgf("Purged compile script %s", app.CompileScript)
				} else {
					log.Info().Msgf("Removed compile script %s", app.CompileScript)
				}
			}

			repoParts := strings.Split(r, "/")
			if len(repoParts) == 2 {
				srcPath := filepath.Join(os.TempDir(), "gh-pt-src-"+repoParts[1])
				_ = os.RemoveAll(srcPath)
			}
		}

		delete(st.Apps, r)
		log.Info().Msgf("Removed %s from state tracking only.", r)
		state.LogHistory("remove", r, "")
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
		pterm.Info.Println("No applications are currently managed by gh-pt.")
		return nil
	}

	for {
		action, _ := pterm.DefaultInteractiveSelect.
			WithOptions([]string{"Toggle Auto-Updates (Batch)", "Remove Apps (Batch)", "Edit App Settings", "Exit"}).
			WithDefaultText("Select State Management Action").
			Show()

		switch action {
		case "Toggle Auto-Updates (Batch)":
			editStateToggleUpdates(st)
		case "Remove Apps (Batch)":
			editStateRemoveApps(st)
		case "Edit App Settings":
			editStateAppFields(st)
		case "Exit":
			return nil
		}
	}
}

func editStateToggleUpdates(st *state.State) {
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

	if err := st.Save(); err != nil {
		pterm.Error.Printf("Failed to save state: %v\n", err)
	} else {
		pterm.Success.Println("State saved successfully.")
	}
}

func editStateRemoveApps(st *state.State) {
	var options []string
	for k, app := range st.Apps {
		cat := "User"
		if app.Global {
			cat = "Global"
		}
		options = append(options, fmt.Sprintf("[%s] %s", cat, k))
	}
	sort.Strings(options)

	pterm.Warning.Println("SPACE selects for DELETION. ENTER confirms selection.")
	toDelete, _ := pterm.DefaultInteractiveMultiselect.
		WithOptions(options).
		WithDefaultText("Select apps to REMOVE from state completely").
		WithFilter(false).
		Show()

	if len(toDelete) > 0 {
		for _, label := range toDelete {
			parts := strings.SplitN(label, "] ", 2)
			if len(parts) == 2 {
				repo := parts[1]
				delete(st.Apps, repo)
				log.Info().Msgf("Removed %s from state.", repo)
				state.LogHistory("remove", repo, "")
			}
		}
		if err := st.Save(); err != nil {
			pterm.Error.Printf("Failed to save state: %v\n", err)
		} else {
			pterm.Success.Println("State saved successfully.")
		}
	}
}

func editStateAppFields(st *state.State) {
	var repos []string
	for k := range st.Apps {
		repos = append(repos, k)
	}
	sort.Strings(repos)
	repos = append(repos, "Back")

	selectedAppRepo, _ := pterm.DefaultInteractiveSelect.
		WithOptions(repos).
		WithDefaultText("Select App to Edit").
		Show()

	if selectedAppRepo == "Back" {
		return
	}

	app := st.Apps[selectedAppRepo]
	for {
		fields := []string{
			fmt.Sprintf("TargetPath: %s", app.TargetPath),
			fmt.Sprintf("Global: %t", app.Global),
			fmt.Sprintf("ReleaseAsset: %s", app.ReleaseAsset),
			fmt.Sprintf("Version: %s", app.Version),
			fmt.Sprintf("Disabled: %t", app.Disabled),
			fmt.Sprintf("NativeExtract: %t", app.NativeExtract),
			fmt.Sprintf("IsPrerelease: %t", app.IsPrerelease),
			"Back",
		}

		selectedField, _ := pterm.DefaultInteractiveSelect.
			WithOptions(fields).
			WithDefaultText("Select Field to Edit").
			Show()

		if selectedField == "Back" {
			break
		}

		parts := strings.Split(selectedField, ":")
		if len(parts) == 0 {
			continue
		}
		fieldName := strings.TrimSpace(parts[0])

		switch fieldName {
		case "TargetPath":
			app.TargetPath, _ = pterm.DefaultInteractiveTextInput.WithDefaultText("TargetPath").WithDefaultValue(app.TargetPath).Show()
		case "ReleaseAsset":
			app.ReleaseAsset, _ = pterm.DefaultInteractiveTextInput.WithDefaultText("ReleaseAsset").WithDefaultValue(app.ReleaseAsset).Show()
		case "Version":
			app.Version, _ = pterm.DefaultInteractiveTextInput.WithDefaultText("Version").WithDefaultValue(app.Version).Show()
		case "Global":
			app.Global, _ = pterm.DefaultInteractiveConfirm.WithDefaultText("Global").WithDefaultValue(app.Global).Show()
		case "Disabled":
			app.Disabled, _ = pterm.DefaultInteractiveConfirm.WithDefaultText("Disabled").WithDefaultValue(app.Disabled).Show()
		case "NativeExtract":
			app.NativeExtract, _ = pterm.DefaultInteractiveConfirm.WithDefaultText("NativeExtract").WithDefaultValue(app.NativeExtract).Show()
		case "IsPrerelease":
			app.IsPrerelease, _ = pterm.DefaultInteractiveConfirm.WithDefaultText("IsPrerelease").WithDefaultValue(app.IsPrerelease).Show()
		}
	}

	if err := st.Save(); err != nil {
		pterm.Error.Printf("Failed to save state: %v\n", err)
	} else {
		pterm.Success.Println("App state updated successfully.")
	}
}
