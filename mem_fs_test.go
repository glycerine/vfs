// Copyright 2012 The LevelDB-Go and Pebble Authors. All rights reserved. Use
// of this source code is governed by a BSD-style license that can be found in
// the LICENSE file.

package vfs

import (
	"fmt"
	"io"
	iofs "io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/cockroachdb/datadriven"
	"github.com/stretchr/testify/require"
)

func runMemFSDataDriven(t *testing.T, path string, fs *MemFS) {
	fsMap := map[string]*MemFS{"initial": fs}
	var f File
	rng := rand.New(rand.NewPCG(0, 0))
	datadriven.RunTest(t, path, func(t *testing.T, td *datadriven.TestData) string {
		var err error
		switch td.Cmd {
		case "create":
			f, err = fs.Create(td.CmdArgs[0].String(), WriteCategoryUnspecified)
		case "link":
			err = fs.Link(td.CmdArgs[0].String(), td.CmdArgs[1].String())
		case "open":
			f, err = fs.Open(td.CmdArgs[0].String())
		case "open-dir":
			f, err = fs.OpenDir(td.CmdArgs[0].String())
		case "open-read-write":
			f, err = fs.OpenReadWrite(td.CmdArgs[0].String(), WriteCategoryUnspecified)
		case "mkdirall":
			err = fs.MkdirAll(td.CmdArgs[0].String(), 0755)
		case "remove":
			err = fs.Remove(td.CmdArgs[0].String())
		case "rename":
			err = fs.Rename(td.CmdArgs[0].String(), td.CmdArgs[1].String())
		case "reuse-for-write":
			f, err = fs.ReuseForWrite(td.CmdArgs[0].String(), td.CmdArgs[1].String(), WriteCategoryUnspecified)
		case "crash-clone":
			p, _ := strconv.Atoi(td.CmdArgs[0].String())
			fsName := td.CmdArgs[1].String()
			newFs := fs.CrashClone(CrashCloneCfg{UnsyncedDataPercent: p, RNG: rng})
			fsMap[fsName] = newFs
		case "switch-fs":
			fsName := td.CmdArgs[0].String()
			fs = fsMap[fsName]
			if fs == nil {
				td.Fatalf(t, "no fs %q", fsName)
			}
		case "f.write":
			_, err = f.Write([]byte(strings.TrimSpace(td.Input)))
		case "f.sync":
			err = f.Sync()
		case "f.read":
			n, _ := strconv.Atoi(td.CmdArgs[0].String())
			buf := make([]byte, n)
			_, err = io.ReadFull(f, buf)
			if err != nil {
				break
			}
			return string(buf)
		case "f.readat":
			n, _ := strconv.Atoi(td.CmdArgs[0].String())
			off, _ := strconv.Atoi(td.CmdArgs[0].String())
			buf := make([]byte, n)
			_, err = f.ReadAt(buf, int64(off))
			if err != nil {
				break
			}
			return string(buf)
		case "f.close":
			f, err = nil, f.Close()
		case "f.stat.name":
			var fi FileInfo
			fi, err = f.Stat()
			if err != nil {
				break
			}
			return fi.Name()
		case "list":
			list, err := fs.List(td.CmdArgs[0].String())
			if err != nil {
				break
			}
			sort.Strings(list)
			return strings.Join(list, "\n")
		case "fs-string":
			return fs.String()
		default:
			t.Fatalf("unknown command %q", td.Cmd)
		}
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return ""
	})
	// Both "" and "/" are allowed to be used to refer to the root of the FS
	// for the purposes of cloning.
	checkClonedIsEquivalent(t, fs, "")
	checkClonedIsEquivalent(t, fs, "/")
}

// Test that the FS can be cloned and that the clone serializes identically.
func checkClonedIsEquivalent(t *testing.T, fs *MemFS, path string) {
	t.Helper()
	clone := NewMem()
	cloned, err := Clone(fs, clone, path, path)
	require.NoError(t, err)
	require.True(t, cloned)
	require.Equal(t, fs.String(), clone.String())
}

func TestMemFSBasics(t *testing.T) {
	runMemFSDataDriven(t, "testdata/memfs_basics", NewMem())
}

func TestMemFSList(t *testing.T) {
	runMemFSDataDriven(t, "testdata/memfs_list", NewMem())
}

func TestMemFSCrashable(t *testing.T) {
	runMemFSDataDriven(t, "testdata/memfs_crashable", NewCrashableMem())
}

func TestMemFile(t *testing.T) {
	want := "foo"
	f := NewMemFile([]byte(want))
	buf, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if got := string(buf); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestMemFSWalkDir(t *testing.T) {
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

	// Test 1: Full walk from root, collect all paths
	var paths []string
	err := fs.WalkDir("/", func(path string, d iofs.DirEntry, err error) error {
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

	// Test 2: SkipDir skips children
	var skipped []string
	err = fs.WalkDir("/", func(path string, d iofs.DirEntry, err error) error {
		require.NoError(t, err)
		skipped = append(skipped, path)
		if path == "/a/b" {
			return iofs.SkipDir
		}
		return nil
	})
	require.NoError(t, err)
	// /a/b is visited but its children are not; /a/d continues
	require.Contains(t, skipped, "/a/b")
	for _, p := range skipped {
		require.NotEqual(t, "/a/b/c", p)
		require.NotEqual(t, "/a/b/f2", p)
	}
	require.Contains(t, skipped, "/a/d")

	// Test 3: Non-existent root
	err = fs.WalkDir("/nonexistent", func(path string, d iofs.DirEntry, err error) error {
		require.Error(t, err)
		return err
	})
	require.Error(t, err)
}

func TestMemFSMountReadOnly(t *testing.T) {
	// Set up a real temp directory with test files.
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "sub"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "data.txt"), []byte("hello from mount"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "sub", "nested.txt"), []byte("nested content"), 0644))

	mfs := NewMem()
	require.NoError(t, mfs.MountReadOnlyRealDir(tmpDir, "/assets"))

	// Test 1: Read fallthrough — open mounted file
	t.Run("ReadFallthrough", func(t *testing.T) {
		f, err := mfs.Open("/assets/data.txt")
		require.NoError(t, err)
		defer f.Close()
		buf, err := io.ReadAll(f)
		require.NoError(t, err)
		require.Equal(t, "hello from mount", string(buf))
	})

	// Test 2: Stat fallthrough
	t.Run("StatFallthrough", func(t *testing.T) {
		info, err := mfs.Stat("/assets/data.txt")
		require.NoError(t, err)
		require.Equal(t, "data.txt", info.Name())
		require.Equal(t, int64(len("hello from mount")), info.Size())
	})

	// Test 3: List fallthrough
	t.Run("ListFallthrough", func(t *testing.T) {
		entries, err := mfs.List("/assets/")
		require.NoError(t, err)
		sort.Strings(entries)
		require.Equal(t, []string{"data.txt", "sub"}, entries)
	})

	// Test 4: ReadDir fallthrough
	t.Run("ReadDirFallthrough", func(t *testing.T) {
		entries, err := mfs.ReadDir("/assets/")
		require.NoError(t, err)
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		sort.Strings(names)
		require.Equal(t, []string{"data.txt", "sub"}, names)
	})

	// Test 5: WalkDir fallthrough
	t.Run("WalkDirFallthrough", func(t *testing.T) {
		var paths []string
		err := mfs.WalkDir("/assets", func(p string, d iofs.DirEntry, err error) error {
			require.NoError(t, err)
			paths = append(paths, p)
			return nil
		})
		require.NoError(t, err)
		sort.Strings(paths)
		expected := []string{"/assets", "/assets/data.txt", "/assets/sub", "/assets/sub/nested.txt"}
		require.Equal(t, expected, paths)
	})

	// Test 6: Disjoint write succeeds
	t.Run("DisjointWrite", func(t *testing.T) {
		require.NoError(t, mfs.MkdirAll("/work", 0755))
		f, err := mfs.Create("/work/out.txt", WriteCategoryUnspecified)
		require.NoError(t, err)
		_, err = f.Write([]byte("overlay data"))
		require.NoError(t, err)
		require.NoError(t, f.Close())

		f, err = mfs.Open("/work/out.txt")
		require.NoError(t, err)
		defer f.Close()
		buf, err := io.ReadAll(f)
		require.NoError(t, err)
		require.Equal(t, "overlay data", string(buf))
	})

	// Test 7: MkdirAll conflict detection
	t.Run("MkdirAllConflict", func(t *testing.T) {
		err := mfs.MkdirAll("/assets", 0755)
		require.Error(t, err)
		require.Contains(t, err.Error(), "conflicts with read-only mount")
	})

	// Test 8: Create conflict detection
	t.Run("CreateConflict", func(t *testing.T) {
		_, err := mfs.Create("/assets/data.txt", WriteCategoryUnspecified)
		require.Error(t, err)
		require.Contains(t, err.Error(), "conflicts with read-only mount")
	})

	// Test 9: Mount rejects duplicate mount point
	t.Run("DuplicateMount", func(t *testing.T) {
		err := mfs.MountReadOnlyRealDir(tmpDir, "/assets")
		require.Error(t, err)
		require.Contains(t, err.Error(), "already mounted")
	})

	// Test 10: No mounts (NewMem) has no regression
	t.Run("NoMounts", func(t *testing.T) {
		plain := NewMem()
		require.Nil(t, plain.Unwrap())

		require.NoError(t, plain.MkdirAll("/dir", 0755))
		f, err := plain.Create("/dir/file", WriteCategoryUnspecified)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		_, err = plain.Open("/nonexistent")
		require.Error(t, err)

		_, err = plain.List("/nonexistent/")
		require.Error(t, err)
	})

	// Test 11: Overlay directory ownership — file not in overlay dir
	// doesn't fall through to mount
	t.Run("OverlayOwnership", func(t *testing.T) {
		_, err := mfs.Open("/work/nonexistent")
		require.Error(t, err)
	})

	// Test 12: Mount non-existent real dir errors
	t.Run("MountNonExistent", func(t *testing.T) {
		plain := NewMem()
		err := plain.MountReadOnlyRealDir("/no/such/dir", "/mnt")
		require.Error(t, err)
	})

	// Test 13: defaultFS rejects MountReadOnlyRealDir
	t.Run("DefaultFSRejects", func(t *testing.T) {
		err := Default.MountReadOnlyRealDir(tmpDir, "/mnt")
		require.Error(t, err)
		require.Contains(t, err.Error(), "only supported on MemFS")
	})

	// Test 14: Open mount point itself as directory
	t.Run("OpenMountPoint", func(t *testing.T) {
		f, err := mfs.Open("/assets")
		require.NoError(t, err)
		defer f.Close()
		info, err := f.Stat()
		require.NoError(t, err)
		require.True(t, info.IsDir())
	})
}

func TestMemFSLock(t *testing.T) {
	filesystems := map[string]FS{}
	fileLocks := map[string]io.Closer{}

	datadriven.RunTest(t, "testdata/memfs_lock", func(t *testing.T, td *datadriven.TestData) string {
		switch td.Cmd {
		case "mkfs":
			for _, arg := range td.CmdArgs {
				filesystems[arg.String()] = NewMem()
			}
			return "OK"

		// lock fs=<filesystem-name> handle=<handle> path=<path>
		case "lock":
			var filesystemName string
			var path string
			var handle string
			td.ScanArgs(t, "fs", &filesystemName)
			td.ScanArgs(t, "path", &path)
			td.ScanArgs(t, "handle", &handle)
			fs := filesystems[filesystemName]
			if fs == nil {
				return fmt.Sprintf("filesystem %q doesn't exist", filesystemName)
			}
			l, err := fs.Lock(path)
			if err != nil {
				return err.Error()
			}
			fileLocks[handle] = l
			return "OK"

		// mkdirall fs=<filesystem-name> path=<path>
		case "mkdirall":
			var filesystemName string
			var path string
			td.ScanArgs(t, "fs", &filesystemName)
			td.ScanArgs(t, "path", &path)
			fs := filesystems[filesystemName]
			if fs == nil {
				return fmt.Sprintf("filesystem %q doesn't exist", filesystemName)
			}
			err := fs.MkdirAll(path, 0755)
			if err != nil {
				return err.Error()
			}
			return "OK"

		// close handle=<handle>
		case "close":
			var handle string
			td.ScanArgs(t, "handle", &handle)
			err := fileLocks[handle].Close()
			delete(fileLocks, handle)
			if err != nil {
				return err.Error()
			}
			return "OK"
		default:
			return fmt.Sprintf("unrecognized command %q", td.Cmd)
		}
	})
}
