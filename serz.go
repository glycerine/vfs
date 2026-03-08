// Copyright 2012 The LevelDB-Go and Pebble Authors. All rights reserved. Use
// of this source code is governed by a BSD-style license that can be found in
// the LICENSE file.

package vfs

import (
	"bytes"
	"fmt"
	"io"
	iofs "io/fs"
	"maps"
	"math/rand/v2"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/cockroachdb/errors/oserror"
	"github.com/glycerine/vfs/invariants"
)

//go:generate greenpack

// DiskUsage summarizes disk space usage on a filesystem.
type DiskUsage struct {
	// Total disk space available to the current process in bytes.
	AvailBytes uint64
	// Total disk space in bytes.
	TotalBytes uint64
	// Used disk space in bytes.
	UsedBytes uint64
}

// SerzMemFS is the serialized to disk version of a MemFS.
type SerzMemFS struct {
	Root *SerzMemNode

	LockedFiles map[string]struct{}

	Crashable bool

	WindowsSemantics bool
	Usage            DiskUsage

	Mounts map[string]string
}

// SerzMemNode is the serialized version of memNode.
type SerzMemNode struct {
	IsDir bool

	Data       []byte
	SyncedData []byte
	ModTime    time.Time

	Children       map[string]*SerzMemNode
	SyncedChildren map[string]*SerzMemNode
}
