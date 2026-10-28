package vfs

import (
	"os"
	"testing"
	"time"
)

func TestFileWrappersHaveFd(t *testing.T) {
	// Use the real filesystem so that we can test vfs.Default, which returns
	// files with Fd().
	tmpf, err := os.CreateTemp("", "pebble-db-fd-file")
	if err != nil {
		t.Fatal(err)
	}
	filename := tmpf.Name()
	defer os.Remove(filename)

	// File wrapper case 1: Check if diskHealthCheckingFile has Fd().
	fs2, closer := WithDiskHealthChecks(Default, 10*time.Second, nil, func(info DiskSlowInfo) {})

	defer closer.Close()
	f2, err := fs2.Open(filename)
	if err != nil {
		t.Fatal(err)
	}
	if got := f2.Fd(); got == 0 {
		t.Fatal("unexpected zero")
	}
	if got := f2.Fd(); got == InvalidFd {
		t.Fatal("disk-health file returned InvalidFd")
	}
	// File wrapper case 2: Check if syncingFile has Fd().
	f3 := NewSyncingFile(f2, SyncingFileOptions{BytesPerSync: 8 << 10 /* 8 KB */})
	if got := f3.Fd(); got == 0 {
		t.Fatal("unexpected zero")
	}
	if got := f3.Fd(); got == InvalidFd {
		t.Fatal("syncing file returned InvalidFd")
	}
	if err := f2.Close(); err != nil {
		t.Fatal(err)
	}
}
