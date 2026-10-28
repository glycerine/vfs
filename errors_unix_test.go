//go:build darwin || dragonfly || freebsd || linux || openbsd || netbsd

package vfs

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestIsNoSpaceError(t *testing.T) {
	err := fmt.Errorf("wrapped: %w", unix.ENOSPC)
	require.True(t, IsNoSpaceError(err))
}
