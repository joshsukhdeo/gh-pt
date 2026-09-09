import os
import re

def read_file(p):
    with open(p, 'r') as f: return f.read()

def write_file(p, c):
    with open(p, 'w') as f: f.write(c)

# 1. Update params.go
params_path = 'params/params.go'
if os.path.exists(params_path):
    c = read_file(params_path)
    c = c.replace(
        'RmSavedState         string            `help:"Remove a saved app from state by repository slug or binary name." group:"State Management"`',
        'RmSavedState         string            `help:"Remove a saved app from state tracking only (does NOT uninstall)." group:"State Management"`\n\tRm                   string            `help:"Uninstall an application and remove it from state." group:"State Management"`\n\tPurge                string            `help:"Completely uninstall an app, remove cached compile scripts, and remove from state." group:"State Management"`\n\tPin                  string            `help:"Pin a specific version in the saved state." group:"State Management"`'
    )
    c = c.replace(
        'Pin                  bool              `default:"false" help:"Pin this installation to the current version (skip during updates)." group:"State Management"`',
        'PinInstall           bool              `name:"pin-install" default:"false" help:"Pin this installation to the current version (skip during updates)." group:"State Management"`'
    )
    write_file(params_path, c)

# 2. Update release.go
release_path = 'release/release.go'
if os.path.exists(release_path):
    c = read_file(release_path)
    c = c.replace('Pinned:              r.CliParams.Pin,', 'Pinned:              r.CliParams.PinInstall,')
    write_file(release_path, c)

# 3. Update root.go
root_path = 'cmd/root.go'
if os.path.exists(root_path):
    c = read_file(root_path)
    c = c.replace(
        'if !r.Update && !r.UpdateAll && !r.ListSavedState && !r.EditSavedState && r.RmSavedState == "" {',
        'if !r.Update && !r.UpdateAll && !r.ListSavedState && !r.EditSavedState && r.RmSavedState == "" && r.Rm == "" && r.Purge == "" && r.Pin == "" {'
    )
    old_rm = '''\tif r.RmSavedState != "" {
		return RmState(r.RmSavedState)
	}'''
    new_rm = '''\tif r.RmSavedState != "" {
		return RmStateOnly(r.RmSavedState)
	}
	if r.Rm != "" {
		return RemoveApp(r.Rm, false)
	}
	if r.Purge != "" {
		return RemoveApp(r.Purge, true)
	}
	if r.Pin != "" {
		return PinAppState(r.Pin)
	}'''
    c = c.replace(old_rm, new_rm)
    write_file(root_path, c)

# 4. Update state_mgmt.go
state_mgmt_path = 'cmd/state_mgmt.go'
if os.path.exists(state_mgmt_path):
    c = read_file(state_mgmt_path)
    c = re.sub(r'func RmState\(target string\) error \{.*?(?=func EditState\(\) error \{)', '''func findTargetApps(st *state.State, target string) []string {
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
	if err != nil { return err }

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
	if err != nil { return err }

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
					cmd = exec.Command("sudo", "dpkg", "-r", pkgName)
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
		}

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
		}

		if purge && app.CompileScript != "" {
			if err := os.Remove(app.CompileScript); err != nil && !os.IsNotExist(err) {
				log.Warn().Err(err).Msgf("Failed to remove compile script %s", app.CompileScript)
			} else if err == nil {
				log.Info().Msgf("Purged compile script %s", app.CompileScript)
			}
		}

		delete(st.Apps, r)
		log.Info().Msgf("Removed %s from managed state.", r)
	}

	return st.Save()
}

func PinAppState(target string) error {
	st, err := state.LoadState()
	if err != nil { return err }

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

''', c, flags=re.DOTALL)
    write_file(state_mgmt_path, c)
