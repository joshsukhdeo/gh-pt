package release

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adrg/xdg"
	"github.com/joshsukhdeo/gh-pt/params"
	"github.com/joshsukhdeo/gh-pt/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectVulkanManifests(t *testing.T) {
	tempData := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tempData)
	xdg.Reload()

	pkgDir := t.TempDir()
	libPath := filepath.Join(pkgDir, "libvulkan_intel.so")
	require.NoError(t, os.WriteFile(libPath, []byte("dummy-so"), 0755))

	jsonContent := `{
		"file_format_version": "1.0.0",
		"ICD": {
			"library_path": "./libvulkan_intel.so",
			"api_version": "1.3.268"
		}
	}`
	jsonPath := filepath.Join(pkgDir, "intel_icd.json")
	require.NoError(t, os.WriteFile(jsonPath, []byte(jsonContent), 0644))

	targets, err := DetectDriverTargets("intel", "compute-runtime", pkgDir, "vulkan", "", false)
	require.NoError(t, err)
	require.Len(t, targets, 1)

	assert.Equal(t, DriverVulkan, targets[0].Subsystem)
	assert.Contains(t, targets[0].DestPath, "vulkan/icd.d")
	assert.Contains(t, targets[0].DestPath, "gh-pt-intel-compute-runtime-intel_icd.json")

	var manifest map[string]interface{}
	require.NoError(t, json.Unmarshal(targets[0].Content, &manifest))
	icd, ok := manifest["ICD"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, libPath, icd["library_path"])
}

func TestDetectOpenCLManifests(t *testing.T) {
	tempConfig := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempConfig)
	xdg.Reload()

	pkgDir := t.TempDir()
	libDir := filepath.Join(pkgDir, "lib")
	require.NoError(t, os.MkdirAll(libDir, 0755))
	libPath := filepath.Join(libDir, "libigdrcl.so")
	require.NoError(t, os.WriteFile(libPath, []byte("dummy-opencl-so"), 0755))

	targets, err := DetectDriverTargets("intel", "compute-runtime", pkgDir, "opencl", "", false)
	require.NoError(t, err)
	require.Len(t, targets, 1)

	assert.Equal(t, DriverOpenCL, targets[0].Subsystem)
	assert.Contains(t, targets[0].DestPath, "OpenCL/vendors")
	assert.Contains(t, targets[0].DestPath, "gh-pt-intel-compute-runtime.icd")
	assert.Equal(t, libPath+"\n", string(targets[0].Content))
}

func TestDetectLdsoTargets(t *testing.T) {
	testSys := t.TempDir()
	t.Setenv("GH_PT_TEST_SYSDIR", testSys)

	pkgDir := t.TempDir()
	libDir := filepath.Join(pkgDir, "lib")
	require.NoError(t, os.MkdirAll(libDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(libDir, "libigc.so"), []byte("dummy-igc"), 0755))

	targets, err := DetectDriverTargets("intel", "intel-graphics-compiler", pkgDir, "ldso", "", false)
	require.NoError(t, err)
	require.Len(t, targets, 1)

	assert.Equal(t, DriverLdSo, targets[0].Subsystem)
	assert.Equal(t, "ldconfig", targets[0].PostHook)
	assert.Contains(t, targets[0].DestPath, "etc/ld.so.conf.d/gh-pt-intel-intel-graphics-compiler.conf")
	assert.Contains(t, string(targets[0].Content), libDir)
}

func TestDetectUdevTargets(t *testing.T) {
	testSys := t.TempDir()
	t.Setenv("GH_PT_TEST_SYSDIR", testSys)

	pkgDir := t.TempDir()
	ruleContent := `SUBSYSTEM=="accel", ATTR{device}=="0x7d1d", MODE="0666"`
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "99-intel-npu.rules"), []byte(ruleContent), 0644))

	targets, err := DetectDriverTargets("intel", "linux-npu-driver", pkgDir, "udev", "", false)
	require.NoError(t, err)
	require.Len(t, targets, 1)

	assert.Equal(t, DriverUdev, targets[0].Subsystem)
	assert.Equal(t, "udevadm", targets[0].PostHook)
	assert.Contains(t, targets[0].DestPath, "etc/udev/rules.d/99-gh-pt-intel-linux-npu-driver-99-intel-npu.rules")
	assert.Equal(t, ruleContent, string(targets[0].Content))
}

func TestDetectPluginTargets_OBS_GIMP_Audio(t *testing.T) {
	tempConfig := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempConfig)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	xdg.Reload()

	pkgDir := t.TempDir()
	obsDir := filepath.Join(pkgDir, "obs-plugins", "64bit")
	require.NoError(t, os.MkdirAll(obsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(obsDir, "obs-openvino.so"), []byte("dummy"), 0755))

	gimpDir := filepath.Join(pkgDir, "plug-ins")
	require.NoError(t, os.MkdirAll(gimpDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(gimpDir, "openvino-plugin.py"), []byte("dummy"), 0755))

	vstDir := filepath.Join(pkgDir, "synth.vst3")
	require.NoError(t, os.MkdirAll(vstDir, 0755))

	clapFile := filepath.Join(pkgDir, "delay.clap")
	require.NoError(t, os.WriteFile(clapFile, []byte("dummy"), 0755))

	// Test OBS
	obsTargets, err := DetectPluginTargets("intel", "openvino-plugins-for-obs-studio", pkgDir, "obs", false)
	require.NoError(t, err)
	require.Len(t, obsTargets, 1)
	assert.Equal(t, PluginOBS, obsTargets[0].Subsystem)
	assert.Contains(t, obsTargets[0].DestPath, "obs-studio/plugins/openvino-plugins-for-obs-studio")

	// Test GIMP
	gimpTargets, err := DetectPluginTargets("intel", "openvino-ai-plugins-gimp", pkgDir, "gimp", false)
	require.NoError(t, err)
	require.Len(t, gimpTargets, 1)
	assert.Equal(t, PluginGIMP, gimpTargets[0].Subsystem)
	assert.Contains(t, gimpTargets[0].DestPath, "GIMP/2.10/plug-ins/openvino-ai-plugins-gimp")

	// Test Audio
	audioTargets, err := DetectPluginTargets("vendor", "my-plugins", pkgDir, "audio", false)
	require.NoError(t, err)
	require.Len(t, audioTargets, 2) // VST3 and CLAP
}

func TestDeployDriverAndPluginManifests(t *testing.T) {
	tempData := t.TempDir()
	tempConfig := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tempData)
	t.Setenv("XDG_CONFIG_HOME", tempConfig)
	xdg.Reload()

	pkgDir := t.TempDir()
	libPath := filepath.Join(pkgDir, "libvulkan_intel.so")
	require.NoError(t, os.WriteFile(libPath, []byte("dummy-so"), 0755))
	jsonContent := `{"ICD": {"library_path": "libvulkan_intel.so"}}`
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "icd.json"), []byte(jsonContent), 0644))

	r := &GithubRelease{
		CliParams: &params.ExecContext{
			Repository: "intel/compute-runtime",
			CommonInstallFlags: params.CommonInstallFlags{
				SidecarFlags: params.SidecarFlags{
					Driver: "vulkan",
				},
			},
		},
	}

	installed, err := r.DeployDriverAndPluginManifests(pkgDir)
	require.NoError(t, err)
	require.Len(t, installed, 1)
	assert.FileExists(t, installed[0])

	// Verify file content has absolute path
	content, err := os.ReadFile(installed[0])
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(content), libPath))
}

func TestDetectDriverTargets_WithDriverMap(t *testing.T) {
	tempData := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tempData)
	xdg.Reload()

	pkgDir := t.TempDir()
	libDir := filepath.Join(pkgDir, "lib")
	require.NoError(t, os.MkdirAll(libDir, 0755))

	// Create test files
	openclIcd := filepath.Join(pkgDir, "intel.icd")
	require.NoError(t, os.WriteFile(openclIcd, []byte("/usr/lib/libigdrcl.so\n"), 0644))

	vulkanJson := filepath.Join(pkgDir, "intel_icd.json")
	vulkanContent := `{"ICD": {"library_path": "./lib/libvulkan_intel.so"}}`
	require.NoError(t, os.WriteFile(vulkanJson, []byte(vulkanContent), 0644))

	ldsoConf := filepath.Join(pkgDir, "intel.conf")
	ldsoContent := "/usr/lib/intel-opencl\n"
	require.NoError(t, os.WriteFile(ldsoConf, []byte(ldsoContent), 0644))

	udevRule := filepath.Join(pkgDir, "99-intel.rules")
	udevContent := `SUBSYSTEM=="accel", MODE="0666"`
	require.NoError(t, os.WriteFile(udevRule, []byte(udevContent), 0644))

	// Test driver map with multiple mappings
	driverMap := `.*\.icd:opencl,.*\.json:vulkan,.*\.conf:ldso,.*\.rules:udev`
	targets, err := DetectDriverTargets("intel", "compute-runtime", pkgDir, "none", driverMap, false)
	require.NoError(t, err)

	// Should find all 4 files mapped to their respective drivers
	assert.Len(t, targets, 4)

	subsystems := make(map[DriverSubsystem]bool)
	for _, t := range targets {
		subsystems[t.Subsystem] = true
	}
	assert.True(t, subsystems[DriverOpenCL])
	assert.True(t, subsystems[DriverVulkan])
	assert.True(t, subsystems[DriverLdSo])
	assert.True(t, subsystems[DriverUdev])
}

func TestInstall_PersistsDriverPluginDriverMapAndSymlinkDir(t *testing.T) {
	tempData := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tempData)
	targetDir := filepath.Join(tempData, "bin")
	require.NoError(t, os.MkdirAll(targetDir, 0755))
	xdg.Reload()

	origExecCommand := execCommand
	execCommand = helperCommand
	origGhExec := ghExec
	ghExec = func(args ...string) (bytes.Buffer, bytes.Buffer, error) {
		for i, a := range args {
			if a == "--dir" && i+1 < len(args) {
				destDir := args[i+1]
				_ = os.WriteFile(filepath.Join(destDir, "app-linux-amd64"), []byte("#!/bin/sh\necho ok\n"), 0755)
			}
		}
		return bytes.Buffer{}, bytes.Buffer{}, nil
	}
	defer func() {
		execCommand = origExecCommand
		ghExec = origGhExec
	}()

	assetJSON, err := json.Marshal([]map[string]interface{}{
		{"name": "app-linux-amd64", "id": 100},
	})
	require.NoError(t, err)

	newMockClient := func() *MockGithubClient {
		return &MockGithubClient{
			GetResponses: map[string]interface{}{
				"repos/owner/repo/releases": []map[string]interface{}{
					{"Tag_name": "v1.0.0", "Id": 1},
				},
			},
			ReqResponses: map[string]*http.Response{
				"repos/owner/repo/releases/1/assets": {
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewReader(assetJSON)),
				},
			},
		}
	}

	// 1. Normal install: persists state
	gr := &GithubRelease{
		CliParams: &params.ExecContext{
			Repository: "owner/repo",
			CommonInstallFlags: params.CommonInstallFlags{
				ReleaseVersion: "latest",
				Type:           []string{"binary"},
				DisablePrompts: true,
				ProgressBar:    "none",
				TargetPath:     targetDir,
				Symlink:        true,
				SidecarFlags: params.SidecarFlags{
					Driver:    "vulkan",
					Plugin:    "vst",
					DriverMap: ".*icd.*\\.json$:vulkan",
				},
			},
		},
		Client: newMockClient(),
	}

	err = gr.Install()
	require.NoError(t, err)

	st, err := state.LoadState()
	require.NoError(t, err)
	app, ok := st.Apps["owner/repo"]
	require.True(t, ok)
	assert.Equal(t, "vulkan", app.Driver)
	assert.Equal(t, "vst", app.Plugin)
	assert.Equal(t, ".*icd.*\\.json$:vulkan", app.DriverMap)
	assert.NotEmpty(t, app.SymlinkDir)

	// 2. Install with NoSaveState: outputs populated on CliParams
	execParams := &params.ExecContext{
		Repository: "owner/repo",
		CommonInstallFlags: params.CommonInstallFlags{
			ReleaseVersion: "latest",
			Type:           []string{"binary"},
			DisablePrompts: true,
			ProgressBar:    "none",
			TargetPath:     targetDir,
			Symlink:        true,
			NoSaveState:    true,
			Overwrite:      true,
			SidecarFlags: params.SidecarFlags{
				Driver:    "vulkan",
				Plugin:    "vst",
				DriverMap: ".*icd.*\\.json$:vulkan",
			},
		},
	}
	grNoSave := &GithubRelease{
		CliParams: execParams,
		Client:    newMockClient(),
	}

	err = grNoSave.Install()
	require.NoError(t, err)

	assert.NotEmpty(t, execParams.InstalledBinaries)
	assert.NotEmpty(t, execParams.ResolvedSymlinkDir)
	assert.Equal(t, app.SymlinkDir, execParams.ResolvedSymlinkDir)
}
