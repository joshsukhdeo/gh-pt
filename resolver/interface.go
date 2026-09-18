package resolver

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// ErrNoNativeManager is returned when no supported native package manager is detected.
var ErrNoNativeManager = errors.New("no supported native package manager found")

// Variables to allow mocking during unit testing
var (
	execCommand   = exec.Command
	lookPath      = exec.LookPath
	osReleasePath = "/etc/os-release"
)

// PackageManager defines the standard interface for system and user-space package managers.
type PackageManager interface {
	Name() string
	IsInstalled() bool
	Install(pkgs []string) error
}

// GetNativeManager detects and returns the native OS package manager based on the OS and installed executables.
func GetNativeManager() (PackageManager, error) {
	switch runtime.GOOS {
	case "linux":
		// Check distro hints in /etc/os-release if available
		distro := detectLinuxDistro()
		if strings.Contains(distro, "fedora") || strings.Contains(distro, "rhel") || strings.Contains(distro, "centos") {
			dnf := NewDnfManager()
			if dnf.IsInstalled() {
				return dnf, nil
			}
		} else if strings.Contains(distro, "arch") || strings.Contains(distro, "manjaro") {
			pacman := NewPacmanManager()
			if pacman.IsInstalled() {
				return pacman, nil
			}
		} else if strings.Contains(distro, "debian") || strings.Contains(distro, "ubuntu") {
			apt := NewAptManager()
			if apt.IsInstalled() {
				return apt, nil
			}
		}

		// Fallback: Check installed managers by preference order
		apt := NewAptManager()
		if apt.IsInstalled() {
			return apt, nil
		}
		dnf := NewDnfManager()
		if dnf.IsInstalled() {
			return dnf, nil
		}
		pacman := NewPacmanManager()
		if pacman.IsInstalled() {
			return pacman, nil
		}

	default:
		// Check available CLI managers across other OSes
		apt := NewAptManager()
		if apt.IsInstalled() {
			return apt, nil
		}
		dnf := NewDnfManager()
		if dnf.IsInstalled() {
			return dnf, nil
		}
		pacman := NewPacmanManager()
		if pacman.IsInstalled() {
			return pacman, nil
		}
	}

	return nil, ErrNoNativeManager
}

// detectLinuxDistro reads osReleasePath and returns normalized lowercase ID and ID_LIKE values.
func detectLinuxDistro() string {
	data, err := os.ReadFile(osReleasePath)
	if err != nil {
		return ""
	}
	content := strings.ToLower(string(data))
	var detected []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "id=") {
			val := strings.Trim(strings.TrimPrefix(line, "id="), "\"")
			if val != "" {
				detected = append(detected, val)
			}
		} else if strings.HasPrefix(line, "id_like=") {
			val := strings.Trim(strings.TrimPrefix(line, "id_like="), "\"")
			if val != "" {
				detected = append(detected, val)
			}
		}
	}
	return strings.Join(detected, " ")
}

// GetManager returns a package manager by name, supporting specific managers as well as "native_os".
func GetManager(name string) (PackageManager, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "native_os", "native":
		return GetNativeManager()
	case "apt":
		return NewAptManager(), nil
	case "dnf":
		return NewDnfManager(), nil
	case "pacman":
		return NewPacmanManager(), nil
	case "mise":
		return NewMiseManager(), nil
	case "uv":
		return NewUvManager(), nil
	case "cargo":
		return NewCargoManager(), nil
	case "vcpkg":
		return NewVcpkgManager(), nil
	default:
		return nil, fmt.Errorf("unsupported package manager: %s", name)
	}
}
