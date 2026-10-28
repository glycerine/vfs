package vfs

import (
	"io"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
)

var filesystemWriteOps = map[string]func(FS) error{
	"Create": func(fs FS) error {
		_, err := fs.Create("foo", WriteCategoryUnspecified)
		return err
	},
	"Lock": func(fs FS) error {
		_, err := fs.Lock("foo")
		return err
	},
	"ReuseForWrite": func(fs FS) error {
		_, err := fs.ReuseForWrite("foo", "bar", WriteCategoryUnspecified)
		return err
	},
	"Link":      func(fs FS) error { return fs.Link("foo", "bar") },
	"MkdirAll":  func(fs FS) error { return fs.MkdirAll("foo", os.ModePerm) },
	"Remove":    func(fs FS) error { return fs.Remove("foo") },
	"RemoveAll": func(fs FS) error { return fs.RemoveAll("foo") },
	"Rename":    func(fs FS) error { return fs.Rename("foo", "bar") },
}

func TestOnDiskFull_FS(t *testing.T) {
	for name, fn := range filesystemWriteOps {
		t.Run(name, func(t *testing.T) {
			innerFS := &enospcMockFS{}
			innerFS.addENOSPC(1)
			var callbackInvocations int
			fs := OnDiskFull(innerFS, func() {
				callbackInvocations++
			})

			// Call this vfs.FS method on the wrapped filesystem. The first
			// call should return ENOSPC. Our registered callback should be
			// invoked, then the method should be retried and return a nil
			// error.
			if err := fn(fs); err != nil {
				t.Fatal(err)
			}
			if callbackInvocations != 1 {
				t.Fatalf("callback invocations = %d, want 1", callbackInvocations)
			}
			// The inner filesystem should be invoked twice because of the
			// retry.
			if innerFS.mu.invocations != 2 {
				t.Fatalf("inner invocations = %d, want 2", innerFS.mu.invocations)
			}
		})
	}
}

func TestOnDiskFull_File(t *testing.T) {
	t.Run("Write", func(t *testing.T) {
		innerFS := &enospcMockFS{bytesWritten: 6}
		var callbackInvocations int
		fs := OnDiskFull(innerFS, func() {
			callbackInvocations++
		})

		f, err := fs.Create("foo", WriteCategoryUnspecified)
		if err != nil {
			t.Fatal(err)
		}

		// The next Write should ENOSPC.
		innerFS.addENOSPC(1)

		// Call the Write method on the wrapped file. The first call should return
		// ENOSPC, but also that six bytes were written. Our registered callback
		// should be invoked, then Write should be retried and return a nil error
		// and five bytes written.
		n, err := f.Write([]byte("hello world"))
		if err != nil {
			t.Fatal(err)
		}
		if n != 11 {
			t.Fatalf("bytes written = %d, want 11", n)
		}
		if callbackInvocations != 1 {
			t.Fatalf("callback invocations = %d, want 1", callbackInvocations)
		}
		// The inner filesystem should be invoked 3 times. Once during Create
		// and twice during Write.
		if innerFS.mu.invocations != 3 {
			t.Fatalf("inner invocations = %d, want 3", innerFS.mu.invocations)
		}
	})
	t.Run("Sync", func(t *testing.T) {
		innerFS := &enospcMockFS{bytesWritten: 6}
		var callbackInvocations int
		fs := OnDiskFull(innerFS, func() {
			callbackInvocations++
		})

		f, err := fs.Create("foo", WriteCategoryUnspecified)
		if err != nil {
			t.Fatal(err)
		}

		// The next Sync should ENOSPC. The callback should be invoked, but a
		// Sync cannot be retried.
		innerFS.addENOSPC(1)

		err = f.Sync()
		if err == nil {
			t.Fatal("expected an error")
		}
		if callbackInvocations != 1 {
			t.Fatalf("callback invocations = %d, want 1", callbackInvocations)
		}
		// The inner filesystem should be invoked 2 times. Once during Create
		// and once during Sync.
		if innerFS.mu.invocations != 2 {
			t.Fatalf("inner invocations = %d, want 2", innerFS.mu.invocations)
		}
	})
}

func TestOnDiskFull_Concurrent(t *testing.T) {
	const concurrentWriters = 10
	innerFS := &enospcMockFS{}
	innerFS.mu.enospcs = concurrentWriters
	var callbackInvocations atomic.Int32
	fs := OnDiskFull(innerFS, func() {
		callbackInvocations.Add(1)
	})

	var wg sync.WaitGroup
	for range concurrentWriters {
		wg.Go(func() {
			_, err := fs.Create("foo", WriteCategoryUnspecified)
			// They all should succeed on retry.
			if err != nil {
				t.Fatal(err)
			}
		})
	}
	wg.Wait()
	// Since all operations should start before the first one returns an
	// ENOSPC, the callback should only be invoked once.
	if got := callbackInvocations.Load(); got != 1 {
		t.Fatalf("callback invocations = %d, want 1", got)
	}
	if want := 2 * concurrentWriters; innerFS.mu.invocations != want {
		t.Fatalf("inner invocations = %d, want %d", innerFS.mu.invocations, want)
	}
}

type enospcMockFS struct {
	FS
	bytesWritten int

	mu struct {
		sync.Mutex
		cond        sync.Cond
		enospcs     int
		invocations int
	}
}

func (fs *enospcMockFS) addENOSPC(n int) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.mu.enospcs += n
}

func (fs *enospcMockFS) maybeENOSPC() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if fs.mu.cond.L == nil {
		fs.mu.cond.L = &fs.mu.Mutex
	}

	fs.mu.invocations++

	// If there are any ENOSPCs left, inject one and return an error.
	if fs.mu.enospcs > 0 {
		fs.mu.enospcs--
		// If there are remaining ENOSPCs, we wait for the other goroutines to
		// reach this point. We wait on the condition variable until all ENOSPCs
		// have been consumed. If we are the last goroutine to reach this point,
		// we wake up any other goroutines waiting on the condition variable.
		// This ensures that all the gorouintes part of the same 'write
		// generation' are blocked together.
		if fs.mu.enospcs == 0 {
			fs.mu.cond.Broadcast()
		}
		for fs.mu.enospcs > 0 {
			fs.mu.cond.Wait()
		}
		// Wrap the error to test error unwrapping.
		err := &os.PathError{Op: "mock", Path: "mock", Err: syscall.ENOSPC}
		return err
	}
	return nil
}

func (fs *enospcMockFS) Create(name string, category DiskWriteCategory) (File, error) {
	if err := fs.maybeENOSPC(); err != nil {
		return nil, err
	}
	return &enospcMockFile{fs: fs}, nil
}

func (fs *enospcMockFS) Link(oldname, newname string) error {
	if err := fs.maybeENOSPC(); err != nil {
		return err
	}
	return nil
}

func (fs *enospcMockFS) Remove(name string) error {
	if err := fs.maybeENOSPC(); err != nil {
		return err
	}
	return nil
}

func (fs *enospcMockFS) RemoveAll(name string) error {
	if err := fs.maybeENOSPC(); err != nil {
		return err
	}
	return nil
}

func (fs *enospcMockFS) Rename(oldname, newname string) error {
	if err := fs.maybeENOSPC(); err != nil {
		return err
	}
	return nil
}

func (fs *enospcMockFS) ReuseForWrite(
	oldname, newname string, category DiskWriteCategory,
) (File, error) {
	if err := fs.maybeENOSPC(); err != nil {
		return nil, err
	}
	return &enospcMockFile{fs: fs}, nil
}

func (fs *enospcMockFS) MkdirAll(dir string, perm os.FileMode) error {
	if err := fs.maybeENOSPC(); err != nil {
		return err
	}
	return nil
}

func (fs *enospcMockFS) Lock(name string) (io.Closer, error) {
	if err := fs.maybeENOSPC(); err != nil {
		return nil, err
	}
	return nil, nil
}

type enospcMockFile struct {
	fs *enospcMockFS
	File
}

func (f *enospcMockFile) Write(b []byte) (int, error) {

	if err := f.fs.maybeENOSPC(); err != nil {
		n := len(b)
		if f.fs.bytesWritten < n {
			n = f.fs.bytesWritten
		}
		return n, err
	}
	return len(b), nil
}

func (f *enospcMockFile) Sync() error {
	return f.fs.maybeENOSPC()
}

// BenchmarkOnDiskFull benchmarks the overhead of the OnDiskFull filesystem
// wrapper during a Write when there is no ENOSPC.
func BenchmarkOnDiskFull(b *testing.B) {
	fs := OnDiskFull(NewMem(), func() {})

	f, err := fs.Create("foo", WriteCategoryUnspecified)
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			b.Error(err)
		}
	}()

	payload := []byte("hello world")
	for i := 0; i < b.N; i++ {
		_, err := f.Write(payload)
		if err != nil {
			b.Fatal(err)
		}
	}
}
