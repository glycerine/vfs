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
	AvailBytes uint64 `zid:"0"`
	// Total disk space in bytes.
	TotalBytes uint64 `zid:"1"`
	// Used disk space in bytes.
	UsedBytes uint64 `zid:"2"`
}

// SerzMemFS is the serialized to disk version of a MemFS.
type SerzMemFS struct {
	Root *SerzMemNode `zid:"0"`

	LockedFiles map[string]struct{} `zid:"1"`

	Crashable bool `zid:"2"`

	WindowsSemantics bool      `zid:"3"`
	Usage            DiskUsage `zid:"4"`

	Mounts map[string]string `zid:"5"`
}

// SerzMemNode is the serialized version of memNode.
type SerzMemNode struct {
	IsDir bool `zid:"0"`

	Data       []byte    `zid:"1"`
	SyncedData []byte    `zid:"2"`
	ModTime    time.Time `zid:"3"`

	Children       map[string]*SerzMemNode `zid:"4"`
	SyncedChildren map[string]*SerzMemNode `zid:"5"`
}
