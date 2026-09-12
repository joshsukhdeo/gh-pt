package params

import (
	"os"
	"os/exec"

	"github.com/alecthomas/kong"
)

type CLI struct {
	// Subcommands
	Install     InstallCmd     `cmd:"" default:"withargs" help:"Install a GitHub release or clone a repository (default)."`
	Ls          StateLsCmd     `cmd:"" help:"List saved state (short format)."`
	Ll          StateLlCmd     `cmd:"" help:"List saved state (long format)."`
	Rm          StateRmCmd     `cmd:"" help:"Uninstall an application and remove it from state."`
	Upgrade     UpgradeCmd     `cmd:"" help:"Update installations."`
	State       StateCmd       `cmd:"" help:"Manage saved state and installations."`
	Config      ConfigCmd      `cmd:"" help:"Manage configuration."`
	Repo        RepoCmd        `cmd:"" help:"Manage source repositories."`
	Scan        ScanCmd        `cmd:"" help:"Security and AI scanning."`
	Vt          VtCmd          `cmd:"" help:"VirusTotal integration."`
	Show        ShowCmd        `cmd:"" help:"Show release information."`
	Source      SourceCmd      `cmd:"" help:"Compile repository from source."`
	Completions CompletionsCmd `cmd:"" help:"Generate shell completions."`
	Search      SearchCmd      `cmd:"" help:"Search for repositories on GitHub."`

	// Global flags
	LogLevel            string           `default:"info" enum:"error,warn,info,debug" short:"l" help:"Log level."`
	LogFormat           string           `default:"console" enum:"console,json" help:"Log output format."`
	LogQuietInteractive bool             `default:"true" negatable:"" help:"Quiet log in interactive mode"`
	Verbose             bool             `short:"V" help:"Enable verbose output (sets log level to debug)."`
	Version             kong.VersionFlag `help:"Show version." env:""`
}

// Shared flags across commands
type CommonInstallFlags struct {
	Interactive          bool              `default:"false" short:"i" help:"Use interactive installation."`
	ReleaseVersion       string            `default:"latest" short:"v" help:"Repository release tag (version) to install."`
	ReleaseAsset         string            `optional:"" short:"a" help:"Name of repository release asset to download."`
	ReleaseAssetRegexp   string            `optional:"" short:"A" help:"Regular expression matching release asset to download."`
	ReleaseAssetRegexps  []string          `kong:"-"`
	Type                 []string          `default:"${install_types}" short:"T" name:"format" env:"GH_PT_TYPE" help:"Comma-separated list of types to match and prioritize."`
	All                  bool              `default:"false" help:"Install all matched assets instead of just the first one."`
	AssetBinaries        []string          `optional:"" short:"b" help:"If release asset is an archive - names of a binaries in the archive to install."`
	AssetBinariesRegexp  string            `optional:"" short:"B" help:"If release asset is an archive - regular expression matching binaries in the archive to install."`
	TargetPath           string            `default:"${install_path}" short:"p" type:"path" help:"Target installation directory (default: ~/.local/bin or /usr/local/bin if --global)."`
	Global               bool              `short:"g" help:"Install globally (e.g. /usr/local/bin) instead of user bin."`
	AddDeps              bool              `short:"y" help:"Automatically resolve and install dependencies without prompting."`
	NoDeps               bool              `short:"n" help:"Do not install dependencies."`
	Rename               map[string]string `optional:"" short:"t" help:"Rename binaries installed at target path."`
	KeepSuffixes         bool              `short:"k" help:"Keep OS/hardware suffixes on extracted binaries."`
	DisablePrompts       bool              `short:"D" env:"GH_PT_DISABLE_PROMPTS" help:"Disable all interactive prompts."`
	NoSaveState          bool              `short:"S" env:"GH_PT_NO_SAVE_STATE" help:"Do not save installation to state."`
	Wine                 string            `default:"off" enum:"force,priority,allow,off" env:"GH_PT_WINE" help:"Wine mode."`
	AllowForeignArch     bool              `env:"GH_PT_ALLOW_FOREIGN_ARCH" help:"Allow installing assets with foreign architectures."`
	AllowRootUserInstall bool              `help:"Allow installation to user-local paths when running as root."`
	Extractor            string            `env:"GH_PT_EXTRACTOR" help:"Archive extractor precedence (default, ouch, native, internal)." default:"${extractor}"`
	TargetPathCreate     bool              `default:"true" negatable:"" help:"Create target installation directory if it does not exist."`
	Overwrite            bool              `default:"false" short:"f" name:"force" aliases:"overwrite" help:"Overwrite target binaries."`
	PinInstall           bool              `name:"pin-install" default:"false" help:"Pin this installation to the current version."`
	DryRun               bool              `default:"false" help:"Show what would be downloaded."`
	VerifyChecksum       bool              `default:"true" help:"Verify asset checksums."`
	SkipVtSandbox        bool              `help:"Bypass VirusTotal sandbox upload for unknown zero-day hashes."`
	Prerelease           bool              `help:"Include prereleases for install, updates, and list filters."`
	Stable               bool              `help:"Include only stable releases."`
	AI                   bool              `help:"Enable AI-assisted installation."`
}

type InstallCmd struct {
	Repository string `arg:"" env:"GH_PT_REPOSITORY" optional:"" predictor:"github_repos" predict:"github_repos" help:"Github repository in OWNER/REPOSITORY_NAME format."`
	CommonInstallFlags
}

type StateCmd struct {
	Edit StateEditCmd `cmd:"" help:"Edit saved state interactively."`
}

type UpgradeCmd struct {
	Repository string `arg:"" optional:"" predictor:"installed_apps" predict:"installed_apps" help:"Optional repository to update."`
	User       bool   `short:"u" name:"user" help:"Update only user installations."`
	Global     bool   `short:"g" name:"global" help:"Update only global installations."`
}

type StateLsCmd struct {
	Filter string `arg:"" optional:"" help:"Optional filter."`
	Global bool   `short:"g" help:"Show global installs only."`
}

type StateLlCmd struct {
	Filter string `arg:"" optional:"" help:"Optional filter."`
	Global bool   `short:"g" help:"Show global installs only."`
}

type StateRmCmd struct {
	Target string `arg:"" predictor:"installed_apps" predict:"installed_apps" help:"Application to remove."`
	Purge  bool   `help:"Completely uninstall and purge."`
	StateOnly bool   `help:"Remove from state only without uninstalling."`
}

type RmCmd = StateRmCmd

type StateEditCmd struct{}

type ConfigCmd struct {
	Ls   ConfigLsCmd   `cmd:"" help:"List config settings."`
	Get  ConfigGetCmd  `cmd:"" help:"Get config setting."`
	Set  ConfigSetCmd  `cmd:"" help:"Set config setting."`
	Rm   ConfigRmCmd   `cmd:"" help:"Remove config setting."`
	Menu ConfigMenuCmd `cmd:"" default:"withargs" help:"Interactive config menu (default)."`
}
type ConfigLsCmd struct{}
type ConfigGetCmd struct {
	Key string `arg:"" predictor:"config_keys" predict:"config_keys"`
}
type ConfigSetCmd struct {
	Key   string `arg:"" predictor:"config_keys" predict:"config_keys"`
	Value string `arg:""`
}
type ConfigRmCmd struct {
	Key string `arg:"" predictor:"config_keys" predict:"config_keys"`
}
type ConfigMenuCmd struct{}

type RepoCmd struct {
	Clone RepoCloneCmd `cmd:"" help:"Clone the repository."`
	Fork  RepoForkCmd  `cmd:"" help:"Fork and clone the repository."`
}

type RepoCloneCmd struct {
	Repository string `arg:"" env:"GH_PT_REPOSITORY" predictor:"github_repos" predict:"github_repos" help:"Github repository."`
	Force      bool   `short:"f" help:"Overwrite existing."`
	MaxDepth   int    `help:"Max clone depth."`
}

type RepoForkCmd struct {
	Repository string `arg:"" env:"GH_PT_REPOSITORY" predictor:"github_repos" predict:"github_repos" help:"Github repository."`
	Force      bool   `short:"f" help:"Overwrite existing."`
	MaxDepth   int    `help:"Max clone depth."`
}

type ScanCmd struct {
	Ai ScanAiCmd `cmd:"" help:"AI safety scan."`
	Vt ScanVtCmd `cmd:"" help:"VirusTotal scan."`
}

type ScanAiCmd struct {
	Target      string `arg:"" optional:"" predictor:"github_repos" predict:"github_repos" help:"Target to scan."`
	Interactive bool   `short:"i" help:"Use interactive AI command from config."`
	AICmd       string `name:"ai-cmd" help:"Command template for AI execution."`
}

type ScanVtCmd struct {
	Target string `arg:"" optional:"" predictor:"github_repos" predict:"github_repos" help:"Target to scan."`
}

type VtCmd struct {
	SetKey VtSetKeyCmd `cmd:"" help:"Set VirusTotal API key."`
}
type VtSetKeyCmd struct {
	Key string `arg:"" help:"VirusTotal API key."`
}

type ShowCmd struct {
	Repository string `arg:"" env:"GH_PT_REPOSITORY" predictor:"github_repos" predict:"github_repos" help:"Github repository."`
	Assets     bool   `help:"Show all available assets."`
	Versions   bool   `help:"Show all release versions."`
	Prerelease bool   `help:"Include prereleases."`
	Stable     bool   `help:"Include only stable releases."`
	Version    string `short:"v" default:"latest" help:"Version to show."`
}

type SourceCmd struct {
	Repository string `arg:"" env:"GH_PT_REPOSITORY" predictor:"github_repos" predict:"github_repos" help:"Github repository."`
	AICmd      string `help:"Command template for AI execution."`
	CommonInstallFlags
}

type CompletionsCmd struct {
	Bash       CompletionsBashCmd       `cmd:"" help:"Generate Bash completions."`
	Zsh        CompletionsZshCmd        `cmd:"" help:"Generate Zsh completions."`
	Powershell CompletionsPowershellCmd `cmd:"" help:"Generate PowerShell completions."`
}

type CompletionsBashCmd struct{}
type CompletionsZshCmd struct{}
type CompletionsPowershellCmd struct{}

type SearchCmd struct {
	Query       string `arg:""`
	Description bool   `short:"d"`
}

var SearchRunner func(c *SearchCmd, ctx *ExecContext) error

func (c *SearchCmd) Run(ctx *ExecContext) error {
	if SearchRunner != nil {
		return SearchRunner(c, ctx)
	}
	args := []string{"search", "repos", c.Query}
	if c.Description {
		args = append(args, "--match", "name,description")
	} else {
		args = append(args, "--match", "name")
	}
	cmd := exec.Command("gh", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// ExecContext holds the flattened execution parameters
type ExecContext struct {
	UpdateAll bool
	Update    bool
	CommonInstallFlags
	Repository          string
	Clone               bool
	Fork                bool
	MaxDepth            int
	CompileFromSource   bool
	AI                  bool
	AICmd               string
	AISafetyScan        bool
	LogLevel            string
	LogFormat           string
	LogQuietInteractive bool
	Verbose             bool
	VTApiKey            string
	Ls                  string
	Ll                  string
	Full                bool
	EditSavedState      bool
	RmSavedState        string
	Rm                  string
	Purge               string
	Pin                 string
	Show                bool
	ShowAssets          bool
	ShowVersions        bool
}
