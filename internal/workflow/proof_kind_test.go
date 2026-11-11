package workflow

import (
	"testing"

	"github.com/andrearaponi/walden/internal/spec"
)

const proofKindViewPlan = `---
status: approved
approved_at: 2026-03-23T10:00:00Z
last_modified: 2026-03-23T10:00:00Z
source_design_approved_at: 2026-03-23T09:30:00Z
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
  - [ ] 2.1 A regression proof on a child leaf
    - Requirements: ` + "`R1.AC2`" + `
    - Design: Overview
    - Verification:
      - command: ["sh", "-c", "true"]
        covers: ["R1.AC2"]
`

// TestExecutableTaskKeepsProofKind: every view the executor builds of a
// delivery leaf carries the declaration, so the fingerprint a record is bound
// to is the one of the declared definition.
func TestExecutableTaskKeepsProofKind(t *testing.T) {
	root := t.TempDir()
	const name = "proof-kind-view"
	setupApprovedFeature(t, root, name, proofKindViewPlan)

	feature, err := spec.LoadFeature(root, name)
	if err != nil {
		t.Fatalf("load the feature: %v", err)
	}
	tree, err := spec.ParseTaskTree(feature.Tasks)
	if err != nil {
		t.Fatalf("parse the plan: %v", err)
	}
	delivery, regression := tree.LeafTasks()[0], tree.LeafTasks()[1]
	want := spec.TaskDefinitionFingerprint(delivery)
	if want == spec.UndeclaredTaskDefinitionFingerprint(delivery) {
		t.Fatal("the definition of the delivery leaf does not hold its declaration")
	}

	readiness, err := ResolveExecutionReadiness(feature)
	if err != nil || readiness.NextTask == nil {
		t.Fatalf("resolve readiness: %v, next task %v", err, readiness.NextTask)
	}
	started, err := StartTask(root, name, "1")
	if err != nil {
		t.Fatalf("start the task: %v", err)
	}
	next := nextRunnableTask(tree)
	if next == nil {
		t.Fatal("no runnable task")
	}
	views := map[string]ExecutableTask{
		"toExecutableTask":          toExecutableTask(delivery),
		"ResolveExecutionReadiness": *readiness.NextTask,
		"StartTask":                 started.Task,
		"nextRunnableTask":          *next,
	}
	for label, view := range views {
		if view.ID != "1" || !view.Proof.Delivery {
			t.Errorf("%s: task %s, delivery=%t: the view lost the declaration", label, view.ID, view.Proof.Delivery)
		}
		if got := executableTaskFingerprint(view); got != want {
			t.Errorf("%s: fingerprint %s, want the fingerprint of the plan %s", label, got, want)
		}
	}

	plain := toExecutableTask(regression)
	if plain.Proof.Delivery {
		t.Error("a leaf without the declaration is viewed as a delivery task")
	}
	if got := executableTaskFingerprint(plain); got != spec.TaskDefinitionFingerprint(regression) {
		t.Errorf("regression leaf: fingerprint %s differs from the plan", got)
	}
}
