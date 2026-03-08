// Copyright 2012 The LevelDB-Go and Pebble Authors. All rights reserved. Use
// of this source code is governed by a BSD-style license that can be found in
// the LICENSE file.

package vfs

import (
	"os"
	"time"

	"github.com/glycerine/greenpack/msgp"
	"github.com/klauspost/compress/s2"
)

var _ = s2.NewWriter

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

	LockedFiles map[string]bool `zid:"1"`

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

func (y *MemFS) Save(path string) error {
	fd, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fd.Close()
	defer fd.Sync()

	// same locking order as Create: first y.cloneMu, then y.mu during walk.
	y.cloneMu.Lock()
	defer y.cloneMu.Unlock()

	y.mu.Lock()
	defer y.mu.Unlock()

	// we should have exclusive access now.
	o := &SerzMemFS{}

	o.Root = y.root.ToSerz()
	o.LockedFiles = make(map[string]bool)
	y.lockedFiles.Range(func(key, value any) bool {
		o.LockedFiles[key.(string)] = true
		return true // true => don't stop the iteration early.
	})
	o.Crashable = y.crashable
	o.WindowsSemantics = y.windowsSemantics
	o.Usage = y.usage
	o.Mounts = make(map[string]string)
	for k, v := range y.mounts {
		o.Mounts[k] = v
	}

	w := msgp.NewWriter(fd)

	//out := bytes.NewBuffer(make([]byte, 0, 1<<20))
	//compressor := s2.NewWriter(out)

	o.EncodeMsg(w)
	w.Flush()
	//compressor.Close()

	return nil
}

func (y *memNode) ToSerz() (o *SerzMemNode) {
	o = &SerzMemNode{
		IsDir:      y.isDir,
		Data:       append([]byte{}, y.mu.data...),
		SyncedData: append([]byte{}, y.mu.syncedData...),
		ModTime:    y.mu.modTime,
	}
	if len(y.children) > 0 {
		o.Children = make(map[string]*SerzMemNode)
		for k, v := range y.children {
			o.Children[k] = v.ToSerz()
		}
	}
	if len(y.syncedChildren) > 0 {
		o.SyncedChildren = make(map[string]*SerzMemNode)
		for k, v := range y.syncedChildren {
			o.SyncedChildren[k] = v.ToSerz()
		}
	}
	return
}

func (m *MemFS) Load(path string) error {

	// clear output m
	*m = MemFS{}

	fd, err := os.Open(path)
	if err != nil {
		return err
	}
	defer fd.Close()
	r := msgp.NewReader(fd)
	s := &SerzMemFS{}

	// fill s
	err = msgp.Decode(r, s)
	if err != nil {
		return err
	}

	// transfer from s to m
	m.root = s.Root.FromSerz()
	for k := range s.LockedFiles {
		m.lockedFiles.Store(k, nil)
	}
	m.crashable = s.Crashable
	m.windowsSemantics = s.WindowsSemantics
	m.usage = s.Usage
	for k, v := range s.Mounts {
		m.mounts[k] = v
	}
	return nil
}

func (s *SerzMemNode) FromSerz() (m *memNode) {
	m = &memNode{
		isDir:          s.IsDir,
		children:       make(map[string]*memNode),
		syncedChildren: make(map[string]*memNode),
	}
	m.mu.data = s.Data
	m.mu.syncedData = s.SyncedData
	m.mu.modTime = s.ModTime

	for k, v := range s.Children {
		m.children[k] = v.FromSerz()
	}
	for k, v := range s.SyncedChildren {
		m.syncedChildren[k] = v.FromSerz()
	}
	return m
}
