package cmd

import (
	"fmt"
	"github.com/joshsukhdeo/gh-pt/config"
	"github.com/joshsukhdeo/gh-pt/params"
)

func RunCommand(cmdStr string, cli *params.CLI) error {
	r := &RootCLI{}

	r.LogLevel = cli.LogLevel
	r.LogFormat = cli.LogFormat
	r.LogQuietInteractive = cli.LogQuietInteractive
	r.Verbose = cli.Verbose

	switch cmdStr {
	case "install", "install <repository>":
		r.CommonInstallFlags = cli.Install.CommonInstallFlags
		r.Repository = cli.Install.Repository
		return r.RunInstall()
	case "ls", "ls <filter>":
		return ListState(&RootCLI{ExecContext: params.ExecContext{Ls: cli.Ls.Filter, CommonInstallFlags: params.CommonInstallFlags{Global: cli.Ls.Global}}})
	case "ll", "ll <filter>":
		return ListState(&RootCLI{ExecContext: params.ExecContext{Ll: cli.Ll.Filter, Full: true, CommonInstallFlags: params.CommonInstallFlags{Global: cli.Ll.Global}}})
	case "rm", "rm <target>":

			if cli.Rm.StateOnly {
				return RmStateOnly(cli.Rm.Target)
			}
		return RemoveApp(cli.Rm.Target, cli.Rm.Purge)
	case "upgrade", "upgrade <repository>":
		r.Repository = cli.Upgrade.Repository
		if !cli.Upgrade.User && !cli.Upgrade.Global {
			r.UpdateAll = true
		} else {
			r.Update = true
			r.Global = cli.Upgrade.Global
		}
		return r.RunInstall()
	case "search", "search <query>":
		return cli.Search.Run(&r.ExecContext)
	case "state edit":
		return EditState()
	case "show", "show <repository>":
		return ShowInfo(&RootCLI{ExecContext: params.ExecContext{Repository: cli.Show.Repository, Show: true, ShowAssets: cli.Show.Assets, ShowVersions: cli.Show.Versions, ShowDescription: cli.Show.Description, ShowReadme: cli.Show.Readme, CommonInstallFlags: params.CommonInstallFlags{ReleaseVersion: cli.Show.Version, Stable: cli.Show.Stable, Prerelease: cli.Show.Prerelease}}})
	case "repo clone":
		r.CommonInstallFlags = cli.Install.CommonInstallFlags // fallback
		r.Repository = cli.Repo.Clone.Repository
		r.Clone = true
		r.Overwrite = cli.Repo.Clone.Force
		r.MaxDepth = cli.Repo.Clone.MaxDepth
		return r.RunInstall()
	case "repo fork":
		r.CommonInstallFlags = cli.Install.CommonInstallFlags
		r.Repository = cli.Repo.Fork.Repository
		r.Fork = true
		r.Overwrite = cli.Repo.Fork.Force
		r.MaxDepth = cli.Repo.Fork.MaxDepth
		return r.RunInstall()
	case "source":
		r.CommonInstallFlags = cli.Source.CommonInstallFlags
		r.Repository = cli.Source.Repository
		r.CompileFromSource = true
		r.AICmd = cli.Source.AICmd
		r.AI = true
		return r.RunInstall()
	case "scan ai":
		aiCmd := cli.Scan.Ai.AICmd
		if aiCmd == "" {
			cfg, _ := config.LoadConfig()
			if cfg != nil {
				if cli.Scan.Ai.Interactive {
					aiCmd = cfg.AI.AIInteractiveCmd
				} else {
					aiCmd = cfg.AI.AICmd
				}
			}
		}
		return RunAIScan(cli.Scan.Ai.Target, aiCmd)
	case "scan vt":
		return RunVTScan(cli.Scan.Vt.Target)
	case "vt set-key":
		return SetVTKey(cli.Vt.SetKey.Key)
	case "config ls":
		return ConfigLs()
	case "config get":
		return ConfigGet(cli.Config.Get.Key)
	case "config set":
		return ConfigSet(cli.Config.Set.Key, cli.Config.Set.Value)
	case "config rm":
		return ConfigRm(cli.Config.Rm.Key)
	case "config menu":
		return ConfigMenu()
	case "completions bash":
		return GenerateCompletions("bash")
	case "completions zsh":
		return GenerateCompletions("zsh")
	case "completions powershell":
		return GenerateCompletions("powershell")
	default:
		return fmt.Errorf("unknown command: %s", cmdStr)
	}
}
