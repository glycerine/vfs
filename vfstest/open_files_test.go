package vfstest

import (
	"bytes"
	"os"
	"testing"

	"github.com/glycerine/vfs"
	"github.com/stretchr/testify/require"
)

func TestOpenFiles(t *testing.T) {
	var buf bytes.Buffer
	fs, dumpOpenFiles := WithOpenFileTracking(vfs.NewMem())
	require.NoError(t, fs.MkdirAll("dir", os.ModePerm))

	type fileOpenOp struct {
		name string
		fn   func() vfs.File
	}
	fileOpenOps := []fileOpenOp{
		{name: "OpenDir", fn: func() vfs.File {
			f, err := fs.OpenDir("dir")
			require.NoError(t, err)
			return f
		}},
		{name: "Create", fn: func() vfs.File {
			f, err := fs.Create("foo", vfs.WriteCategoryUnspecified)
			require.NoError(t, err)
			return f
		}},
		{name: "Open", fn: func() vfs.File {
			f, err := fs.Open("foo")
			require.NoError(t, err)
			return f
		}},
		{name: "OpenReadWrite", fn: func() vfs.File {
			f, err := fs.Open("foo")
			require.NoError(t, err)
			return f
		}},
		{name: "ReuseForWrite", fn: func() vfs.File {
			f, err := fs.ReuseForWrite("foo", "bar", vfs.WriteCategoryUnspecified)
			require.NoError(t, err)
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
				require.Greater(t, buf.Len(), 0)
				require.NoError(t, f.Close())
				buf.Reset()
				dumpOpenFiles(&buf)
				require.Equal(t, 0, buf.Len())
			})
		}
	})
	t.Run("noleaks", func(t *testing.T) {
		for _, op := range fileOpenOps {
			t.Run(op.name, func(t *testing.T) {
				f := op.fn()
				require.NoError(t, f.Close())
				buf.Reset()
				dumpOpenFiles(&buf)
				require.Equal(t, 0, buf.Len())
			})
		}
	})
}
