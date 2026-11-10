package spec

import (
	"strings"
	"testing"
)

// The expected digest was computed with shasum over the 135 bytes of the
// block, not with this package: the oracle is outside the implementation.
func TestStatementFingerprintWorkedExample(t *testing.T) {
	body := strings.Join([]string{
		"# Requirements Document",
		"",
		"#### Acceptance Criteria",
		"",
		"1. `R1.AC1` WHEN a lease expires, the system SHALL release its lock.",
		"   - Acceptance check: another holder can acquire a lock after its lease expires.",
		"",
	}, "\n")

	const wantBlock = "WHEN a lease expires, the system SHALL release its lock.\n" +
		"- Acceptance check: another holder can acquire a lock after its lease expires."
	if len(wantBlock) != 135 {
		t.Fatalf("the worked example is 135 bytes, the literal has %d", len(wantBlock))
	}

	block, defined := StatementBlocks(body)["R1.AC1"]
	if !defined || block != wantBlock {
		t.Fatalf("block = %q, want %q", block, wantBlock)
	}

	const want = "sha256:e67254d8fdb6ca639ff69fcbc659fa4d2d7d7783fa4dc0a1092e4830fa115b5b"
	if got := StatementFingerprint(block); got != want {
		t.Fatalf("fingerprint = %s, want %s", got, want)
	}
	if got := StatementFingerprints(body)["R1.AC1"]; got != want {
		t.Fatalf("StatementFingerprints gives %s, want %s", got, want)
	}
	if StatementScheme != "walden/statement-block/v1" {
		t.Fatalf("scheme = %q", StatementScheme)
	}
}

func TestStatementBlocks(t *testing.T) {
	t.Run("continuation", func(t *testing.T) {
		blocks := StatementBlocks(strings.Join([]string{
			"1. `R1.AC1` WHEN x happens, the system SHALL do y.",
			"   - Acceptance check: y is observed.",
			"   - Note: a second continuation line.",
			"2. `R1.AC2` The system SHALL do z.",
		}, "\n"))
		if want := "WHEN x happens, the system SHALL do y.\n- Acceptance check: y is observed.\n- Note: a second continuation line."; blocks["R1.AC1"] != want {
			t.Fatalf("R1.AC1 = %q, want %q", blocks["R1.AC1"], want)
		}
		if want := "The system SHALL do z."; blocks["R1.AC2"] != want {
			t.Fatalf("R1.AC2 = %q, want %q", blocks["R1.AC2"], want)
		}
	})

	t.Run("blank-line-ends", func(t *testing.T) {
		blocks := StatementBlocks(strings.Join([]string{
			"1. `R1.AC1` The system SHALL do y.",
			"   - Acceptance check: y is observed.",
			"",
			"   An indented paragraph after a blank line is prose, not the statement.",
		}, "\n"))
		if want := "The system SHALL do y.\n- Acceptance check: y is observed."; blocks["R1.AC1"] != want {
			t.Fatalf("R1.AC1 = %q, want %q", blocks["R1.AC1"], want)
		}
	})

	t.Run("shallower-line-ends", func(t *testing.T) {
		blocks := StatementBlocks(strings.Join([]string{
			"  1. `R1.AC1` The system SHALL do y.",
			"     - Acceptance check: y is observed.",
			"  A line at the definition's own indentation ends the block.",
			"Prose at the margin.",
		}, "\n"))
		if want := "The system SHALL do y.\n- Acceptance check: y is observed."; blocks["R1.AC1"] != want {
			t.Fatalf("R1.AC1 = %q, want %q", blocks["R1.AC1"], want)
		}
	})

	t.Run("fenced-ignored", func(t *testing.T) {
		blocks := StatementBlocks(strings.Join([]string{
			"```markdown",
			"1. `R9.AC9` An example inside a fence defines nothing.",
			"```",
			"1. `R1.AC1` The system SHALL do y.",
			"   ```",
			"   a fence under a definition ends its block",
			"   ```",
		}, "\n"))
		if _, defined := blocks["R9.AC9"]; defined {
			t.Fatal("a definition inside a fenced block was read")
		}
		if want := "The system SHALL do y."; blocks["R1.AC1"] != want {
			t.Fatalf("R1.AC1 = %q, want %q", blocks["R1.AC1"], want)
		}
	})

	t.Run("inline-comment-kept", func(t *testing.T) {
		blocks := StatementBlocks("1. `R1.AC1` The system SHALL do y. <!-- assumed: y is cheap -->\n")
		if want := "The system SHALL do y. <!-- assumed: y is cheap -->"; blocks["R1.AC1"] != want {
			t.Fatalf("R1.AC1 = %q, want %q", blocks["R1.AC1"], want)
		}
	})

	t.Run("crlf", func(t *testing.T) {
		unix := "1. `R1.AC1` The system SHALL do y.\n   - Acceptance check: y is observed.\n"
		windows := strings.ReplaceAll(unix, "\n", "\r\n")
		if want := "The system SHALL do y.\n- Acceptance check: y is observed."; StatementBlocks(windows)["R1.AC1"] != want {
			t.Fatalf("block = %q, want %q", StatementBlocks(windows)["R1.AC1"], want)
		}
		if StatementFingerprints(unix)["R1.AC1"] != StatementFingerprints(windows)["R1.AC1"] {
			t.Fatal("line endings changed the fingerprint")
		}
		if strings.Contains(StatementBlocks(windows)["R1.AC1"], "\r") {
			t.Fatal("a carriage return reached the block")
		}
	})

	t.Run("nfr-and-constraint", func(t *testing.T) {
		blocks := StatementBlocks(strings.Join([]string{
			"- `NFR1` Compatibility: earlier ledgers load as before (bridged by `R1.AC1`).",
			"  A continuation of the requirement.",
			"- `C1` Go standard library only.",
			"| Preserve | `other#R1.AC1` | a table row defines nothing |",
		}, "\n"))
		if want := "Compatibility: earlier ledgers load as before (bridged by `R1.AC1`).\nA continuation of the requirement."; blocks["NFR1"] != want {
			t.Fatalf("NFR1 = %q, want %q", blocks["NFR1"], want)
		}
		if want := "Go standard library only."; blocks["C1"] != want {
			t.Fatalf("C1 = %q, want %q", blocks["C1"], want)
		}
		if _, defined := blocks["R1.AC1"]; defined {
			t.Fatal("a mention inside another statement was read as a definition")
		}
		if len(blocks) != 2 {
			t.Fatalf("blocks = %v, want NFR1 and C1 only", blocks)
		}
	})

	t.Run("first-definition-wins", func(t *testing.T) {
		blocks := StatementBlocks("1. `R1.AC1` The first text.\n2. `R1.AC1` A repeated identifier.\n")
		if want := "The first text."; blocks["R1.AC1"] != want {
			t.Fatalf("R1.AC1 = %q, want %q", blocks["R1.AC1"], want)
		}
	})

	t.Run("rewording-and-reordering", func(t *testing.T) {
		first := "1. `R1.AC1` The system SHALL do y.\n   - Acceptance check: y is observed.\n"
		second := "2. `R1.AC2` The system SHALL do z.\n   - Acceptance check: z is observed.\n"
		base := StatementFingerprints(first + second)

		// Reordering the statements, or renumbering the list, changes none.
		swapped := StatementFingerprints(strings.Replace(second, "2. ", "1. ", 1) + strings.Replace(first, "1. ", "2. ", 1))
		if swapped["R1.AC1"] != base["R1.AC1"] || swapped["R1.AC2"] != base["R1.AC2"] {
			t.Fatal("reordering changed a fingerprint")
		}

		reworded := StatementFingerprints(strings.Replace(first, "do y", "do y promptly", 1) + second)
		if reworded["R1.AC1"] == base["R1.AC1"] {
			t.Fatal("rewording the statement kept its fingerprint")
		}
		if reworded["R1.AC2"] != base["R1.AC2"] {
			t.Fatal("rewording one statement changed another")
		}

		rechecked := StatementFingerprints(strings.Replace(first, "y is observed", "y is observed twice", 1) + second)
		if rechecked["R1.AC1"] == base["R1.AC1"] {
			t.Fatal("changing the acceptance check kept the fingerprint")
		}
	})
}
