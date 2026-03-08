package vfs

import (
	iofs "io/fs"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSerz101(t *testing.T) {
	// same as TestMemFSWalkDir, but interrupt with
	// serialize and deserialize in the middle.

	fs := NewMem()

	// Create directory structure
	require.NoError(t, fs.MkdirAll("/a/b/c", 0755))
	require.NoError(t, fs.MkdirAll("/a/d", 0755))
	require.NoError(t, fs.MkdirAll("/x", 0755))

	// Create files
	for _, p := range []string{"/a/b/c/f1", "/a/b/f2", "/a/d/f3", "/x/f4", "/top"} {
		f, err := fs.Create(p, WriteCategoryUnspecified)
		require.NoError(t, err)
		require.NoError(t, f.Close())
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
		require.NoError(t, err)
		paths = append(paths, path)
		return nil
	})
	require.NoError(t, err)

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
	require.Equal(t, expected, paths)
	//vv("good: done with Serz101")
}
