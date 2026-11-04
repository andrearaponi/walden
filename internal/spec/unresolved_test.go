package spec

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestScanUnresolved pins the in-order scan the review and release gates
// share: one subtest per criterion of what it reports and ignores, plus its
// purity. Body lines count from 1, as proof errors count them.
func TestScanUnresolved(t *testing.T) {
	t.Run("marker", func(t *testing.T) {
		requireScan(t, "design.md", lines(
			"# Feature Design",
			"",
			"Prose with [decision: which store backs this?] in it.",
			"- A list item holding [decision: which queue?]",
			"| Option | [decision: which cache?] |",
		), marker(3), marker(4), marker(5))
		requireScan(t, "requirements.md", "[decision: one] and [decision: two]", marker(1), marker(1))
	})

	t.Run("unterminated_comment", func(t *testing.T) {
		requireScan(t, "design.md", lines(
			"Intro.",
			"<!-- assumed: still drafting",
			"more text",
		), comment(2))
		requireScan(t, "design.md", lines(
			"Intro.",
			"<!-- assumed: still drafting",
			"more text -->",
		))
		requireScan(t, "design.md", "Text before <!-- an opener that never closes", comment(1))
	})

	t.Run("code_span", func(t *testing.T) {
		requireScan(t, "design.md", lines(
			"Quote `[decision: x]` and ``[decision: y]`` freely.",
			"Quote `<!--` and ``a `<!--` b`` as well.",
		))
		// A quoted opener starts no comment, so a later terminator hides
		// nothing before it.
		requireScan(t, "design.md", lines(
			"Quoted `<!--` then [decision: which one?]",
			"later -->",
		), marker(1))
	})

	t.Run("fence", func(t *testing.T) {
		requireScan(t, "design.md", lines(
			"```markdown",
			"[decision: inside a backtick fence]",
			"<!-- inside a backtick fence",
			"```",
			"~~~",
			"[decision: inside a tilde fence]",
			"~~~",
			"- A list item:",
			"  ```text",
			"  [decision: inside an indented fence]",
			"  ```",
			"~~~~",
			"~~~",
			"[decision: inside a longer fence]",
			"~~~~",
			"```",
			"[decision: closed by a longer closer]",
			"`````",
			"After the fences [decision: still scanned]",
		), marker(19))
	})

	t.Run("proof_line", func(t *testing.T) {
		proofs := lines(
			"- [ ] 1. Task",
			"  - Verification:",
			`    - command: ["grep", "-c", "[decision:", "notes.md"]`,
			`      expect_output: "<!-- generated"`,
		)
		requireScan(t, "tasks.md", proofs)
		requireScan(t, filepath.Join(".walden", "specs", "demo", "tasks.md"), proofs)
		requireScan(t, "design.md", proofs, marker(3), comment(4))
		// Only proof lines are exempt: the rest of tasks.md is scanned.
		requireScan(t, "tasks.md", lines(proofs, "- [ ] 2. A title holding [decision: still open]"), marker(5))
	})

	t.Run("terminated_comment", func(t *testing.T) {
		requireScan(t, "design.md", "<!-- assumed: no [decision: checkpoints remain -->")
		requireScan(t, "design.md", lines(
			"<!-- assumed: the store is local,",
			"so no [decision: stays open",
			"-->",
		))
		// The terminator is searched from the opener itself, so an opener
		// whose own dashes close it hides nothing after it.
		requireScan(t, "design.md", "<!--> then [decision: after an empty comment]", marker(1))
	})

	t.Run("order", func(t *testing.T) {
		requireScan(t, "design.md", lines(
			"<!--",
			"```",
			"-->",
			"After the comment [decision: one]",
		), marker(4))
		requireScan(t, "design.md", lines(
			"<!-- a ` b -->",
			"After the comment [decision: two]",
		), marker(2))
		requireScan(t, "design.md", "<!-- a --> tail <!-- b --> [decision: three]", marker(1))
		requireScan(t, "design.md", "<!-- a --> tail [decision: four] <!-- b -->", marker(1))
		requireScan(t, "design.md", "<!-- `x --> [decision: five] `", marker(1))
	})

	t.Run("unclosed", func(t *testing.T) {
		requireScan(t, "design.md", lines(
			"<!-- never closed",
			"[decision: after the comment]",
		), comment(1), marker(2))
		requireScan(t, "design.md", lines(
			"```",
			"[decision: after a backtick fence]",
		), marker(2))
		requireScan(t, "design.md", lines(
			"~~~",
			"[decision: after a tilde fence]",
		), marker(2))
		requireScan(t, "design.md", "a ` b [decision: after a lone backtick]", marker(1))
		// A code span never crosses a line.
		requireScan(t, "design.md", lines(
			"``",
			"[decision: on the next line]",
			"``",
		), marker(2))
	})

	t.Run("pure", func(t *testing.T) {
		body := lines(
			"Intro.",
			"[decision: which store?]",
			"<!-- open",
		)
		want := []Unresolved{marker(2), comment(3)}
		dir := t.TempDir()
		for _, path := range []string{"design.md", filepath.Join(dir, "missing", "design.md"), "/nonexistent/requirements.md"} {
			requireScan(t, path, body, want...)
		}
		requireScan(t, "design.md", strings.ReplaceAll(body, "\n", "\r\n"), want...)

		// The document on disk is never read: only the body counts.
		onDisk := filepath.Join(dir, "design.md")
		if err := os.WriteFile(onDisk, []byte("[decision: on disk only]\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", onDisk, err)
		}
		requireScan(t, onDisk, "A clean body.")

		// Only the base name tasks.md exempts proof lines.
		proof := `    - command: ["echo", "[decision:"]`
		requireScan(t, filepath.Join(dir, "tasks.md"), proof)
		requireScan(t, filepath.Join(dir, "tasks.md.bak"), proof, marker(1))
		requireScan(t, filepath.Join(dir, "tasks.md", "design.md"), proof, marker(1))

		if first, second := ScanUnresolved("design.md", body), ScanUnresolved("design.md", body); !slices.Equal(first, second) {
			t.Fatalf("repeated scans disagree: %v then %v", first, second)
		}
	})
}

func requireScan(t *testing.T, documentPath, body string, want ...Unresolved) {
	t.Helper()
	if got := ScanUnresolved(documentPath, body); !slices.Equal(got, want) {
		t.Fatalf("ScanUnresolved(%q) = %v, want %v\nbody:\n%s", documentPath, got, want, body)
	}
}

func lines(parts ...string) string {
	return strings.Join(parts, "\n")
}

func marker(line int) Unresolved {
	return Unresolved{Kind: OpenDecisionMarker, Line: line}
}

func comment(line int) Unresolved {
	return Unresolved{Kind: UnterminatedComment, Line: line}
}
