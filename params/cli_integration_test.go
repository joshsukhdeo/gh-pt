package params_test

import (
	"encoding/json"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/joshsukhdeo/gh-pt/cmd"
	"github.com/joshsukhdeo/gh-pt/params"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parseWithTestVars(t *testing.T, args []string) (*params.CLI, *kong.Kong) {
	t.Helper()
	var cli params.CLI
	parser, err := kong.New(&cli,
		kong.Vars{
			"install_types": "deb,rpm,appimage,tar.gz",
			"install_path":  "/default/bin",
			"clone_path":    "~/src",
			"fork_path":     "~/src/forks",
			"extractor":     "default",
			"version":       "2.0.0",
		},
	)
	require.NoError(t, err)
	kCtx, err := parser.Parse(args)
	require.NoError(t, err)
	_ = kCtx
	return &cli, parser
}

func parseWithTestVarsErr(t *testing.T, args []string) (*params.CLI, error) {
	var cli params.CLI
	parser, err := kong.New(
		&cli,
		kong.Vars{
			"install_types": "deb,rpm",
			"install_path":  "/default/bin",
			"clone_path":    "~/src",
			"fork_path":     "~/src/forks",
			"extractor":     "default",
			"version":       "2.0.0",
		},
	)
	if err != nil {
		return nil, err
	}
	_, parseErr := parser.Parse(args)
	return &cli, parseErr
}

func TestCLI_TestFlagSerialization(t *testing.T) {
	cli, _ := parseWithTestVars(t, []string{"install", "cli/cli", "--test", "--global", "-p", "/custom/bin"})
	assert.True(t, cli.Test)
	assert.Equal(t, "cli/cli", cli.Install.Repository)
	assert.True(t, cli.Install.Global)
	assert.Equal(t, "/custom/bin", cli.Install.TargetPath)

	data, err := json.Marshal(cli)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"Repository":"cli/cli"`)
	assert.Contains(t, string(data), `"Global":true`)
}

func TestKong_InstallCmd_RejectsSourceFlags(t *testing.T) {
	_, err := parseWithTestVarsErr(t, []string{"install", "cli/cli", "--no-compile-container"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown flag")

	_, err = parseWithTestVarsErr(t, []string{"install", "cli/cli", "--target-os=linux"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown flag")
}

func TestKong_SourceCmd_RejectsReleaseAssetFlags(t *testing.T) {
	_, err := parseWithTestVarsErr(t, []string{"source", "cli/cli", "--verify-checksum"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown flag")

	_, err = parseWithTestVarsErr(t, []string{"source", "cli/cli", "--skip-vt-sandbox"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown flag")

	_, err = parseWithTestVarsErr(t, []string{"source", "cli/cli", "--format=tar.gz"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown flag")

	_, err = parseWithTestVarsErr(t, []string{"source", "cli/cli", "--extractor=ouch"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown flag")
}

func TestKong_UpgradeCmd_AcceptsContainerFlags(t *testing.T) {
	cli, err := parseWithTestVarsErr(t, []string{"upgrade", "--no-compile-container", "--no-ai-container"})
	require.NoError(t, err)
	assert.True(t, cli.Upgrade.NoCompileContainer)
	assert.True(t, cli.Upgrade.NoAIContainer)
}

func TestKong_PrunedFlags_FailGracefully(t *testing.T) {
	prunedFlags := []string{
		"--env-inject=FOO=bar",
		"--self-inflicted-technical-debt",
		"--BARBAROUS",
		"--allow-root-user-install",
		"--ai-setup-sidecars",
		"--LE-RETROGROUCH",
		"--retrograde-stopgap",
	}

	for _, flag := range prunedFlags {
		t.Run(flag, func(t *testing.T) {
			_, err := parseWithTestVarsErr(t, []string{"install", "test/app", flag})
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "unknown flag")
		})
	}
}

func TestCLI_SidecarFlags(t *testing.T) {
	t.Run("All Sidecar Flags Explicit", func(t *testing.T) {
		cli, _ := parseWithTestVars(t, []string{
			"install", "test/app",
			"-S", `plugins/.*\.so|data/.*`,
			"--sidecar-symlink-to", "/etc/plugins",
			"--sidecar-symlink-to", "/var/lib/plugins",
			"--include-sidecars",
			"--sidecar-mode", "custom-path:/my/sidecars",
		})

		// Sidecars is now a regex pattern string
		assert.Equal(t, `plugins/.*\.so|data/.*`, cli.Install.Sidecars)
		assert.Equal(t, []string{"/etc/plugins", "/var/lib/plugins"}, cli.Install.SidecarSymlinkTo)
		assert.True(t, cli.Install.IncludeSidecars)
		assert.Equal(t, "custom-path:/my/sidecars", cli.Install.SidecarMode)
	})

	t.Run("SidecarMode Auto-Enables IncludeSidecars", func(t *testing.T) {
		cli, _ := parseWithTestVars(t, []string{
			"install", "test/app",
			"--sidecar-mode=bin",
		})
		assert.Equal(t, "bin", cli.Install.SidecarMode)
		assert.True(t, cli.Install.IncludeSidecars, "specifying --sidecar-mode=bin should auto-enable IncludeSidecars")
	})

	t.Run("SidecarMode None Does Not Enable IncludeSidecars", func(t *testing.T) {
		cli, _ := parseWithTestVars(t, []string{
			"install", "test/app",
			"--sidecar-mode=none",
		})
		assert.Equal(t, "none", cli.Install.SidecarMode)
		assert.False(t, cli.Install.IncludeSidecars, "specifying --sidecar-mode=none should not auto-enable IncludeSidecars")
	})

	t.Run("SidecarMode Default Does Not Enable IncludeSidecars", func(t *testing.T) {
		cli, _ := parseWithTestVars(t, []string{
			"install", "test/app",
			"--sidecar-mode=default",
		})
		assert.Equal(t, "default", cli.Install.SidecarMode)
		assert.False(t, cli.Install.IncludeSidecars, "specifying --sidecar-mode=default should not auto-enable IncludeSidecars")
	})

	t.Run("SidecarMode With Explicit NoIncludeSidecars", func(t *testing.T) {
		cli, _ := parseWithTestVars(t, []string{
			"install", "test/app",
			"--sidecar-mode=bin",
			"--no-include-sidecars",
		})
		assert.Equal(t, "bin", cli.Install.SidecarMode)
		assert.False(t, cli.Install.IncludeSidecars, "explicit --no-include-sidecars should override auto-enabling")
	})

	t.Run("SidecarMode With Explicit IncludeSidecars False", func(t *testing.T) {
		cli, _ := parseWithTestVars(t, []string{
			"install", "test/app",
			"--sidecar-mode=bin",
			"--include-sidecars=false",
		})
		assert.Equal(t, "bin", cli.Install.SidecarMode)
		assert.False(t, cli.Install.IncludeSidecars, "explicit --include-sidecars=false should override auto-enabling")
	})

	t.Run("ToExecContext Resolution", func(t *testing.T) {
		cli, _ := parseWithTestVars(t, []string{
			"install", "test/app",
			"--sidecar-mode=bin",
		})
		execCtx := cli.ToExecContext()
		assert.Equal(t, "test/app", execCtx.Repository)
		assert.Equal(t, "bin", execCtx.SidecarMode)
		assert.True(t, execCtx.IncludeSidecars)

		// Manual struct without parsing
		manualCmd := params.InstallCmd{
			Repository: "owner/repo",
			SharedInstallSourceFlags: params.SharedInstallSourceFlags{
				DriverPluginFlags: params.DriverPluginFlags{
					SidecarMode: "same_dest",
				},
			},
		}
		manualExecCtx := manualCmd.ToExecContext()
		assert.Equal(t, "owner/repo", manualExecCtx.Repository)
		assert.Equal(t, "same_dest", manualExecCtx.SidecarMode)
		assert.True(t, manualExecCtx.IncludeSidecars)
	})
}

func TestCLI_Indicators(t *testing.T) {
	// Standard emoji outputs
	assert.Equal(t, "📌🎯", cmd.GetStateIndicator(true, true, false, false, false, false))
	assert.Equal(t, "📌🗻🎯", cmd.GetStateIndicator(true, true, false, false, true, false))
	assert.Equal(t, "🧪🎯", cmd.GetStateIndicator(true, false, true, false, false, false))

	// Text fallback outputs when disabled
	assert.Equal(t, "^@", cmd.GetStateIndicator(true, true, false, false, false, true))
	assert.Equal(t, "^*@", cmd.GetStateIndicator(true, true, false, false, true, true))
	assert.Equal(t, "!@", cmd.GetStateIndicator(true, false, true, false, false, true))
}

func TestCLI_NoColor_NoEmojis(t *testing.T) {
	t.Run("Install Flags", func(t *testing.T) {
		cli, _ := parseWithTestVars(t, []string{"install", "cli/cli", "--no-color", "--no-emojis"})
		assert.True(t, cli.Install.NoColor)
		assert.True(t, cli.Install.NoEmojis)
	})

	t.Run("Install Flag Aliases", func(t *testing.T) {
		cli, _ := parseWithTestVars(t, []string{"install", "cli/cli", "--no-colors", "--no-emoji"})
		assert.True(t, cli.Install.NoColor)
		assert.True(t, cli.Install.NoEmojis)
	})

	t.Run("Upgrade Flags", func(t *testing.T) {
		cli, _ := parseWithTestVars(t, []string{"upgrade", "cli/cli", "--no-color", "--no-emojis"})
		assert.True(t, cli.Upgrade.NoColor)
		assert.True(t, cli.Upgrade.NoEmojis)
	})

	t.Run("Show Flags", func(t *testing.T) {
		cli, _ := parseWithTestVars(t, []string{"show", "cli/cli", "--no-color", "--no-emojis"})
		assert.True(t, cli.Show.NoColor)
		assert.True(t, cli.Show.NoEmojis)
	})

	t.Run("Ls and Ll Flags", func(t *testing.T) {
		cliLs, _ := parseWithTestVars(t, []string{"ls", "--no-color", "--no-emojis"})
		assert.True(t, cliLs.Ls.NoColor)
		assert.True(t, cliLs.Ls.NoEmojis)

		cliLl, _ := parseWithTestVars(t, []string{"ll", "--no-color", "--no-emojis"})
		assert.True(t, cliLl.Ll.NoColor)
		assert.True(t, cliLl.Ll.NoEmojis)
	})
}

func TestCLI_InsecureAllowUnsignedFlags(t *testing.T) {
	t.Run("Install Flag", func(t *testing.T) {
		cli, _ := parseWithTestVars(t, []string{"install", "cli/cli", "--insecure-allow-unsigned"})
		assert.True(t, cli.Install.InsecureAllowUnsigned)
	})

	t.Run("Install Flag Alias skip-checksums", func(t *testing.T) {
		cli, _ := parseWithTestVars(t, []string{"install", "cli/cli", "--skip-checksums"})
		assert.True(t, cli.Install.InsecureAllowUnsigned)
	})

	t.Run("Upgrade Flag", func(t *testing.T) {
		cli, _ := parseWithTestVars(t, []string{"upgrade", "cli/cli", "--insecure-allow-unsigned"})
		assert.True(t, cli.Upgrade.InsecureAllowUnsigned)
	})

	t.Run("Upgrade Flag Alias skip-checksums", func(t *testing.T) {
		cli, _ := parseWithTestVars(t, []string{"upgrade", "cli/cli", "--skip-checksums"})
		assert.True(t, cli.Upgrade.InsecureAllowUnsigned)
	})
}

func TestCLI_DriverAndPluginFlags(t *testing.T) {
	t.Run("Driver flag auto-enables sidecars", func(t *testing.T) {
		cli, _ := parseWithTestVars(t, []string{"install", "intel/compute-runtime", "--driver=opencl"})
		assert.Equal(t, "opencl", cli.Install.Driver)
		assert.True(t, cli.Install.IncludeSidecars)

		execCtx := cli.ToExecContext()
		assert.Equal(t, "opencl", execCtx.Driver)
		assert.True(t, execCtx.IncludeSidecars)
	})

	t.Run("Plugin flag auto-enables sidecars", func(t *testing.T) {
		cli, _ := parseWithTestVars(t, []string{"install", "intel/openvino-plugins-for-obs-studio", "--plugin=obs"})
		assert.Equal(t, "obs", cli.Install.Plugin)
		assert.True(t, cli.Install.IncludeSidecars)

		execCtx := cli.ToExecContext()
		assert.Equal(t, "obs", execCtx.Plugin)
		assert.True(t, execCtx.IncludeSidecars)
	})

	t.Run("Plugins alias flag works", func(t *testing.T) {
		cli, _ := parseWithTestVars(t, []string{"install", "vendor/audio-pack", "--plugins=vst"})
		assert.Equal(t, "vst", cli.Install.Plugin)
		assert.True(t, cli.Install.IncludeSidecars)
	})

	t.Run("Driver none does not auto-enable sidecars", func(t *testing.T) {
		cli, _ := parseWithTestVars(t, []string{"install", "some/app", "--driver=none"})
		assert.Equal(t, "none", cli.Install.Driver)
		assert.False(t, cli.Install.IncludeSidecars)
	})

	t.Run("Explicit --no-include-sidecars overrides auto-enable", func(t *testing.T) {
		cli, _ := parseWithTestVars(t, []string{"install", "intel/compute-runtime", "--driver=vulkan", "--no-include-sidecars"})
		assert.Equal(t, "vulkan", cli.Install.Driver)
		assert.False(t, cli.Install.IncludeSidecars)
	})
}

func TestCLI_CommonRepoFlags(t *testing.T) {
	t.Run("install with --repo flag", func(t *testing.T) {
		cli, _ := parseWithTestVars(t, []string{"install", "--repo", "cli/cli"})
		assert.Empty(t, cli.Install.Repository)
		assert.Equal(t, "cli/cli", cli.Install.Repo)
	})

	t.Run("install with --http-url flag", func(t *testing.T) {
		cli, _ := parseWithTestVars(t, []string{"install", "--http-url", "https://storage.openvinotoolkit.org/packages/"})
		assert.Empty(t, cli.Install.Repository)
		assert.Equal(t, "https://storage.openvinotoolkit.org/packages/", cli.Install.HTTPURL)
	})

	t.Run("install with --http alias flag", func(t *testing.T) {
		cli, _ := parseWithTestVars(t, []string{"install", "--http", "https://storage.openvinotoolkit.org/packages/"})
		assert.Empty(t, cli.Install.Repository)
		assert.Equal(t, "https://storage.openvinotoolkit.org/packages/", cli.Install.HTTPURL)
	})

	t.Run("install with -u user flag", func(t *testing.T) {
		cli, _ := parseWithTestVars(t, []string{"install", "cli/cli", "-u"})
		assert.True(t, cli.Install.User)
	})

	t.Run("source with --repo flag", func(t *testing.T) {
		cli, _ := parseWithTestVars(t, []string{"source", "--repo", "cli/cli"})
		assert.Empty(t, cli.Source.Repository)
		assert.Equal(t, "cli/cli", cli.Source.Repo)
	})

	t.Run("show with --repo flag", func(t *testing.T) {
		cli, _ := parseWithTestVars(t, []string{"show", "--repo", "cli/cli"})
		assert.Empty(t, cli.Show.Repository)
		assert.Equal(t, "cli/cli", cli.Show.Repo)
	})

	t.Run("show with --http-url flag", func(t *testing.T) {
		cli, _ := parseWithTestVars(t, []string{"show", "--http-url", "https://storage.openvinotoolkit.org/packages/"})
		assert.Empty(t, cli.Show.Repository)
		assert.Equal(t, "https://storage.openvinotoolkit.org/packages/", cli.Show.HTTPURL)
	})

	t.Run("upgrade with --repo flag", func(t *testing.T) {
		cli, _ := parseWithTestVars(t, []string{"upgrade", "--repo", "cli/cli"})
		assert.Empty(t, cli.Upgrade.Repository)
		assert.Equal(t, "cli/cli", cli.Upgrade.Repo)
	})
}
