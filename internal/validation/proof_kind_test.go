package validation

import (
	"strings"
	"testing"
)

// proofKindPlan is a plan with a group and one child leaf. The placeholders
// take a line with its newline, or nothing.
const proofKindPlan = `---
status: %STATUS%
approved_at:
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at:
---

# Implementation Plan

- [ ] 1. Build feature
%PARENT%  - [ ] 1.1 Add implementation
    - Requirements: __BT__R1.AC1__BT__, __BT__NFR1__BT__
    - Design: Todo flow
%CHILD%    - Verification:
      - command: ["go", "test", "./..."]
        covers: ["R1.AC1"]
`

func validateProofKindPlan(t *testing.T, status, parent, child string) Result {
	t.Helper()
	root := t.TempDir()
	plan := strings.NewReplacer("%STATUS%", status, "%PARENT%", parent, "%CHILD%", child).Replace(proofKindPlan)
	writeFeatureFile(t, root, "proof-kind", "requirements.md", validRequirements)
	writeFeatureFile(t, root, "proof-kind", "design.md", validDraftDesign)
	writeFeatureFile(t, root, "proof-kind", "tasks.md", plan)

	result, err := ValidateFeatureWithScope(root, "proof-kind", ScopeFullSpec)
	if err != nil {
		t.Fatalf("validate the %s plan: %v", status, err)
	}
	return result
}

// TestValidateProofKind holds validation to the declaration rules for a plan
// in draft and in review: a draft that validates must not fail on the same
// line once it is opened for review.
func TestValidateProofKind(t *testing.T) {
	for _, status := range []string{"draft", "in-review"} {
		if result := validateProofKindPlan(t, status, "", "    - Proof: delivery\n"); !result.Valid {
			t.Errorf("%s: the declaration on a leaf does not validate: %s", status, result.Message)
		}

		unknown := validateProofKindPlan(t, status, "", "    - Proof: delivry\n")
		if unknown.Valid {
			t.Errorf("%s: a misspelled declaration validated", status)
		}
		for _, want := range []string{`unknown proof kind "delivry" for task "1.1"`, `expected "delivery"`} {
			if !strings.Contains(unknown.Message, want) {
				t.Errorf("%s: %q does not hold %q", status, unknown.Message, want)
			}
		}

		empty := validateProofKindPlan(t, status, "", "    - Proof:\n")
		if empty.Valid || !strings.Contains(empty.Message, `unknown proof kind "" for task "1.1"`) {
			t.Errorf("%s: an empty declaration: valid=%t, %q", status, empty.Valid, empty.Message)
		}

		parent := validateProofKindPlan(t, status, "  - Proof: delivery\n", "")
		if parent.Valid {
			t.Errorf("%s: a declaration on a task with subtasks validated", status)
		}
		if want := `non-leaf task "1" cannot declare a proof kind`; !strings.Contains(parent.Message, want) {
			t.Errorf("%s: %q does not hold %q", status, parent.Message, want)
		}
	}

	// Draft and review report the same defect.
	for name, lines := range map[string][2]string{
		"unknown": {"", "    - Proof: delivry\n"},
		"parent":  {"  - Proof: delivery\n", ""},
	} {
		draft := validateProofKindPlan(t, "draft", lines[0], lines[1])
		review := validateProofKindPlan(t, "in-review", lines[0], lines[1])
		if draft.Message != review.Message {
			t.Errorf("%s: different defects\n draft: %s\nreview: %s", name, draft.Message, review.Message)
		}
	}
}
