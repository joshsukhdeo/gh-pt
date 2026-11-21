package compile

import (
	"os/exec"
	"strings"
	"testing"
)

// TestContainerSecurityHardening verifies that container execution has proper security controls
func TestContainerSecurityHardening(t *testing.T) {
	// Skip if no container runtime available
	runtimeName, err := DetectContainerRuntime()
	if err != nil {
		t.Skip("No container runtime available")
	}

	// Use fully qualified image name for podman compatibility
	image := "docker.io/library/alpine:latest"

	// Test 1: Network isolation
	t.Run("NetworkIsolation", func(t *testing.T) {
		cmd := exec.Command(runtimeName, "run", "--rm", "--network=none", image, "sh", "-c", "ping -c 1 8.8.8.8 2>&1 || echo NETWORK_BLOCKED")
		output, _ := cmd.CombinedOutput()
		if !strings.Contains(string(output), "NETWORK_BLOCKED") {
			t.Errorf("Network should be disabled in container. Output: %s", string(output))
		}
	})

	// Test 2: Capability dropping
	t.Run("CapabilityDropping", func(t *testing.T) {
		cmd := exec.Command(runtimeName, "run", "--rm", "--cap-drop=ALL", "--cap-add=DAC_OVERRIDE", image, "sh", "-c", "cat /proc/1/status | grep CapEff")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to check capabilities: %v, output: %s", err, string(output))
		}
		// DAC_OVERRIDE is 0x2, should be the only capability
		if !strings.Contains(string(output), "0000000000000002") {
			t.Errorf("Expected only DAC_OVERRIDE capability, got: %s", string(output))
		}
	})

	// Test 3: Read-only filesystem
	t.Run("ReadOnlyFilesystem", func(t *testing.T) {
		cmd := exec.Command(runtimeName, "run", "--rm", "--read-only", image, "sh", "-c", "touch /test 2>&1 || echo READONLY_BLOCKED")
		output, _ := cmd.CombinedOutput()
		if !strings.Contains(string(output), "READONLY_BLOCKED") {
			t.Errorf("Root filesystem should be read-only. Output: %s", string(output))
		}
	})

	// Test 4: Non-root user
	t.Run("NonRootUser", func(t *testing.T) {
		cmd := exec.Command(runtimeName, "run", "--rm", "--user=1000:1000", image, "id", "-u")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to check user: %v, output: %s", err, string(output))
		}
		if !strings.Contains(string(output), "1000") {
			t.Errorf("Expected user 1000, got: %s", string(output))
		}
	})

	// Test 5: No-new-privileges
	t.Run("NoNewPrivileges", func(t *testing.T) {
		cmd := exec.Command(runtimeName, "run", "--rm", "--security-opt=no-new-privileges", image, "cat", "/proc/1/status")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to check no-new-privileges: %v, output: %s", err, string(output))
		}
		if !strings.Contains(string(output), "NoNewPrivs:\t1") {
			t.Errorf("no-new-privileges should be set. Output: %s", string(output))
		}
	})

	// Test 6: Tmpfs for /tmp
	t.Run("TmpfsForTmp", func(t *testing.T) {
		cmd := exec.Command(runtimeName, "run", "--rm", "--read-only", "--tmpfs=/tmp:size=100M", image, "mount")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to check mounts: %v, output: %s", err, string(output))
		}
		if !strings.Contains(string(output), "tmpfs on /tmp") {
			t.Errorf("/tmp should be mounted as tmpfs. Output: %s", string(output))
		}
	})
}

// TestBuildContainerCommand verifies that the command builder includes security flags
func TestBuildContainerCommand(t *testing.T) {
	cfg := &Config{
		RepoDir:          "/test/repo",
		InstallDirTarget: "/test/install",
	}

	runtimeName := "docker"
	image := "alpine:latest"

	args := BuildContainerCommand(cfg, runtimeName, image)

	// Check for security flags
	expectedFlags := []string{
		"--network=none",
		"--cap-drop=ALL",
		"--cap-add=DAC_OVERRIDE",
		"--security-opt=no-new-privileges",
		"--read-only",
		"--user=1000:1000",
	}

	for _, flag := range expectedFlags {
		found := false
		for _, arg := range args {
			if strings.Contains(arg, flag) || arg == flag {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected security flag %s not found in command", flag)
		}
	}

	// Check for read-only source mount
	foundSourceRO := false
	for i, arg := range args {
		if arg == "-v" && i+1 < len(args) && strings.Contains(args[i+1], "/build:ro") {
			foundSourceRO = true
			break
		}
	}
	if !foundSourceRO {
		t.Error("Source directory should be mounted read-only")
	}
}

func TestDetectHostOS(t *testing.T) {
	hostOS, hostArch := DetectHostOS()
	if hostOS == "" || hostArch == "" {
		t.Errorf("expected non-empty host OS and Arch, got %s / %s", hostOS, hostArch)
	}
}

func TestDetectContainerImage(t *testing.T) {
	img := DetectContainerImage()
	if img == "" {
		t.Error("expected non-empty container image")
	}
	if !strings.Contains(img, ":") {
		t.Errorf("expected image with tag, got %s", img)
	}
}

func TestDetectContainerRuntime_NotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // empty PATH
	runtime, err := DetectContainerRuntime()
	if err == nil {
		t.Errorf("expected error when no container runtime in PATH, got %s", runtime)
	}
	if !strings.Contains(err.Error(), "no container runtime found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestDetectCrossCompileConfig(t *testing.T) {
	tests := []struct {
		targetOS   string
		targetArch string
		expectedCC string
	}{
		{"linux", "amd64", "x86_64-linux-gnu-gcc"},
		{"linux", "arm64", "aarch64-linux-gnu-gcc"},
		{"linux", "arm", "arm-linux-gnueabihf-gcc"},
		{"windows", "amd64", "x86_64-w64-mingw32-gcc"},
		{"darwin", "arm64", "o64-clang"},
	}

	for _, tt := range tests {
		cfg := DetectCrossCompileConfig(tt.targetOS, tt.targetArch)
		if cfg.TargetOS != tt.targetOS {
			t.Errorf("expected targetOS %s, got %s", tt.targetOS, cfg.TargetOS)
		}
		if cfg.TargetArch != tt.targetArch {
			t.Errorf("expected targetArch %s, got %s", tt.targetArch, cfg.TargetArch)
		}
		if cfg.Compiler != tt.expectedCC {
			t.Errorf("expected compiler %s for %s/%s, got %s", tt.expectedCC, tt.targetOS, tt.targetArch, cfg.Compiler)
		}
		if cfg.EnvVars["CC"] != tt.expectedCC {
			t.Errorf("expected CC env %s, got %s", tt.expectedCC, cfg.EnvVars["CC"])
		}
	}
}

func TestPrepareCrossCompileEnv(t *testing.T) {
	crossCfg := DetectCrossCompileConfig("linux", "arm64")
	cfg := &Config{RepoDir: t.TempDir()}

	err := PrepareCrossCompileEnv(cfg, crossCfg)
	if err != nil {
		t.Fatalf("PrepareCrossCompileEnv failed: %v", err)
	}

	if crossCfg.EnvVars["GOOS"] != "linux" {
		t.Errorf("expected GOOS=linux, got %s", crossCfg.EnvVars["GOOS"])
	}
	if crossCfg.EnvVars["GOARCH"] != "arm64" {
		t.Errorf("expected GOARCH=arm64, got %s", crossCfg.EnvVars["GOARCH"])
	}
	if crossCfg.EnvVars["CGO_ENABLED"] != "1" {
		t.Errorf("expected CGO_ENABLED=1, got %s", crossCfg.EnvVars["CGO_ENABLED"])
	}

	// Test without compiler
	crossCfgNoCC := &CrossCompileConfig{TargetOS: "freebsd", TargetArch: "riscv64", EnvVars: make(map[string]string)}
	err = PrepareCrossCompileEnv(cfg, crossCfgNoCC)
	if err != nil {
		t.Fatalf("PrepareCrossCompileEnv failed: %v", err)
	}
	if crossCfgNoCC.EnvVars["CGO_ENABLED"] != "0" {
		t.Errorf("expected CGO_ENABLED=0 without compiler, got %s", crossCfgNoCC.EnvVars["CGO_ENABLED"])
	}
}

func TestGetCrossCompileImage(t *testing.T) {
	hostOS, hostArch := DetectHostOS()

	// Matching host target
	imgHost := GetCrossCompileImage(hostOS, hostArch)
	if imgHost != DetectContainerImage() {
		t.Errorf("expected host matching image %s, got %s", DetectContainerImage(), imgHost)
	}

	// Distinct cross-compilation targets
	imgLinux := GetCrossCompileImage("linux", "mips64")
	if imgLinux != "ubuntu:22.04" {
		t.Errorf("expected ubuntu:22.04, got %s", imgLinux)
	}

	imgWindows := GetCrossCompileImage("windows", "amd64")
	if imgWindows != "ubuntu:22.04" {
		t.Errorf("expected ubuntu:22.04, got %s", imgWindows)
	}

	imgDarwin := GetCrossCompileImage("darwin", "arm64")
	if imgDarwin != "ubuntu:22.04" {
		t.Errorf("expected ubuntu:22.04, got %s", imgDarwin)
	}
}

func TestValidateCrossCompileTools(t *testing.T) {
	hostOS, hostArch := DetectHostOS()

	// Native build should always succeed
	if err := ValidateCrossCompileTools(hostOS, hostArch); err != nil {
		t.Errorf("expected nil for native compile tools check, got %v", err)
	}

	// Bogus path should fail for non-native targets
	t.Setenv("PATH", t.TempDir())
	if err := ValidateCrossCompileTools("windows", "amd64"); err == nil {
		t.Error("expected error for missing MinGW tools in empty PATH")
	}
	if err := ValidateCrossCompileTools("darwin", "arm64"); err == nil {
		t.Error("expected error for missing osxcross tools in empty PATH")
	}
	if err := ValidateCrossCompileTools("linux", "s390x"); err == nil {
		t.Error("expected error for missing cross gcc in empty PATH")
	}
}

func TestGetContainerMountsAndEnvVars(t *testing.T) {
	cfg := &Config{
		RepoDir:          "/my/repo",
		InstallDirTarget: "/my/target",
		TargetPath:       "/my/target/bin",
	}

	mounts := GetContainerMounts(cfg)
	if len(mounts) != 4 {
		t.Fatalf("expected 4 mount args, got %d", len(mounts))
	}
	if mounts[0] != "-v" || mounts[1] != "/my/repo:/build:ro" {
		t.Errorf("unexpected repo mount: %v", mounts[:2])
	}
	if mounts[2] != "-v" || mounts[3] != "/my/target:/install:rw" {
		t.Errorf("unexpected install mount: %v", mounts[2:])
	}

	envVars := GetContainerEnvVars(cfg)
	expectedEnvs := []string{
		"GHPT_TARGET_BASE_DIR=/my/target",
		"GHPT_TARGET_BIN_DIR=/my/target/bin",
		"GHPT_TARGET_LIB_DIR=/my/target/lib",
		"GHPT_TARGET_SHARE_DIR=/my/target/share",
		"GHPT_TARGET_PATH=/my/target/bin",
	}

	for _, expected := range expectedEnvs {
		found := false
		for i, arg := range envVars {
			if arg == "-e" && i+1 < len(envVars) && envVars[i+1] == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected env var %s in %v", expected, envVars)
		}
	}
}

func TestExecuteAndRunContainer_NoRuntime(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // empty PATH ensures runtime detection fails

	cfg := &Config{
		RepoDir:          t.TempDir(),
		InstallDirTarget: t.TempDir(),
	}

	err := ExecuteInContainer(cfg)
	if err == nil {
		t.Error("expected error from ExecuteInContainer without container runtime")
	}

	err = RunContainer(cfg, &ContainerConfig{InstallDir: cfg.InstallDirTarget, RepoDir: cfg.RepoDir})
	if err == nil {
		t.Error("expected error from RunContainer without container runtime")
	}
}

func TestExecuteContainer_Error(t *testing.T) {
	cfg := &Config{
		RepoDir:          t.TempDir(),
		InstallDirTarget: t.TempDir(),
	}

	err := ExecuteContainer(cfg, "nonexistent-fake-runtime-binary", "alpine:latest")
	if err == nil {
		t.Error("expected error executing nonexistent container runtime")
	}
}
