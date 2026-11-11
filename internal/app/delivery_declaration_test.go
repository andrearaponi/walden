package app

import (
	"bytes"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/testutil"
)

// deliveryDeclarationTasks declares a delivery proof on a top-level leaf,
// before its Verification line, and on a child leaf, after its block.
const deliveryDeclarationTasks = `---
status: draft
approved_at:
last_modified: 2026-07-11T13:00:00Z
approved_fingerprint:
source_design_approved_at:
source_design_fingerprint:
---

# Implementation Plan

- [ ] 1. A delivery on a top-level leaf
  - Requirements: ` + "`R1.AC1`" + `
  - Design: Overview
  - Proof: delivery
  - Verification:
    - command: ["sh", "-c", "true"]
      covers: ["R1.AC1"]
- [ ] 2. A group
  - [ ] 2.1 A delivery on a child leaf
    - Requirements: ` + "`R1.AC2`" + `
    - Design: Overview
    - Verification:
      - command: ["sh", "-c", "true"]
        covers: ["R1.AC2"]
    - Proof: delivery
`

// addDeliveryFeature writes a feature with the given plan and approves its
// chain through the review gates, as a user would.
func addDeliveryFeature(t *testing.T, root, name, tasks string) {
	t.Helper()
	writeStatusFeatureFile(t, root, name, "requirements.md", releasableRequirements)
	writeStatusFeatureFile(t, root, name, "design.md", releasableDesign)
	writeStatusFeatureFile(t, root, name, "tasks.md", tasks)

	var stdout, stderr bytes.Buffer
	for _, phase := range []string{"requirements", "design", "tasks"} {
		for _, action := range []string{"open", "approve"} {
			stdout.Reset()
			stderr.Reset()
			if code := Run([]string{"review", action, name, "--phase", phase}, &stdout, &stderr); code != 0 {
				t.Fatalf("review %s %s failed: %s%s", action, phase, stdout.String(), stderr.String())
			}
		}
	}
}

// TestDeliveryDeclarationEndToEnd takes a plan that declares delivery proofs
// through validation and execution. The validator and the executor read the
// plan with different entry points: a line one accepts and the other rejects
// would validate and then dead-end at the first task.
func TestDeliveryDeclarationEndToEnd(t *testing.T) {
	root := chdirContract(t)
	const feature = "delivery-declared"
	addDeliveryFeature(t, root, feature, deliveryDeclarationTasks)
	overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: "ok"}, testutil.Response{Stdout: "ok"}))

	steps := [][]string{
		{"validate", feature, "--all"},
		{"task", "status", feature},
		{"task", "start", feature, "1"},
		{"task", "complete", feature, "1"},
		{"task", "status", feature},
		{"task", "start", feature, "2.1"},
		{"task", "complete", feature, "2.1"},
		{"task", "status", feature},
		{"validate", feature, "--all"},
	}
	for _, args := range steps {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code != 0 {
			t.Fatalf("walden %s exited %d:\n%s%s", strings.Join(args, " "), code, stdout.String(), stderr.String())
		}
	}
}
