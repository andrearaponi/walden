package spec

import "testing"

func proofKindLeaves(t *testing.T, lines map[string]string) map[string]*Task {
	t.Helper()
	tree, err := parseProofKindBody(t, proofKindBody(lines))
	if err != nil {
		t.Fatalf("parse the plan: %v", err)
	}
	leaves := map[string]*Task{}
	for _, task := range tree.LeafTasks() {
		leaves[task.ID] = task
	}
	return leaves
}

// TestTaskFingerprintBindsProofKind: the declaration is part of the task's
// definition, and of nothing else.
func TestTaskFingerprintBindsProofKind(t *testing.T) {
	plain := proofKindLeaves(t, nil)
	cases := map[string]struct {
		lines    map[string]string
		declared string
		other    string
	}{
		"top-level leaf": {map[string]string{"%LEAF_BEFORE%": "  - Proof: delivery\n"}, "1", "2.1"},
		"child leaf":     {map[string]string{"%CHILD_AFTER%": "    - Proof: delivery\n"}, "2.1", "1"},
	}
	for name, tc := range cases {
		leaves := proofKindLeaves(t, tc.lines)
		declared, other := leaves[tc.declared], leaves[tc.other]

		if TaskDefinitionFingerprint(declared) == TaskDefinitionFingerprint(plain[tc.declared]) {
			t.Errorf("%s: the declaration does not change the fingerprint of task %s", name, tc.declared)
		}
		if TaskDefinitionFingerprint(other) != TaskDefinitionFingerprint(plain[tc.other]) {
			t.Errorf("%s: the declaration on task %s changed the fingerprint of task %s", name, tc.declared, tc.other)
		}

		// Without the declaration the task has the fingerprint of the same
		// task in the plan that never held the line.
		if UndeclaredTaskDefinitionFingerprint(declared) != TaskDefinitionFingerprint(plain[tc.declared]) {
			t.Errorf("%s: the undeclared fingerprint of task %s is not the one of the plan without the line", name, tc.declared)
		}
		if !declared.Proof.Delivery {
			t.Errorf("%s: computing the undeclared fingerprint cleared the declaration", name)
		}
		if UndeclaredTaskDefinitionFingerprint(other) != TaskDefinitionFingerprint(other) {
			t.Errorf("%s: a task without the declaration has two fingerprints", name)
		}
	}

	// Where the line sits is presentation: before the Verification line or
	// after its block, the definition is the same.
	before := proofKindLeaves(t, map[string]string{"%LEAF_BEFORE%": "  - Proof: delivery\n"})["1"]
	after := proofKindLeaves(t, map[string]string{"%LEAF_AFTER%": "  - Proof: delivery\n"})["1"]
	if TaskDefinitionFingerprint(before) != TaskDefinitionFingerprint(after) {
		t.Error("the position of the declaration changes the fingerprint")
	}
}
