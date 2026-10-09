package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProofParsingDocs pins the proof-block rule to the format reference and
// its consequence for existing evidence to the changelog.
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
	unreleased := changelog[start:]
	if end := strings.Index(unreleased[1:], "\n## ["); end >= 0 {
		unreleased = unreleased[:end+1]
	}
	require("CHANGELOG.md [Unreleased]", unreleased,
		"`covers: []`",
		"becomes `stale-spec` until `walden verify`",
	)
}
