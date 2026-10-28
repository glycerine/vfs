//go:build !race

package buildtags

// Race is true if we were built with the "race" build tag.
const Race = false
