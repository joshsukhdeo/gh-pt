package cmd

import (
	"fmt"
	"github.com/joshsukhdeo/gh-pt/config"
	"github.com/joshsukhdeo/gh-pt/params"
)

func RunCommand(cmdStr string, cli *params.CLI) error {
	r := &RootCLI{}

	r.ExecContext.LogLevel = cli.LogLevel
	r.ExecContext.LogFormat = cli.LogFormat
	r.ExecContext.LogQuietInteractive = cli.LogQuietInteractive
	r.ExecContext.Verbose = cli.Verbose

	switch cmdStr {
	case "install", "install <repository>":
		r.ExecContext.CommonInstallFlags = cli.Install.CommonInstallFlags
		r.ExecContext.Repository = cli.Install.Repository
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
		r.ExecContext.Repository = cli.Upgrade.Repository
		if !cli.Upgrade.User && !cli.Upgrade.Global {
			r.ExecContext.UpdateAll = true
		} else {
			r.ExecContext.Update = true
			r.ExecContext.CommonInstallFlags.Global = cli.Upgrade.Global
		}
		return r.RunInstall()
	case "search", "search <query>":
		return cli.Search.Run(&r.ExecContext)
	case "state edit":
		return EditState()
	case "show", "show <repository>":
		return ShowInfo(&RootCLI{ExecContext: params.ExecContext{Repository: cli.Show.Repository, Show: true, ShowAssets: cli.Show.Assets, ShowVersions: cli.Show.Versions, ShowDescription: cli.Show.Description, ShowReadme: cli.Show.Readme, CommonInstallFlags: params.CommonInstallFlags{ReleaseVersion: cli.Show.Version, Stable: cli.Show.Stable, Prerelease: cli.Show.Prerelease}}})
	case "repo clone":
		r.ExecContext.CommonInstallFlags = cli.Install.CommonInstallFlags // fallback
		r.ExecContext.Repository = cli.Repo.Clone.Repository
		r.ExecContext.Clone = true
		r.ExecContext.CommonInstallFlags.Overwrite = cli.Repo.Clone.Force
		r.ExecContext.MaxDepth = cli.Repo.Clone.MaxDepth
		return r.RunInstall()
	case "repo fork":
		r.ExecContext.CommonInstallFlags = cli.Install.CommonInstallFlags
		r.ExecContext.Repository = cli.Repo.Fork.Repository
		r.ExecContext.Fork = true
		r.ExecContext.CommonInstallFlags.Overwrite = cli.Repo.Fork.Force
		r.ExecContext.MaxDepth = cli.Repo.Fork.MaxDepth
		return r.RunInstall()
	case "source", "source <repository>":
		r.ExecContext.CommonInstallFlags = cli.Source.CommonInstallFlags
		r.ExecContext.Repository = cli.Source.Repository
		r.ExecContext.CompileFromSource = true
		r.ExecContext.AI = true
		aiCmd := cli.Source.AICmd
		if aiCmd == "" {
			cfg, _ := config.LoadConfig()
			if cfg != nil {
				if cli.Source.Interactive {
					aiCmd = cfg.AI.AIInteractiveCmd
				} else {
					aiCmd = cfg.AI.AICmd
				}
			}
		}
		r.ExecContext.AICmd = aiCmd
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
