package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/joshsukhdeo/gh-pt/ai"
	"github.com/joshsukhdeo/gh-pt/params"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureOutput captures stdout during function execution.
func captureHelperOutput(f func() error) (string, error) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w

	outChan := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		outChan <- buf.String()
	}()

	execErr := f()
	_ = w.Close()
	os.Stdout = oldStdout
	out := <-outChan
	_ = r.Close()
	return out, execErr
}

func TestRunHelper_NoFlags(t *testing.T) {
	h := &params.HelperCmd{}
	err := RunHelper(h)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no helper flag specified")
}

func TestResolveTargetBaseDir(t *testing.T) {
	t.Run("explicit GHPT_TARGET_BASE_DIR", func(t *testing.T) {
		custom := filepath.Join(t.TempDir(), "custom-target")
		t.Setenv("GHPT_TARGET_BASE_DIR", custom)
		assert.Equal(t, custom, resolveTargetBaseDir())
	})

	t.Run("fallback to HOME", func(t *testing.T) {
		tempHome := t.TempDir()
		t.Setenv("GHPT_TARGET_BASE_DIR", "")
		t.Setenv("HOME", tempHome)
		expected := filepath.Join(tempHome, ".local")
		assert.Equal(t, expected, resolveTargetBaseDir())
	})

	t.Run("fallback to empty HOME", func(t *testing.T) {
		t.Setenv("GHPT_TARGET_BASE_DIR", "")
		t.Setenv("HOME", "")
		t.Setenv("USERPROFILE", "")
		expected := filepath.Join("/tmp", ".local")
		assert.Equal(t, expected, resolveTargetBaseDir())
	})
}

func TestHelperGetSystemInfo(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("GHPT_TARGET_BASE_DIR", filepath.Join(tempHome, "custom"))
	t.Setenv("USER", "testuser")
	t.Setenv("HOME", tempHome)

	h := &params.HelperCmd{GetSystemInfo: true}
	out, err := captureHelperOutput(func() error {
		return RunHelper(h)
	})
	require.NoError(t, err)

	assert.Contains(t, out, fmt.Sprintf("os: %s", runtime.GOOS))
	assert.Contains(t, out, fmt.Sprintf("arch: %s", runtime.GOARCH))
	assert.Contains(t, out, "user: testuser")
	assert.Contains(t, out, "target_base_dir:")
	assert.Contains(t, out, "install_bin:")
}

func TestDetectHelperShell(t *testing.T) {
	t.Run("custom shell", func(t *testing.T) {
		t.Setenv("SHELL", "/bin/zsh")
		assert.Equal(t, "zsh", detectHelperShell())
	})

	t.Run("empty shell", func(t *testing.T) {
		t.Setenv("SHELL", "")
		assert.Equal(t, "unknown", detectHelperShell())
	})
}

func TestDetectHelperDistro(t *testing.T) {
	name, ver, _ := detectHelperDistro()
	if runtime.GOOS == "linux" {
		assert.NotEmpty(t, name)
		assert.NotEmpty(t, ver)
	} else {
		assert.Equal(t, "unknown", name)
	}
}

func TestDetectHelperPkgManagersAndCompilers(t *testing.T) {
	// These run system checks and return string slices without panics
	pkgManagers := detectHelperPkgManagers()
	assert.IsType(t, []string{}, pkgManagers)

	compilers := detectHelperCompilers()
	assert.IsType(t, []string{}, compilers)
}

func TestDetectHelperHardware(t *testing.T) {
	if runtime.GOOS == "linux" {
		_ = detectHelperCPUModel()
		ram := detectHelperRAMGB()
		assert.GreaterOrEqual(t, ram, 0)
	} else {
		assert.Equal(t, "unknown", detectHelperCPUModel())
		assert.Equal(t, 0, detectHelperRAMGB())
	}
}

func TestHelperGetManifest(t *testing.T) {
	t.Run("missing GHPT_REPO_PATH", func(t *testing.T) {
		t.Setenv("GHPT_REPO_PATH", "")
		h := &params.HelperCmd{GetManifest: true}
		err := RunHelper(h)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "GHPT_REPO_PATH not set")
	})

	t.Run("missing manifest file", func(t *testing.T) {
		repoDir := t.TempDir()
		t.Setenv("GHPT_REPO_PATH", repoDir)
		h := &params.HelperCmd{GetManifest: true}
		err := RunHelper(h)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot read manifest")
	})

	t.Run("successful manifest dump", func(t *testing.T) {
		repoDir := t.TempDir()
		ghptDir := filepath.Join(repoDir, ".ghpt")
		require.NoError(t, os.MkdirAll(ghptDir, 0755))
		manifestJSON := `{"dependencies":[{"name":"curl","manager":"apt"}]}`
		require.NoError(t, os.WriteFile(filepath.Join(ghptDir, "manifest.json"), []byte(manifestJSON), 0644))

		t.Setenv("GHPT_REPO_PATH", repoDir)
		h := &params.HelperCmd{GetManifest: true}
		out, err := captureHelperOutput(func() error {
			return RunHelper(h)
		})
		require.NoError(t, err)
		assert.Contains(t, out, `"name":"curl"`)
	})
}

func TestHelperValidateManifest(t *testing.T) {
	t.Run("missing GHPT_REPO_PATH", func(t *testing.T) {
		t.Setenv("GHPT_REPO_PATH", "")
		h := &params.HelperCmd{ValidateManifest: true}
		assert.Error(t, RunHelper(h))
	})

	t.Run("invalid json", func(t *testing.T) {
		repoDir := t.TempDir()
		ghptDir := filepath.Join(repoDir, ".ghpt")
		require.NoError(t, os.MkdirAll(ghptDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(ghptDir, "manifest.json"), []byte("{not json"), 0644))

		t.Setenv("GHPT_REPO_PATH", repoDir)
		h := &params.HelperCmd{ValidateManifest: true}
		err := RunHelper(h)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid JSON")
	})

	t.Run("missing dependency name", func(t *testing.T) {
		repoDir := t.TempDir()
		ghptDir := filepath.Join(repoDir, ".ghpt")
		require.NoError(t, os.MkdirAll(ghptDir, 0755))
		badManifest := `{"dependencies":[{"manager":"apt"}]}`
		require.NoError(t, os.WriteFile(filepath.Join(ghptDir, "manifest.json"), []byte(badManifest), 0644))

		t.Setenv("GHPT_REPO_PATH", repoDir)
		h := &params.HelperCmd{ValidateManifest: true}
		err := RunHelper(h)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing name")
	})

	t.Run("unknown package manager", func(t *testing.T) {
		repoDir := t.TempDir()
		ghptDir := filepath.Join(repoDir, ".ghpt")
		require.NoError(t, os.MkdirAll(ghptDir, 0755))
		badManifest := `{"dependencies":[{"name":"foo","manager":"fake_pm_bogus"}]}`
		require.NoError(t, os.WriteFile(filepath.Join(ghptDir, "manifest.json"), []byte(badManifest), 0644))

		t.Setenv("GHPT_REPO_PATH", repoDir)
		h := &params.HelperCmd{ValidateManifest: true}
		err := RunHelper(h)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown package manager 'fake_pm_bogus'")
	})

	t.Run("valid manifest", func(t *testing.T) {
		repoDir := t.TempDir()
		ghptDir := filepath.Join(repoDir, ".ghpt")
		require.NoError(t, os.MkdirAll(ghptDir, 0755))
		goodManifest := `{"dependencies":[{"name":"git","manager":"apt"},{"name":"ripgrep","manager":"cargo"}]}`
		require.NoError(t, os.WriteFile(filepath.Join(ghptDir, "manifest.json"), []byte(goodManifest), 0644))

		t.Setenv("GHPT_REPO_PATH", repoDir)
		h := &params.HelperCmd{ValidateManifest: true}
		out, err := captureHelperOutput(func() error {
			return RunHelper(h)
		})
		require.NoError(t, err)
		assert.Contains(t, out, "manifest.json is valid")
	})
}

func TestHelperValidateCompileScript(t *testing.T) {
	t.Run("missing GHPT_REPO_PATH", func(t *testing.T) {
		t.Setenv("GHPT_REPO_PATH", "")
		h := &params.HelperCmd{ValidateCompileScript: true}
		assert.Error(t, RunHelper(h))
	})

	t.Run("missing body.sh", func(t *testing.T) {
		repoDir := t.TempDir()
		t.Setenv("GHPT_REPO_PATH", repoDir)
		h := &params.HelperCmd{ValidateCompileScript: true}
		err := RunHelper(h)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot read body.sh")
	})

	t.Run("forbidden pattern: apt install", func(t *testing.T) {
		repoDir := t.TempDir()
		ghptDir := filepath.Join(repoDir, ".ghpt")
		require.NoError(t, os.MkdirAll(ghptDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(ghptDir, "body.sh"), []byte("apt-get install -y curl\n"), 0644))

		t.Setenv("GHPT_REPO_PATH", repoDir)
		h := &params.HelperCmd{ValidateCompileScript: true}
		err := RunHelper(h)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "forbidden pattern in body.sh")
	})

	t.Run("forbidden pattern: sudo", func(t *testing.T) {
		repoDir := t.TempDir()
		ghptDir := filepath.Join(repoDir, ".ghpt")
		require.NoError(t, os.MkdirAll(ghptDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(ghptDir, "body.sh"), []byte("sudo make install\n"), 0644))

		t.Setenv("GHPT_REPO_PATH", repoDir)
		h := &params.HelperCmd{ValidateCompileScript: true}
		err := RunHelper(h)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "privilege escalation: sudo")
	})

	t.Run("forbidden pattern: system write /usr/local/", func(t *testing.T) {
		repoDir := t.TempDir()
		ghptDir := filepath.Join(repoDir, ".ghpt")
		require.NoError(t, os.MkdirAll(ghptDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(ghptDir, "body.sh"), []byte("cp binary /usr/local/bin/\n"), 0644))

		t.Setenv("GHPT_REPO_PATH", repoDir)
		h := &params.HelperCmd{ValidateCompileScript: true}
		err := RunHelper(h)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "direct system write: /usr/local/")
	})

	t.Run("valid body.sh", func(t *testing.T) {
		repoDir := t.TempDir()
		ghptDir := filepath.Join(repoDir, ".ghpt")
		require.NoError(t, os.MkdirAll(ghptDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(ghptDir, "body.sh"), []byte("cargo build --release\nghpt helper --install bin=target/release/app\n"), 0644))

		t.Setenv("GHPT_REPO_PATH", repoDir)
		h := &params.HelperCmd{ValidateCompileScript: true}
		out, err := captureHelperOutput(func() error {
			return RunHelper(h)
		})
		require.NoError(t, err)
		assert.Contains(t, out, "body.sh validation passed")
	})
}

func TestHelperViewTargetDirsAndFiles(t *testing.T) {
	baseDir := t.TempDir()
	t.Setenv("GHPT_TARGET_BASE_DIR", baseDir)

	// Create a nested file tree
	binDir := filepath.Join(baseDir, "bin")
	shareDir := filepath.Join(baseDir, "share", "doc")
	require.NoError(t, os.MkdirAll(binDir, 0755))
	require.NoError(t, os.MkdirAll(shareDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "myapp"), []byte("echo hi"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(shareDir, "README.md"), []byte("docs"), 0644))

	t.Run("ViewTargetDirs", func(t *testing.T) {
		h := &params.HelperCmd{ViewTargetDirs: true}
		out, err := captureHelperOutput(func() error {
			return RunHelper(h)
		})
		require.NoError(t, err)
		assert.Contains(t, out, "bin/")
		assert.Contains(t, out, "myapp")
		assert.Contains(t, out, "share/")
	})

	t.Run("ViewInstalledFiles", func(t *testing.T) {
		h := &params.HelperCmd{ViewInstalledFiles: true}
		out, err := captureHelperOutput(func() error {
			return RunHelper(h)
		})
		require.NoError(t, err)
		assert.Contains(t, out, filepath.Join(binDir, "myapp"))
		assert.Contains(t, out, filepath.Join(shareDir, "README.md"))
	})
}

func TestHelperInstall(t *testing.T) {
	baseDir := t.TempDir()
	t.Setenv("GHPT_TARGET_BASE_DIR", baseDir)

	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "mybinary")
	require.NoError(t, os.WriteFile(srcFile, []byte("#!/bin/sh\necho test"), 0755))

	srcSubdir := filepath.Join(srcDir, "assets")
	require.NoError(t, os.MkdirAll(srcSubdir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(srcSubdir, "data.txt"), []byte("asset-data"), 0644))

	t.Run("invalid format (missing =)", func(t *testing.T) {
		h := &params.HelperCmd{Install: "bin/foo"}
		err := RunHelper(h)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid --install format")
	})

	t.Run("path traversal attack prevention", func(t *testing.T) {
		h := &params.HelperCmd{Install: "../../etc=" + srcFile}
		err := RunHelper(h)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "outside target base dir")
	})

	t.Run("missing source", func(t *testing.T) {
		h := &params.HelperCmd{Install: "bin/ghost=" + filepath.Join(srcDir, "nonexistent")}
		err := RunHelper(h)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not exist")
	})

	t.Run("install file successfully", func(t *testing.T) {
		h := &params.HelperCmd{Install: "bin/mybinary=" + srcFile}
		out, err := captureHelperOutput(func() error {
			return RunHelper(h)
		})
		require.NoError(t, err)
		assert.Contains(t, out, "installed:")

		installedFile := filepath.Join(baseDir, "bin", "mybinary")
		data, err := os.ReadFile(installedFile)
		require.NoError(t, err)
		assert.Equal(t, "#!/bin/sh\necho test", string(data))

		info, err := os.Stat(installedFile)
		require.NoError(t, err)
		assert.True(t, info.Mode()&0111 != 0, "installed binary should be executable")
	})

	t.Run("install directory successfully", func(t *testing.T) {
		h := &params.HelperCmd{Install: "share/assets=" + srcSubdir}
		out, err := captureHelperOutput(func() error {
			return RunHelper(h)
		})
		require.NoError(t, err)
		assert.Contains(t, out, "installed:")

		installedAsset := filepath.Join(baseDir, "share", "assets", "data.txt")
		data, err := os.ReadFile(installedAsset)
		require.NoError(t, err)
		assert.Equal(t, "asset-data", string(data))
	})
}

func TestHelperGetBodyTemplate(t *testing.T) {
	h := &params.HelperCmd{GetBodyTemplate: true}
	out, err := captureHelperOutput(func() error {
		return RunHelper(h)
	})
	require.NoError(t, err)
	assert.NotEmpty(t, out)
}

func TestHelperAppendAndRemoveFromManifest(t *testing.T) {
	repoDir := t.TempDir()
	ghptDir := filepath.Join(repoDir, ".ghpt")
	require.NoError(t, os.MkdirAll(ghptDir, 0755))
	t.Setenv("GHPT_REPO_PATH", repoDir)

	t.Run("append invalid format", func(t *testing.T) {
		h := &params.HelperCmd{AppendManifest: "bad_format"}
		err := RunHelper(h)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid format")
	})

	t.Run("append multiple valid dependencies", func(t *testing.T) {
		h := &params.HelperCmd{AppendManifest: "apt=libssl-dev@1.1, cargo=tokio"}
		out, err := captureHelperOutput(func() error {
			return RunHelper(h)
		})
		require.NoError(t, err)
		assert.Contains(t, out, "manifest updated")

		data, err := os.ReadFile(filepath.Join(ghptDir, "manifest.json"))
		require.NoError(t, err)

		var m ai.Manifest
		require.NoError(t, json.Unmarshal(data, &m))
		require.Len(t, m.Dependencies, 2)
		assert.Equal(t, "libssl-dev", m.Dependencies[0].Name)
		assert.Equal(t, "apt", m.Dependencies[0].Manager)
		assert.Equal(t, "1.1", m.Dependencies[0].Version)
		assert.Equal(t, "tokio", m.Dependencies[1].Name)
		assert.Equal(t, "cargo", m.Dependencies[1].Manager)
	})

	t.Run("remove invalid format", func(t *testing.T) {
		h := &params.HelperCmd{RemoveFromManifest: "invalid_format"}
		err := RunHelper(h)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid format")
	})

	t.Run("remove dependency", func(t *testing.T) {
		h := &params.HelperCmd{RemoveFromManifest: "apt=libssl-dev"}
		out, err := captureHelperOutput(func() error {
			return RunHelper(h)
		})
		require.NoError(t, err)
		assert.Contains(t, out, "manifest updated")

		data, err := os.ReadFile(filepath.Join(ghptDir, "manifest.json"))
		require.NoError(t, err)

		var m ai.Manifest
		require.NoError(t, json.Unmarshal(data, &m))
		require.Len(t, m.Dependencies, 1)
		assert.Equal(t, "tokio", m.Dependencies[0].Name)
	})
}

func TestHelperRunCompileScript(t *testing.T) {
	t.Run("missing GHPT_REPO_PATH", func(t *testing.T) {
		t.Setenv("GHPT_REPO_PATH", "")
		h := &params.HelperCmd{RunCompileScript: true}
		assert.Error(t, RunHelper(h))
	})

	t.Run("missing body.sh", func(t *testing.T) {
		repoDir := t.TempDir()
		t.Setenv("GHPT_REPO_PATH", repoDir)
		h := &params.HelperCmd{RunCompileScript: true}
		err := RunHelper(h)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot read body.sh")
	})

	t.Run("successful script run", func(t *testing.T) {
		repoDir := t.TempDir()
		ghptDir := filepath.Join(repoDir, ".ghpt")
		require.NoError(t, os.MkdirAll(ghptDir, 0755))

		// Write a harmless body.sh that creates a sentinel file
		sentinel := filepath.Join(repoDir, "sentinel.txt")
		bodyScript := fmt.Sprintf("echo 'compiled' > %s\n", sentinel)
		require.NoError(t, os.WriteFile(filepath.Join(ghptDir, "body.sh"), []byte(bodyScript), 0644))

		t.Setenv("GHPT_REPO_PATH", repoDir)
		t.Setenv("GHPT_TARGET_BASE_DIR", filepath.Join(repoDir, "dist"))
		t.Setenv("GHPT_COMPILE_CONTAINER", "") // Direct execution

		h := &params.HelperCmd{RunCompileScript: true}
		out, err := captureHelperOutput(func() error {
			return RunHelper(h)
		})
		require.NoError(t, err)
		assert.Contains(t, out, "compile.sh completed successfully")

		sentinelData, err := os.ReadFile(sentinel)
		require.NoError(t, err)
		assert.Equal(t, "compiled\n", string(sentinelData))
	})
}

func TestHelperRunCompileScript_ContainerMissing(t *testing.T) {
	repoDir := t.TempDir()
	ghptDir := filepath.Join(repoDir, ".ghpt")
	require.NoError(t, os.MkdirAll(ghptDir, 0755))

	bodyScript := "echo 'compile'\n"
	require.NoError(t, os.WriteFile(filepath.Join(ghptDir, "body.sh"), []byte(bodyScript), 0644))

	t.Setenv("GHPT_REPO_PATH", repoDir)
	t.Setenv("GHPT_COMPILE_CONTAINER", "1")
	t.Setenv("PATH", t.TempDir()) // empty PATH ensures podman/docker lookups fail

	h := &params.HelperCmd{RunCompileScript: true}
	err := RunHelper(h)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "container runtime (podman or docker) not found")
}
