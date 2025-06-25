package hostinger

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/filebrowser/filebrowser/v2/files"
)

func TestGetFilenameFromQuery(t *testing.T) {
	base := &files.FileInfo{Path: "/base"}

	tests := []struct {
		nameParam string
		want      string
		wantErr   bool
	}{
		{nameParam: "file.txt", want: "/base/file.txt", wantErr: false},
		{nameParam: "sub/dir/file.txt", want: "/base/sub/dir/file.txt", wantErr: false},
		{nameParam: "", want: "", wantErr: true},
		{nameParam: "%2Fetc%2Fpasswd", want: "/base/etc/passwd", wantErr: false}, // encoded /etc/passwd
		{nameParam: "   spaced.txt   ", want: "/base/spaced.txt", wantErr: false},
	}

	for _, tc := range tests {
		t.Run(tc.nameParam, func(t *testing.T) {
			req := &http.Request{URL: &url.URL{RawQuery: "name=" + url.QueryEscape(tc.nameParam)}}
			got, err := GetFilenameFromQuery(req, base)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error for input %q, got none", tc.nameParam)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error for input %q: %v", tc.nameParam, err)
				return
			}
			if got != tc.want {
				t.Errorf("input %q: want %q, got %q", tc.nameParam, tc.want, got)
			}
		})
	}
}
