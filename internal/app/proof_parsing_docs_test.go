package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProofParsingDocs pins the proof-block rule to the format reference and
// its consequence for existing evidence to the changelog, under [Unreleased]
// until the release that ships the change takes the entry over.
func TestProofParsingDocs(t *testing.T) {
	root := authoringSourceRoot(t)
	read := func(relative string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(root, relative))
		if err != nil {
			t.Fatalf("read %s: %v", relative, err)
		}
		return string(data)
	}
	require := func(name, text string, fragments ...string) {
		t.Helper()
		for _, fragment := range fragments {
			if !strings.Contains(text, fragment) {
				t.Errorf("%s does not mention %q", name, fragment)
			}
		}
	}

	format := read("docs/reference/spec-format.md")
	require("docs/reference/spec-format.md", format,
		"blank lines and single-line HTML comments",
		"Any other line indented under `Verification:` is an error",
		"outside a block",
		"`covers: []` is equivalent to omitting `covers`",
	)

	changelog := read("CHANGELOG.md")
	start := strings.Index(changelog, "## [Unreleased]")
	if start < 0 {
		t.Fatal("CHANGELOG.md has no [Unreleased] section")
	}
	end := strings.Index(changelog[start:], "\n## [0.13.0]")
	if end < 0 {
		t.Fatal("CHANGELOG.md has no [0.13.0] section below [Unreleased]")
	}
	require("CHANGELOG.md between [Unreleased] and [0.13.0]", changelog[start:start+end+1],
		"`covers: []`",
		"becomes `stale-spec` until `walden verify`",
	)
}
