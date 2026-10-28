//go:build race || slowbuild

package buildtags

// SlowBuild is true if this is an instrumented testing build that is likely
// to be significantly slower (like race or address sanitizer builds).
//
// Slow builds are either race builds or those built with a `slowbuild` tag.
const SlowBuild = true
