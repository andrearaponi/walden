package workflow

import (
	"context"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/testutil"
)

// When git reports an uncommitted path and does not say where the root lies
// in its working tree, the path cannot be placed under the root and no
// identity is derived: a verified task reads unattested and verify refuses to
// run. The path is not dropped, which would leave the identity of the commit
// in its place.
func TestUnplacedPathLeavesNoIdentity(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeVerifyFixture(t, root)
	overrideIdentityRunner(t, identityYielding("100644 blob aaa\tmain.go\n"))
	completeBoth(t, root)

	_, before, err := EvidenceReport(ctx, root, "todo-app-demo")
	if err != nil {
		t.Fatalf("EvidenceReport on the committed tree: %v", err)
	}
	if len(before) != 2 {
		t.Fatalf("the fixture has %d task(s), want two", len(before))
	}
	for _, entry := range before {
		if entry.State != evidence.StateVerified || entry.CurrentIdentity == "" {
			t.Fatalf("task %s reads %s with identity %q before the change, want verified", entry.TaskID, entry.State, entry.CurrentIdentity)
		}
	}

	// The scripted git of this package answers every question but the
	// listing with its status text. Asked where the root lies, it answers
	// with a status line, which is no answer.
	overrideIdentityRunner(t, &scriptedIdentityRunner{
		statusResponse: shell.Response{ExitCode: 0, Stdout: " M main.go\x00"},
		lsTreeResponse: shell.Response{ExitCode: 0, Stdout: "100644 blob aaa\tmain.go\n"},
	})

	_, after, err := EvidenceReport(ctx, root, "todo-app-demo")
	if err != nil {
		t.Fatalf("EvidenceReport with a path that cannot be placed: %v", err)
	}
	for _, entry := range after {
		if entry.State != evidence.StateUnattested {
			t.Errorf("task %s reads %s, want unattested", entry.TaskID, entry.State)
		}
		if entry.CurrentIdentity != "" {
			t.Errorf("task %s is given the current identity %q, want none", entry.TaskID, entry.CurrentIdentity)
		}
	}

	proofs := testutil.NewFakeRunner(testutil.Response{Stdout: "ok"}, testutil.Response{Stdout: "ok"})
	_, err = Verify(ctx, root, "todo-app-demo", false, false, proofs)
	if err == nil || !strings.Contains(err.Error(), "usable initial code identity") {
		t.Errorf("Verify returned %v, want the refusal for an identity that is not available", err)
	}
	if calls := proofs.Calls(); len(calls) != 0 {
		t.Errorf("Verify started %d proof(s) without an identity", len(calls))
	}
}
