package release

import (
	"context"
	"strings"
	"testing"
)

// Strict composes with the waiver: committed state stays required while the
// pending work is waived — each knob keeps exactly its own meaning.
func TestReleaseCheckStrictComposesWithWaiver(t *testing.T) {
	root := halfDoneRepo(t)

	// Committed state, waived pending: strict passes.
	composed, err := ReleaseCheck(context.Background(), root, "gate-demo", Options{Strict: true, AllowPending: true, WaiverReason: "deferred"})
	if err != nil {
		t.Fatalf("ReleaseCheck: %v", err)
	}
	if !composed.Releasable() {
		t.Fatalf("strict+waiver on committed state did not certify: %+v", composed)
	}
	if composed.Completion() != CompletionWithWaivers {
		t.Fatalf("completion = %s, want with-waivers", composed.Completion())
	}

	// Dirty .walden/ under strict still blocks, waiver or not.
	gitIn(t, root, "rm", "-q", "--cached", ".walden/evidence/gate-demo.json")
	dirty, err := ReleaseCheck(context.Background(), root, "gate-demo", Options{Strict: true, AllowPending: true, WaiverReason: "deferred"})
	if err != nil {
		t.Fatalf("ReleaseCheck dirty: %v", err)
	}
	if dirty.Releasable() {
		t.Fatal("strict certified with uncommitted .walden state despite the waiver")
	}
	if !strings.Contains(strings.Join(dirty.WorktreeBlockers, " "), "--strict") {
		t.Fatalf("strict blocker missing: %v", dirty.WorktreeBlockers)
	}
}
