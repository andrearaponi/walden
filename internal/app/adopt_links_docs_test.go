package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAdoptLegacyLinkDocumentation holds the documents to what adopt does
// with the approval time a link records. It checks that the facts are named,
// not that sentences are exact, so a reworded paragraph does not fail it and
// a dropped fact does.
func TestAdoptLegacyLinkDocumentation(t *testing.T) {
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

	t.Run("adoption guide", func(t *testing.T) {
		const name = "docs/adoption.md"
		text := read(t, name)

		// The cause of the block, among the causes of the class.
		require(t, name, text, "upstream approval time")
		// What the trust assumption of the backfill covers.
		require(t, name, text, "the body of a document, not its link")
		// The steps for a chain the plan reports.
		require(t, name, text, "the last one first", "the first one first", "`walden review open`", "`walden review approve`")
	})

	t.Run("CLI reference", func(t *testing.T) {
		const name = "docs/reference/cli.md"
		const heading = "### `walden adopt [<feature>] [--apply] [--json]`"
		text := read(t, name)
		start := strings.Index(text, heading)
		if start < 0 {
			t.Fatalf("%s has no entry %q", name, heading)
		}
		entry := text[start+len(heading):]
		if end := strings.Index(entry, "\n### "); end >= 0 {
			entry = entry[:end]
		}

		require(t, name, entry, "upstream approval time", "never sealed", "warns")
	})

	t.Run("changelog", func(t *testing.T) {
		const name = "CHANGELOG.md"
		const previousRelease = "\n## [0.15.2]"
		text := read(t, name)
		end := strings.Index(text, previousRelease)
		if end < 0 {
			t.Fatalf("%s has no heading for 0.15.2", name)
		}
		// Everything above the previous release: the Unreleased section
		// today, the section of the release that ships the change afterwards.
		above := text[:end]

		require(t, name+" above 0.15.2", above,
			"`walden adopt`",
			"approval time",
			"`backfill`",
			"`blocked`",
			"warns",
			"v0.10.0",
			"v0.15.2")
	})

	// A sealed link whose times differ is left by one of two histories, and
	// the documents name both wherever they name the report.
	t.Run("both histories", func(t *testing.T) {
		const guide = "docs/adoption.md"
		const heading = "\n## Sealed links that record another approval\n"
		text := read(t, guide)
		start := strings.Index(text, heading)
		if start < 0 {
			t.Fatalf("%s has no section %q", guide, strings.TrimSpace(heading))
		}
		section := text[start+len(heading):]
		if end := strings.Index(section, "\n## "); end >= 0 {
			section = section[:end]
		}
		// The two histories with the releases that leave them.
		require(t, guide+", section on sealed links", section,
			"approved again without a change", "v0.4.0", "v0.9.0",
			"backfill", "v0.10.0", "v0.15.2")
		// The search of the git history that tells them apart, for a design
		// and for a plan, and the commit of it to read.
		require(t, guide+", section on sealed links", section,
			"git log -p -S'source_requirements_fingerprint: sha256'",
			"git log -p -S'source_design_fingerprint: sha256'",
			"most recent",
			"`approved_at`")

		const reference = "docs/reference/cli.md"
		const entryHeading = "### `walden adopt [<feature>] [--apply] [--json]`"
		cli := read(t, reference)
		entryStart := strings.Index(cli, entryHeading)
		if entryStart < 0 {
			t.Fatalf("%s has no entry %q", reference, entryHeading)
		}
		entry := cli[entryStart+len(entryHeading):]
		if end := strings.Index(entry, "\n### "); end >= 0 {
			entry = entry[:end]
		}
		require(t, reference+", entry of adopt", entry,
			"approved again without a change", "a backfill by v0.10.0 to v0.15.2")

		const changelog = "CHANGELOG.md"
		log := read(t, changelog)
		end := strings.Index(log, "\n## [0.15.2]")
		if end < 0 {
			t.Fatalf("%s has no heading for 0.15.2", changelog)
		}
		require(t, changelog+" above 0.15.2", log[:end],
			"v0.10.0", "v0.15.2", "v0.4.0", "v0.9.0", "approved again without a change")
	})
}
