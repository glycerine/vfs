package crtime

import (
	"sync/atomic"
	"time"
)

// Mono is a monotonic timestamp represented as a duration from an arbitrary
// process-local epoch. Only differences between Mono values are meaningful.
type Mono time.Duration

var epoch = time.Now()

// NowMono returns the current monotonic timestamp.
func NowMono() Mono { return Mono(time.Since(epoch)) }

// Sub returns the duration between two monotonic timestamps.
func (m Mono) Sub(other Mono) time.Duration { return time.Duration(m - other) }

// Elapsed returns the time elapsed since m.
func (m Mono) Elapsed() time.Duration { return NowMono().Sub(m) }

// AtomicMono provides atomic access to a Mono value.
type AtomicMono struct{ value atomic.Int64 }

func (a *AtomicMono) Load() Mono       { return Mono(a.value.Load()) }
func (a *AtomicMono) Store(value Mono) { a.value.Store(int64(value)) }
