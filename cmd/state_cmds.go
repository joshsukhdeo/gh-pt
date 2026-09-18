package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/joshsukhdeo/gh-pt/state"
	"github.com/pterm/pterm"
)

// StateCat dumps the raw state file to stdout
func StateCat() error {
	path := state.GetStatePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("state file does not exist at %s", path)
		}
		return fmt.Errorf("failed to read state file: %w", err)
	}

	fmt.Print(string(data))
	return nil
}

// StateView returns the state file path
func StateView() error {
	path := state.GetStatePath()
	fmt.Println(path)
	return nil
}

// StateAdd adds a new state entry for a repository
func StateAdd(repo string, app *state.InstalledApp, force bool) error {
	st, err := state.LoadState()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	// Check if already exists
	if _, exists := st.Apps[repo]; exists && !force {
		return fmt.Errorf("saved state already exists for %s (use --force to overwrite)", repo)
	}

	app.Repository = repo
	if err := st.AddApp(app); err != nil {
		return fmt.Errorf("failed to add state: %w", err)
	}

	pterm.Success.Printf("Added state for %s\n", repo)
	return nil
}

// StateRm removes a state entry with confirmation
func StateRm(target string, force bool) error {
	st, err := state.LoadState()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	// Find the app by repo or binary name
	var foundKey string
	var foundApp *state.InstalledApp
	for key, app := range st.Apps {
		if key == target || strings.EqualFold(key, target) {
			foundKey = key
			foundApp = app
			break
		}
		// Check if target matches any installed binary
		for _, bin := range app.InstalledBinaries {
			if bin == target {
				foundKey = key
				foundApp = app
				break
			}
		}
		if foundKey != "" {
			break
		}
	}

	if foundKey == "" {
		return fmt.Errorf("no state entry found for %s", target)
	}

	// Confirmation prompt unless forced
	if !force {
		confirmed, err := pterm.DefaultInteractiveConfirm.
			WithDefaultValue(false).
			Show(fmt.Sprintf("Remove state for %s?", foundKey))
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println("Cancelled")
			return nil
		}
	}

	delete(st.Apps, foundKey)
	if err := st.Save(); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	pterm.Success.Printf("Removed state for %s\n", foundKey)
	if foundApp != nil && len(foundApp.InstalledBinaries) > 0 {
		pterm.Info.Printf("Note: Installed binaries not removed: %s\n", strings.Join(foundApp.InstalledBinaries, ", "))
	}

	return nil
}

// StateUpdate updates specific fields of a state entry
func StateUpdate(target string, updates map[string]interface{}) error {
	st, err := state.LoadState()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	// Find the app
	var foundKey string
	var foundApp *state.InstalledApp
	for key, app := range st.Apps {
		if key == target || strings.EqualFold(key, target) {
			foundKey = key
			foundApp = app
			break
		}
	}

	if foundKey == "" {
		return fmt.Errorf("no state entry found for %s", target)
	}

	// Apply updates
	for field, value := range updates {
		switch field {
		case "target_path":
			if v, ok := value.(string); ok {
				foundApp.TargetPath = v
			}
		case "release_asset":
			if v, ok := value.(string); ok {
				foundApp.ReleaseAsset = v
			}
		case "release_regexp":
			if v, ok := value.(string); ok {
				foundApp.ReleaseRegexp = v
			}
		case "version":
			if v, ok := value.(string); ok {
				foundApp.Version = v
			}
		case "extractor":
			if v, ok := value.(string); ok {
				foundApp.Extractor = v
			}
		case "compile_script":
			if v, ok := value.(string); ok {
				foundApp.CompileScript = v
			}
		case "asset_binaries_regexp":
			if v, ok := value.(string); ok {
				foundApp.AssetBinariesRegexp = v
			}
		case "containing_archive":
			if v, ok := value.(string); ok {
				foundApp.ContainingArchive = v
			}
		case "type":
			if v, ok := value.([]string); ok {
				foundApp.Type = v
			}
		case "asset_binaries":
			if v, ok := value.([]string); ok {
				foundApp.AssetBinaries = v
			}
		case "installed_asset_names":
			if v, ok := value.([]string); ok {
				foundApp.InstalledAssetNames = v
			}
		case "installed_assets_full_names":
			if v, ok := value.([]string); ok {
				foundApp.InstalledAssetsFullNames = v
			}
		case "rename":
			if v, ok := value.(map[string]string); ok {
				foundApp.Rename = v
			}
		case "max_depth":
			if v, ok := value.(int); ok {
				foundApp.MaxDepth = v
			}
		case "global":
			if v, ok := value.(bool); ok {
				foundApp.Global = v
			}
		case "disabled":
			if v, ok := value.(bool); ok {
				foundApp.Disabled = v
			}
		case "all":
			if v, ok := value.(bool); ok {
				foundApp.All = v
			}
		case "pinned":
			if v, ok := value.(bool); ok {
				foundApp.Pinned = v
			}
		case "clone":
			if v, ok := value.(bool); ok {
				foundApp.Clone = v
			}
		case "fork":
			if v, ok := value.(bool); ok {
				foundApp.Fork = v
			}
		case "is_prerelease":
			if v, ok := value.(bool); ok {
				foundApp.IsPrerelease = v
			}
		default:
			pterm.Warning.Printf("Unknown field: %s\n", field)
		}
	}

	if err := st.Save(); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	pterm.Success.Printf("Updated state for %s\n", foundKey)
	return nil
}

// StateEdit opens an interactive form to edit state entries
func StateEdit() error {
	st, err := state.LoadState()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	if len(st.Apps) == 0 {
		pterm.Info.Println("No applications in state")
		return nil
	}

	// Build list of repos
	var repos []string
	for repo := range st.Apps {
		repos = append(repos, repo)
	}

	// Select repo to edit
	var selectedRepo string
	if err := huh.NewSelect[string]().
		Title("Select application to edit").
		Options(huh.NewOptions(repos...)...).
		Value(&selectedRepo).
		Run(); err != nil {
		return err
	}

	app := st.Apps[selectedRepo]

	// Prepare form fields
	var targetPath = app.TargetPath
	var releaseAsset = app.ReleaseAsset
	var releaseRegexp = app.ReleaseRegexp
	var version = app.Version
	var extractor = app.Extractor
	var compileScript = app.CompileScript
	var assetBinariesRegexp = app.AssetBinariesRegexp
	var typeStr = strings.Join(app.Type, ",")
	var assetBinariesStr = strings.Join(app.AssetBinaries, ",")
	var renameStr = mapToString(app.Rename)
	var maxDepthStr = strconv.Itoa(app.MaxDepth)
	var installedAssetNamesStr = strings.Join(app.InstalledAssetNames, ",")
	var installedAssetsFullNamesStr = strings.Join(app.InstalledAssetsFullNames, ",")
	var containingArchive = app.ContainingArchive

	var global = app.Global
	var disabled = app.Disabled
	var all = app.All
	var pinned = app.Pinned
	var clone = app.Clone
	var fork = app.Fork
	var isPrerelease = app.IsPrerelease

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Target Path").
				Value(&targetPath),
			huh.NewInput().
				Title("Release Asset").
				Value(&releaseAsset),
			huh.NewInput().
				Title("Release Regexp").
				Value(&releaseRegexp),
			huh.NewInput().
				Title("Version").
				Value(&version),
			huh.NewInput().
				Title("Extractor").
				Value(&extractor),
			huh.NewInput().
				Title("Compile Script").
				Value(&compileScript),
			huh.NewInput().
				Title("Asset Binaries Regexp").
				Value(&assetBinariesRegexp),
			huh.NewInput().
				Title("Types (comma-separated)").
				Value(&typeStr),
			huh.NewInput().
				Title("Asset Binaries (comma-separated)").
				Value(&assetBinariesStr),
			huh.NewInput().
				Title("Rename Map (key1=val1,key2=val2)").
				Value(&renameStr),
			huh.NewInput().
				Title("Max Depth").
				Value(&maxDepthStr),
			huh.NewInput().
				Title("Installed Asset Names (comma-separated)").
				Value(&installedAssetNamesStr),
			huh.NewInput().
				Title("Installed Assets Full Names (comma-separated)").
				Value(&installedAssetsFullNamesStr),
			huh.NewInput().
				Title("Containing Archive").
				Value(&containingArchive),
		),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Global").
				Value(&global),
			huh.NewConfirm().
				Title("Disabled").
				Value(&disabled),
			huh.NewConfirm().
				Title("All").
				Value(&all),
			huh.NewConfirm().
				Title("Pinned").
				Value(&pinned),
			huh.NewConfirm().
				Title("Clone").
				Value(&clone),
			huh.NewConfirm().
				Title("Fork").
				Value(&fork),
			huh.NewConfirm().
				Title("Is Prerelease").
				Value(&isPrerelease),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	// Apply changes
	app.TargetPath = targetPath
	app.ReleaseAsset = releaseAsset
	app.ReleaseRegexp = releaseRegexp
	app.Version = version
	app.Extractor = extractor
	app.CompileScript = compileScript
	app.AssetBinariesRegexp = assetBinariesRegexp
	app.Type = parseCommaSeparated(typeStr)
	app.AssetBinaries = parseCommaSeparated(assetBinariesStr)
	app.Rename = parseMap(renameStr)
	if depth, err := strconv.Atoi(maxDepthStr); err == nil {
		app.MaxDepth = depth
	}
	app.InstalledAssetNames = parseCommaSeparated(installedAssetNamesStr)
	app.InstalledAssetsFullNames = parseCommaSeparated(installedAssetsFullNamesStr)
	app.ContainingArchive = containingArchive

	app.Global = global
	app.Disabled = disabled
	app.All = all
	app.Pinned = pinned
	app.Clone = clone
	app.Fork = fork
	app.IsPrerelease = isPrerelease

	if err := st.Save(); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	pterm.Success.Printf("Updated state for %s\n", selectedRepo)
	return nil
}

// Helper functions for map/slice conversion
func mapToString(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	var pairs []string
	for k, v := range m {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(pairs, ",")
}

func parseMap(s string) map[string]string {
	if s == "" {
		return nil
	}
	m := make(map[string]string)
	pairs := strings.Split(s, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			m[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return m
}

func parseCommaSeparated(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// GetStateFields returns list of editable fields for autocomplete
func GetStateFields() []string {
	return []string{
		"target_path",
		"release_asset",
		"release_regexp",
		"version",
		"extractor",
		"compile_script",
		"asset_binaries_regexp",
		"type",
		"asset_binaries",
		"rename",
		"max_depth",
		"installed_asset_names",
		"installed_assets_full_names",
		"containing_archive",
		"global",
		"disabled",
		"all",
		"pinned",
		"clone",
		"fork",
		"is_prerelease",
		"sidecars",
		"sidecar_target_path",
		"installed_sidecars",
	}
}

// ParseStateUpdate parses field=value pairs for state update
func ParseStateUpdate(pairs []string) (map[string]interface{}, error) {
	updates := make(map[string]interface{})

	for _, pair := range pairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid format: %s (expected field=value)", pair)
		}

		field := parts[0]
		value := parts[1]

		// Parse value based on field type
		switch field {
		case "target_path", "release_asset", "release_regexp", "version", "extractor", "compile_script", "asset_binaries_regexp", "containing_archive", "sidecar_target_path":
			updates[field] = value
		case "type", "asset_binaries", "installed_asset_names", "installed_assets_full_names", "sidecars":
			updates[field] = parseCommaSeparated(value)
		case "rename":
			updates[field] = parseMap(value)
		case "max_depth":
			if v, err := strconv.Atoi(value); err == nil {
				updates[field] = v
			} else {
				return nil, fmt.Errorf("invalid max_depth value: %s", value)
			}
		case "global", "disabled", "all", "pinned", "clone", "fork", "is_prerelease":
			updates[field] = strings.ToLower(value) == "true" || value == "1"
		default:
			return nil, fmt.Errorf("unknown field: %s", field)
		}
	}

	return updates, nil
}

// DumpStateJSON returns pretty-printed JSON of state
func DumpStateJSON() (string, error) {
	st, err := state.LoadState()
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}
