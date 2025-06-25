package hostinger

import (
	"context"
	"slices"
	"testing"

	"github.com/spf13/afero"
)

func TestAlgoToExtension(t *testing.T) {
	tests := []struct {
		algo    string
		want    string
		wantErr bool
	}{
		{algo: "", want: ".zip", wantErr: false},
		{algo: QueryTrue, want: ".zip", wantErr: false},
		{algo: "zip", want: ".zip", wantErr: false},
		{algo: "tar", want: ".tar", wantErr: false},
		{algo: "targz", want: ".tar.gz", wantErr: false},
		{algo: "tarbz2", want: ".tar.bz2", wantErr: false},
		{algo: "tarxz", want: ".tar.xz", wantErr: false},
		{algo: "tarlz4", want: ".tar.lz4", wantErr: false},
		{algo: "tarsz", want: ".tar.sz", wantErr: false},
		{algo: "unknown", want: "", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.algo, func(t *testing.T) {
			got, err := AlgoToExtension(tc.algo)
			if tc.wantErr {
				if err == nil {
					t.Errorf("algo: %s expects err, got: %s", tc.algo, tc.want)
				}
				return
			}
			if err != nil {
				t.Errorf("algo: %s expects ext: %s, got err: %v", tc.algo, tc.want, err)
				return
			}
			if got != tc.want {
				t.Errorf("algo: %s expects ext: %s, got: %s", tc.algo, tc.want, got)
			}
		})
	}
}

func TestGatherFiles(t *testing.T) {
	fs := afero.NewMemMapFs()

	_ = fs.MkdirAll("/data/dir1/subdir", 0755)
	_ = fs.MkdirAll("/data/dir2", 0755)
	_ = afero.WriteFile(fs, "/data/dir1/a.txt", []byte("A"), 0644)
	_ = afero.WriteFile(fs, "/data/dir1/subdir/b.txt", []byte("B"), 0644)
	_ = afero.WriteFile(fs, "/data/dir2/c.txt", []byte("C"), 0644)

	fileInfos, err := GatherFiles(fs, []string{"/data/dir1", "/data/dir2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := []string{}
	for _, f := range fileInfos {
		found = append(found, f.NameInArchive)
	}

	expected := []string{"dir1/a.txt", "dir1/subdir/b.txt", "dir2/c.txt"}
	for _, want := range expected {
		if !slices.Contains(found, want) {
			t.Errorf("expected to find %q in archive files, but did not", want)
		}
	}
}

func TestArchive(t *testing.T) {
	fs := afero.NewMemMapFs()

	_ = fs.MkdirAll("/data", 0755)
	_ = afero.WriteFile(fs, "/data/a.txt", []byte("A"), 0644)
	_ = afero.WriteFile(fs, "/data/b.txt", []byte("B"), 0644)

	archivePath := "/out/archive"
	filenames := []string{"/data/a.txt", "/data/b.txt"}

	if err := Archive(context.Background(), fs, archivePath, "zip", filenames); err != nil {
		t.Fatalf("Archive failed: %v", err)
	}

	archiveFile := archivePath + ".zip"

	info, err := fs.Stat(archiveFile)
	if err != nil {
		t.Fatalf("archive file not found: %v", err)
	}

	if info.Size() == 0 {
		t.Errorf("archive file is empty")
	}
}

func TestUnarchive(t *testing.T) {
	fs := afero.NewMemMapFs()

	_ = fs.MkdirAll("/data/subdir", 0755)
	_ = afero.WriteFile(fs, "/data/a.txt", []byte("A"), 0644)
	_ = afero.WriteFile(fs, "/data/b.txt", []byte("B"), 0644)
	_ = afero.WriteFile(fs, "/data/subdir/c.txt", []byte("C"), 0644)

	archivePath := "/archive"
	filenames := []string{"/data/a.txt", "/data/b.txt", "/data/subdir"}
	if err := Archive(context.Background(), fs, archivePath, "zip", filenames); err != nil {
		t.Fatalf("Archive failed: %v", err)
	}

	destDir := "/extracted"
	if err := Unarchive(context.Background(), archivePath+".zip", destDir, fs, true); err != nil {
		t.Fatalf("Unarchive failed: %v", err)
	}

	tests := map[string]string{
		"/extracted/a.txt":        "A",
		"/extracted/b.txt":        "B",
		"/extracted/subdir/c.txt": "C",
	}
	for path, want := range tests {
		got, err := afero.ReadFile(fs, path)
		if err != nil {
			t.Errorf("expected file %q to exist, but got error: %v", path, err)
			continue
		}
		if string(got) != want {
			t.Errorf("file %q: expected content %q, got %q", path, want, string(got))
		}
	}
}
