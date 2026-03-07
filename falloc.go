package vfs

import (
	"fmt"
	//"golang.org/x/sys/unix"
)

// fortunately, this is the same number on linux and darwin.
// const FALLOC_FL_INSERT_RANGE = unix.FALLOC_FL_INSERT_RANGE
const FALLOC_FL_INSERT_RANGE = 32

const linux_FALLOC_FL_COLLAPSE_RANGE = 8
const linux_FALLOC_FL_PUNCH_HOLE = 2 // linux
const darwin_F_PUNCHHOLE = 99        // from sys/fcntl.h:319

// PunchBelowBytes gives the threshold for
// hole punching to preserve sparsity of small files.
// This is set to 64MB to reflect that Apple's Filesystem (APFS)
// will not reliably make a sparse file that is smaller
// than 32MB or so (depending on configuration) and so
// for files < PunchBelowBytes, we will come back after
// copying the file and punch out holes that should be there.
const PunchBelowBytes = 64 << 20

var ErrShortAlloc = fmt.Errorf("smaller extent than requested was allocated.")

// allocated probably zero in this case, especially since
// we asked for "all-or-nothing"
var ErrFileTooLarge = fmt.Errorf("extent requested was too large.")

func panicOn(err error) {
	if err != nil {
		panic(err)
	}
}
