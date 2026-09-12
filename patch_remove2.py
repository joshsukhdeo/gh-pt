import sys

with open('cmd/state_mgmt.go', 'r') as f:
    content = f.read()

old_code = """
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
"""

new_code = """
			} else if app.TargetPath != "" && !app.Clone && !app.Fork {
				if len(app.InstalledBinaries) > 0 {
					for _, binName := range app.InstalledBinaries {
						binPath, err := safeDeletePath(app.TargetPath, binName)
						if err != nil {
							log.Warn().Err(err).Msgf("Skipping unsafe binary name %q", binName)
							continue
						}
						if err := os.Remove(binPath); err != nil && !os.IsNotExist(err) {
							log.Warn().Err(err).Msgf("Failed to remove binary %s", binPath)
						} else if err == nil {
							log.Info().Msgf("Deleted %s", binPath)
						}
					}
				} else {
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
			}
"""

new_content = content.replace(old_code.strip(), new_code.strip())

with open('cmd/state_mgmt.go', 'w') as f:
    f.write(new_content)
