package spec

import (
	"reflect"
	"testing"
)

func deliveryOnlyOf(t *testing.T, body string) []string {
	t.Helper()
	tree, err := ParseTaskTree(Document{Exists: true, Path: "tasks.md", Body: body})
	if err != nil {
		t.Fatalf("parse the plan: %v", err)
	}
	return DeliveryOnlyStatements(tree)
}

// TestDeliveryOnlyStatements: a statement is listed when a delivery proof
// covers it and no regression proof does.
func TestDeliveryOnlyStatements(t *testing.T) {
	const plan = `# Implementation Plan

- [ ] 1. A delivery
  - Requirements: ` + "`R1.AC1`, `R1.AC3`, `R3.AC1`" + `
  - Design: Architecture
  - Proof: delivery
  - Verification:
    - command: ["true"]
      covers: ["R1.AC1", "R1.AC3"]
    - command: ["true"]
      covers: ["NFR1"]
- [ ] 2. A regression proof
  - Requirements: ` + "`R1.AC2`, `R1.AC3`" + `
  - Design: Architecture
  - Verification:
    - command: ["true"]
      covers: ["R1.AC2", "R1.AC3"]
- [ ] 3. A group
  - [ ] 3.1 Another delivery
    - Requirements: ` + "`R2.AC1`, `R1.AC10`" + `
    - Design: Architecture
    - Proof: delivery
    - Verification:
      - command: ["true"]
        covers: ["R2.AC1", "R1.AC10", "C2"]
`
	// R1.AC1 is covered by a delivery proof alone, R1.AC2 by a regression
	// proof alone, R1.AC3 by both. R3.AC1 is named by the delivery task and
	// covered by no step: it is not covered at all, which another report
	// says. The order is the one of a requirements document.
	want := []string{"R1.AC1", "R1.AC10", "R2.AC1", "NFR1", "C2"}
	if got := deliveryOnlyOf(t, plan); !reflect.DeepEqual(got, want) {
		t.Errorf("delivery-only statements = %v, want %v", got, want)
	}

	// Without a delivery proof there is nothing to list.
	const plain = `# Implementation Plan

- [ ] 1. A regression proof
  - Requirements: ` + "`R1.AC1`" + `
  - Design: Architecture
  - Verification:
    - command: ["true"]
      covers: ["R1.AC1"]
`
	if got := deliveryOnlyOf(t, plain); len(got) != 0 {
		t.Errorf("a plan without delivery proofs lists %v", got)
	}

	// A delivery proof in the legacy format names no statement in covers.
	const legacy = `# Implementation Plan

- [ ] 1. A delivery
  - Requirements: ` + "`R1.AC1`" + `
  - Design: Architecture
  - Proof: delivery
  - Verification: ` + "`true`" + `
`
	if got := deliveryOnlyOf(t, legacy); len(got) != 0 {
		t.Errorf("a legacy delivery proof lists %v", got)
	}
}
