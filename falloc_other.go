//go:build !darwin

package vfs

import (
	"os"

	"golang.org/x/sys/unix"
)

// commannd 1 what seaweedfs does to pre-allocate space
// that is not used yet. Not sure--does this also result in a sparse
// file on linux, but maybe not on Mac??
const FALLOC_FL_KEEP_SIZE = unix.FALLOC_FL_KEEP_SIZE // = 1

func fallocate(fd *os.File, mode uint32, off int64, length int64) (allocated int64, err error) {
	return
}
