package app

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestStalenessDiagnosticsDocumentation holds the documents to what the
// explanations mean. It checks that the facts are named, not that sentences
// are exact, so a reworded paragraph does not fail it and a dropped fact
// does.
func TestStalenessDiagnosticsDocumentation(t *testing.T) {
	root := authoringSourceRoot(t)
	read := func(relative string) string {
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
	// section returns the text from a heading to the next heading of the
	// same or a higher level.
	section := func(t *testing.T, name, text, heading string) string {
		t.Helper()
		start := strings.Index(text, heading)
		if start < 0 {
			t.Fatalf("%s has no section %q", name, heading)
		}
		rest := text[start+len(heading):]
		if end := strings.Index(rest, "\n## "); end >= 0 {
			rest = rest[:end]
		}
		return rest
	}

	t.Run("reference", func(t *testing.T) {
		const name = "docs/reference/json.md"
		evidence := section(t, name, read(name), "## Evidence: one shape, three surfaces")

		// The recorded facts.
		require(t, name, evidence, "`tree`", "`commit`", "`paths`", "`paths_omitted`",
			"`statements`", "`walden/statement-block/v1`", "`duration_ms`")
		// The explanations.
		require(t, name, evidence, "`explanation`", "`explanation.code`", "`explanation.spec`",
			"`recorded_commit`", "`approvals`", "`statements_unchanged`",
			"`current_tree`", "`untracked`", "`recorded_duration_ms`")
		// Every case in which an explanation is unavailable.
		require(t, name, evidence, "`no-commit`", "`paths-not-kept`", "`commit-missing`",
			"`identity-mismatch`", "`no-statement-fingerprints`")
		// Both limits, each with its number.
		for limit, pattern := range map[string]string{
			"recording limit": `recording limit[^.\n]*\b100\b`,
			"display limit":   `display limit[^.\n]*\b10\b`,
		} {
			if !regexp.MustCompile(pattern).MatchString(evidence) {
				t.Errorf("%s does not state the %s with its number", name, limit)
			}
		}
	})

	t.Run("code-identity", func(t *testing.T) {
		const name = "docs/lifecycle.md"
		lifecycle := read(name)
		start := strings.Index(lifecycle, "the **evidence ledger**")
		end := strings.Index(lifecycle, "What the ledger deliberately")
		if start < 0 || end < start {
			t.Fatalf("%s lost the description of the ledger's facts", name)
		}
		facts := lifecycle[start:end]

		require(t, name, facts, "code identity", "`.walden/`", "untracked", "ignored",
			"committing unchanged content")
		// The earlier sentence left out untracked files.
		if strings.Contains(facts, "every tracked file's content") {
			t.Errorf("%s still describes the identity as a digest over tracked files only", name)
		}
	})

	t.Run("changelog", func(t *testing.T) {
		changelog := read("CHANGELOG.md")
		// The entry sits in the section of the release that ships it, under
		// the unreleased section until then: anywhere above the heading of
		// the release that preceded this change.
		end := strings.Index(changelog, "\n## [0.14.4]")
		if end < 0 {
			t.Fatal("CHANGELOG.md has no [0.14.4] section")
		}
		require(t, "CHANGELOG.md above [0.14.4]", changelog[:end],
			"`walden evidence status`", "`stale-code`", "`stale-spec`")
	})

	t.Run("adoption", func(t *testing.T) {
		const name = "docs/reference/cli.md"
		const heading = "### `walden adopt"
		reference := read(name)
		start := strings.Index(reference, heading)
		if start < 0 {
			t.Fatalf("%s has no section %q", name, heading)
		}
		// The section runs to the next heading of any level.
		adoption := reference[start+len(heading):]
		if end := strings.Index(adoption, "\n#"); end >= 0 {
			adoption = adoption[:end]
		}

		// Apply re-proves through verify, so the records it writes keep
		// what every record keeps; its own timing stays out of the ledger.
		require(t, name+", adoption section", adoption,
			"keeps the same facts as any other record",
			"`elapsed_ms` is not written to the ledger")
		// The sentence this replaces, in full: it was true of the adoption
		// diagnostics and is no longer true of the ledger.
		const obsolete = "No new fields are persisted in the ledger"
		if strings.Contains(adoption, obsolete) {
			t.Errorf("%s still says %q in its adoption section", name, obsolete)
		}
	})
}
