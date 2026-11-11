package evidence

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/shell"
)

// scriptedHistory answers the two git commands a search runs: the history
// listing with its ids, and the tree listing of each id with one file whose
// content names the tree. It records every call.
type scriptedHistory struct {
	ids       []string
	trees     map[string]string // commit id → tree name
	failLog   bool
	calls     []string
	requested []string // the -n value of each history listing
}

func (h *scriptedHistory) Run(_ context.Context, name string, args ...string) (shell.Response, error) {
	// args: --no-optional-locks -C <root> <subcommand> ...
	if name != "git" || len(args) < 4 {
		return shell.Response{ExitCode: 1}, fmt.Errorf("unexpected command %s %v", name, args)
	}
	subcommand := args[3]
	h.calls = append(h.calls, subcommand)
	switch subcommand {
	case "log":
		for index, arg := range args {
			if arg == "-n" && index+1 < len(args) {
				h.requested = append(h.requested, args[index+1])
			}
		}
		if h.failLog {
			return shell.Response{ExitCode: 128, Stderr: "fatal: not a git repository"}, nil
		}
		return shell.Response{Stdout: strings.Join(h.ids, "\n") + "\n"}, nil
	case "ls-tree":
		commit := args[len(args)-1]
		return shell.Response{Stdout: "100644 blob " + h.trees[commit] + "\tmain.go\n"}, nil
	}
	return shell.Response{ExitCode: 1}, fmt.Errorf("unexpected git %s", subcommand)
}

func (h *scriptedHistory) count(subcommand string) int {
	total := 0
	for _, call := range h.calls {
		if call == subcommand {
			total++
		}
	}
	return total
}

func commitID(index int) string { return fmt.Sprintf("%040x", index+1) }

// identityOfTree is the code identity of the tree a scripted commit holds.
func identityOfTree(name string) string {
	return RebuildManifest("100644 blob "+name+"\tmain.go\n", nil).Digest()
}

// scriptedCommits builds a history of n commits, newest first, each with a
// tree of its own, named tree-<index>.
func scriptedCommits(n int) *scriptedHistory {
	history := &scriptedHistory{trees: map[string]string{}}
	for index := 0; index < n; index++ {
		history.ids = append(history.ids, commitID(index))
		history.trees[commitID(index)] = fmt.Sprintf("tree-%d", index)
	}
	return history
}

func TestTreeSearch(t *testing.T) {
	ctx := context.Background()

	t.Run("newest", func(t *testing.T) {
		// A real history: the base, a change, a commit that touches only
		// .walden/, and a commit that restores the base content.
		root, base := committedRepo(t)
		runner := shell.NewExecRunner()
		identity := func() string {
			id, ok := Identity(ctx, runner, root)
			if !ok {
				t.Fatal("the identity of the fixture cannot be captured")
			}
			return id
		}
		baseIdentity := identity()

		writeTreeFile(t, root, "a.txt", "changed\n", 0o644)
		historyGit(t, root, "commit", "-qam", "change")
		changedIdentity := identity()
		writeTreeFile(t, root, ".walden/evidence/feature.json", "{}\n", 0o644)
		historyGit(t, root, "add", "-A")
		historyGit(t, root, "commit", "-qm", "evidence only")
		evidenceOnly := historyGit(t, root, "rev-parse", "HEAD")
		if identity() != changedIdentity {
			t.Fatal("a commit under .walden/ moved the code identity")
		}
		writeTreeFile(t, root, "a.txt", "alpha\n", 0o644)
		historyGit(t, root, "commit", "-qam", "restore")
		restored := historyGit(t, root, "rev-parse", "HEAD")
		if identity() != baseIdentity || restored == base {
			t.Fatal("the fixture must end on the base content in a new commit")
		}

		search := NewTreeSearch(runner, root, SealSearchLimit)
		if commit, found := search.Commit(ctx, baseIdentity); !found || commit != restored {
			t.Errorf("base content: commit %q found=%t, want the newest commit that holds it, %s", commit, found, restored)
		}
		// Two commits hold the changed code: the one that changed it and the
		// one that touched only .walden/. The newest is named.
		if commit, found := search.Commit(ctx, changedIdentity); !found || commit != evidenceOnly {
			t.Errorf("changed content: commit %q found=%t, want %s", commit, found, evidenceOnly)
		}
		if commit, found := search.Commit(ctx, "sha256:never-committed"); found {
			t.Errorf("an identity no commit holds is found at %s", commit)
		}
		if search.Truncated() || search.Covered() != 4 {
			t.Errorf("covered %d commit(s), truncated=%t, want the four of the history", search.Covered(), search.Truncated())
		}
		if _, found := search.Commit(ctx, ""); found {
			t.Error("an empty identity is found")
		}

		// A small limit covers the newest commits only.
		short := NewTreeSearch(runner, root, 2)
		if _, found := short.Commit(ctx, "sha256:never-committed"); found || !short.Truncated() || short.Covered() != 2 {
			t.Errorf("limit 2: covered %d, truncated=%t", short.Covered(), short.Truncated())
		}
		if commit, found := short.Commit(ctx, changedIdentity); !found || commit != evidenceOnly {
			t.Errorf("limit 2: the second commit is not found: %q %t", commit, found)
		}
	})

	t.Run("one history listing", func(t *testing.T) {
		history := scriptedCommits(6)
		search := NewTreeSearch(history, "/repo", SealSearchLimit)
		if history.count("log")+history.count("ls-tree") != 0 {
			t.Fatalf("a search asks git before the first question: %v", history.calls)
		}

		// The second commit: two trees are listed, and no more.
		if commit, found := search.Commit(ctx, identityOfTree("tree-1")); !found || commit != commitID(1) {
			t.Fatalf("tree-1: %q %t", commit, found)
		}
		if history.count("ls-tree") != 2 {
			t.Errorf("after a match at the second commit %d tree(s) were listed, want two", history.count("ls-tree"))
		}
		// A commit already listed costs nothing.
		if commit, found := search.Commit(ctx, identityOfTree("tree-0")); !found || commit != commitID(0) || history.count("ls-tree") != 2 {
			t.Errorf("tree-0: %q %t after %d listing(s)", commit, found, history.count("ls-tree"))
		}
		// A later question continues where the search stopped.
		if commit, found := search.Commit(ctx, identityOfTree("tree-3")); !found || commit != commitID(3) || history.count("ls-tree") != 4 {
			t.Errorf("tree-3: %q %t after %d listing(s), want four", commit, found, history.count("ls-tree"))
		}
		// An identity no commit holds lists the rest, once.
		for attempt := 0; attempt < 2; attempt++ {
			if _, found := search.Commit(ctx, "sha256:absent"); found || history.count("ls-tree") != 6 {
				t.Errorf("absent identity, attempt %d: found=%t after %d listing(s), want six", attempt, found, history.count("ls-tree"))
			}
		}
		if history.count("log") != 1 {
			t.Errorf("the history was listed %d time(s), want once", history.count("log"))
		}
		for _, call := range history.calls {
			if call != "log" && call != "ls-tree" {
				t.Errorf("the search ran git %s", call)
			}
		}
		if search.Truncated() || search.Covered() != 6 {
			t.Errorf("covered %d, truncated=%t", search.Covered(), search.Truncated())
		}
	})

	t.Run("limit", func(t *testing.T) {
		// One commit more than the limit: the search asks for 201 ids, lists
		// 200 trees and says that it stopped.
		history := scriptedCommits(SealSearchLimit + 1)
		search := NewTreeSearch(history, "/repo", SealSearchLimit)
		beyond := identityOfTree(fmt.Sprintf("tree-%d", SealSearchLimit))
		if commit, found := search.Commit(ctx, beyond); found {
			t.Errorf("a tree beyond the limit is found at %s", commit)
		}
		if history.count("ls-tree") != SealSearchLimit || history.count("log") != 1 {
			t.Errorf("%d tree(s) and %d history listing(s), want %d and one", history.count("ls-tree"), history.count("log"), SealSearchLimit)
		}
		if len(history.requested) != 1 || history.requested[0] != fmt.Sprint(SealSearchLimit+1) {
			t.Errorf("the history listing asked for %v commit(s), want %d", history.requested, SealSearchLimit+1)
		}
		if !search.Truncated() || search.Covered() != SealSearchLimit {
			t.Errorf("covered %d, truncated=%t, want the limit reported", search.Covered(), search.Truncated())
		}
		// The last commit within the limit is found.
		if commit, found := search.Commit(ctx, identityOfTree(fmt.Sprintf("tree-%d", SealSearchLimit-1))); !found || commit != commitID(SealSearchLimit-1) {
			t.Errorf("the last commit within the limit: %q %t", commit, found)
		}

		// Exactly the limit: nothing is cut.
		exact := scriptedCommits(SealSearchLimit)
		search = NewTreeSearch(exact, "/repo", SealSearchLimit)
		if _, found := search.Commit(ctx, "sha256:absent"); found || search.Truncated() || search.Covered() != SealSearchLimit {
			t.Errorf("exactly the limit: covered %d, truncated=%t", search.Covered(), search.Truncated())
		}
	})

	t.Run("no history", func(t *testing.T) {
		history := scriptedCommits(3)
		history.failLog = true
		search := NewTreeSearch(history, "/repo", SealSearchLimit)
		if commit, found := search.Commit(ctx, identityOfTree("tree-0")); found {
			t.Errorf("without a readable history a commit is found: %s", commit)
		}
		if history.count("ls-tree") != 0 || search.Covered() != 0 || search.Truncated() {
			t.Errorf("listed %d tree(s), covered %d, truncated=%t", history.count("ls-tree"), search.Covered(), search.Truncated())
		}
		// The failure is not retried at every question.
		search.Commit(ctx, identityOfTree("tree-1"))
		if history.count("log") != 1 {
			t.Errorf("the history was listed %d time(s), want once", history.count("log"))
		}
	})
}
