package spec

import (
	"strconv"
	"strings"
	"testing"
)

// blockLinePlan holds one line on line 9, inside the Verification block of
// task 1.1, between two steps.
const blockLinePlan = `# Implementation Plan

- [ ] 1. Container
  - [ ] 1.1 Child
    - Requirements: ` + "`R1.AC1`" + `
    - Design: Parser
    - Verification:
      - command: ["go", "test", "./..."]
%s
      - command: ["go", "vet", "./..."]
`

const unrecognizedLineExpectation = ` — expected a "- command:" step, an expect_exit, expect_output, covers or timeout attribute, a blank line or a single-line HTML comment`

func TestParseTaskTreeRejectsUnrecognizedProofLines(t *testing.T) {
	cases := []struct {
		name string
		line string
		want string
	}{
		{
			name: "misspelled attribute",
			line: `        expect_ouput: "ok"`,
			want: `line 9: unrecognized line in the Verification block of task "1.1": "expect_ouput: \"ok\""` + unrecognizedLineExpectation,
		},
		{
			name: "unknown attribute",
			line: `        note: "not proof"`,
			want: `line 9: unrecognized line in the Verification block of task "1.1": "note: \"not proof\""` + unrecognizedLineExpectation,
		},
		{
			name: "prose at step depth",
			line: "      Prose explaining the step.",
			want: `line 9: unrecognized line in the Verification block of task "1.1": "Prose explaining the step."` + unrecognizedLineExpectation,
		},
		{
			name: "comment opener alone",
			line: "      <!--",
			want: `line 9: unrecognized line in the Verification block of task "1.1": "<!--"` + unrecognizedLineExpectation,
		},
		{
			name: "misindented metadata keeps its error",
			line: "      - Requirements: `R1.AC1`",
			want: `line 9: invalid metadata indentation for task "1.1": expected 4 spaces`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := strings.Replace(blockLinePlan, "%s", tc.line, 1)
			_, err := ParseTaskTree(Document{Exists: true, Path: "tasks.md", Body: body})
			if err == nil {
				t.Fatalf("the plan parsed, so %q ended the block and the second step was dropped", strings.TrimSpace(tc.line))
			}
			if err.Error() != tc.want {
				t.Fatalf("unexpected error:\n got: %v\nwant: %s", err, tc.want)
			}
		})
	}

	t.Run("lines count from the first body line", func(t *testing.T) {
		text := "---\nstatus: draft\nlast_modified: 2026-10-09T00:00:00Z\n---\n\n" +
			strings.Replace(blockLinePlan, "%s", `        expect_ouput: "ok"`, 1)
		document, err := ParseDocument("tasks.md", []byte(text))
		if err != nil {
			t.Fatalf("load the document: %v", err)
		}
		document.Exists = true
		bodyLine := 0
		for index, line := range strings.Split(document.Body, "\n") {
			if strings.Contains(line, "expect_ouput") {
				bodyLine = index + 1
			}
		}
		fileLine := 0
		for index, line := range strings.Split(text, "\n") {
			if strings.Contains(line, "expect_ouput") {
				fileLine = index + 1
			}
		}
		if bodyLine == 0 || bodyLine == fileLine {
			t.Fatalf("fixture does not separate body line %d from file line %d", bodyLine, fileLine)
		}
		_, err = ParseTaskTree(document)
		if err == nil || !strings.HasPrefix(err.Error(), "line "+strconv.Itoa(bodyLine)+": unrecognized line") {
			t.Fatalf("error does not count from the first body line (%d): %v", bodyLine, err)
		}
	})
}

func TestParseTaskTreeSkipsSingleLineHTMLComments(t *testing.T) {
	const plan = `# Implementation Plan

- [ ] 1. Flat task
  - Requirements: ` + "`R1.AC1`" + `
  - Design: Parser
  - Verification:
    - command: ["go", "test", "./..."]
      expect_exit: 0
      <!-- assumed: coverage follows the exit check -->
      covers: ["R1.AC1"]
    <!-- assumed: vet runs after the tests -->
    - command: ["go", "vet", "./..."]
`
	task := parsePlanTask(t, plan, "1")
	if len(task.Proof.Steps) != 2 {
		t.Fatalf("read %d steps around the comments; the plan declares 2", len(task.Proof.Steps))
	}
	if covers := task.Proof.Steps[0].Covers; len(covers) != 1 || covers[0] != "R1.AC1" {
		t.Fatalf("the attribute after a comment was not applied: covers %q", covers)
	}
}

func TestParseTaskTreeRejectsProofLinesOutsideBlock(t *testing.T) {
	const head = "# Implementation Plan\n\n- [ ] 1. Flat task\n  - Requirements: `R1.AC1`\n  - Design: Parser\n"
	const expectation = ` — expected under its task's "- Verification:" line`
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "step after a metadata-level line",
			body: head + "  - Verification:\n    - command: [\"go\", \"test\", \"./...\"]\n  Note at the metadata level.\n    - command: [\"go\", \"vet\", \"./...\"]\n",
			want: `line 9: proof line outside a Verification block, after task "1": "- command: [\"go\", \"vet\", \"./...\"]"` + expectation,
		},
		{
			name: "step after a legacy one-line proof",
			body: head + "  - Verification: go test ./...\n    - command: [\"go\", \"vet\", \"./...\"]\n",
			want: `line 7: proof line outside a Verification block, after task "1": "- command: [\"go\", \"vet\", \"./...\"]"` + expectation,
		},
		{
			name: "attribute after its block ended",
			body: head + "  - Verification:\n    - command: [\"go\", \"test\", \"./...\"]\n\nProse that ends the block.\n      expect_exit: 0\n",
			want: `line 10: proof line outside a Verification block, after task "1": "expect_exit: 0"` + expectation,
		},
		{
			name: "step at column zero",
			body: head + "  - Verification:\n    - command: [\"go\", \"test\", \"./...\"]\n- command: [\"go\", \"vet\", \"./...\"]\n",
			want: `line 8: proof line outside a Verification block, after task "1": "- command: [\"go\", \"vet\", \"./...\"]"` + expectation,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseTaskTree(Document{Exists: true, Path: "tasks.md", Body: tc.body})
			if err == nil {
				t.Fatal("the plan parsed, so the proof line outside its block was dropped")
			}
			if err.Error() != tc.want {
				t.Fatalf("unexpected error:\n got: %v\nwant: %s", err, tc.want)
			}
		})
	}
}
