package workflow

import (
	"context"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
)

// Completion is the implementation step: a proof that mutates the tree
// (generators, builds) must complete normally — purity binds verify only.
func TestCompleteTaskMutatingProofSucceeds(t *testing.T) {
	root := t.TempDir()
	writeVerifyFixture(t, root)
	overrideIdentityRunner(t, &dynamicIdentityRunner{root: root})

	result, err := CompleteTask(context.Background(), root, "todo-app-demo", "1.1", &sideEffectRunner{root: root})
	if err != nil {
		t.Fatalf("mutating completion proof was rejected: %v", err)
	}
	if len(result.CompletedTasks) == 0 || result.CompletedTasks[0] != "1.1" {
		t.Fatalf("task 1.1 not completed: %+v", result.CompletedTasks)
	}
}

func TestCompleteEvidenceBindsPostProofTree(t *testing.T) {
	root := t.TempDir()
	writeVerifyFixture(t, root)
	identityRunnerForRoot := &dynamicIdentityRunner{root: root}
	overrideIdentityRunner(t, identityRunnerForRoot)

	preIdentity, ok := evidence.Identity(context.Background(), identityRunnerForRoot, root)
	if !ok {
		t.Fatal("pre-completion identity not computed")
	}

	if _, err := CompleteTask(context.Background(), root, "todo-app-demo", "1.1", &sideEffectRunner{root: root}); err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}

	postIdentity, ok := evidence.Identity(context.Background(), identityRunnerForRoot, root)
	if !ok {
		t.Fatal("post-completion identity not computed")
	}

	ledger, err := evidence.Load(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("load ledger: %v", err)
	}
	record, exists := ledger.Tasks["1.1"]
	if !exists {
		t.Fatal("no evidence recorded for task 1.1")
	}
	if record.CodeIdentity == preIdentity {
		t.Fatal("evidence bound the pre-proof tree; it must bind the tree the proof left behind")
	}
	if record.CodeIdentity != postIdentity {
		t.Fatalf("evidence identity %s does not match the post-proof tree %s", record.CodeIdentity, postIdentity)
	}
}
