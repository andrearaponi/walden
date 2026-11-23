package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRepoInitDocumentation holds the documents to what a run of repo init
// does to files that exist. It checks that the facts are named, not that
// sentences are exact, so a reworded paragraph does not fail it and a
// dropped fact does.
func TestRepoInitDocumentation(t *testing.T) {
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
		const heading = "### `walden repo init [--json]`"
		text := read(t, name)
		start := strings.Index(text, heading)
		if start < 0 {
			t.Fatalf("%s has no entry %q", name, heading)
		}
		entry := text[start+len(heading):]
		if end := strings.Index(entry, "\n### "); end >= 0 {
			entry = entry[:end]
		}

		// The three behaviors, and the way back to a template.
		require(t, name, entry,
			"creates the files that are missing",
			"keeps the ones that exist",
			"The only change it makes to an existing file is the version",
			"delete it and run the command again")
		if strings.Contains(strings.ToLower(entry), "idempotent") {
			t.Errorf("%s still sums repo init up as idempotent", name)
		}
	})

	t.Run("CI guide", func(t *testing.T) {
		const name = "docs/ci.md"
		text := read(t, name)

		// A run after an upgrade, and the regeneration.
		require(t, name, text,
			"refreshes the pin",
			"keeps your adjustments",
			"delete it and run `walden repo init` again")
		if strings.Contains(text, "Regenerating after a CLI upgrade re-pins it") {
			t.Errorf("%s still says that regenerating re-pins the workflow", name)
		}
	})

	t.Run("changelog", func(t *testing.T) {
		const name = "CHANGELOG.md"
		const previousRelease = "\n## [0.15.0]"
		text := read(t, name)
		end := strings.Index(text, previousRelease)
		if end < 0 {
			t.Fatalf("%s has no heading for 0.15.0", name)
		}
		// Everything above the previous release: the Unreleased section
		// today, the section of the release that ships the fix afterwards.
		above := text[:end]

		require(t, name+" above 0.15.0", above,
			"`walden repo init` no longer overwrites",
			"`.walden/constitution.md`",
			"`.walden/lessons.md`",
			"`.walden/.gitignore`",
			"pull request template",
			"install step")
	})
}
