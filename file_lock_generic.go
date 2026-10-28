//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package vfs

import (
	"fmt"
	"io"
	"runtime"
)

func (defFS) Lock(name string) (io.Closer, error) {
	return nil, fmt.Errorf("pebble: file locking is not implemented on %s/%s",
		runtime.GOOS, runtime.GOARCH)
}
