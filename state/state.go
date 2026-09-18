package state

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/gofrs/flock"
)

type InstalledApp struct {
	Repository               string            `json:"repository"`
	TargetPath               string            `json:"target_path"`
	Global                   bool              `json:"global"`
	ReleaseAsset             string            `json:"release_asset"`
	ReleaseRegexp            string            `json:"release_regexp"`
	Version                  string            `json:"version"`
	Rename                   map[string]string `json:"target_binaries"`
	Disabled                 bool              `json:"disabled"`
	Type                     []string          `json:"type"`
	All                      bool              `json:"all"`
	AssetBinaries            []string          `json:"asset_binaries"`
	AssetBinariesRegexp      string            `json:"asset_binaries_regexp"`
	InstalledBinaries        []string          `json:"installed_binaries,omitempty"`
	InstalledAssetNames      []string          `json:"installed_asset_names,omitempty"`
	InstalledAssetsFullNames []string          `json:"installed_assets_full_names,omitempty"`
	ContainingArchive        string            `json:"containing_archive,omitempty"`
	PackageNames             []string          `json:"package_names,omitempty"`
	Pinned                   bool              `json:"pinned,omitempty"`
	Extractor                string            `json:"extractor,omitempty"`
	Clone                    bool              `json:"clone,omitempty"`
	Fork                     bool              `json:"fork,omitempty"`
	MaxDepth                 int               `json:"max_depth,omitempty"`
	CompileScript            string            `json:"compile_script,omitempty"`
	IsPrerelease             bool              `json:"is_prerelease,omitempty"`
	LastVTScan               string            `json:"last_vt_scan,omitempty"`
	LastAIScan               string            `json:"last_ai_scan,omitempty"`
	SymlinkDir               string            `json:"symlink_dir,omitempty"`
	Hooks                    []string          `json:"hooks,omitempty"`
	SystemPackages           []string          `json:"system_packages,omitempty"`
	Sidecars                 []string          `json:"sidecars,omitempty"`
	SidecarTargetPath        string            `json:"sidecar_target_path,omitempty"`
	SidecarSymlinkTo         []string          `json:"sidecar_symlink_to,omitempty"`
	IncludeSidecars          bool              `json:"include_sidecars,omitempty"`
	InstalledSidecars        []string          `json:"installed_sidecars,omitempty"`
	EnvInject                []string          `json:"env_inject,omitempty"`
	FallbackReleases         int               `json:"fallback_releases,omitempty"`
}

type StateManager interface {
	Save() error
	AddApp(app *InstalledApp) error
}

type State struct {
	Version        int                      `json:"version,omitempty"`
	Apps           map[string]*InstalledApp `json:"apps"`
	Repos          map[string]*InstalledApp `json:"repos,omitempty"`
	SystemPackages []string                 `json:"system_packages,omitempty"`
	Hooks          map[string]string        `json:"hooks,omitempty"`
}

var _ StateManager = (*State)(nil)

func GetStatePath() string {
	return filepath.Join(xdg.DataHome, "gh-pt", "state.json")
}

func (s *State) migrateV1toV2() error {
	if s.Version >= 2 {
		return nil
	}
	s.Version = 2
	if s.Apps == nil {
		s.Apps = make(map[string]*InstalledApp)
	}
	if s.Repos == nil {
		s.Repos = make(map[string]*InstalledApp)
	}
	if s.Hooks == nil {
		s.Hooks = make(map[string]string)
	}
	if s.SystemPackages == nil {
		s.SystemPackages = []string{}
	}

	sysPkgSet := make(map[string]bool)
	for _, pkg := range s.SystemPackages {
		sysPkgSet[pkg] = true
	}

	// Segregate apps and repos
	for repoName, app := range s.Apps {
		if len(app.PackageNames) > 0 && len(app.SystemPackages) == 0 {
			app.SystemPackages = append([]string{}, app.PackageNames...)
		}
		for _, pkg := range app.SystemPackages {
			if !sysPkgSet[pkg] {
				sysPkgSet[pkg] = true
				s.SystemPackages = append(s.SystemPackages, pkg)
			}
		}

		if app.Clone || app.Fork {
			s.Repos[repoName] = app
			delete(s.Apps, repoName)
		}
	}

	for _, repo := range s.Repos {
		if len(repo.PackageNames) > 0 && len(repo.SystemPackages) == 0 {
			repo.SystemPackages = append([]string{}, repo.PackageNames...)
		}
		for _, pkg := range repo.SystemPackages {
			if !sysPkgSet[pkg] {
				sysPkgSet[pkg] = true
				s.SystemPackages = append(s.SystemPackages, pkg)
			}
		}
	}

	return nil
}

func migrateV1toV2(states ...*State) error {
	if len(states) == 0 {
		st, err := LoadState()
		if err != nil {
			return err
		}
		if err := st.migrateV1toV2(); err != nil {
			return err
		}
		return st.Save()
	}
	return states[0].migrateV1toV2()
}

func LoadState() (*State, error) {
	path := GetStatePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &State{
				Version:        2,
				Apps:           make(map[string]*InstalledApp),
				Repos:          make(map[string]*InstalledApp),
				Hooks:          make(map[string]string),
				SystemPackages: []string{},
			}, nil
		}
		return nil, err
	}

	var s State
	err = json.Unmarshal(data, &s)
	if err != nil {
		return nil, err
	}
	if s.Apps == nil {
		s.Apps = make(map[string]*InstalledApp)
	}
	if s.Repos == nil {
		s.Repos = make(map[string]*InstalledApp)
	}
	if s.Hooks == nil {
		s.Hooks = make(map[string]string)
	}
	if s.SystemPackages == nil {
		s.SystemPackages = []string{}
	}

	if s.Version < 2 {
		// Create a durable backup of the V1 state before migrating
		if f, err := os.Create(path + ".v1.bak"); err == nil {
			_, _ = f.Write(data)
			_ = f.Sync()
			_ = f.Close()
		}
		_ = s.migrateV1toV2()
	}

	return &s, nil
}

func (s *State) Save() error {
	path := GetStatePath()
	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return err
	}

	lock := flock.New(path + ".lock")
	if err := lock.Lock(); err != nil {
		return err
	}
	defer func() { _ = lock.Unlock() }()

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (s *State) AddApp(app *InstalledApp) error {
	if s.Apps == nil {
		s.Apps = make(map[string]*InstalledApp)
	}
	s.Apps[app.Repository] = app
	return s.Save()
}
