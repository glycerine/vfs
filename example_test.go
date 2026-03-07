package vfs

/*
import (
	"io"
	"os"
	"path/filepath"
	//"github.com/cockroachdb/pebble/vfs"
)

// CopyToMemFS copies a file (or recursively a directory) from the real FS into a vfs.MemFS.
func CopyToMemFS(mem vfs.FS, realSrcPath, memDestPath string) error {
	return filepath.Walk(realSrcPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Compute the relative path, then join with memDestPath
		rel, err := filepath.Rel(realSrcPath, path)
		if err != nil {
			return err
		}
		destPath := vfs.Default.PathJoin(memDestPath, rel)

		if info.IsDir() {
			return mem.MkdirAll(destPath, 0755)
		}

		return copyFileToMemFS(mem, path, destPath)
	})
}

func copyFileToMemFS(mem vfs.FS, realSrc, memDest string) error {
	// Open the source file from the real FS
	src, err := os.Open(realSrc)
	if err != nil {
		return err
	}
	defer src.Close()

	// Create the destination file in MemFS
	dst, err := mem.Create(memDest)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

func TestWithAssets(t *testing.T) {
	mem := vfs.NewMem()

	// Copy your real test assets into the MemFS
	if err := CopyToMemFS(mem, "./testdata", "testdata"); err != nil {
		t.Fatalf("failed to copy assets into MemFS: %v", err)
	}

	// Now open a pebble DB on the MemFS
	db, err := pebble.Open("testdata", &pebble.Options{FS: mem})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// ... your test logic
}
*/
