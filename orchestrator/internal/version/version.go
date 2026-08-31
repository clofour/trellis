// Package version reports the version embedded by the Go toolchain.
package version

import "runtime/debug"

// Current returns the module version, or "dev" for an unversioned local build.
func Current() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
