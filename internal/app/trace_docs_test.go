package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTraceDocumentation holds the documents to what `walden trace` does. It
// checks that the facts are named, not that sentences are exact, so a
// reworded paragraph does not fail it and a dropped fact does.
func TestTraceDocumentation(t *testing.T) {
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
	// section returns what follows a heading up to the next heading of the
	// same or a higher level.
	section := func(t *testing.T, name, text, heading string, ends ...string) string {
		t.Helper()
		start := strings.Index(text, heading+"\n")
		if start < 0 {
			t.Fatalf("%s has no heading %q", name, heading)
		}
		body := text[start+len(heading):]
		cut := len(body)
		for _, end := range ends {
			if index := strings.Index(body, end); index >= 0 && index < cut {
				cut = index
			}
		}
		return body[:cut]
	}

	t.Run("CLI reference", func(t *testing.T) {
		const name = "docs/reference/cli.md"
		const heading = "### `walden trace <feature> [--markdown] [--json]`"
		text := read(t, name)
		entry := section(t, name, text, heading, "\n### ", "\n## ")

		// The entry sits with the evidence commands, after evidence status.
		evidence := strings.Index(text, "\n## Evidence\n")
		status := strings.Index(text, "### `walden evidence status <feature> [--json]`")
		verify := strings.Index(text, "### `walden verify ")
		at := strings.Index(text, heading)
		if evidence < 0 || !(evidence < status && status < at && at < verify) {
			t.Errorf("%s: the entry of trace is not under Evidence, after evidence status and before verify", name)
		}

		// What it joins.
		require(t, name, entry, "acceptance criterion", "`Requirements`", "`covers`", "evidence state")
		// The three kinds of hole.
		require(t, name, entry, "`unnamed`", "`uncovered`", "`not-standing`")
		// The three formats, and the limit of a shown command.
		require(t, name, entry, "Text by default", "`--markdown`", "`--json`", "120 characters")
		// That it judges nothing.
		require(t, name, entry, "not a gate", "exits `0`", "`walden release check`", "does not judge whether a proof proves its statement")
	})

	t.Run("JSON reference", func(t *testing.T) {
		const name = "docs/reference/json.md"
		text := read(t, name)
		entry := section(t, name, text, "## Trace (`trace --json`)", "\n## ")
		if at, last := strings.Index(text, "## Trace (`trace --json`)"), strings.Index(text, "## Error paths and stability"); last < 0 || at > last {
			t.Errorf("%s: the section of the trace is not before the one on error paths", name)
		}

		// The block, and the fields of a statement, a proof step, a leaf and
		// a hole.
		require(t, name, entry, "`result.trace`", "`feature`", "`statements`", "`leaves`", "`holes`")
		require(t, name, entry, "`id`", "`kind`", "`named_by`", "`proofs`")
		require(t, name, entry, "`task_id`", "`step`", "`command`", "`delivery`", "`state`")
		require(t, name, entry, "`design_refs`", "`statement`")
		// The values of the two kinds.
		require(t, name, entry, "`criterion`", "`nfr`", "`constraint`", "`unnamed`", "`uncovered`", "`not-standing`")
	})

	t.Run("changelog", func(t *testing.T) {
		const name = "CHANGELOG.md"
		const previousRelease = "\n## [0.15.3]"
		text := read(t, name)
		end := strings.Index(text, previousRelease)
		if end < 0 {
			t.Fatalf("%s has no heading for 0.15.3", name)
		}
		// Everything above the previous release: the Unreleased section
		// today, the section of the release that ships the command afterwards.
		above := text[:end]
		require(t, name+" above 0.15.3", above, "`walden trace", "`--markdown`", "`--json`", "read-only")
	})
}
