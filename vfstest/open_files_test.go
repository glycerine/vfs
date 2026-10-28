package vfstest

import (
	"bytes"
	"os"
	"testing"

	"github.com/glycerine/vfs"
)

func TestOpenFiles(t *testing.T) {
	var buf bytes.Buffer
	fs, dumpOpenFiles := WithOpenFileTracking(vfs.NewMem())
	if err := fs.MkdirAll("dir", os.ModePerm); err != nil {
		t.Fatal(err)
	}

	type fileOpenOp struct {
		name string
		fn   func() vfs.File
	}
	fileOpenOps := []fileOpenOp{
		{name: "OpenDir", fn: func() vfs.File {
			f, err := fs.OpenDir("dir")
			if err != nil {
				t.Fatal(err)
			}
			return f
		}},
		{name: "Create", fn: func() vfs.File {
			f, err := fs.Create("foo", vfs.WriteCategoryUnspecified)
			if err != nil {
				t.Fatal(err)
			}
			return f
		}},
		{name: "Open", fn: func() vfs.File {
			f, err := fs.Open("foo")
			if err != nil {
				t.Fatal(err)
			}
			return f
		}},
		{name: "OpenReadWrite", fn: func() vfs.File {
			f, err := fs.Open("foo")
			if err != nil {
				t.Fatal(err)
			}
			return f
		}},
		{name: "ReuseForWrite", fn: func() vfs.File {
			f, err := fs.ReuseForWrite("foo", "bar", vfs.WriteCategoryUnspecified)
			if err != nil {
				t.Fatal(err)
			}
			return f
		}},
	}

	t.Run("leaks", func(t *testing.T) {
		for _, op := range fileOpenOps {
			t.Run(op.name, func(t *testing.T) {
				f := op.fn()
				buf.Reset()
				dumpOpenFiles(&buf)
				t.Log(buf.String())
				if buf.Len() == 0 {
					t.Fatal("open-file dump is empty")
				}
				if err := f.Close(); err != nil {
					t.Fatal(err)
				}
				buf.Reset()
				dumpOpenFiles(&buf)
				if buf.Len() != 0 {
					t.Fatalf("open-file dump after close: %s", &buf)
				}
			})
		}
	})
	t.Run("noleaks", func(t *testing.T) {
		for _, op := range fileOpenOps {
			t.Run(op.name, func(t *testing.T) {
				f := op.fn()
				if err := f.Close(); err != nil {
					t.Fatal(err)
				}
				buf.Reset()
				dumpOpenFiles(&buf)
				if buf.Len() != 0 {
					t.Fatalf("open-file dump after close: %s", &buf)
				}
			})
		}
	})
}
