//go:build darwin || dragonfly || freebsd || linux || openbsd || netbsd

package vfs

import (
	"errors"
	"golang.org/x/sys/unix"
)

var errNotEmpty = unix.ENOTEMPTY

// IsNoSpaceError returns true if the given error indicates that the disk is
// out of space.
func IsNoSpaceError(err error) bool {
	return errors.Is(err, unix.ENOSPC)
}
