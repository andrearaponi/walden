// Package selfupdate resolves, downloads, verifies, and installs Walden
// release binaries, then re-syncs installed skills through the new binary.
// It is the only package in the module that talks to the network, and only
// on explicit invocation of the update command.
package selfupdate

import (
	"fmt"
	"net/http"
	"path"
	"strconv"
	"strings"
)

// releaseTag is a parsed vMAJOR.MINOR.PATCH release tag.
type releaseTag struct {
	major, minor, patch int
}

// parseReleaseTag parses a strict vMAJOR.MINOR.PATCH tag. Pseudo-versions,
// pre-release suffixes, and bare version numbers are rejected.
func parseReleaseTag(tag string) (releaseTag, error) {
	formatErr := fmt.Errorf("invalid release tag %q: expected vMAJOR.MINOR.PATCH (e.g. v0.7.0)", tag)

	if !strings.HasPrefix(tag, "v") {
		return releaseTag{}, formatErr
	}
	parts := strings.Split(tag[1:], ".")
	if len(parts) != 3 {
		return releaseTag{}, formatErr
	}

	numbers := make([]int, 3)
	for i, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 {
			return releaseTag{}, formatErr
		}
		numbers[i] = value
	}
	return releaseTag{numbers[0], numbers[1], numbers[2]}, nil
}

// less reports whether t precedes other in numeric release order.
func (t releaseTag) less(other releaseTag) bool {
	if t.major != other.major {
		return t.major < other.major
	}
	if t.minor != other.minor {
		return t.minor < other.minor
	}
	return t.patch < other.patch
}

// resolveTarget returns the release tag to install: the pinned tag when set
// (validated, no network), otherwise the tag the releases/latest redirect
// points at. The GitHub REST API is never consulted: following the redirect
// avoids its unauthenticated rate limit.
func resolveTarget(client *http.Client, baseURL, pinned string) (string, error) {
	if pinned != "" {
		if _, err := parseReleaseTag(pinned); err != nil {
			return "", err
		}
		return pinned, nil
	}

	// Copy the client so stopping at the first redirect does not leak into
	// the caller's client, which downloads assets through real redirects.
	probe := *client
	probe.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}

	latestURL := baseURL + "/releases/latest"
	resp, err := probe.Get(latestURL)
	if err != nil {
		return "", fmt.Errorf("resolve latest release from %s: %w (pin a release explicitly with --version <tag>)", latestURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	location := resp.Header.Get("Location")
	tag := path.Base(location)
	if location == "" || parseTagFails(tag) {
		return "", fmt.Errorf("could not resolve the latest release tag from %s (got %q); pin a release explicitly with --version <tag>", latestURL, location)
	}
	return tag, nil
}

func parseTagFails(tag string) bool {
	_, err := parseReleaseTag(tag)
	return err != nil
}
