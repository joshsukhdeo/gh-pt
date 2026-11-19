package compile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joshsukhdeo/gh-pt/ai"
	"github.com/joshsukhdeo/gh-pt/safety"
	"github.com/joshsukhdeo/gh-pt/state"
)

// Config holds the configuration for a compile operation.
type Config struct {
	Repository        string
	RepoDir           string
	BuildDir          string
	ScriptPath        string
	ManifestPath      string
	TargetPath        string
	SymlinkDir        string
	InstallDirTarget  string
	Track             string // "stable", "prerelease", "latest-commit"
	Version           string
	CompileScript     string
	Driver            string
	Plugin            string
	DriverMap         string
	IncludeSidecars   bool
	SidecarMode       string
	InstalledSidecars []string
}

// CompileResult contains the results of a compile operation.
type CompileResult struct {
	Version        string
	InstalledFiles []string
	Sidecars       []string
	Dependencies   []ai.Dependency
	CommitHash     string
}

// SaveCompileState persists compile metadata to state.
func SaveCompileState(cfg *Config, result *CompileResult) error {
	st, err := state.LoadState()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	// Create or update install map entry
	entry := st.GetInstallMap(cfg.Repository)
	if entry == nil {
		entry = &state.InstallMapEntry{}
	}

	// Record dependencies
	if len(result.Dependencies) > 0 {
		entry.Deps = &state.InstallStep{
			Pkg: &state.InstallPkg{
				Manager:   "multiple",
				PackageID: formatDependencies(result.Dependencies),
			},
		}
	}

	// Record installed files
	entry.Installed = &state.InstallStep{
		Files: result.InstalledFiles,
	}

	if err := st.SetInstallMap(cfg.Repository, entry); err != nil {
		return fmt.Errorf("failed to set install map: %w", err)
	}

	// Update app entry
	app, exists := st.Apps[cfg.Repository]
	if !exists {
		app = &state.InstalledApp{
			Repository: cfg.Repository,
			TargetPath: cfg.TargetPath,
		}
	}

	app.Version = result.Version
	app.CompileScript = cfg.ScriptPath
	app.SymlinkDir = cfg.SymlinkDir
	app.Sidecars = strings.Join(result.Sidecars, ",")
	app.Driver = cfg.Driver
	app.Plugin = cfg.Plugin
	app.DriverMap = cfg.DriverMap
	app.IncludeSidecars = cfg.IncludeSidecars
	app.SidecarMode = cfg.SidecarMode
	if len(result.Sidecars) > 0 {
		app.InstalledSidecars = result.Sidecars
	} else if len(cfg.InstalledSidecars) > 0 {
		app.InstalledSidecars = cfg.InstalledSidecars
	}

	st.Apps[cfg.Repository] = app

	return st.Save()
}

// formatDependencies creates a comma-separated list of dependencies.
func formatDependencies(deps []ai.Dependency) string {
	var names []string
	for _, dep := range deps {
		names = append(names, dep.Name)
	}
	return strings.Join(names, ", ")
}

// Clean removes the build directory and compile artifacts.
func Clean(repoDir string) error {
	if err := safety.RemoveAll(repoDir); err != nil {
		return fmt.Errorf("failed to remove build directory: %w", err)
	}
	return nil
}

// GetBuildPath returns the build directory for a repository.
func GetBuildPath(repo string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	parts := strings.Split(repo, "/")
	if len(parts) < 2 {
		return ""
	}
	repoName := parts[len(parts)-1]
	return filepath.Join(homeDir, "builds", repoName)
}

// ParseManifest reads and parses a manifest.json file.
func ParseManifest(manifestPath string) (*ai.Manifest, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest ai.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	return &manifest, nil
}

// WriteManifest writes a manifest to file.
func WriteManifest(manifestPath string, manifest *ai.Manifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(manifestPath), 0755); err != nil {
		return fmt.Errorf("failed to create manifest directory: %w", err)
	}

	return os.WriteFile(manifestPath, data, 0644)
}
