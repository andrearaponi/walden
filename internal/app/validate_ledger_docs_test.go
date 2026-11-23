package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestValidateLedgerDocumentation holds the documents to what validate does
// with an evidence ledger. It checks that the facts are named, not that
// sentences are exact, so a reworded paragraph does not fail it and a
// dropped fact does.
func TestValidateLedgerDocumentation(t *testing.T) {
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

	t.Run("CLI reference", func(t *testing.T) {
		const name = "docs/reference/cli.md"
		const heading = "### `walden validate [<feature>] [--all] [--json]`"
		text := read(t, name)
		start := strings.Index(text, heading)
		if start < 0 {
			t.Fatalf("%s has no entry %q", name, heading)
		}
		entry := text[start+len(heading):]
		if end := strings.Index(entry, "\n### "); end >= 0 {
			entry = entry[:end]
		}

		// The ledger among what is checked, and the finding.
		require(t, name, entry,
			"evidence ledger",
			"`.walden/evidence/<feature>.json`",
			"cannot be read",
			"conflict markers")
	})

	t.Run("CI guide", func(t *testing.T) {
		const name = "docs/ci.md"
		text := read(t, name)

		// The workflow fails on such a ledger, and gets the check with its pin.
		require(t, name, text,
			"evidence ledger that cannot be read",
			"when the pin is refreshed")
	})

	t.Run("changelog", func(t *testing.T) {
		const name = "CHANGELOG.md"
		const previousRelease = "\n## [0.15.1]"
		text := read(t, name)
		end := strings.Index(text, previousRelease)
		if end < 0 {
			t.Fatalf("%s has no heading for 0.15.1", name)
		}
		// Everything above the previous release: the Unreleased section
		// today, the section of the release that ships the change afterwards.
		above := text[:end]

		require(t, name+" above 0.15.1", above,
			"`walden validate`",
			"evidence ledger")
	})
}
