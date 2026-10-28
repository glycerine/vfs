//go:build !tracing

package buildtags

// Tracing indicates if the tracing tag is used.
//
// This tag enables low-level tracing code in the block cache.
const Tracing = false
