package hostinger

import (
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/filebrowser/filebrowser/v2/files"
)

const QueryTrue = "true"

func GetFilenameFromQuery(r *http.Request, f *files.FileInfo) (string, error) {
	name := r.URL.Query().Get("name")
	name, err := url.QueryUnescape(strings.ReplaceAll(name, "+", "%2B"))
	if err != nil {
		return "", err
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("empty name provided")
	}

	name = filepath.Clean("/" + name)

	return filepath.Join(f.Path, name), nil
}
