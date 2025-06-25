package hostinger

import (
	"testing"

	"github.com/spf13/afero"
)

func TestFileExists(t *testing.T) {
	fs := afero.NewMemMapFs()

	_ = fs.MkdirAll("/data", 0755)
	_ = afero.WriteFile(fs, "/data/a.txt", []byte("A"), 0644)

	tests := []struct {
		path string
		want bool
	}{
		{path: "/data", want: true},
		{path: "/data/a.txt", want: true},
		{path: "/data/non-existent", want: false},
		{path: "/invalid", want: false},
		{path: "a.txt", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			got := FileExists(fs, tc.path)
			if got != tc.want {
				t.Errorf("path %s: want %t got %t", tc.path, tc.want, got)
			}
		})
	}
}
