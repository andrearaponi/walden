package app

import (
	"runtime/debug"
	"strconv"
	"strings"
)

// buildInfoVersion reads the main module version from the Go build info. It
// is a seam so tests can simulate go-install and source builds.
var buildInfoVersion = func() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return info.Main.Version
}

// effectiveVersion resolves the version this binary runs as. Release builds
// carry it via -ldflags; go-install builds carry it as the module version in
// the build info; anything else is a source build and reports "dev".
func effectiveVersion() string {
	if Version != "dev" {
		return Version
	}
	if v := buildInfoVersion(); isReleaseVersion(v) {
		return v
	}
	return "dev"
}

// isReleaseVersion reports whether v is an exact release tag of the form
// vMAJOR.MINOR.PATCH. Pseudo-versions and "(devel)" do not qualify.
func isReleaseVersion(v string) bool {
	if !strings.HasPrefix(v, "v") {
		return false
	}
	parts := strings.Split(v[1:], ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if _, err := strconv.Atoi(part); err != nil || part == "" {
			return false
		}
	}
	return true
}
