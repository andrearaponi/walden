package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEARSGrammarDocs pins the EARS grammar and its warnings to the format
// reference and the form changes to the changelog, under [Unreleased] until
// the release that ships the change takes the entry over.
func TestEARSGrammarDocs(t *testing.T) {
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
		"counts only at the start of the criterion or after a comma",
		"anywhere before `SHALL` when written in capitals",
		"any two or more kinds of clause form a complex criterion",
		"`WHERE` → `WHILE` → `WHEN`/`IF`",
		"clauses out of the EARS order",
		"is the pronoun `it` or `they`",
		"never fail validation",
	)

	changelog := read("CHANGELOG.md")
	start := strings.Index(changelog, "## [Unreleased]")
	if start < 0 {
		t.Fatal("CHANGELOG.md has no [Unreleased] section")
	}
	end := strings.Index(changelog[start:], "\n## [0.13.1]")
	if end < 0 {
		t.Fatal("CHANGELOG.md has no [0.13.1] section below [Unreleased]")
	}
	require("CHANGELOG.md between [Unreleased] and [0.13.1]", changelog[start:start+end+1],
		"out of the EARS order",
		"the pronoun `it` or `they`",
		"counted complex or unwanted by mistake change form",
	)
}
