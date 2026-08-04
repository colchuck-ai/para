// Package version reports para's own version: the ldflags override when
// present, otherwise the module version go install stamped into the binary.
package version

import "runtime/debug"

// ldflagsVersion is set via:
//
//	go build -ldflags "-X github.com/colchuck-ai/para/internal/version.ldflagsVersion=1.2.3"
//
// Left empty, String falls back to the build's module version.
var ldflagsVersion string

// String returns para's version: the ldflags override if set, else the
// version go install stamped via runtime/debug.ReadBuildInfo (so
// `go install .../cmd/para@v1.2.3` reports correctly with no ldflags at all),
// else "dev" for a local, unversioned build.
func String() string {
	if ldflagsVersion != "" {
		return ldflagsVersion
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return "dev"
}
