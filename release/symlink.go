package release

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/joshsukhdeo/gh-pt/selector"
	"github.com/rs/zerolog/log"
)

func computeSymlinkDirName(repository string, installedBinaryName string) string {
	parts := strings.Split(repository, "/")
	var ownerid, repoid string
	if len(parts) >= 2 {
		ownerid = parts[0]
		repoid = parts[1]
	} else if len(parts) == 1 {
		repoid = parts[0]
	}

	cleanBin := strings.ToLower(installedBinaryName)
	cleanRepo := strings.ToLower(repoid)
	cleanOwner := strings.ToLower(ownerid)

	if cleanBin == cleanRepo {
		return repoid
	}

	if cleanOwner != "" && cleanBin == cleanOwner {
		return ownerid + "-" + repoid
	}

	versionRegex := regexp.MustCompile(`[-_.]?(v?\d+\.\d+.*|x86_64|amd64|arm64|linux|windows|darwin|mac|apple).*$`)
	cleaned := versionRegex.ReplaceAllString(installedBinaryName, "")
	cleaned = strings.TrimRight(cleaned, " .-")
	
	if cleaned != "" {
		return cleaned
	}

	return repoid
}

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
			return os.MkdirAll(target, info.Mode())
		}
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		}
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
			log.Warn().Err(err).Str("file", src).Msg("failed to close source file")
		}
	}()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer func() {
		if err := out.Close(); err != nil {
			log.Warn().Err(err).Str("file", dst).Msg("failed to close destination file")
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
				log.Warn().Err(err).Str("file", path).Msg("failed to close source file")
			}
		}()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer func() {
			if err := out.Close(); err != nil {
				log.Warn().Err(err).Str("file", target).Msg("failed to close destination file")
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

	appDir := computeSymlinkDirName(r.CliParams.Repository, binaries[0].Name)
	symlinkDir := filepath.Join(homeDir, "src", "apps", appDir)

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

		if _, err := os.Stat(destPath); err == nil {
			if r.CliParams.Overwrite {
				if err := os.Remove(destPath); err != nil {
					log.Warn().Err(err).Str("path", destPath).Msg("failed to remove existing file before overwrite")
				}
			} else {
				return "", fmt.Errorf("%s already exists; use force to overwrite", destPath)
			}
		}

		if err := os.Symlink(srcPath, destPath); err != nil {
			return "", err
		}

		r.InstalledBinaries = append(r.InstalledBinaries, filepath.Base(destPath))
	}

	return symlinkDir, nil
}
