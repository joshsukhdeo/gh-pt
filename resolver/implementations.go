package resolver

import (
	"os"
)

// Compile-time interface checks
var (
	_ PackageManager = (*AptManager)(nil)
	_ PackageManager = (*DnfManager)(nil)
	_ PackageManager = (*PacmanManager)(nil)
	_ PackageManager = (*MiseManager)(nil)
	_ PackageManager = (*UvManager)(nil)
	_ PackageManager = (*CargoManager)(nil)
	_ PackageManager = (*VcpkgManager)(nil)
)

// AptManager manages packages via apt-get or apt.
type AptManager struct {
	UseSudo bool
}

func NewAptManager() *AptManager {
	return &AptManager{}
}

func (m *AptManager) Name() string {
	return "apt"
}

func (m *AptManager) IsInstalled() bool {
	if _, err := lookPath("apt-get"); err == nil {
		return true
	}
	if _, err := lookPath("apt"); err == nil {
		return true
	}
	return false
}

func (m *AptManager) Install(pkgs []string) error {
	if len(pkgs) == 0 {
		return nil
	}
	bin := "apt-get"
	if _, err := lookPath("apt-get"); err != nil {
		bin = "apt"
	}
	args := append([]string{"install", "-y"}, pkgs...)
	if m.UseSudo {
		args = append([]string{bin}, args...)
		bin = "sudo"
	}
	cmd := execCommand(bin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// DnfManager manages packages via dnf.
type DnfManager struct {
	UseSudo bool
}

func NewDnfManager() *DnfManager {
	return &DnfManager{}
}

func (m *DnfManager) Name() string {
	return "dnf"
}

func (m *DnfManager) IsInstalled() bool {
	_, err := lookPath("dnf")
	return err == nil
}

func (m *DnfManager) Install(pkgs []string) error {
	if len(pkgs) == 0 {
		return nil
	}
	bin := "dnf"
	args := append([]string{"install", "-y"}, pkgs...)
	if m.UseSudo {
		args = append([]string{bin}, args...)
		bin = "sudo"
	}
	cmd := execCommand(bin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// PacmanManager manages packages via pacman.
type PacmanManager struct {
	UseSudo bool
}

func NewPacmanManager() *PacmanManager {
	return &PacmanManager{}
}

func (m *PacmanManager) Name() string {
	return "pacman"
}

func (m *PacmanManager) IsInstalled() bool {
	_, err := lookPath("pacman")
	return err == nil
}

func (m *PacmanManager) Install(pkgs []string) error {
	if len(pkgs) == 0 {
		return nil
	}
	bin := "pacman"
	args := append([]string{"-S", "--noconfirm"}, pkgs...)
	if m.UseSudo {
		args = append([]string{bin}, args...)
		bin = "sudo"
	}
	cmd := execCommand(bin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// MiseManager manages tools via mise.
type MiseManager struct{}

func NewMiseManager() *MiseManager {
	return &MiseManager{}
}

func (m *MiseManager) Name() string {
	return "mise"
}

func (m *MiseManager) IsInstalled() bool {
	_, err := lookPath("mise")
	return err == nil
}

func (m *MiseManager) Install(pkgs []string) error {
	if len(pkgs) == 0 {
		return nil
	}
	args := append([]string{"install"}, pkgs...)
	cmd := execCommand("mise", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// UvManager manages python dependencies via uv pip.
type UvManager struct{}

func NewUvManager() *UvManager {
	return &UvManager{}
}

func (m *UvManager) Name() string {
	return "uv"
}

func (m *UvManager) IsInstalled() bool {
	_, err := lookPath("uv")
	return err == nil
}

func (m *UvManager) Install(pkgs []string) error {
	if len(pkgs) == 0 {
		return nil
	}
	args := append([]string{"pip", "install"}, pkgs...)
	cmd := execCommand("uv", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// CargoManager manages rust binaries/crates via cargo.
type CargoManager struct{}

func NewCargoManager() *CargoManager {
	return &CargoManager{}
}

func (m *CargoManager) Name() string {
	return "cargo"
}

func (m *CargoManager) IsInstalled() bool {
	_, err := lookPath("cargo")
	return err == nil
}

func (m *CargoManager) Install(pkgs []string) error {
	if len(pkgs) == 0 {
		return nil
	}
	args := append([]string{"install"}, pkgs...)
	cmd := execCommand("cargo", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// VcpkgManager manages C/C++ libraries via vcpkg.
type VcpkgManager struct{}

func NewVcpkgManager() *VcpkgManager {
	return &VcpkgManager{}
}

func (m *VcpkgManager) Name() string {
	return "vcpkg"
}

func (m *VcpkgManager) IsInstalled() bool {
	_, err := lookPath("vcpkg")
	return err == nil
}

func (m *VcpkgManager) Install(pkgs []string) error {
	if len(pkgs) == 0 {
		return nil
	}
	args := append([]string{"install"}, pkgs...)
	cmd := execCommand("vcpkg", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
