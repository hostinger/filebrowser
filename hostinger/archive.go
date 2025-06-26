package hostinger

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/mholt/archives"
	"github.com/spf13/afero"

	fbErrors "github.com/filebrowser/filebrowser/v2/errors"
	"github.com/filebrowser/filebrowser/v2/files"
	"github.com/filebrowser/filebrowser/v2/fileutils"
)

func AlgoToExtension(algo string) (string, error) {
	switch algo {
	case "zip", QueryTrue, "":
		return ".zip", nil
	case "tar":
		return ".tar", nil
	case "targz":
		return ".tar.gz", nil
	case "tarbz2":
		return ".tar.bz2", nil
	case "tarxz":
		return ".tar.xz", nil
	case "tarlz4":
		return ".tar.lz4", nil
	case "tarsz":
		return ".tar.sz", nil
	default:
		return "", fmt.Errorf("unsupported archive algorithm")
	}
}

func Unarchive(ctx context.Context, src, dst string, afs afero.Fs, overwrite bool) error {
	reader, err := afs.Open(src)
	if err != nil {
		return fmt.Errorf("archive open: %w", err)
	}

	format, _, err := archives.Identify(ctx, src, reader)
	if err != nil {
		return fmt.Errorf("archive identify: %w", err)
	}

	symlinkFn := LinkerFn(afs)

	exctractFn := func(_ context.Context, file archives.FileInfo) error {
		fullpath := filepath.Join(dst, filepath.Clean(file.NameInArchive))

		if file.IsDir() {
			return afs.MkdirAll(fullpath, file.Mode())
		}

		if !overwrite && FileExists(afs, fullpath) {
			return fbErrors.ErrExist
		}

		if err := afs.MkdirAll(filepath.Dir(fullpath), files.PermDir); err != nil {
			return fmt.Errorf("extract mkdir: %w", err)
		}

		if file.Mode()&os.ModeSymlink != 0 {
			if file.LinkTarget == "" {
				return fmt.Errorf("extract symlink target is empty")
			}

			return symlinkFn(file.LinkTarget, fullpath)
		}

		srcFd, err := file.Open()
		if err != nil {
			return fmt.Errorf("extract open: %w", err)
		}

		defer srcFd.Close()

		dstFd, err := afs.Create(fullpath)
		if err != nil {
			return fmt.Errorf("extract create file: %w", err)
		}

		defer dstFd.Close()

		_, err = io.Copy(dstFd, srcFd)
		return err
	}

	if ex, ok := format.(archives.Extractor); ok {
		return ex.Extract(ctx, reader, exctractFn)
	}

	return fbErrors.ErrInvalidDataType
}

func Archive(ctx context.Context, afs afero.Fs, archive, algo string, filenames []string) error {
	extension, err := AlgoToExtension(algo)
	if err != nil {
		return fbErrors.ErrInvalidRequestParams
	}

	archive += extension

	format, _, err := archives.Identify(ctx, archive, nil)
	if err != nil {
		return err
	}

	archiver, ok := format.(archives.Archiver)
	if !ok {
		return fbErrors.ErrInvalidRequestParams
	}

	if _, err = afs.Stat(archive); err == nil {
		return fbErrors.ErrExist
	}

	err = afs.MkdirAll(filepath.Dir(archive), files.PermDir)
	if err != nil {
		return err
	}

	fileInfos, err := GatherFiles(afs, filenames)
	if err != nil {
		return err
	}

	out, err := afs.Create(archive)
	if err != nil {
		return err
	}

	defer out.Close()

	return archiver.Archive(ctx, out, fileInfos)
}

func GatherFiles(afs afero.Fs, filenames []string) ([]archives.FileInfo, error) {
	symlinkFn := LinkReaderFn(afs)
	commonDir := fileutils.CommonPrefix(filepath.Separator, filenames...)

	fileInfos := []archives.FileInfo{}
	for _, filename := range filenames {
		err := afero.Walk(afs, filename, func(path string, info fs.FileInfo, err error) error {
			nameInArchive := strings.TrimPrefix(path, commonDir)
			nameInArchive = strings.TrimPrefix(nameInArchive, string(filepath.Separator))

			if info.IsDir() && nameInArchive == "" {
				return nil
			}

			var linkTarget string
			if info.Mode()&os.ModeSymlink != 0 {
				linkTarget, err = symlinkFn(path)
				if err != nil {
					return err
				}
			}

			fileInfos = append(fileInfos, archives.FileInfo{
				FileInfo:      info,
				NameInArchive: nameInArchive,
				LinkTarget:    linkTarget,
				Open: func() (fs.File, error) {
					return afs.Open(path)
				},
			})

			return err
		})
		if err != nil {
			return nil, err
		}
	}

	return fileInfos, nil
}
