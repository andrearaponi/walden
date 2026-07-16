package release

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/testutil"
	"github.com/andrearaponi/walden/internal/workflow"
)

func TestReleaseReportCompletion(t *testing.T) {
	complete := ReleaseReport{Features: []FeatureCertification{{Feature: "a"}, {Feature: "b"}}}
	if got := complete.Completion(); got != CompletionComplete {
		t.Fatalf("no pending tasks classified as %q, want %q", got, CompletionComplete)
	}

	withPending := ReleaseReport{Features: []FeatureCertification{
		{Feature: "a"},
		{Feature: "b", Pending: []string{"1.2"}},
	}}
	if got := withPending.Completion(); got != CompletionWithPending {
		t.Fatalf("pending tasks classified as %q, want %q", got, CompletionWithPending)
	}

	// Integration: the green gate repo has every task executed.
	root := gateRepo(t)
	report, err := ReleaseCheck(context.Background(), root, "gate-demo", Options{})
	if err != nil {
		t.Fatalf("ReleaseCheck: %v", err)
	}
	if !report.Releasable() || report.Completion() != CompletionComplete {
		t.Fatalf("green repo completion = %q (releasable=%t), want complete", report.Completion(), report.Releasable())
	}
}

func TestReleaseCheckCertifiedCommit(t *testing.T) {
	root := gateRepo(t)

	head := exec.Command("git", "rev-parse", "HEAD")
	head.Dir = root
	out, err := head.Output()
	if err != nil {
		t.Fatalf("rev-parse fixture HEAD: %v", err)
	}
	expected := strings.TrimSpace(string(out))

	report, err := ReleaseCheck(context.Background(), root, "gate-demo", Options{})
	if err != nil {
		t.Fatalf("ReleaseCheck: %v", err)
	}
	if report.CertifiedCommit != expected {
		t.Fatalf("certified commit = %q, want %q", report.CertifiedCommit, expected)
	}
}

func TestReleaseCheckCertifiedCommitEmptyWithoutGit(t *testing.T) {
	root := t.TempDir()
	addFeature(t, root, "gate-demo", certifiableRequirements(), certifiableDesign(""), certifiableTasks())
	runner := testutil.NewFakeRunner(
		testutil.Response{Stdout: "ok", ExitCode: 0},
		testutil.Response{Stdout: "ok", ExitCode: 0},
	)
	if _, err := workflow.CompleteAllTasks(context.Background(), root, "gate-demo", runner); err != nil {
		t.Fatalf("complete: %v", err)
	}

	report, err := ReleaseCheck(context.Background(), root, "gate-demo", Options{})
	if err != nil {
		t.Fatalf("ReleaseCheck: %v", err)
	}
	if report.CertifiedCommit != "" {
		t.Fatalf("expected empty certified commit without git, got %q", report.CertifiedCommit)
	}
	if report.Releasable() {
		t.Fatal("a repository without git must stay blocked (existing fail-closed contract)")
	}
}
