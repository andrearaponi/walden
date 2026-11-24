package release

import (
	"context"
	"reflect"
	"testing"

	"github.com/andrearaponi/walden/internal/shell"
)

// recordingGateGit keeps the argv of every git invocation it forwards.
type recordingGateGit struct {
	inner shell.Runner
	argv  [][]string
}

func (r *recordingGateGit) Run(ctx context.Context, name string, args ...string) (shell.Response, error) {
	r.argv = append(r.argv, append([]string{name}, args...))
	return r.inner.Run(ctx, name, args...)
}

// subcommands returns the git subcommand of each recorded invocation. Every
// invocation of the gate reads `git --no-optional-locks -C <root> <subcommand>`:
// one that does not fails the test.
func (r *recordingGateGit) subcommands(t *testing.T) []string {
	t.Helper()
	subcommands := make([]string, 0, len(r.argv))
	for _, argv := range r.argv {
		if len(argv) < 5 || argv[0] != "git" || argv[1] != "--no-optional-locks" || argv[2] != "-C" {
			t.Fatalf("a git invocation does not carry the options of the gate: %q", argv)
		}
		subcommands = append(subcommands, argv[4])
	}
	return subcommands
}

// recordGateGit records the git invocations of fn through the seam of the
// gate, and puts the runner back.
func recordGateGit(t *testing.T, fn func()) *recordingGateGit {
	t.Helper()
	recorder := &recordingGateGit{inner: gitRunner}
	previous := gitRunner
	gitRunner = recorder
	defer func() { gitRunner = previous }()
	fn()
	return recorder
}

// TestReleaseCheckGitSequence fixes the git subcommands of one verdict and
// their order, on a committed tree and on a tree with a changed file. It was
// written against the gate as it stood before its identity capture and its
// derivation were given names, and it passes unchanged after.
func TestReleaseCheckGitSequence(t *testing.T) {
	root := isolatedGateRepo(t)
	completeGateTask(t, root, "1.1")
	completeGateTask(t, root, "1.2")
	commitAll(t, root, "evidence")

	verdict := func() []string {
		t.Helper()
		recorder := recordGateGit(t, func() {
			if _, err := ReleaseCheck(context.Background(), root, deliveryGateFeature, Options{}); err != nil {
				t.Fatalf("ReleaseCheck: %v", err)
			}
		})
		return recorder.subcommands(t)
	}

	// HEAD, the identity of the tree at that commit, the worktree criterion,
	// and HEAD again to see that it did not move during the judgment.
	committed := []string{"rev-parse", "status", "ls-tree", "status", "rev-parse"}
	if got := verdict(); !reflect.DeepEqual(got, committed) {
		t.Errorf("git subcommands of a verdict on a committed tree = %v, want %v", got, committed)
	}

	// A changed file is hashed after the listing of the commit.
	write(t, root, "main.go", "package main // changed\n")
	changed := []string{"rev-parse", "status", "ls-tree", "hash-object", "status", "rev-parse"}
	if got := verdict(); !reflect.DeepEqual(got, changed) {
		t.Errorf("git subcommands of a verdict on a changed tree = %v, want %v", got, changed)
	}
}
