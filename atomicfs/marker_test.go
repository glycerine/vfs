package atomicfs

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/glycerine/vfs"
	"github.com/glycerine/vfs/errorfs"
	"github.com/glycerine/vfs/internal/datadriven"
)

func TestMarker_FilenameRoundtrip(t *testing.T) {
	filenames := []string{
		"marker.foo.000003.MANIFEST-000021",
		"marker.bar.000003.MANIFEST-000021",
		"marker.version.000003.1",
		"marker.version.000003.1.2.3.4",
		"marker.current.500000.MANIFEST-000001",
		"marker.current.18446744073709551615.MANIFEST-000001",
	}
	for _, testFilename := range filenames {
		t.Run(testFilename, func(t *testing.T) {
			name, iter, value, err := parseMarkerFilename(testFilename)
			if err != nil {
				t.Fatal(err)
			}

			filename := markerFilename(name, iter, value)
			if filename != testFilename {
				t.Fatalf("filename = %q, want %q", filename, testFilename)
			}
		})
	}
}

func TestMarker_Parsefilename(t *testing.T) {
	testCases := map[string]bool{
		"marker.current.000003.MANIFEST-000021":  false,
		"marker.current.10.MANIFEST-000021":      false,
		"marker.v.10.1.2.3.4":                    false,
		"marker.name.18446744073709551615.value": false,
		"marke.current.000003.MANIFEST-000021":   true,
		"marker.current.foo.MANIFEST-000021":     true,
		"marker.current.ffffff.MANIFEST-000021":  true,
	}
	for filename, wantErr := range testCases {
		t.Run(filename, func(t *testing.T) {
			_, _, _, err := parseMarkerFilename(filename)
			if (err != nil) != wantErr {
				t.Fatalf("error = %v, wantErr %t", err, wantErr)
			}
		})
	}
}

func TestMarker(t *testing.T) {
	markers := map[string]*Marker{}
	memFS := vfs.NewMem()

	var buf bytes.Buffer
	datadriven.RunTest(t, "testdata/marker", func(t *testing.T, td *datadriven.TestData) string {
		switch td.Cmd {
		case "list":
			ls, err := memFS.List(td.CmdArgs[0].String())
			if err != nil {
				return err.Error()
			}
			sort.Strings(ls)
			buf.Reset()
			for _, filename := range ls {
				fmt.Fprintln(&buf, filename)
			}
			return buf.String()

		case "locate":
			var dir, marker string
			td.ScanArgs(t, "dir", &dir)
			td.ScanArgs(t, "marker", &marker)
			m, v, err := LocateMarker(memFS, dir, marker)
			if err != nil {
				return err.Error()
			}
			p := memFS.PathJoin(dir, marker)
			if oldMarker := markers[p]; oldMarker != nil {
				if err := oldMarker.Close(); err != nil {
					return err.Error()
				}
			}

			markers[p] = m
			return v

		case "mkdir-all":
			if len(td.CmdArgs) != 1 {
				return "usage: mkdir-all <dir>"
			}
			if err := memFS.MkdirAll(td.CmdArgs[0].String(), os.ModePerm); err != nil {
				return err.Error()
			}
			return ""

		case "move":
			var dir, marker string
			td.ScanArgs(t, "dir", &dir)
			td.ScanArgs(t, "marker", &marker)
			m := markers[memFS.PathJoin(dir, marker)]
			if m == nil {
				t.Fatal("unexpected nil")
			}
			err := m.Move(td.Input)
			if err != nil {
				return err.Error()
			}
			return ""

		case "next-iter":
			var dir, marker string
			td.ScanArgs(t, "dir", &dir)
			td.ScanArgs(t, "marker", &marker)
			m := markers[memFS.PathJoin(dir, marker)]
			if m == nil {
				t.Fatal("unexpected nil")
			}
			return fmt.Sprintf("%d", m.NextIter())

		case "read":
			var dir, marker string
			td.ScanArgs(t, "dir", &dir)
			td.ScanArgs(t, "marker", &marker)
			v, err := ReadMarker(memFS, dir, marker)
			if err != nil {
				return err.Error()
			}
			return v

		case "remove-obsolete":
			var dir, marker string
			td.ScanArgs(t, "dir", &dir)
			td.ScanArgs(t, "marker", &marker)
			m := markers[memFS.PathJoin(dir, marker)]
			if m == nil {
				t.Fatal("unexpected nil")
			}
			obsoleteCount := len(m.obsoleteFiles)
			if err := m.RemoveObsolete(); err != nil {
				t.Fatal(err)
			}
			removedCount := obsoleteCount - len(m.obsoleteFiles)
			return fmt.Sprintf("Removed %d files.", removedCount)

		case "touch":
			for filename := range strings.SplitSeq(td.Input, "\n") {
				f, err := memFS.Create(filename, vfs.WriteCategoryUnspecified)
				if err != nil {
					return err.Error()
				}
				if err := f.Close(); err != nil {
					return err.Error()
				}
			}
			return ""

		default:
			panic(fmt.Sprintf("unknown command %q", td.Cmd))
		}
	})
}

func TestMarker_StrictSync(t *testing.T) {
	// Use an in-memory FS that strictly enforces syncs.
	mem := vfs.NewCrashableMem()
	syncDir := func(dir string) {
		fdir, err := mem.OpenDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		if err := fdir.Sync(); err != nil {
			t.Fatal(err)
		}
		if err := fdir.Close(); err != nil {
			t.Fatal(err)
		}
	}

	if err := mem.MkdirAll("foo", os.ModePerm); err != nil {
		t.Fatal(err)
	}
	syncDir("")
	m, v, err := LocateMarker(mem, "foo", "bar")
	if err != nil {
		t.Fatal(err)
	}
	if v != "" {
		t.Fatalf("value = %q, want empty", v)
	}
	if err := m.Move("hello"); err != nil {
		t.Fatal(err)
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}

	// Discard any unsynced writes to make sure we set up the test
	// preconditions correctly.
	crashFS := mem.CrashClone(vfs.CrashCloneCfg{UnsyncedDataPercent: 0})
	m, v, err = LocateMarker(crashFS, "foo", "bar")
	if err != nil {
		t.Fatal(err)
	}
	if v != "hello" {
		t.Fatalf("value = %q, want hello", v)
	}
	if err := m.Move("hello-world"); err != nil {
		t.Fatal(err)
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}

	// Discard any unsynced writes.
	crashFS = crashFS.CrashClone(vfs.CrashCloneCfg{UnsyncedDataPercent: 0})
	m, v, err = LocateMarker(crashFS, "foo", "bar")
	if err != nil {
		t.Fatal(err)
	}
	if v != "hello-world" {
		t.Fatalf("value = %q, want hello-world", v)
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestMarker_FaultTolerance attempts a series of operations on atomic
// markers, injecting errors at successively higher indexed operations.
// It completes when an error is never injected, because the index is
// higher than the number of filesystem operations performed by the
// test.
func TestMarker_FaultTolerance(t *testing.T) {
	done := false
	for i := 1; !done && i < 1000; i++ {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			var count atomic.Int32
			count.Store(int32(i))
			inj := errorfs.InjectorFunc(func(op errorfs.Op) error {
				// Don't inject on Sync errors. They're fatal.
				if op.Kind == errorfs.OpFileSync {
					return nil
				}
				if v := count.Add(-1); v == 0 {
					return errorfs.ErrInjected
				}
				return nil
			})

			mem := vfs.NewMem()
			fs := errorfs.Wrap(mem, inj)
			markers := map[string]*Marker{}
			ops := []struct {
				op    string
				name  string
				value string
			}{
				{op: "locate", name: "foo", value: ""},
				{op: "locate", name: "foo", value: ""},
				{op: "locate", name: "bar", value: ""},
				{op: "rm-obsolete", name: "foo"},
				{op: "move", name: "bar", value: "california"},
				{op: "rm-obsolete", name: "bar"},
				{op: "move", name: "bar", value: "california"},
				{op: "move", name: "bar", value: "new-york"},
				{op: "locate", name: "bar", value: "new-york"},
				{op: "move", name: "bar", value: "california"},
				{op: "rm-obsolete", name: "bar"},
				{op: "locate", name: "bar", value: "california"},
				{op: "move", name: "foo", value: "connecticut"},
				{op: "locate", name: "foo", value: "connecticut"},
			}

			for _, op := range ops {
				runOp := func() error {
					switch op.op {
					case "locate":
						m, v, err := LocateMarker(fs, "", op.name)
						if err != nil {
							return err
						}
						if m == nil {
							t.Fatal("unexpected nil")
						}
						if v != op.value {
							t.Fatalf("value = %q, want %q", v, op.value)
						}
						if existingMarker := markers[op.name]; existingMarker != nil {
							if err := existingMarker.Close(); err != nil {
								t.Fatal(err)
							}
						}
						markers[op.name] = m
						return nil
					case "move":
						m := markers[op.name]
						if m == nil {
							t.Fatal("unexpected nil")
						}
						return m.Move(op.value)
					case "rm-obsolete":
						m := markers[op.name]
						if m == nil {
							t.Fatal("unexpected nil")
						}
						return m.RemoveObsolete()
					default:
						panic("unreachable")
					}
				}

				// Run the operation, if it fails with the injected
				// error, retry it exactly once. The retry should always
				// succeed.
				err := runOp()
				if errors.Is(err, errorfs.ErrInjected) {
					err = runOp()
				}
				if err != nil {
					t.Fatal(err)
				}
			}

			for _, m := range markers {
				if err := m.Close(); err != nil {
					t.Fatal(err)
				}
			}

			// Stop if the number of operations in the test case is
			// fewer than `i`.
			done = count.Load() > 0
		})
	}
}
