package release

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The gate judges and never writes: after a full certification the worktree
// is byte-identical — no proof ran, no state moved.
func TestReleaseCheckStaysReadOnly(t *testing.T) {
	root := gateRepo(t)

	ledgerPath := filepath.Join(root, ".walden", "evidence", "gate-demo.json")
	ledgerBefore, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatalf("read ledger before: %v", err)
	}

	statusOf := func() string {
		command := exec.Command("git", "status", "--porcelain")
		command.Dir = root
		out, err := command.Output()
		if err != nil {
			t.Fatalf("git status: %v", err)
		}
		return string(out)
	}
	if before := statusOf(); before != "" {
		t.Fatalf("fixture not clean before the check: %q", before)
	}

	report, err := ReleaseCheck(context.Background(), root, "", Options{})
	if err != nil {
		t.Fatalf("ReleaseCheck: %v", err)
	}
	if !report.Releasable() {
		t.Fatalf("green fixture not releasable: %+v", report)
	}

	if after := statusOf(); after != "" {
		t.Fatalf("release check dirtied the worktree: %q", after)
	}
	ledgerAfter, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatalf("read ledger after: %v", err)
	}
	if string(ledgerBefore) != string(ledgerAfter) {
		t.Fatal("release check modified the evidence ledger")
	}
}
