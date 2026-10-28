//go:build !cgo

package buildtags

// Cgo is true if we were built with the "cgo" build tag.
const Cgo = false
