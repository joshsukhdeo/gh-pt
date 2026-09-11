package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/alecthomas/kong"
	"github.com/posener/complete"
	"github.com/willabides/kongplete"
	"github.com/joshsukhdeo/gh-pt/cmd"
	"github.com/joshsukhdeo/gh-pt/config"
	"github.com/joshsukhdeo/gh-pt/params"
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

	kongplete.Complete(parser, kongplete.WithPredictor("file", complete.PredictFiles("*")))
	ctx, err := parser.Parse(os.Args[1:])
	parser.FatalIfErrorf(err)

	err = cmd.RunCommand(ctx.Command(), &cli)
	if err != nil {
		os.Exit(1)
	}
}
