package validation

import (
	"reflect"
	"strings"
	"testing"
)

// deliveryOnlyPlan holds a delivery leaf and a regression leaf. The
// placeholders take the frontmatter, the declaration line and the covers of
// the regression proof.
const deliveryOnlyPlan = `---
%HEADER%
---

# Implementation Plan

- [ ] 1. Build feature
  - [ ] 1.1 Publish the notes
    - Requirements: __BT__R1.AC1__BT__, __BT__NFR1__BT__
    - Design: Todo flow
%DECLARATION%    - Verification:
      - command: ["grep", "-q", "released", "notes.txt"]
        covers: ["R1.AC1"]
  - [ ] 1.2 Keep it deterministic
    - Requirements: __BT__NFR1__BT__
    - Design: Todo flow
    - Verification:
      - command: ["go", "test", "./..."]
        covers: [%REGRESSION_COVERS%]
`

var deliveryOnlyHeaders = map[string]string{
	"draft":     "status: draft\napproved_at:\nlast_modified: 2026-03-21T14:20:00Z\nsource_design_approved_at:",
	"in-review": "status: in-review\napproved_at:\nlast_modified: 2026-03-21T14:20:00Z\nsource_design_approved_at:",
	"approved":  "status: approved\napproved_at: 2026-03-21T14:20:00Z\nlast_modified: 2026-03-21T14:20:00Z\nsource_design_approved_at: 2026-03-21T14:10:00Z",
}

const deliveryOnlyWarning = "statement(s) covered only by delivery proofs, which are not proven again once attested: "

func validateDeliveryOnly(t *testing.T, status, declaration, regressionCovers string, scope Scope) Result {
	t.Helper()
	root := t.TempDir()
	plan := strings.NewReplacer(
		"%HEADER%", deliveryOnlyHeaders[status], "%DECLARATION%", declaration, "%REGRESSION_COVERS%", regressionCovers,
	).Replace(deliveryOnlyPlan)
	writeFeatureFile(t, root, "delivery-only", "requirements.md", validRequirements)
	writeFeatureFile(t, root, "delivery-only", "design.md", validDesign)
	writeFeatureFile(t, root, "delivery-only", "tasks.md", plan)

	result, err := ValidateFeatureWithScope(root, "delivery-only", scope)
	if err != nil {
		t.Fatalf("validate the %s plan: %v", status, err)
	}
	if !result.Valid {
		t.Fatalf("the %s plan does not validate: %s", status, result.Message)
	}
	return result
}

func deliveryOnlyWarnings(result Result) []string {
	found := []string{}
	for _, warning := range result.Warnings {
		if strings.HasPrefix(warning, deliveryOnlyWarning) {
			found = append(found, warning)
		}
	}
	return found
}

// TestValidateWarnsDeliveryOnly: validation names the statements that only
// delivery proofs cover, before the plan is approved and after.
func TestValidateWarnsDeliveryOnly(t *testing.T) {
	const declaration = "    - Proof: delivery\n"
	for _, status := range []string{"draft", "in-review", "approved"} {
		// R1.AC1 is covered by the delivery proof alone; NFR1 by the
		// regression proof.
		declared := validateDeliveryOnly(t, status, declaration, `"NFR1"`, ScopeFullSpec)
		if got, want := deliveryOnlyWarnings(declared), []string{deliveryOnlyWarning + "R1.AC1"}; !reflect.DeepEqual(got, want) {
			t.Errorf("%s: warnings = %v, want %v", status, got, want)
		}

		// A regression proof covers R1.AC1 too: nothing to name.
		both := validateDeliveryOnly(t, status, declaration, `"NFR1", "R1.AC1"`, ScopeFullSpec)
		if got := deliveryOnlyWarnings(both); len(got) != 0 {
			t.Errorf("%s: a statement a regression proof covers too is named: %v", status, got)
		}

		// Apart from the warning, the result is the one of the same plan
		// without the declaration.
		plain := validateDeliveryOnly(t, status, "", `"NFR1"`, ScopeFullSpec)
		if got := deliveryOnlyWarnings(plain); len(got) != 0 {
			t.Errorf("%s: a plan without delivery proofs is warned: %v", status, got)
		}
		without := declared
		without.Warnings = nil
		for _, warning := range declared.Warnings {
			if !strings.HasPrefix(warning, deliveryOnlyWarning) {
				without.Warnings = append(without.Warnings, warning)
			}
		}
		without.SpecDir, plain.SpecDir = "", ""
		if !reflect.DeepEqual(without, plain) {
			t.Errorf("%s: the declaration changed more than the warning:\n got %+v\nwant %+v", status, without, plain)
		}
	}

	// While validation does not cover the plan there is no warning: with a
	// design still in draft the current phase stops before the tasks.
	root := t.TempDir()
	plan := strings.NewReplacer(
		"%HEADER%", deliveryOnlyHeaders["draft"], "%DECLARATION%", declaration, "%REGRESSION_COVERS%", `"NFR1"`,
	).Replace(deliveryOnlyPlan)
	writeFeatureFile(t, root, "delivery-only", "requirements.md", validRequirements)
	writeFeatureFile(t, root, "delivery-only", "design.md", validDraftDesign)
	writeFeatureFile(t, root, "delivery-only", "tasks.md", plan)
	result, err := ValidateFeatureWithScope(root, "delivery-only", ScopeCurrentPhase)
	if err != nil || !result.Valid {
		t.Fatalf("validate the design phase: %v, %s", err, result.Message)
	}
	if got := deliveryOnlyWarnings(result); len(got) != 0 {
		t.Errorf("a plan outside the validated scope is warned: %v", got)
	}
}
