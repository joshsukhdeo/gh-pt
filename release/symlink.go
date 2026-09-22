package release

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/joshsukhdeo/gh-pt/selector"
)

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			_ = os.Remove(target) // Ignore error, will fail if it's a directory but that's fine
			return os.Symlink(link, target)
		}
		_ = os.Remove(target) // Remove existing file before overwrite
		// Ensure parent directory is writable before we attempt to write
		_ = os.Chmod(filepath.Dir(target), 0755)
		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := in.Close(); err != nil {
			log.Warn("failed to close source file", "error", err, "file", src)
		}
	}()
	_ = os.Remove(dst) // Prevent permission denied if existing file is read-only
	_ = os.Chmod(filepath.Dir(dst), 0755)
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer func() {
		if err := out.Close(); err != nil {
			log.Warn("failed to close destination file", "error", err, "file", dst)
		}
	}()
	_, err = io.Copy(out, in)
	return err
}

func copyFS(fileSystem fs.FS, dst string) error {
	return fs.WalkDir(fileSystem, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(dst, path)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		in, err := fileSystem.Open(path)
		if err != nil {
			return err
		}
		defer func() {
			if err := in.Close(); err != nil {
				log.Warn("failed to close source file", "error", err, "file", path)
			}
		}()
		_ = os.Remove(target) // Prevent permission denied if existing file is read-only
		_ = os.Chmod(filepath.Dir(target), 0755)
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer func() {
			if err := out.Close(); err != nil {
				log.Warn("failed to close destination file", "error", err, "file", target)
			}
		}()
		_, err = io.Copy(out, in)
		return err
	})
}

func (r *GithubRelease) executeSymlinkInstall(binaries []*selector.SelectorItem, assetPath string) (string, error) {
	if len(binaries) == 0 {
		return "", fmt.Errorf("no binaries to symlink")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// Parse owner and repo from repository string
	parts := strings.Split(r.CliParams.Repository, "/")
	var ownerID, repoID string
	if len(parts) >= 2 {
		ownerID = parts[0]
		repoID = parts[1]
	} else if len(parts) == 1 {
		ownerID = ""
		repoID = parts[0]
	}

	// Always use ~/src/apps/{ownerID}/{repoID}
	symlinkDir := filepath.Join(homeDir, "src", "apps", ownerID, repoID)

	if err := os.MkdirAll(symlinkDir, 0755); err != nil {
		return "", err
	}

	// Copy all files to symlinkDir
	if binaries[0].ExtractDir != "" {
		if err := copyDir(binaries[0].ExtractDir, symlinkDir); err != nil {
			return "", err
		}
	} else if binaries[0].Compressed && binaries[0].Fs != nil {
		if err := copyFS(binaries[0].Fs, symlinkDir); err != nil {
			return "", err
		}
	} else {
		// Single file asset
		targetFile := filepath.Join(symlinkDir, filepath.Base(assetPath))
		if err := copyFile(assetPath, targetFile, 0755); err != nil {
			return "", err
		}
	}

	// Symlink appropriate exes
	for _, binary := range binaries {
		var srcPath string
		if binary.ExtractDir != "" {
			// Find relative path from ExtractDir
			rel, err := filepath.Rel(binary.ExtractDir, binary.DownloadPath)
			if err != nil {
				rel = binary.Name
			}
			srcPath = filepath.Join(symlinkDir, rel)
		} else if binary.Compressed {
			srcPath = filepath.Join(symlinkDir, binary.FsPath)
		} else {
			srcPath = filepath.Join(symlinkDir, filepath.Base(assetPath))
		}

		destPath := r.resolveDestinationPath(binary.Name)

		if _, err := os.Lstat(destPath); err == nil {
			if r.CliParams.Overwrite || r.CliParams.IsUpgradeCmd {
				if err := os.Remove(destPath); err != nil {
					log.Warn("failed to remove existing file before overwrite", "error", err, "path", destPath)
				}
			} else {
				return "", fmt.Errorf("%s already exists; use force to overwrite", destPath)
			}
		}

		if err := os.Symlink(srcPath, destPath); err != nil {
			return "", err
		}

		if r.UI != nil {
			r.UI.Update(6, r.ResolvedVersion, filepath.Base(assetPath), binary.Name, destPath, "")
		} else {
			log.Info("processing selected release asset binary for symlink",
				"repository", r.CliParams.Repository,
				"release name", r.ResolvedVersion,
				"release asset name", filepath.Base(assetPath),
				"release asset binary", binary.Name)

			log.Infof("will install symlink %s -> %s", destPath, srcPath)
		}

		r.InstalledBinaries = append(r.InstalledBinaries, filepath.Base(destPath))
	}

	return symlinkDir, nil
}
