package spec

import (
	"strings"
	"testing"
)

// proofKindPlan is a plan with a top-level leaf, a group and its child leaf.
// The placeholders take a line with its newline, or nothing.
const proofKindPlan = `# Implementation Plan

- [ ] 1. A top-level leaf
  - Requirements: ` + "`R1.AC1`" + `
  - Design: Architecture
%LEAF_BEFORE%  - Verification:
    - command: ["sh", "-c", "true"]
      covers: ["R1.AC1"]
%LEAF_AFTER%- [ ] 2. A group
%PARENT%  - [ ] 2.1 A child leaf
    - Requirements: ` + "`R1.AC2`" + `
    - Design: Architecture
%CHILD_BEFORE%    - Verification:
      - command: ["sh", "-c", "true"]
        covers: ["R1.AC2"]
%CHILD_AFTER%`

func proofKindBody(lines map[string]string) string {
	body := proofKindPlan
	for _, key := range []string{"%LEAF_BEFORE%", "%LEAF_AFTER%", "%PARENT%", "%CHILD_BEFORE%", "%CHILD_AFTER%"} {
		body = strings.Replace(body, key, lines[key], 1)
	}
	return body
}

func parseProofKindBody(t *testing.T, body string) (TaskTree, error) {
	t.Helper()
	return ParseTaskTree(Document{Exists: true, Path: "tasks.md", Body: body})
}

// deliveryLeaves returns the ids of the leaves a plan declares as delivery tasks.
func deliveryLeaves(tree TaskTree) []string {
	ids := []string{}
	for _, task := range tree.LeafTasks() {
		if task.Proof.Delivery {
			ids = append(ids, task.ID)
		}
	}
	return ids
}

func TestProofKindDeclaration(t *testing.T) {
	t.Run("accepted", func(t *testing.T) {
		cases := []struct {
			name  string
			lines map[string]string
			want  string
		}{
			{"no line", nil, ""},
			{"top-level leaf, before Verification", map[string]string{"%LEAF_BEFORE%": "  - Proof: delivery\n"}, "1"},
			{"top-level leaf, after the block", map[string]string{"%LEAF_AFTER%": "  - Proof: delivery\n"}, "1"},
			{"top-level leaf, legacy four spaces", map[string]string{"%LEAF_AFTER%": "    - Proof: delivery\n"}, "1"},
			{"child leaf, before Verification", map[string]string{"%CHILD_BEFORE%": "    - Proof: delivery\n"}, "2.1"},
			{"child leaf, after the block", map[string]string{"%CHILD_AFTER%": "    - Proof: delivery\n"}, "2.1"},
			{"both leaves", map[string]string{"%LEAF_BEFORE%": "  - Proof: delivery\n", "%CHILD_AFTER%": "    - Proof: delivery\n"}, "1,2.1"},
			{"spaces around the value", map[string]string{"%LEAF_BEFORE%": "  - Proof:   delivery  \n"}, "1"},
		}
		for _, tc := range cases {
			tree, err := parseProofKindBody(t, proofKindBody(tc.lines))
			if err != nil {
				t.Errorf("%s: %v", tc.name, err)
				continue
			}
			if got := strings.Join(deliveryLeaves(tree), ","); got != tc.want {
				t.Errorf("%s: delivery leaves = %q, want %q", tc.name, got, tc.want)
			}
			for _, task := range tree.LeafTasks() {
				if len(task.Proof.Steps) != 1 || task.Proof.Steps[0].Argv[0] != "sh" || len(task.Proof.Steps[0].Covers) != 1 {
					t.Errorf("%s: task %s lost its proof: %+v", tc.name, task.ID, task.Proof)
				}
			}
		}

		// A legacy single-line proof keeps the declaration too.
		legacy := strings.Replace(proofKindBody(map[string]string{"%LEAF_BEFORE%": "  - Proof: delivery\n"}),
			"  - Verification:\n    - command: [\"sh\", \"-c\", \"true\"]\n      covers: [\"R1.AC1\"]\n", "  - Verification: `true`\n", 1)
		tree, err := parseProofKindBody(t, legacy)
		if err != nil {
			t.Fatalf("legacy proof with a declaration: %v", err)
		}
		if got := strings.Join(deliveryLeaves(tree), ","); got != "1" {
			t.Errorf("legacy proof: delivery leaves = %q, want 1", got)
		}
		if tree.LeafTasks()[0].Proof.LegacyCommand == "" {
			t.Errorf("legacy proof: the command is gone: %+v", tree.LeafTasks()[0].Proof)
		}
	})

	t.Run("unknown-value", func(t *testing.T) {
		for _, value := range []string{"delivry", "", "regression", "Delivery", "`delivery`", "delivery, lasting"} {
			line := "  - Proof: " + value + "\n"
			if value == "" {
				line = "  - Proof:\n"
			}
			_, err := parseProofKindBody(t, proofKindBody(map[string]string{"%LEAF_BEFORE%": line}))
			if err == nil {
				t.Errorf("value %q: the plan parsed", value)
				continue
			}
			for _, want := range []string{
				"unknown proof kind " + quoted(value) + ` for task "1"`,
				`expected "delivery"`,
				"a task without a Proof line holds a regression proof",
			} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("value %q: error %q does not hold %q", value, err, want)
				}
			}
		}

		// The child is named when the child declares it.
		_, err := parseProofKindBody(t, proofKindBody(map[string]string{"%CHILD_BEFORE%": "    - Proof: delivry\n"}))
		if err == nil || !strings.Contains(err.Error(), `unknown proof kind "delivry" for task "2.1"`) {
			t.Errorf("child leaf: error %v does not name the task and the value", err)
		}
	})

	t.Run("parent", func(t *testing.T) {
		_, err := parseProofKindBody(t, proofKindBody(map[string]string{"%PARENT%": "  - Proof: delivery\n"}))
		if err == nil {
			t.Fatal("a plan whose parent declares a proof kind parsed")
		}
		if want := `non-leaf task "2" cannot declare a proof kind`; !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not hold %q", err, want)
		}

		// The same line passes on the leaf of that parent.
		tree, err := parseProofKindBody(t, proofKindBody(map[string]string{"%CHILD_BEFORE%": "    - Proof: delivery\n"}))
		if err != nil {
			t.Fatalf("the same line on the leaf: %v", err)
		}
		if got := strings.Join(deliveryLeaves(tree), ","); got != "2.1" {
			t.Errorf("delivery leaves = %q, want 2.1", got)
		}
	})

	t.Run("layout", func(t *testing.T) {
		const indentation = `invalid metadata indentation for task "1": expected 2 or 4 spaces`
		for name, line := range map[string]string{
			"six spaces before Verification": "      - Proof: delivery\n",
			"column zero":                    "- Proof: delivery\n",
		} {
			_, err := parseProofKindBody(t, proofKindBody(map[string]string{"%LEAF_BEFORE%": line}))
			if err == nil || !strings.Contains(err.Error(), indentation) {
				t.Errorf("%s: error %v, want %q", name, err, indentation)
			}
		}
		// Deeper than the block it follows: the line ends the block and gets
		// the indentation error of a metadata line, not the error of a proof
		// line.
		_, err := parseProofKindBody(t, proofKindBody(map[string]string{"%LEAF_AFTER%": "      - Proof: delivery\n"}))
		if err == nil || !strings.Contains(err.Error(), indentation) {
			t.Errorf("deeper than the block: error %v, want %q", err, indentation)
		}

		// A line that only starts like the declaration stays what it was:
		// text the parser skips. Existing plans hold "- Proof scope:" lines.
		for name, lines := range map[string]map[string]string{
			"proof scope before Verification": {"%LEAF_BEFORE%": "  - Proof scope: the test covers both cases\n"},
			"proof scope on a child":          {"%CHILD_BEFORE%": "    - Proof scope: delivery\n"},
			"proofs":                          {"%LEAF_BEFORE%": "  - Proofs: delivery\n"},
		} {
			tree, err := parseProofKindBody(t, proofKindBody(lines))
			if err != nil {
				t.Errorf("%s: %v", name, err)
				continue
			}
			if got := deliveryLeaves(tree); len(got) != 0 {
				t.Errorf("%s: read as a declaration on %v", name, got)
			}
		}
	})
}

func quoted(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}

// TestCheckTaskLayoutProofKind holds the draft check to the verdicts and the
// messages of the parser, so a draft that validates cannot fail on the
// declaration once it is in review.
func TestCheckTaskLayoutProofKind(t *testing.T) {
	cases := []struct {
		name  string
		lines map[string]string
	}{
		{"no line", nil},
		{"leaf before Verification", map[string]string{"%LEAF_BEFORE%": "  - Proof: delivery\n"}},
		{"leaf after the block", map[string]string{"%LEAF_AFTER%": "  - Proof: delivery\n"}},
		{"child after the block", map[string]string{"%CHILD_AFTER%": "    - Proof: delivery\n"}},
		{"misspelled", map[string]string{"%LEAF_BEFORE%": "  - Proof: delivry\n"}},
		{"empty", map[string]string{"%CHILD_BEFORE%": "    - Proof:\n"}},
		{"parent", map[string]string{"%PARENT%": "  - Proof: delivery\n"}},
		{"six spaces", map[string]string{"%LEAF_BEFORE%": "      - Proof: delivery\n"}},
		{"column zero", map[string]string{"%LEAF_BEFORE%": "- Proof: delivery\n"}},
		{"deeper than the block", map[string]string{"%LEAF_AFTER%": "      - Proof: delivery\n"}},
		{"proof scope", map[string]string{"%LEAF_BEFORE%": "  - Proof scope: the test covers both cases\n"}},
	}
	rejected := 0
	for _, tc := range cases {
		body := proofKindBody(tc.lines)
		_, parseErr := parseProofKindBody(t, body)
		layoutErr := CheckTaskLayout(body)
		switch {
		case parseErr == nil && layoutErr != nil:
			t.Errorf("%s: the parser accepts what the draft check rejects: %v", tc.name, layoutErr)
		case parseErr != nil && layoutErr == nil:
			t.Errorf("%s: the draft check accepts what the parser rejects: %v", tc.name, parseErr)
		case parseErr != nil && parseErr.Error() != layoutErr.Error():
			t.Errorf("%s: different messages\n parser: %v\n  draft: %v", tc.name, parseErr, layoutErr)
		}
		if parseErr != nil {
			rejected++
		}
	}
	if rejected != 6 {
		t.Errorf("%d case(s) rejected, want the six defects", rejected)
	}

	// An incomplete draft is still legal: the declaration alone, with no
	// proof yet, passes the draft check.
	draft := "# Implementation Plan\n\n- [ ] 1. A leaf still being written\n  - Proof: delivery\n"
	if err := CheckTaskLayout(draft); err != nil {
		t.Errorf("an incomplete draft with the declaration: %v", err)
	}
}
