package selector

import (
	"io"
	"os"
)

func IsActuallyExecutable(item *SelectorItem) bool {
	var f io.ReadCloser
	var err error

	if item.Compressed && item.Fs != nil {
		f, err = item.Fs.Open(item.FsPath)
	} else if item.DownloadPath != "" {
		f, err = os.Open(item.DownloadPath)
	}

	if err != nil || f == nil {
		return false // Assume not an executable if we can't read it
	}
	defer f.Close()

	buf := make([]byte, 4)
	n, _ := f.Read(buf)
	if n < 2 {
		return false
	}

	// ELF
	if n >= 4 && buf[0] == 0x7f && buf[1] == 'E' && buf[2] == 'L' && buf[3] == 'F' {
		return true
	}
	// Mach-O
	if n >= 4 {
		m := uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3])
		if m == 0xfeedface || m == 0xfeedfacf || m == 0xcefaedfe || m == 0xcffaedfe {
			return true
		}
	}
	// PE / MZ
	if buf[0] == 'M' && buf[1] == 'Z' {
		return true
	}
	// Shebang script
	if buf[0] == '#' && buf[1] == '!' {
		return true
	}
    // AppImage (squashfs etc, but they usually have ELF header)

	return false
}
