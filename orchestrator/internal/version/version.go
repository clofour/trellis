// Package version reports the version embedded by the Go toolchain.
package version

import "runtime/debug"

// version is overridden at build time via -ldflags for release builds.
var version string

// Current returns the module version, or "dev" for an unversioned local build.
func Current() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
