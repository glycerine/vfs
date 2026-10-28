//go:build darwin || dragonfly || freebsd || linux || openbsd || netbsd

package vfs

import (
	"fmt"
	"testing"

	"golang.org/x/sys/unix"
)

func TestIsNoSpaceError(t *testing.T) {
	err := fmt.Errorf("wrapped: %w", unix.ENOSPC)
	if !IsNoSpaceError(err) {
		t.Fatalf("IsNoSpaceError(%v) = false, want true", err)
	}
}
