package main

import (
	"strings"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/alecthomas/kong"
	"github.com/joshsukhdeo/gh-pt/cmd"
	"github.com/joshsukhdeo/gh-pt/config"
	"github.com/joshsukhdeo/gh-pt/params"
	"github.com/posener/complete"
	"github.com/willabides/kongplete"
)

func main() {
	if _, err := exec.LookPath("gh"); err != nil {
		fmt.Fprintln(os.Stderr, "Error: GitHub CLI ('gh') is not installed or not in PATH. It is required for gh-pt. Please install it from https://cli.github.com/")
		os.Exit(1)
	}

	if len(os.Args) == 1 {
		os.Args = append(os.Args, "--help")
	}
	var cli params.CLI
	cfg, _ := config.LoadConfig()

	vars := kong.Vars{
		"install_types": cmd.GetDefaultInstallTypes(),
		"install_path":  cmd.GetDefaultTargetPath(),
		"clone_path":    cmd.GetDefaultClonePath(),
		"fork_path":     cmd.GetDefaultForkPath(),
		"version":       "2.0.0",
		"extractor":     "default",
	}

	if cfg != nil {
		if cfg.Core.InstallTypes != "" {
			vars["install_types"] = cfg.Core.InstallTypes
		}
		if cfg.Paths.InstallPath != "" {
			vars["install_path"] = cfg.Paths.InstallPath
		}
		if cfg.Paths.ClonePath != "" {
			vars["clone_path"] = cfg.Paths.ClonePath
		}
		if cfg.Core.Extractor != "" {
			vars["extractor"] = cfg.Core.Extractor
		}
		if cfg.Paths.ForkPath != "" {
			vars["fork_path"] = cfg.Paths.ForkPath
		}
	}

	parser := kong.Must(&cli,
		kong.Name(filepath.Base(os.Args[0])),
		kong.Description(`Install binaries for a Github repository release interactively or non-interactively.`),
		kong.DefaultEnvars(cmd.GetEnvPrefix()),
		vars)

	kongplete.Complete(parser,
		kongplete.WithPredictor("file", complete.PredictFiles("*")),
		kongplete.WithPredictor("installed_apps", cmd.PredictInstalledApps),
		kongplete.WithPredictor("github_repos", cmd.PredictGithubRepos),
		kongplete.WithPredictor("config_keys", cmd.PredictConfigKeys),
	)
	ctx, err := parser.Parse(os.Args[1:])
	parser.FatalIfErrorf(err)

	if cli.Test {
		importJsonBytes, _ := json.MarshalIndent(cli, "", "  ")
		fmt.Println(string(importJsonBytes))
		os.Exit(0)
	}

	err = cmd.RunCommand(ctx.Command(), &cli)
	if err != nil {
		if strings.HasPrefix(err.Error(), "Warning:") {
			fmt.Fprintf(os.Stderr, "\033[33m%v\033[0m\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "\033[31mError: %v\033[0m\n", err)
		}
		os.Exit(1)
	}
}
