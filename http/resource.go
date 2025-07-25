package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v3/disk"
	"github.com/spf13/afero"

	fbErrors "github.com/filebrowser/filebrowser/v2/errors"
	"github.com/filebrowser/filebrowser/v2/files"
	"github.com/filebrowser/filebrowser/v2/fileutils"
	"github.com/filebrowser/filebrowser/v2/hostinger"
)

var resourceGetHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	file, err := files.NewFileInfo(&files.FileOptions{
		Fs:         d.user.Fs,
		Path:       r.URL.Path,
		Modify:     d.user.Perm.Modify,
		Expand:     true,
		ReadHeader: d.server.TypeDetectionByHeader,
		Checker:    d,
		Content:    true,
	})

	// if the path does not exist and its the trash dir - create it
	if os.IsNotExist(err) && d.user.TrashDir != "" {
		if d.user.FullPath(r.URL.Path) == d.user.FullPath(d.user.TrashDir) {
			err = d.user.Fs.MkdirAll(d.user.TrashDir, 0775) //nolint:gomnd
			if err != nil {
				return errToStatus(err), err
			}

			file, err = files.NewFileInfo(&files.FileOptions{
				Fs:         d.user.Fs,
				Path:       r.URL.Path,
				Modify:     d.user.Perm.Modify,
				Expand:     true,
				ReadHeader: d.server.TypeDetectionByHeader,
				Checker:    d,
				Content:    true,
			})
		}
	}

	if err != nil {
		return errToStatus(err), err
	}

	if file.IsSymlink && symlinkOutOfScope(d, file) {
		return errToStatus(fbErrors.ErrNotExist), fbErrors.ErrNotExist
	}

	if r.URL.Query().Get("disk_usage") == hostinger.QueryTrue {
		du, inodes, err := fileutils.DiskUsage(file.Fs, file.Path)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		file.DiskUsage = du
		file.Inodes = inodes
		file.Content = ""
		return renderJSON(w, r, file)
	}

	if file.IsDir {
		file.Listing.Sorting = d.user.Sorting
		file.Listing.ApplySort()
		file.Listing.FilterItems(func(fi *files.FileInfo) bool {
			// remove files that should be hidden
			_, exists := d.server.HiddenFiles[fi.Name]
			if exists {
				return false
			}

			// remove symlinks that link outside base path
			if fi.IsSymlink {
				link := fi.Link
				isAbs := filepath.IsAbs(link)

				if !isAbs {
					link = filepath.Join(d.user.FullPath(file.Path), link)
				}
				link = filepath.Clean(link)

				if !strings.HasPrefix(link, d.server.Root) {
					return false
				}

				if isAbs {
					fi.Link = strings.TrimPrefix(link, d.server.Root)
				}
			}

			return true
		})
		return renderJSON(w, r, file)
	}

	if checksum := r.URL.Query().Get("checksum"); checksum != "" {
		err := file.Checksum(checksum)
		if errors.Is(err, fbErrors.ErrInvalidOption) {
			return http.StatusBadRequest, nil
		} else if err != nil {
			return http.StatusInternalServerError, err
		}

		// do not waste bandwidth if we just want the checksum
		file.Content = ""
	}

	return renderJSON(w, r, file)
})

func resourceDeleteHandler(fileCache FileCache) handleFunc {
	return withUser(func(_ http.ResponseWriter, r *http.Request, d *data) (int, error) {
		if r.URL.Path == "/" || !d.user.Perm.Delete {
			return http.StatusForbidden, nil
		}

		file, err := files.NewFileInfo(&files.FileOptions{
			Fs:         d.user.Fs,
			Path:       r.URL.Path,
			Modify:     d.user.Perm.Modify,
			Expand:     false,
			ReadHeader: d.server.TypeDetectionByHeader,
			Checker:    d,
		})
		if err != nil {
			return errToStatus(err), err
		}

		// delete thumbnails
		err = delThumbs(r.Context(), fileCache, file)
		if err != nil {
			return errToStatus(err), err
		}

		skipTrash := r.URL.Query().Get("skip_trash") == hostinger.QueryTrue

		if d.user.TrashDir == "" || skipTrash {
			err = d.RunHook(func() error {
				return d.user.Fs.RemoveAll(r.URL.Path)
			}, "delete", r.URL.Path, "", d.user)
		} else {
			src := r.URL.Path
			dst := d.user.TrashDir

			if !d.Check(src) || !d.Check(dst) {
				return http.StatusForbidden, nil
			}

			src = path.Clean("/" + src)
			dst = path.Clean("/" + dst)

			err = d.user.Fs.MkdirAll(dst, 0775) //nolint:gomnd
			if err != nil {
				return errToStatus(err), err
			}

			dst = path.Join(dst, file.Name)
			err = fileutils.MoveFile(d.user.Fs, src, dst)
		}

		if err != nil {
			return errToStatus(err), err
		}

		return http.StatusNoContent, nil
	})
}

func resourcePostHandler(fileCache FileCache) handleFunc {
	return withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		if !d.user.Perm.Create || !d.Check(r.URL.Path) {
			return http.StatusForbidden, nil
		}

		// Directories creation on POST.
		if strings.HasSuffix(r.URL.Path, "/") {
			err := d.user.Fs.MkdirAll(r.URL.Path, files.PermDir)
			return errToStatus(err), err
		}

		// Archive creation on POST.
		if strings.HasSuffix(r.URL.Path, "/archive") {
			if !d.user.Perm.Create {
				return http.StatusForbidden, nil
			}

			err := archiveHandler(r, d)
			return errToStatus(err), err
		}

		file, err := files.NewFileInfo(&files.FileOptions{
			Fs:         d.user.Fs,
			Path:       r.URL.Path,
			Modify:     d.user.Perm.Modify,
			Expand:     false,
			ReadHeader: d.server.TypeDetectionByHeader,
			Checker:    d,
		})
		if err == nil {
			if r.URL.Query().Get("override") != hostinger.QueryTrue {
				return http.StatusConflict, nil
			}

			// Permission for overwriting the file
			if !d.user.Perm.Modify {
				return http.StatusForbidden, nil
			}

			err = delThumbs(r.Context(), fileCache, file)
			if err != nil {
				return errToStatus(err), err
			}
		}

		err = d.RunHook(func() error {
			info, writeErr := writeFile(d.user.Fs, r.URL.Path, r.Body)
			if writeErr != nil {
				return writeErr
			}

			etag := fmt.Sprintf(`"%x%x"`, info.ModTime().UnixNano(), info.Size())
			w.Header().Set("ETag", etag)
			return nil
		}, "upload", r.URL.Path, "", d.user)

		if err != nil {
			_ = d.user.Fs.RemoveAll(r.URL.Path)
		}

		return errToStatus(err), err
	})
}

var resourcePutHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	if !d.user.Perm.Modify || !d.Check(r.URL.Path) {
		return http.StatusForbidden, nil
	}

	// Only allow PUT for files.
	if strings.HasSuffix(r.URL.Path, "/") {
		return http.StatusMethodNotAllowed, nil
	}

	exists, err := afero.Exists(d.user.Fs, r.URL.Path)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if !exists {
		return http.StatusNotFound, nil
	}

	err = d.RunHook(func() error {
		info, writeErr := writeFile(d.user.Fs, r.URL.Path, r.Body)
		if writeErr != nil {
			return writeErr
		}

		etag := fmt.Sprintf(`"%x%x"`, info.ModTime().UnixNano(), info.Size())
		w.Header().Set("ETag", etag)
		return nil
	}, "save", r.URL.Path, "", d.user)

	return errToStatus(err), err
})

func checkSrcDstAccess(src, dst string, d *data) error {
	if !d.Check(src) || !d.Check(dst) {
		return fbErrors.ErrPermissionDenied
	}

	if dst == "/" || src == "/" {
		return fbErrors.ErrPermissionDenied
	}

	if err := checkParent(src, dst); err != nil {
		return fbErrors.ErrInvalidRequestParams
	}

	return nil
}

func resourcePatchHandler(fileCache FileCache) handleFunc {
	return withUser(func(_ http.ResponseWriter, r *http.Request, d *data) (int, error) {
		src := r.URL.Path
		dst := r.URL.Query().Get("destination")
		action := r.URL.Query().Get("action")

		if action == "chmod" {
			err := chmodActionHandler(r, d)
			return errToStatus(err), err
		}

		dst, err := url.QueryUnescape(dst)
		if err != nil {
			return errToStatus(err), err
		}

		err = checkSrcDstAccess(src, dst, d)
		if err != nil {
			return errToStatus(err), err
		}

		override := r.URL.Query().Get("override") == hostinger.QueryTrue
		rename := r.URL.Query().Get("rename") == hostinger.QueryTrue
		unarchive := action == "unarchive"
		if !override && !rename && !unarchive {
			if _, err = d.user.Fs.Stat(dst); err == nil {
				return http.StatusConflict, nil
			}
		}

		if rename {
			dst = addVersionSuffix(dst, d.user.Fs)
		}

		// Permission for overwriting the file
		if override && !d.user.Perm.Modify {
			return http.StatusForbidden, nil
		}

		err = d.RunHook(func() error {
			if unarchive {
				if !d.user.Perm.Create {
					return fbErrors.ErrPermissionDenied
				}

				return hostinger.Unarchive(r.Context(), src, dst, d.user.Fs, override)
			}
			return patchAction(r.Context(), action, src, dst, d, fileCache)
		}, action, src, dst, d.user)

		return errToStatus(err), err
	})
}

func checkParent(src, dst string) error {
	rel, err := filepath.Rel(src, dst)
	if err != nil {
		return err
	}

	rel = filepath.ToSlash(rel)
	if !strings.HasPrefix(rel, "../") && rel != ".." && rel != "." {
		return fbErrors.ErrSourceIsParent
	}

	return nil
}

// Checks if path contains symlink to out-of-scope targets.
// Returns error ErrNotExist if it does.
func symlinkOutOfScope(d *data, file *files.FileInfo) bool {
	var err error

	link := ""
	if lsf, ok := d.user.Fs.(afero.LinkReader); ok {
		if link, err = lsf.ReadlinkIfPossible(file.Path); err != nil {
			return false
		}
	}

	if !filepath.IsAbs(link) {
		link = filepath.Join(d.user.FullPath(file.Path), link)
	}
	link = filepath.Clean(link)

	return !strings.HasPrefix(link, d.server.Root)
}

func addVersionSuffix(source string, fs afero.Fs) string {
	counter := 1
	dir, name := path.Split(source)
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)

	for {
		if _, err := fs.Stat(source); err != nil {
			break
		}
		renamed := fmt.Sprintf("%s(%d)%s", base, counter, ext)
		source = path.Join(dir, renamed)
		counter++
	}

	return source
}

func writeFile(fs afero.Fs, dst string, in io.Reader) (os.FileInfo, error) {
	dir, _ := path.Split(dst)
	err := fs.MkdirAll(dir, files.PermDir)
	if err != nil {
		return nil, err
	}

	file, err := fs.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, files.PermFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	_, err = io.Copy(file, in)
	if err != nil {
		return nil, err
	}

	// Gets the info about the file.
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	return info, nil
}

func delThumbs(ctx context.Context, fileCache FileCache, file *files.FileInfo) error {
	for _, previewSizeName := range PreviewSizeNames() {
		size, _ := ParsePreviewSize(previewSizeName)
		if err := fileCache.Delete(ctx, previewCacheKey(file, size)); err != nil {
			return err
		}
	}

	return nil
}

func patchAction(ctx context.Context, action, src, dst string, d *data, fileCache FileCache) error {
	switch action {
	case "copy":
		if !d.user.Perm.Create {
			return fbErrors.ErrPermissionDenied
		}

		return fileutils.CopyScoped(d.user.Fs, src, dst, d.server.Root)
	case "rename":
		if !d.user.Perm.Rename {
			return fbErrors.ErrPermissionDenied
		}
		src = path.Clean("/" + src)
		dst = path.Clean("/" + dst)

		file, err := files.NewFileInfo(&files.FileOptions{
			Fs:         d.user.Fs,
			Path:       src,
			Modify:     d.user.Perm.Modify,
			Expand:     false,
			ReadHeader: false,
			Checker:    d,
		})
		if err != nil {
			return err
		}

		// delete thumbnails
		err = delThumbs(ctx, fileCache, file)
		if err != nil {
			return err
		}

		return fileutils.MoveFile(d.user.Fs, src, dst)
	default:
		return fmt.Errorf("unsupported action %s: %w", action, fbErrors.ErrInvalidRequestParams)
	}
}

type DiskUsageResponse struct {
	Total uint64 `json:"total"`
	Used  uint64 `json:"used"`
}

//lint:ignore U1000 unused in this fork
//nolint:deadcode
var diskUsage = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	file, err := files.NewFileInfo(&files.FileOptions{
		Fs:         d.user.Fs,
		Path:       r.URL.Path,
		Modify:     d.user.Perm.Modify,
		Expand:     false,
		ReadHeader: false,
		Checker:    d,
		Content:    false,
	})
	if err != nil {
		return errToStatus(err), err
	}
	fPath := file.RealPath()
	if !file.IsDir {
		return renderJSON(w, r, &DiskUsageResponse{
			Total: 0,
			Used:  0,
		})
	}

	usage, err := disk.UsageWithContext(r.Context(), fPath)
	if err != nil {
		return errToStatus(err), err
	}
	return renderJSON(w, r, &DiskUsageResponse{
		Total: usage.Total,
		Used:  usage.Used,
	})
})

func archiveHandler(r *http.Request, d *data) error {
	dir, err := files.NewFileInfo(&files.FileOptions{
		Fs:         d.user.Fs,
		Path:       strings.TrimSuffix(r.URL.Path, "/archive"),
		Modify:     d.user.Perm.Modify,
		Expand:     false,
		ReadHeader: false,
		Checker:    d,
	})
	if err != nil {
		return err
	}

	algo := r.URL.Query().Get("algo")

	archive, err := hostinger.GetFilenameFromQuery(r, dir)
	if err != nil {
		return fbErrors.ErrInvalidRequestParams
	}

	filenames, err := parseQueryFiles(r, dir, d.user)
	if err != nil {
		return fbErrors.ErrInvalidRequestParams
	}

	return hostinger.Archive(r.Context(), d.user.Fs, archive, algo, filenames)
}

func chmodActionHandler(r *http.Request, d *data) error {
	target := r.URL.Path
	perms := r.URL.Query().Get("permissions")
	recursive := r.URL.Query().Get("recursive") == hostinger.QueryTrue
	recursionType := r.URL.Query().Get("type")

	if !d.user.Perm.Modify {
		return fbErrors.ErrPermissionDenied
	}

	if !d.Check(target) || target == "/" {
		return fbErrors.ErrPermissionDenied
	}

	mode, err := strconv.ParseUint(perms, 10, 32)
	if err != nil {
		return fbErrors.ErrInvalidRequestParams
	}

	permMode := normalizeFileMode(mode)

	info, err := d.user.Fs.Stat(target)
	if err != nil {
		return err
	}

	if recursive && info.IsDir() {
		var recFilter func(i os.FileInfo) bool

		switch recursionType {
		case "directories":
			recFilter = func(i os.FileInfo) bool {
				return i.IsDir()
			}
		case "files":
			recFilter = func(i os.FileInfo) bool {
				return !i.IsDir()
			}
		default:
			recFilter = func(_ os.FileInfo) bool {
				return true
			}
		}

		return afero.Walk(d.user.Fs, target, func(name string, info os.FileInfo, err error) error {
			if err == nil {
				if recFilter(info) {
					err = d.user.Fs.Chmod(name, os.FileMode(permMode))
				}
			}
			return err
		})
	}

	return d.user.Fs.Chmod(target, os.FileMode(permMode))
}

func normalizeFileMode(m uint64) uint32 {
	fullPerms := 511
	if m > math.MaxInt32 {
		return uint32(fullPerms)
	}

	return uint32(m)
}
