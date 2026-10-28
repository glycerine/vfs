package vfs

import (
	iofs "io/fs"
	"os"
	"slices"
	"testing"
)

var _ = os.Remove

func TestSerz101(t *testing.T) {
	// same as TestMemFSWalkDir, but interrupt with
	// serialize and deserialize in the middle.

	fs := NewMem()

	// Create directory structure
	if err := fs.MkdirAll("/a/b/c", 0755); err != nil {
		t.Fatal(err)
	}
	if err := fs.MkdirAll("/a/d", 0755); err != nil {
		t.Fatal(err)
	}
	if err := fs.MkdirAll("/x", 0755); err != nil {
		t.Fatal(err)
	}

	// Create files
	for _, p := range []string{"/a/b/c/f1", "/a/b/f2", "/a/d/f3", "/x/f4", "/top"} {
		f, err := fs.Create(p, WriteCategoryUnspecified)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}

	// serz
	path := "basic_memfs_serz101.green.s2"
	panicOn(fs.Save(path))
	defer os.Remove(path) // cldeanup after test.

	// deserz into fs2
	fs2 := &MemFS{}
	panicOn(fs2.Load(path))

	// Test 1: Full walk from root, collect all paths
	var paths []string
	err := fs2.WalkDir("/", func(path string, d iofs.DirEntry, err error) error {
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{
		"/",
		"/a",
		"/a/b",
		"/a/b/c",
		"/a/b/c/f1",
		"/a/b/f2",
		"/a/d",
		"/a/d/f3",
		"/top",
		"/x",
		"/x/f4",
	}
	if !slices.Equal(paths, expected) {
		t.Fatalf("paths = %v, want %v", paths, expected)
	}
	//vv("good: done with Serz101")
}
