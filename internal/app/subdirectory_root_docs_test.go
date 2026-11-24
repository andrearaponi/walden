package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSubdirectoryRootDocumentation holds the lifecycle guide and the
// changelog to what a root in a subdirectory of its repository gets. It
// checks that the facts are named, not that sentences are exact, so a
// reworded paragraph does not fail it and a dropped fact does.
func TestSubdirectoryRootDocumentation(t *testing.T) {
	root := authoringSourceRoot(t)
	read := func(t *testing.T, relative string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(root, relative))
		if err != nil {
			t.Fatalf("read %s: %v", relative, err)
		}
		return string(data)
	}
	require := func(t *testing.T, name, text string, fragments ...string) {
		t.Helper()
		for _, fragment := range fragments {
			if !strings.Contains(text, fragment) {
				t.Errorf("%s does not mention %q", name, fragment)
			}
		}
	}

	t.Run("lifecycle guide", func(t *testing.T) {
		const name = "docs/lifecycle.md"
		text := read(t, name)
		// The statement sits beside the description of the code identity:
		// after it, and before the paragraph on what the ledger leaves out.
		start := strings.Index(text, "the code photograph")
		end := strings.Index(text, "What the ledger deliberately does")
		if start < 0 || end < start {
			t.Fatalf("%s lost the description of the code identity or the paragraph after it", name)
		}
		require(t, name+", beside the code identity", text[start:end],
			"subdirectory of its repository",
			"files under the root",
			"uncommitted ones included",
			"no file outside",
		)
	})

	t.Run("changelog", func(t *testing.T) {
		const name = "CHANGELOG.md"
		const label = "- **Breaking (gate criteria contract): "
		text := read(t, name)
		released := strings.Index(text, "\n## [0.16.0]")
		if released < 0 {
			t.Fatalf("%s has no heading for v0.16.0", name)
		}
		above := text[:released]

		var entry string
		for rest := above; ; {
			at := strings.Index(rest, label)
			if at < 0 {
				break
			}
			candidate := rest[at:]
			if next := strings.Index(candidate[1:], "\n"); next >= 0 {
				candidate = candidate[:next+1]
			}
			if strings.Contains(candidate, "subdirectory of its repository") {
				entry = candidate
				break
			}
			rest = rest[at+len(label):]
		}
		if entry == "" {
			t.Fatalf("above the heading of v0.16.0, %s holds no entry labelled as a breaking change of gate criteria for a root in a subdirectory of its repository", name)
		}

		// The three verdicts that change.
		require(t, name+", the entry", entry,
			"uncommitted content no longer certifies",
			"`.walden/` warns in default mode",
			"`walden release check --strict`",
			"portfolio",
		)
		// What it says of the identity and of what must not move.
		require(t, name+", the entry", entry,
			"files under it and no file outside it",
			"root at the top level",
			"no recorded evidence goes stale",
		)
		// Evidence recorded by an earlier release, and what to do about it.
		require(t, name+", the entry", entry,
			"earlier release",
			"not examined again",
			"may rest on uncommitted content",
			"`walden verify <feature> --all`",
			"delivered task stays delivered until its proof or its statements change",
		)
	})
}
