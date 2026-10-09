package validation

import (
	"strings"
	"testing"
)

// TestValidateDraftAndInReviewReportSameProofDefect validates one plan with a
// backticked covers in draft and in review: both must report the defect the
// task parser names.
func TestValidateDraftAndInReviewReportSameProofDefect(t *testing.T) {
	const plan = `---
status: %s
approved_at:
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at:
---

# Implementation Plan

- [ ] 1. Build feature
  - [ ] 1.1 Add implementation
    - Requirements: __BT__R1.AC1__BT__, __BT__NFR1__BT__
    - Design: Todo flow
    - Verification:
      - command: ["go", "test", "./..."]
        covers: __BT__R1.AC1__BT__
      - command: ["go", "vet", "./..."]
`
	messages := map[string]string{}
	for _, status := range []string{"draft", "in-review"} {
		root := t.TempDir()
		writeFeatureFile(t, root, "proof-test", "requirements.md", validRequirements)
		writeFeatureFile(t, root, "proof-test", "design.md", validDraftDesign)
		writeFeatureFile(t, root, "proof-test", "tasks.md", strings.Replace(plan, "%s", status, 1))

		result, err := ValidateFeatureWithScope(root, "proof-test", ScopeFullSpec)
		if err != nil {
			t.Fatalf("validate the %s plan: %v", status, err)
		}
		if result.Valid {
			t.Fatalf("the %s plan validated although its covers is not a JSON array", status)
		}
		messages[status] = result.Message
	}

	if messages["draft"] != messages["in-review"] {
		t.Fatalf("draft and review report different defects:\n draft: %s\nreview: %s", messages["draft"], messages["in-review"])
	}
	if !strings.Contains(messages["draft"], `invalid covers for task "1.1"`) {
		t.Fatalf("the defect does not name the covers form: %s", messages["draft"])
	}
}
