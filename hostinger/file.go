package hostinger

import (
	"fmt"
	"os"

	"github.com/spf13/afero"
)

func FileExists(fs afero.Fs, name string) bool {
	_, err := fs.Stat(name)
	return !os.IsNotExist(err)
}

func LinkerFn(fs afero.Fs) func(oldname, newname string) error {
	if linker, ok := fs.(afero.Linker); ok {
		return linker.SymlinkIfPossible
	}

	return func(string, string) error {
		return fmt.Errorf("symlinks not supported")
	}
}

func LinkReaderFn(fs afero.Fs) func(string) (string, error) {
	if linker, ok := fs.(afero.LinkReader); ok {
		return linker.ReadlinkIfPossible
	}

	return func(string) (string, error) {
		return "", fmt.Errorf("symlinks not supported")
	}
}
