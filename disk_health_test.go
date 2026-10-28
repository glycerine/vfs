package vfs

import (
	"errors"
	"fmt"
	"io"
	iofs "io/fs"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glycerine/vfs/internal/crtime"
)

type mockFile struct {
	syncAndWriteDuration time.Duration
}

func (m mockFile) Close() error {
	return nil
}

func (m mockFile) Truncate(size int64) error {
	panic("unimplemented")
}

func (m mockFile) Read(p []byte) (n int, err error) {
	panic("unimplemented")
}

func (m mockFile) ReadAt(p []byte, off int64) (n int, err error) {
	panic("unimplemented")
}

func (m mockFile) Write(p []byte) (n int, err error) {
	time.Sleep(m.syncAndWriteDuration)
	return len(p), nil
}

func (m mockFile) WriteAt(p []byte, ofs int64) (n int, err error) {
	time.Sleep(m.syncAndWriteDuration)
	return len(p), nil
}

func (m mockFile) Prefetch(offset, length int64) error {
	panic("unimplemented")
}

func (m mockFile) Preallocate(int64, int64) error {
	time.Sleep(m.syncAndWriteDuration)
	return nil
}

func (m mockFile) Stat() (FileInfo, error) {
	panic("unimplemented")
}

func (m mockFile) Fd() uintptr {
	return InvalidFd
}

func (m mockFile) Sync() error {
	time.Sleep(m.syncAndWriteDuration)
	return nil
}

func (m mockFile) SyncData() error {
	time.Sleep(m.syncAndWriteDuration)
	return nil
}

func (m mockFile) SyncTo(int64) (fullSync bool, err error) {
	time.Sleep(m.syncAndWriteDuration)
	return false, nil
}

func (m mockFile) Name() string {
	panic("unimplemented")
	return ""
}

var _ File = &mockFile{}

type mockFS struct {
	create        func(string, DiskWriteCategory) (File, error)
	link          func(string, string) error
	list          func(string) ([]string, error)
	lock          func(string) (io.Closer, error)
	mkdirAll      func(string, os.FileMode) error
	open          func(string, ...OpenOption) (File, error)
	openDir       func(string) (File, error)
	pathBase      func(string) string
	pathJoin      func(...string) string
	pathDir       func(string) string
	remove        func(string) error
	removeAll     func(string) error
	rename        func(string, string) error
	reuseForWrite func(string, string, DiskWriteCategory) (File, error)
	stat          func(string) (FileInfo, error)
	getDiskUsage  func(string) (DiskUsage, error)
	readDir       func(string) ([]os.DirEntry, error)
	walkDir       func(path string, f iofs.WalkDirFunc) error
}

func (m mockFS) Create(name string, category DiskWriteCategory) (File, error) {
	if m.create == nil {
		panic("unimplemented")
	}
	return m.create(name, category)
}

func (m mockFS) Link(oldname, newname string) error {
	if m.link == nil {
		panic("unimplemented")
	}
	return m.link(oldname, newname)
}

func (m mockFS) Open(name string, opts ...OpenOption) (File, error) {
	if m.open == nil {
		panic("unimplemented")
	}
	return m.open(name, opts...)
}

func (m mockFS) OpenReadWrite(
	name string, category DiskWriteCategory, opts ...OpenOption,
) (File, error) {
	panic("unimplemented")
}

func (m mockFS) OpenDir(name string) (File, error) {
	if m.openDir == nil {
		panic("unimplemented")
	}
	return m.openDir(name)
}

func (m mockFS) Remove(name string) error {
	if m.remove == nil {
		panic("unimplemented")
	}
	return m.remove(name)
}

func (m mockFS) RemoveAll(name string) error {
	if m.removeAll == nil {
		panic("unimplemented")
	}
	return m.removeAll(name)
}

func (m mockFS) Rename(oldname, newname string) error {
	if m.rename == nil {
		panic("unimplemented")
	}
	return m.rename(oldname, newname)
}

func (m mockFS) ReuseForWrite(oldname, newname string, category DiskWriteCategory) (File, error) {
	if m.reuseForWrite == nil {
		panic("unimplemented")
	}
	return m.reuseForWrite(oldname, newname, category)
}

func (m mockFS) MkdirAll(dir string, perm os.FileMode) error {
	if m.mkdirAll == nil {
		panic("unimplemented")
	}
	return m.mkdirAll(dir, perm)
}

func (m mockFS) Lock(name string) (io.Closer, error) {
	if m.lock == nil {
		panic("unimplemented")
	}
	return m.lock(name)
}

func (m mockFS) List(dir string) ([]string, error) {
	if m.list == nil {
		panic("unimplemented")
	}
	return m.list(dir)
}

func (m mockFS) Stat(name string) (FileInfo, error) {
	if m.stat == nil {
		panic("unimplemented")
	}
	return m.stat(name)
}

func (m mockFS) PathBase(path string) string {
	if m.pathBase == nil {
		panic("unimplemented")
	}
	return m.pathBase(path)
}

func (m mockFS) PathJoin(elem ...string) string {
	if m.pathJoin == nil {
		panic("unimplemented")
	}
	return m.pathJoin(elem...)
}

func (m mockFS) PathDir(path string) string {
	if m.pathDir == nil {
		panic("unimplemented")
	}
	return m.pathDir(path)
}

func (m mockFS) GetDiskUsage(path string) (DiskUsage, error) {
	if m.getDiskUsage == nil {
		panic("unimplemented")
	}
	return m.getDiskUsage(path)
}

func (m mockFS) Unwrap() FS { return nil }

func (m mockFS) IsReal() bool { return false }

func (m mockFS) ReadDir(dirname string) ([]os.DirEntry, error) {
	if m.readDir == nil {
		panic("unimplemented")
	}
	return m.ReadDir(dirname)
}

func (m mockFS) WalkDir(path string, f iofs.WalkDirFunc) error {
	if m.walkDir == nil {
		panic("unimplemented")
	}
	return m.WalkDir(path, f)
}

func (m mockFS) MountReadOnlyRealDir(fromRealDir string, mountPointInsideDir string) error {
	return fmt.Errorf("mockFS: MountReadOnlyRealDir not supported")
}

var _ FS = &mockFS{}

func TestDiskHealthChecking_WriteStatsCollector(t *testing.T) {
	writeBuffer := []byte("test")
	writeSizeInBytes := uint64(len(writeBuffer))

	testCases := []struct {
		desc            string
		writeCategories []DiskWriteCategory
		numWrites       int
		wantStats       []DiskWriteStatsAggregate
	}{
		{
			desc:            "no write registered",
			writeCategories: []DiskWriteCategory{},
			numWrites:       0,
			wantStats:       []DiskWriteStatsAggregate(nil),
		},
		{
			desc:            "multiple writes to a single category",
			writeCategories: []DiskWriteCategory{"test1"},
			numWrites:       2,
			wantStats: []DiskWriteStatsAggregate{
				{Category: "test1", BytesWritten: 2 * writeSizeInBytes},
			},
		},
		{
			desc:            "writes to multiple categories",
			writeCategories: []DiskWriteCategory{"test1", "test2"},
			numWrites:       2,
			wantStats: []DiskWriteStatsAggregate{
				{Category: "test1", BytesWritten: 2 * writeSizeInBytes},
				{Category: "test2", BytesWritten: 2 * writeSizeInBytes},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			statsCollector := NewDiskWriteStatsCollector()
			mockFS := &mockFS{create: func(name string, category DiskWriteCategory) (File, error) {
				return mockFile{syncAndWriteDuration: 0}, nil
			}}
			fs, closer := WithDiskHealthChecks(mockFS, time.Millisecond, statsCollector, func(info DiskSlowInfo) {})
			defer closer.Close()

			for _, category := range tc.writeCategories {
				f, err := fs.Create("test", category)
				if err != nil {
					t.Fatal(err)
				}
				for i := 0; i < tc.numWrites; i++ {
					n, err := f.Write(writeBuffer)
					if err != nil {
						t.Fatal(err)
					}
					if got := uint64(n); got != writeSizeInBytes {
						t.Fatalf("write size = %d, want %d", got, writeSizeInBytes)
					}
				}
				err = f.Close()
				if err != nil {
					t.Fatal(err)
				}
			}
			expectedStats := statsCollector.GetStats()
			if !slices.Equal(expectedStats, tc.wantStats) {
				t.Fatalf("stats = %v, want %v", expectedStats, tc.wantStats)
			}
		})
	}
}

func TestDiskHealthChecking_File(t *testing.T) {
	oldTickInterval := defaultTickInterval
	defaultTickInterval = time.Millisecond
	if runtime.GOOS == "windows" {
		t.Skipf("skipped on windows due to unreliable runtimes")
	}

	defer func() { defaultTickInterval = oldTickInterval }()

	const (
		slowThreshold = 50 * time.Millisecond
	)

	fiveKB := make([]byte, 5*writeSizePrecision)
	testCases := []struct {
		op               OpType
		writeSize        int
		writeDuration    time.Duration
		fn               func(f File)
		createWriteDelta time.Duration
	}{
		{
			op:            OpTypeWrite,
			writeSize:     5 * writeSizePrecision, // five KB
			writeDuration: 100 * time.Millisecond,
			fn:            func(f File) { f.Write(fiveKB) },
		},
		{
			op:            OpTypeSync,
			writeSize:     0,
			writeDuration: 100 * time.Millisecond,
			fn:            func(f File) { f.Sync() },
		},
	}
	for _, tc := range testCases {
		t.Run(tc.op.String(), func(t *testing.T) {
			diskSlow := make(chan DiskSlowInfo, 3)
			mockFS := &mockFS{create: func(name string, category DiskWriteCategory) (File, error) {
				return mockFile{syncAndWriteDuration: tc.writeDuration}, nil
			}}
			fs, closer := WithDiskHealthChecks(mockFS, slowThreshold, nil,
				func(info DiskSlowInfo) {
					diskSlow <- info
				})
			defer closer.Close()
			dhFile, _ := fs.Create("test", WriteCategoryUnspecified)
			defer dhFile.Close()

			// Writing after file creation tests computation of delta between file
			// creation time & write time.
			time.Sleep(tc.createWriteDelta)

			tc.fn(dhFile)

			select {
			case i := <-diskSlow:
				d := i.Duration
				if d.Seconds() < slowThreshold.Seconds() {
					t.Fatalf("expected %0.1f to be greater than threshold %0.1f", d.Seconds(), slowThreshold.Seconds())
				}
				if i.WriteSize != tc.writeSize {
					t.Fatalf("write size = %d, want %d", i.WriteSize, tc.writeSize)
				}
				if i.OpType != tc.op {
					t.Fatalf("operation = %v, want %v", i.OpType, tc.op)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("disk stall detector did not detect slow disk operation")
			}
		})
	}
}

func TestDiskHealthChecking_NotTooManyOps(t *testing.T) {
	numBitsForOpType := 64 - deltaBits - writeSizeBits
	numOpTypesAllowed := int(math.Pow(2, float64(numBitsForOpType)))
	numOpTypes := int(opTypeMax)
	if numOpTypes > numOpTypesAllowed {
		t.Fatalf("%d operation types exceed limit %d", numOpTypes, numOpTypesAllowed)
	}
}

func TestDiskHealthChecking_File_PackingAndUnpacking(t *testing.T) {
	testCases := []struct {
		desc          string
		delta         time.Duration
		writeSize     int64
		opType        OpType
		wantDelta     time.Duration
		wantWriteSize int
	}{
		// Write op with write size in bytes.
		{
			desc:          "write, sized op",
			delta:         3000 * time.Millisecond,
			writeSize:     1024, // 1 KB.
			opType:        OpTypeWrite,
			wantDelta:     3000 * time.Millisecond,
			wantWriteSize: 1024,
		},
		// Sync op. No write size. Max-ish delta that packing scheme can handle.
		{
			desc:          "sync, no write size",
			delta:         34 * time.Hour * 24 * 365,
			writeSize:     0,
			opType:        OpTypeSync,
			wantDelta:     34 * time.Hour * 24 * 365,
			wantWriteSize: 0,
		},
		// Delta is negative (e.g. due to clock sync). Set to
		// zero.
		{
			desc:          "delta negative",
			delta:         -5,
			writeSize:     5120, // 5 KB
			opType:        OpTypeWrite,
			wantDelta:     0,
			wantWriteSize: 5120,
		},
		// Write size in bytes is larger than can fit in 20 bits.
		// Round down to max that can fit in 20 bits.
		{
			desc:          "write size truncated",
			delta:         231 * time.Millisecond,
			writeSize:     2097152000, // too big!
			opType:        OpTypeWrite,
			wantDelta:     231 * time.Millisecond,
			wantWriteSize: 1073740800, // (2^20-1) * writeSizePrecision ~= a bit less than one GB
		},
		// Write size in bytes is max representable less than the ceiling.
		{
			desc:          "write size barely not truncated",
			delta:         231 * time.Millisecond,
			writeSize:     1073739776, // max representable less than the ceiling
			opType:        OpTypeWrite,
			wantDelta:     231 * time.Millisecond,
			wantWriteSize: 1073739776, // since can fit, unchanged
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			packed := pack(tc.delta, tc.writeSize, tc.opType)
			gotDelta, gotWriteSize, gotOpType := unpack(packed)

			if gotDelta != tc.wantDelta {
				t.Fatalf("delta = %s, want %s", gotDelta, tc.wantDelta)
			}
			if gotWriteSize != tc.wantWriteSize {
				t.Fatalf("write size = %d, want %d", gotWriteSize, tc.wantWriteSize)
			}
			if gotOpType != tc.opType {
				t.Fatalf("operation = %v, want %v", gotOpType, tc.opType)
			}
		})
	}
}

func TestDiskHealthChecking_File_Underflow(t *testing.T) {
	f := &mockFile{}
	hcFile := newDiskHealthCheckingFile(f, 1*time.Second, "test-category", nil,
		func(opType OpType, writeSizeInBytes int, duration time.Duration) {
			// We expect to panic before sending the event.
			t.Fatalf("unexpected slow disk event")
		})
	defer hcFile.Close()

	t.Run("too large delta leads to panic", func(t *testing.T) {
		// Given the packing scheme, 35 years of process uptime will lead to a delta
		// that is too large to fit in the packed int64.
		tCreate := crtime.NowMono() - crtime.Mono(35*time.Hour*24*365)
		hcFile.createTime = tCreate

		// Assert that the time since tCreate (in milliseconds) is indeed greater
		// than the max delta that can fit.
		if elapsed := tCreate.Elapsed().Milliseconds(); elapsed <= 1<<deltaBits-1 {
			t.Fatalf("elapsed milliseconds = %d, expected overflow threshold > %d", elapsed, int64(1<<deltaBits-1))
		}

		// Attempting to start the clock for a new operation on the file should
		// trigger a panic, as the calculated delta from the file creation time would
		// result in integer overflow.
		func() {
			defer func() {
				if recover() == nil {
					t.Fatal("expected panic")
				}
			}()
			_, _ = hcFile.Write([]byte("uh oh"))
		}()
	})
	t.Run("pretty large delta but not too large leads to no panic", func(t *testing.T) {
		// Given the packing scheme, 34 years of process uptime will lead to a delta
		// that is just small enough to fit in the packed int64.
		tCreate := crtime.NowMono() - crtime.Mono(34*time.Hour*24*365)
		hcFile.createTime = tCreate

		if elapsed := tCreate.Elapsed().Milliseconds(); elapsed >= 1<<deltaBits-1 {
			t.Fatalf("elapsed milliseconds = %d, expected below threshold %d", elapsed, int64(1<<deltaBits-1))
		}
		_, _ = hcFile.Write([]byte("should be fine"))
	})
}

var (
	errInjected = errors.New("injected error")
)

// filesystemOpsMockFS returns a filesystem that will block until it reads from
// the provided channel on filesystem operations.
func filesystemOpsMockFS(ch chan struct{}) *mockFS {
	return &mockFS{
		create: func(name string, category DiskWriteCategory) (File, error) {
			<-ch
			return nil, errInjected
		},
		link: func(oldname, newname string) error {
			<-ch
			return errInjected
		},
		mkdirAll: func(string, os.FileMode) error {
			<-ch
			return errInjected
		},
		remove: func(name string) error {
			<-ch
			return errInjected
		},
		removeAll: func(name string) error {
			<-ch
			return errInjected
		},
		rename: func(oldname, newname string) error {
			<-ch
			return errInjected
		},
		reuseForWrite: func(oldname, newname string, category DiskWriteCategory) (File, error) {
			<-ch
			return nil, errInjected
		},
	}
}

func stallFilesystemOperations(fs FS) []filesystemOperation {
	return []filesystemOperation{
		{
			"create", OpTypeCreate, func() {
				f, _ := fs.Create("foo", WriteCategoryUnspecified)
				if f != nil {
					f.Close()
				}
			},
		},
		{
			"link", OpTypeLink, func() { _ = fs.Link("foo", "bar") },
		},
		{
			"mkdirall", OpTypeMkdirAll, func() { _ = fs.MkdirAll("foo", os.ModePerm) },
		},
		{
			"remove", OpTypeRemove, func() { _ = fs.Remove("foo") },
		},
		{
			"removeall", OpTypeRemoveAll, func() { _ = fs.RemoveAll("foo") },
		},
		{
			"rename", OpTypeRename, func() { _ = fs.Rename("foo", "bar") },
		},
		{
			"reuseforwrite", OpTypeReuseForWrite, func() { _, _ = fs.ReuseForWrite("foo", "bar", WriteCategoryUnspecified) },
		},
	}
}

type filesystemOperation struct {
	name   string
	opType OpType
	f      func()
}

func TestDiskHealthChecking_Filesystem(t *testing.T) {
	const stallThreshold = 10 * time.Millisecond
	if runtime.GOOS == "windows" {
		t.Skipf("skipped on windows due to unreliable runtimes")
	}

	// Wrap with disk-health checking, counting each stall via stallCount.
	var expectedOpType OpType
	var stallCount atomic.Uint64
	unstall := make(chan struct{})
	var lastOpType OpType
	fs, closer := WithDiskHealthChecks(filesystemOpsMockFS(unstall), stallThreshold, nil,
		func(info DiskSlowInfo) {
			if info.WriteSize != 0 {
				t.Fatalf("write size = %d, want 0", info.WriteSize)
			}
			stallCount.Add(1)
			if lastOpType != info.OpType {
				if info.OpType != expectedOpType {
					t.Fatalf("operation = %v, want %v", info.OpType, expectedOpType)
				}
				lastOpType = info.OpType
				// Sending on `unstall` releases the blocked filesystem
				// operation, allowing the test to proceed.
				unstall <- struct{}{}
			}
		})

	defer closer.Close()
	fs.(*diskHealthCheckingFS).tickInterval = 5 * time.Millisecond
	ops := stallFilesystemOperations(fs)
	for _, o := range ops {
		t.Run(o.name, func(t *testing.T) {
			expectedOpType = o.opType
			before := stallCount.Load()
			// o.f() will perform the filesystem operation and block within the
			// mock filesystem until the disk stall detector notices the stall
			// and sends to the `unstall` channel.
			o.f()
			after := stallCount.Load()
			if after <= before {
				t.Fatalf("stall count did not increase: before=%d after=%d", before, after)
			}
		})
	}
}

// TestDiskHealthChecking_Filesystem_Close tests the behavior of repeatedly
// closing and reusing a filesystem wrapped by WithDiskHealthChecks. This is a
// permitted usage because it allows (*pebble.Options).EnsureDefaults to wrap
// with disk-health checking by default, and to clean up the long-running
// goroutine on (*pebble.DB).Close, while still allowing the FS to be used
// multiple times.
func TestDiskHealthChecking_Filesystem_Close(t *testing.T) {
	const stallThreshold = 10 * time.Millisecond
	stallChan := make(chan struct{}, 1)
	mockFS := &mockFS{
		create: func(name string, category DiskWriteCategory) (File, error) {
			<-stallChan
			return &mockFile{}, nil
		},
	}

	files := []string{"foo", "bar", "bax"}
	var lastPath string
	stalled := make(chan string)
	fs, closer := WithDiskHealthChecks(mockFS, stallThreshold, nil,
		func(info DiskSlowInfo) {
			if lastPath != info.Path {
				lastPath = info.Path
				stalled <- info.Path
			}
		})
	fs.(*diskHealthCheckingFS).tickInterval = 5 * time.Millisecond

	var wg sync.WaitGroup
	for _, filename := range files {
		filename := filename
		// Create will stall, and the detector should write to the stalled channel
		// with the filename.
		wg.Add(1)
		go func() {
			defer wg.Done()
			f, _ := fs.Create(filename, WriteCategoryUnspecified)
			if f != nil {
				f.Close()
			}
		}()

		select {
		case stalledPath := <-stalled:
			if stalledPath != filename {
				t.Fatalf("stalled path = %q, want %q", stalledPath, filename)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("timed out waiting for stall")
		}
		// Unblock the call to Create().
		stallChan <- struct{}{}

		// Invoke the closer. This will cause the long-running goroutine to
		// exit, but the fs should still be usable and should still detect
		// subsequent stalls on the next iteration.
		if err := closer.Close(); err != nil {
			t.Fatal(err)
		}
	}
	wg.Wait()
}

func TestDiskSlowInfo(t *testing.T) {
	info := DiskSlowInfo{
		Path:      "/some/really/deep/path/to/wal/000123.log",
		OpType:    OpTypeWrite,
		WriteSize: 123,
		Duration:  5 * time.Second,
	}

	result := info.String()

	// Essential information should be present
	for _, want := range []string{"disk slowness detected", "write", filepath.FromSlash(info.Path), "(123 bytes)", "5.0s"} {
		if !strings.Contains(result, want) {
			t.Errorf("result %q does not contain %q", result, want)
		}
	}

	// Should not contain just the filename without full path
	if strings.Contains(result, " 000123.log ") {
		t.Errorf("result unexpectedly contains bare filename: %q", result)
	}
}
