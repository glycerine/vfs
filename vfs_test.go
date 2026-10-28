package vfs

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/glycerine/vfs/internal/datadriven"
	"github.com/glycerine/vfs/internal/oserror"
)

func normalizeError(err error) error {
	// It is OS-specific which errors match IsExist, IsNotExist, and
	// IsPermission, with OS-specific error messages. We normalize to the
	// oserror.Err* errors which have standard error messages across
	// platforms.
	switch {
	case oserror.IsExist(err):
		return oserror.ErrExist
	case oserror.IsNotExist(err):
		return oserror.ErrNotExist
	case oserror.IsPermission(err):
		return oserror.ErrPermission
	}
	return err
}

// vfsTestFS is similar to loggingFS but is more specific to the vfs test. It
// logs more operations and logs return values and errors.
// It also supports injecting an error on Link.
type vfsTestFS struct {
	FS
	base    string
	w       io.Writer
	linkErr error
}

func (fs vfsTestFS) stripBase(path string) string {
	if strings.HasPrefix(path, fs.base+"/") {
		return path[len(fs.base)+1:]
	}
	return path
}

func (fs vfsTestFS) Create(name string, category DiskWriteCategory) (File, error) {
	f, err := fs.FS.Create(name, category)
	fmt.Fprintf(fs.w, "create: %s [%v]\n", fs.stripBase(name), normalizeError(err))
	return vfsTestFSFile{f, fs.PathBase(name), fs.w}, err
}

func (fs vfsTestFS) Link(oldname, newname string) error {
	err := fs.linkErr
	if err == nil {
		err = fs.FS.Link(oldname, newname)
	}
	fmt.Fprintf(fs.w, "link: %s -> %s [%v]\n",
		fs.stripBase(oldname), fs.stripBase(newname), normalizeError(err))
	return err
}

func (fs vfsTestFS) ReuseForWrite(
	oldname, newname string, category DiskWriteCategory,
) (File, error) {
	f, err := fs.FS.ReuseForWrite(oldname, newname, category)
	if err == nil {
		f = vfsTestFSFile{f, fs.PathBase(newname), fs.w}
	}
	fmt.Fprintf(fs.w, "reuseForWrite: %s -> %s [%v]\n",
		fs.stripBase(oldname), fs.stripBase(newname), normalizeError(err))
	return f, err
}

func (fs vfsTestFS) MkdirAll(dir string, perm os.FileMode) error {
	err := fs.FS.MkdirAll(dir, perm)
	fmt.Fprintf(fs.w, "mkdir: %s [%v]\n", fs.stripBase(dir), normalizeError(err))
	return err
}

func (fs vfsTestFS) Open(name string, opts ...OpenOption) (File, error) {
	f, err := fs.FS.Open(name, opts...)
	fmt.Fprintf(fs.w, "open: %s [%v]\n", fs.stripBase(name), normalizeError(err))
	return vfsTestFSFile{f, fs.stripBase(name), fs.w}, err
}

func (fs vfsTestFS) Remove(name string) error {
	err := fs.FS.Remove(name)
	fmt.Fprintf(fs.w, "remove: %s [%v]\n", fs.stripBase(name), normalizeError(err))
	return err
}

func (fs vfsTestFS) RemoveAll(name string) error {
	err := fs.FS.RemoveAll(name)
	fmt.Fprintf(fs.w, "remove-all: %s [%v]\n", fs.stripBase(name), normalizeError(err))
	return err
}

type vfsTestFSFile struct {
	File
	name string
	w    io.Writer
}

func (f vfsTestFSFile) Close() error {
	err := f.File.Close()
	fmt.Fprintf(f.w, "close: %s [%v]\n", f.name, err)
	return err
}

func (f vfsTestFSFile) Preallocate(off, n int64) error {
	err := f.File.Preallocate(off, n)
	fmt.Fprintf(f.w, "preallocate(off=%d,n=%d): %s [%v]\n", off, n, f.name, err)
	return err
}

func (f vfsTestFSFile) Sync() error {
	err := f.File.Sync()
	fmt.Fprintf(f.w, "sync: %s [%v]\n", f.name, err)
	return err
}

func (f vfsTestFSFile) SyncData() error {
	err := f.File.SyncData()
	fmt.Fprintf(f.w, "sync-data: %s [%v]\n", f.name, err)
	return err
}

func (f vfsTestFSFile) SyncTo(length int64) (fullSync bool, err error) {
	fullSync, err = f.File.SyncTo(length)
	fmt.Fprintf(f.w, "sync-to(%d): %s [%t,%v]\n", length, f.name, fullSync, err)
	return fullSync, err
}

func runTestVFS(t *testing.T, baseFS FS, dir string) {
	var buf bytes.Buffer
	fs := vfsTestFS{FS: baseFS, base: dir, w: &buf}

	datadriven.RunTest(t, "testdata/vfs", func(t *testing.T, td *datadriven.TestData) string {
		switch td.Cmd {
		case "define":
			buf.Reset()

			for _, arg := range td.CmdArgs {
				switch arg.Key {
				case "linkErr":
					if len(arg.Vals) != 1 {
						return fmt.Sprintf("%s: %s expected 1 value", td.Cmd, arg.Key)
					}
					switch arg.Vals[0] {
					case "ErrExist":
						fs.linkErr = oserror.ErrExist
					case "ErrNotExist":
						fs.linkErr = oserror.ErrNotExist
					case "ErrPermission":
						fs.linkErr = oserror.ErrPermission
					default:
						fs.linkErr = errors.New(arg.Vals[0])
					}
				default:
					return fmt.Sprintf("%s: unknown arg: %s", td.Cmd, arg.Key)
				}
			}

			for line := range strings.SplitSeq(td.Input, "\n") {
				parts := strings.Fields(line)
				if len(parts) == 0 {
					return "<op> [<args>]"
				}

				switch parts[0] {
				case "clone":
					if len(parts) < 3 {
						return "clone <src> <dest> [disk|mem] [link] [sync]"
					}
					dstFS := fs
					var opts []CloneOption
					for _, p := range parts[3:] {
						switch p {
						case "disk":
							dstFS = vfsTestFS{FS: Default, base: dir, w: &buf}
						case "mem":
							dstFS = vfsTestFS{FS: NewMem(), base: dir, w: &buf}
						case "link":
							opts = append(opts, CloneTryLink)
						case "sync":
							opts = append(opts, CloneSync)
						default:
							return fmt.Sprintf("unrecognized argument %q", p)
						}
					}

					_, _ = Clone(fs, dstFS, fs.PathJoin(dir, parts[1]), fs.PathJoin(dir, parts[2]), opts...)

				case "create":
					if len(parts) != 2 {
						return "create <name>"
					}
					f, _ := fs.Create(fs.PathJoin(dir, parts[1]), WriteCategoryUnspecified)
					f.Close()

				case "link":
					if len(parts) != 3 {
						return "link <oldname> <newname>"
					}
					_ = fs.Link(fs.PathJoin(dir, parts[1]), fs.PathJoin(dir, parts[2]))

				case "link-or-copy":
					if len(parts) != 3 {
						return "link-or-copy <oldname> <newname>"
					}
					_ = LinkOrCopy(fs, fs.PathJoin(dir, parts[1]), fs.PathJoin(dir, parts[2]))

				case "reuseForWrite":
					if len(parts) != 3 {
						return "reuseForWrite <oldname> <newname>"
					}
					_, _ = fs.ReuseForWrite(fs.PathJoin(dir, parts[1]), fs.PathJoin(dir, parts[2]), WriteCategoryUnspecified)

				case "list":
					if len(parts) != 2 {
						return "list <dir>"
					}
					paths, _ := fs.List(fs.PathJoin(dir, parts[1]))
					sort.Strings(paths)
					for _, p := range paths {
						fmt.Fprintln(&buf, p)
					}

				case "mkdir":
					if len(parts) != 2 {
						return "mkdir <dir>"
					}
					_ = fs.MkdirAll(fs.PathJoin(dir, parts[1]), 0755)

				case "remove":
					if len(parts) != 2 {
						return "remove <name>"
					}
					_ = fs.Remove(fs.PathJoin(dir, parts[1]))

				case "remove-all":
					if len(parts) != 2 {
						return "remove-all <name>"
					}
					_ = fs.RemoveAll(fs.PathJoin(dir, parts[1]))
				}
			}

			return buf.String()

		default:
			return fmt.Sprintf("unknown command: %s", td.Cmd)
		}
	})
}

func TestVFS(t *testing.T) {
	t.Run("mem", func(t *testing.T) {
		runTestVFS(t, NewMem(), "")
	})
	if runtime.GOOS != "windows" {
		t.Run("disk", func(t *testing.T) {
			dir, err := os.MkdirTemp("", "test-vfs")
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				_ = os.RemoveAll(dir)
			}()
			runTestVFS(t, Default, dir)
		})
	}
}

func TestVFSGetDiskUsage(t *testing.T) {
	dir, err := os.MkdirTemp("", "test-free-space")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.RemoveAll(dir)
	}()
	_, err = Default.GetDiskUsage(dir)
	if got := err; got != nil {
		t.Fatalf("got %v, want nil", got)
	}
}

func TestVFSCreateLinkSemantics(t *testing.T) {
	dir, err := os.MkdirTemp("", "test-create-link")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	for _, fs := range []FS{Default, NewMem()} {
		t.Run(fmt.Sprintf("%T", fs), func(t *testing.T) {
			writeFile := func(path, contents string) {
				path = fs.PathJoin(dir, path)
				f, err := fs.Create(path, WriteCategoryUnspecified)
				if err != nil {
					t.Fatal(err)
				}
				_, err = f.Write([]byte(contents))
				if err != nil {
					t.Fatal(err)
				}
				if err := f.Close(); err != nil {
					t.Fatal(err)
				}
			}
			readFile := func(path string) string {
				path = fs.PathJoin(dir, path)
				f, err := fs.Open(path)
				if err != nil {
					t.Fatal(err)
				}
				b, err := io.ReadAll(f)
				if err != nil {
					t.Fatal(err)
				}
				if err := f.Close(); err != nil {
					t.Fatal(err)
				}
				return string(b)
			}
			if err := fs.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}

			// Write a file 'foo' and create a hardlink at 'bar'.
			writeFile("foo", "foo")
			if err := fs.Link(fs.PathJoin(dir, "foo"), fs.PathJoin(dir, "bar")); err != nil {
				t.Fatal(err)
			}

			// Both files should contain equal contents, because they're backed by
			// the same inode.
			if got := readFile("foo"); got != "foo" {
				t.Fatalf("foo contains %q", got)
			}
			if got := readFile("bar"); got != "foo" {
				t.Fatalf("bar contains %q, want foo", got)
			}

			// Calling Create on 'bar' must NOT truncate 'foo'. It should create a
			// new file at path 'bar' with a new inode.
			writeFile("bar", "bar")

			if got := readFile("foo"); got != "foo" {
				t.Fatalf("foo contains %q", got)
			}
			if got := readFile("bar"); got != "bar" {
				t.Fatalf("bar contains %q", got)
			}
		})
	}
}

// TestVFSRootDirName ensures that opening the root directory on both the
// Default and MemFS works and returns a File which has the name of the
// path separator.
func TestVFSRootDirName(t *testing.T) {
	for _, fs := range []FS{Default, NewMem()} {
		sep := sep
		if fs == Default {
			sep = string(os.PathSeparator)
		}
		rootDir, err := fs.Open(sep)
		if err != nil {
			t.Fatal(err)
		}
		fi, err := rootDir.Stat()
		if err != nil {
			t.Fatal(err)
		}
		if got := fi.Name(); got != sep {
			t.Fatalf("root name = %q, want %q", got, sep)
		}
	}
}

// TestOpType is intended to catch operations that have been added without an
// associated string, which could result in a runtime panic.
func TestOpType(t *testing.T) {
	for i := 0; i < int(opTypeMax); i++ {
		_ = OpType(i).String()
	}
}
