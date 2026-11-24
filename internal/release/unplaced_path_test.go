package release

import (
	"context"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/shell"
)

// gitWithoutPosition wraps real git and fails the one question that says
// where the root lies in its working tree. It counts how often it was asked.
func gitWithoutPosition(t *testing.T) *int {
	t.Helper()
	asked := 0
	previous := gitRunner
	t.Cleanup(func() { gitRunner = previous })
	gitRunner = integrityGitRunner(func(ctx context.Context, name string, args ...string) (shell.Response, error) {
		for _, arg := range args {
			if arg == "--show-prefix" {
				asked++
				return shell.Response{ExitCode: 128, Stderr: "injected: git cannot say where it runs"}, nil
			}
		}
		return shell.NewExecRunner().Run(ctx, name, args...)
	})
	return &asked
}

// The gate does not certify on an identity it could not complete. With an
// uncommitted path that git cannot place under the root, the identity is not
// available: no task reads verified and the verdict carries the blocker for a
// missing identity. On a committed tree nothing needs placing and the same
// git is enough.
func TestReleaseCheckBlocksOnUnplacedPath(t *testing.T) {
	root := isolatedGateRepo(t)
	completeGateTask(t, root, "1.1")
	completeGateTask(t, root, "1.2")
	commitAll(t, root, "evidence")
	asked := gitWithoutPosition(t)

	report, cert := gateCertification(t, root, Options{})
	if !report.Releasable() {
		t.Fatalf("a committed tree is not certified with a git that cannot say where it runs: %+v", report.WorktreeBlockers)
	}
	if *asked != 0 {
		t.Fatalf("the gate asked where the root lies %d time(s) on a committed tree", *asked)
	}
	if states := gateStates(cert); states["1.2"] != evidence.StateVerified {
		t.Fatalf("states on the committed tree = %v", states)
	}

	write(t, root, "main.go", "package main // changed\n")
	report, cert = gateCertification(t, root, Options{})
	if report.Releasable() {
		t.Fatal("the gate certified a tree whose uncommitted path it could not place")
	}
	if *asked == 0 {
		t.Error("the gate did not ask where the root lies with an uncommitted path")
	}
	states := gateStates(cert)
	if states["1.2"] != evidence.StateUnattested {
		t.Errorf("the regression task reads %s, want unattested", states["1.2"])
	}
	for id, state := range states {
		if state == evidence.StateVerified {
			t.Errorf("task %s reads verified without an identity", id)
		}
	}
	blockers := strings.Join(report.WorktreeBlockers, "\n")
	if !strings.Contains(blockers, "certification requires a git-backed code identity") {
		t.Errorf("the blocker for a missing identity is absent:\n%s", blockers)
	}
	if !strings.Contains(blockers, "uncommitted: main.go") {
		t.Errorf("the uncommitted path is not named:\n%s", blockers)
	}
}

// When git does not say where the root lies, the gate cannot tell the
// .walden/ of the root from any other directory of that name in the
// repository, so no uncommitted path counts as Walden state: each one blocks.
func TestWorktreeCriterionWithoutPosition(t *testing.T) {
	root := isolatedGateRepo(t)
	completeGateTask(t, root, "1.1")
	completeGateTask(t, root, "1.2")
	commitAll(t, root, "evidence")
	write(t, root, ".walden/scratch.txt", "scratch\n")

	report, _ := gateCertification(t, root, Options{})
	if !report.Releasable() || len(report.WaldenDirty) != 1 || report.WaldenDirty[0] != ".walden/scratch.txt" {
		t.Fatalf("with a git that answers, a path under .walden/ warns: releasable %t, walden %v, blockers %v", report.Releasable(), report.WaldenDirty, report.WorktreeBlockers)
	}

	asked := gitWithoutPosition(t)
	report, _ = gateCertification(t, root, Options{})
	if *asked == 0 {
		t.Fatal("the worktree criterion did not ask where the root lies")
	}
	if report.Releasable() || len(report.WaldenDirty) != 0 {
		t.Errorf("without a position a path under .walden/ still only warns: releasable %t, walden %v", report.Releasable(), report.WaldenDirty)
	}
	if blockers := strings.Join(report.WorktreeBlockers, "\n"); !strings.Contains(blockers, "uncommitted: .walden/scratch.txt — commit it before certifying") {
		t.Errorf("the path is not a blocker:\n%s", blockers)
	}
}
