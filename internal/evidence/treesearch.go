package evidence

import (
	"context"
	"strconv"
	"strings"

	"github.com/andrearaponi/walden/internal/shell"
)

// SealSearchLimit is the largest number of commits a search lists: the most
// recent ones reachable from HEAD.
const SealSearchLimit = 200

// TreeSearch finds the commit whose tree has a given code identity, among
// the most recent commits reachable from HEAD. It is how adoption establishes
// that a recorded pass ran on a tree a commit holds.
//
// The search compares code identities, the value a record is bound to, not
// git tree ids: a tree id includes .walden/, so two commits with the same
// code and different ledgers would not match.
//
// It is lazy and keeps what it learns. The history is listed once, at the
// first question. Trees are listed one commit at a time, newest first, and a
// question stops at the commit it was asked for; the next question continues
// from there. One search therefore serves every task and feature of a run
// and lists at most its limit of trees.
type TreeSearch struct {
	runner shell.Runner
	root   string
	limit  int

	loaded    bool
	commits   []string          // newest first, at most limit
	truncated bool              // the history holds more commits than limit
	next      int               // the first commit whose tree is not listed yet
	found     map[string]string // identity → the newest commit that has it
}

// NewTreeSearch prepares a search over at most limit commits. Nothing is
// asked of git until the first question.
func NewTreeSearch(runner shell.Runner, root string, limit int) *TreeSearch {
	return &TreeSearch{runner: runner, root: root, limit: limit, found: map[string]string{}}
}

// Commit returns the newest commit, among those the search covers, whose
// tree has the identity.
func (s *TreeSearch) Commit(ctx context.Context, identity string) (string, bool) {
	if identity == "" {
		return "", false
	}
	if commit, known := s.found[identity]; known {
		return commit, true
	}
	s.load(ctx)
	for s.next < len(s.commits) {
		commit := s.commits[s.next]
		s.next++
		response, err := Git(ctx, s.runner, s.root, "ls-tree", "-r", commit)
		if err != nil || response.ExitCode != 0 {
			// A commit whose tree cannot be read holds no tree to match.
			continue
		}
		listed := RebuildManifest(response.Stdout, nil).Digest()
		if _, known := s.found[listed]; !known {
			s.found[listed] = commit
		}
		if listed == identity {
			return commit, true
		}
	}
	return "", false
}

// load lists the history once. One id more than the limit is asked for, so
// the search can tell a history it covers whole from one it stops in.
func (s *TreeSearch) load(ctx context.Context) {
	if s.loaded {
		return
	}
	s.loaded = true
	response, err := Git(ctx, s.runner, s.root, "log", "--format=%H", "-n", strconv.Itoa(s.limit+1), "HEAD")
	if err != nil || response.ExitCode != 0 {
		return
	}
	for _, line := range strings.Split(response.Stdout, "\n") {
		// The ids come from git, and only an object id goes back to it.
		if id := strings.TrimSpace(line); objectID(id) {
			s.commits = append(s.commits, id)
		}
	}
	if len(s.commits) > s.limit {
		s.commits, s.truncated = s.commits[:s.limit], true
	}
}

// Covered returns how many commits the search covers: the limit, or fewer
// when the history is shorter. It is zero before the first question and when
// the history cannot be read.
func (s *TreeSearch) Covered() int {
	return len(s.commits)
}

// Truncated reports that the history holds more commits than the search
// covers: a tree it did not find may still be in an earlier commit.
func (s *TreeSearch) Truncated() bool {
	return s.truncated
}
