package config

import (
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Paths PathsConfig `yaml:",inline"`
	AI    AIConfig    `yaml:",inline"`
	Core  CoreConfig  `yaml:",inline"`
}

type PathsConfig struct {
	InstallPath string `yaml:"install_path"`
	GlobalPath  string `yaml:"global_path"`
	ClonePath   string `yaml:"clone_path"`
	ForkPath    string `yaml:"fork_path"`
}

type AIConfig struct {
	AICmd            string `yaml:"ai_cmd"`
	AIInteractiveCmd string `yaml:"ai_interactive_cmd"`
}

type CoreConfig struct {
	InstallTypes    string `yaml:"install_types"`
	AddDeps         bool   `yaml:"add_deps"`
	NoDeps          bool   `yaml:"no_deps"`
	DisablePrompts  bool   `yaml:"disable_prompts"`
	NoSaveState     bool   `yaml:"no_save_state"`
	Wine            string `yaml:"wine"`
	Extractor       string `yaml:"extractor"`
	KeepSuffixes    bool   `yaml:"keep_suffixes"`
	VTApiKey        string `yaml:"vt_api_key"`
	AllowPrerelease bool   `yaml:"allow_prerelease"`
	DisableIcons    bool   `yaml:"disable_icons"`
	LogToFile       bool   `yaml:"log_to_file"`
	Symlink         bool   `yaml:"symlink"`
}

func GetConfigPath() string {
	return filepath.Join(xdg.ConfigHome, "gh-pt", "config.yml")
}

func LoadConfig() (*Config, error) {
	path := GetConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}

	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func SaveConfig(cfg *Config) error {
	path := GetConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
