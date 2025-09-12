package fileutils

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
)

// Copy copies a file or folder from one place to another.
func Copy(afs afero.Fs, src, dst string, fileMode, dirMode fs.FileMode) error {
	if src = path.Clean("/" + src); src == "" {
		return os.ErrNotExist
	}

	if dst = path.Clean("/" + dst); dst == "" {
		return os.ErrNotExist
	}

	if src == "/" || dst == "/" {
		// Prohibit copying from or to the virtual root directory.
		return os.ErrInvalid
	}

	if dst == src {
		return os.ErrInvalid
	}

	info, err := afs.Stat(src)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return CopyDir(afs, src, dst, fileMode, dirMode)
	}

	return CopyFile(afs, src, dst, fileMode, dirMode)
}

// Same as Copy, but checks scope in symlinks
func CopyScoped(afs afero.Fs, src, dst string, fileMode, dirMode fs.FileMode, scope string) error {
	if src = path.Clean("/" + src); src == "" {
		return os.ErrNotExist
	}

	if dst = path.Clean("/" + dst); dst == "" {
		return os.ErrNotExist
	}

	if src == "/" || dst == "/" {
		// Prohibit copying from or to the virtual root directory.
		return os.ErrInvalid
	}

	if dst == src {
		return os.ErrInvalid
	}

	info, err := afs.Stat(src)
	if err != nil {
		return err
	}

	switch info.Mode() & fs.ModeType {
	case fs.ModeDir:
		return CopyDirScoped(afs, src, dst, fileMode, dirMode, scope)
	case fs.ModeSymlink:
		return CopySymLinkScoped(afs, src, dst, scope)
	default:
		return CopyFile(afs, src, dst, fileMode, dirMode)
	}
}

func CopySymLinkScoped(afs afero.Fs, source, dest, scope string) error {
	if reader, ok := afs.(afero.LinkReader); ok {
		link, err := reader.ReadlinkIfPossible(source)
		if err != nil {
			return err
		}

		if filepath.IsAbs(link) {
			link = strings.TrimPrefix(link, scope)
			link = filepath.Join(string(os.PathSeparator), link)
		} else {
			link = filepath.Clean(filepath.Join(filepath.Dir(source), link))
		}

		if linker, ok := afs.(afero.Linker); ok {
			return linker.SymlinkIfPossible(link, dest)
		}
		return nil
	}
	return nil
}
