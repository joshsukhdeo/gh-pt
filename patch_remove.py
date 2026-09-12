import sys

with open('cmd/state_mgmt.go', 'r') as f:
    content = f.read()

old_code = """
					if _, err := exec.LookPath("dpkg"); err == nil {
						cmd = execCommand("sudo", "dpkg", "-r", pkgName)
					} else if _, err := exec.LookPath("rpm"); err == nil {
						cmd = execCommand("sudo", "rpm", "-e", pkgName)
					} else if _, err := exec.LookPath("pacman"); err == nil {
						cmd = execCommand("sudo", "pacman", "-R", "--noconfirm", pkgName)
					} else if _, err := exec.LookPath("pkg"); err == nil {
						cmd = execCommand("sudo", "pkg", "delete", "-y", pkgName)
					}
"""

new_code = """
					if _, err := exec.LookPath("dpkg"); err == nil {
						if purge {
							cmd = execCommand("sudo", "dpkg", "-P", pkgName)
						} else {
							cmd = execCommand("sudo", "dpkg", "-r", pkgName)
						}
					} else if _, err := exec.LookPath("rpm"); err == nil {
						cmd = execCommand("sudo", "rpm", "-e", pkgName)
					} else if _, err := exec.LookPath("pacman"); err == nil {
						if purge {
							cmd = execCommand("sudo", "pacman", "-Rn", "--noconfirm", pkgName)
						} else {
							cmd = execCommand("sudo", "pacman", "-R", "--noconfirm", pkgName)
						}
					} else if _, err := exec.LookPath("pkg"); err == nil {
						cmd = execCommand("sudo", "pkg", "delete", "-y", pkgName)
					}
"""

new_content = content.replace(old_code.strip(), new_code.strip())

with open('cmd/state_mgmt.go', 'w') as f:
    f.write(new_content)
