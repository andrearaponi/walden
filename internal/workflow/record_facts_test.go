package workflow

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/spec"
	"github.com/andrearaponi/walden/internal/testutil"
)

// factsGit runs git in a fixture repository.
func factsGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	data, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v (%s)", args, err, data)
	}
	return strings.TrimSpace(string(data))
}

// factsRepo builds a temporary git repository that reads no global or system
// configuration, so the machine's commit signing, hooks, file-system monitor
// and line-ending conversion cannot reach the test. The identity runner is
// the real one here, and it inherits the same environment. Tests that use
// this helper set process environment and are not parallel.
func factsRepo(t *testing.T) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	root := t.TempDir()
	factsGit(t, root, "init", "-q", "-b", "main")
	factsGit(t, root, "config", "user.email", "fixture@walden.test")
	factsGit(t, root, "config", "user.name", "Fixture")
	return root
}

func factsFile(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// factsCommittedRepo holds one commit of three files and the feature fixture.
func factsCommittedRepo(t *testing.T) (root, head string) {
	t.Helper()
	root = factsRepo(t)
	factsFile(t, root, "main.go", "package main\n")
	factsFile(t, root, "old.go", "package main // old\n")
	factsFile(t, root, ".gitignore", "*.log\n")
	factsGit(t, root, "add", ".")
	factsGit(t, root, "commit", "-qm", "base")
	writeVerifyFixture(t, root)
	return root, factsGit(t, root, "rev-parse", "HEAD")
}

func factsLedger(t *testing.T, root string) evidence.Document {
	t.Helper()
	ledger, err := evidence.Load(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("load evidence: %v", err)
	}
	return ledger
}

// factsVerifyAll re-proves both fixture tasks with passing proofs.
func factsVerifyAll(t *testing.T, root string) {
	t.Helper()
	runner := testutil.NewFakeRunner(
		testutil.Response{Stdout: "ok", ExitCode: 0},
		testutil.Response{Stdout: "ok", ExitCode: 0},
	)
	result, err := Verify(context.Background(), root, "todo-app-demo", true, false, runner)
	if err != nil || len(result.Failed) != 0 || len(result.Outcomes) != 2 {
		t.Fatalf("verify --all: %v, %+v", err, result)
	}
}

func TestRecordTreeFacts(t *testing.T) {
	ids := []string{"1.1", "1.2"}

	t.Run("commit", func(t *testing.T) {
		root, head := factsCommittedRepo(t)

		completeBoth(t, root)
		ledger := factsLedger(t, root)
		for _, id := range ids {
			record := ledger.Tasks[id]
			if record.Execution == nil || record.Execution.Origin != "complete" {
				t.Fatalf("task %s: not a completion record: %+v", id, record.Execution)
			}
			if record.Tree == nil || record.Tree.Commit != head {
				t.Fatalf("task %s completed at HEAD %s, record names %+v", id, head, record.Tree)
			}
			if len(record.Tree.Paths) != 0 || record.Tree.PathsOmitted != 0 {
				t.Fatalf("task %s: a clean tree has no differing path: %+v", id, record.Tree)
			}
		}

		factsVerifyAll(t, root)
		ledger = factsLedger(t, root)
		for _, id := range ids {
			record := ledger.Tasks[id]
			if record.Execution == nil || record.Execution.Origin != "verify" {
				t.Fatalf("task %s: not a verify record: %+v", id, record.Execution)
			}
			if record.Tree == nil || record.Tree.Commit != head {
				t.Fatalf("task %s verified at HEAD %s, record names %+v", id, head, record.Tree)
			}
		}
	})

	t.Run("differing-paths", func(t *testing.T) {
		root, head := factsCommittedRepo(t)
		factsFile(t, root, "main.go", "package main // edited\n")
		factsFile(t, root, "notes.txt", "notes\n")
		if err := os.Remove(filepath.Join(root, "old.go")); err != nil {
			t.Fatal(err)
		}
		factsFile(t, root, "debug.log", "ignored\n")

		// Git itself is the oracle of a content identity.
		want := map[string]string{
			"main.go":   "100644:" + factsGit(t, root, "hash-object", "--", "main.go"),
			"notes.txt": "100644:" + factsGit(t, root, "hash-object", "--", "notes.txt"),
			"old.go":    "",
		}

		completeBoth(t, root)
		factsVerifyAll(t, root)
		ledger := factsLedger(t, root)
		for _, id := range ids {
			tree := ledger.Tasks[id].Tree
			if tree == nil || tree.Commit != head {
				t.Fatalf("task %s: tree facts %+v, want commit %s", id, tree, head)
			}
			if !reflect.DeepEqual(tree.Paths, want) {
				t.Fatalf("task %s: differing paths %v, want %v", id, tree.Paths, want)
			}
			for path := range tree.Paths {
				if path == "debug.log" || strings.HasPrefix(path, ".walden/") {
					t.Fatalf("task %s: %s is outside the identity and must not be recorded", id, path)
				}
			}
		}
	})

	t.Run("recording-limit", func(t *testing.T) {
		root, head := factsCommittedRepo(t)
		for i := 0; i < evidence.RecordingLimit; i++ {
			factsFile(t, root, fmt.Sprintf("gen/f%03d.txt", i), fmt.Sprintf("%d\n", i))
		}

		completeBoth(t, root)
		ledger := factsLedger(t, root)
		for _, id := range ids {
			tree := ledger.Tasks[id].Tree
			if tree == nil || len(tree.Paths) != evidence.RecordingLimit || tree.PathsOmitted != 0 {
				t.Fatalf("task %s: at the limit every path is kept, got %d paths and %d omitted", id, len(treePaths(tree)), treeOmitted(tree))
			}
		}

		factsFile(t, root, "gen/one-more.txt", "over\n")
		factsVerifyAll(t, root)
		ledger = factsLedger(t, root)
		for _, id := range ids {
			tree := ledger.Tasks[id].Tree
			if tree == nil || tree.Commit != head || tree.Paths != nil || tree.PathsOmitted != evidence.RecordingLimit+1 {
				t.Fatalf("task %s: over the limit the record keeps the count alone, got %d paths and %d omitted", id, len(treePaths(tree)), treeOmitted(tree))
			}
		}
	})

	t.Run("no-commit", func(t *testing.T) {
		root := factsRepo(t)
		factsFile(t, root, "main.go", "package main\n")
		writeVerifyFixture(t, root)

		completeBoth(t, root)
		ledger := factsLedger(t, root)
		for _, id := range ids {
			record := ledger.Tasks[id]
			if record.Tree != nil {
				t.Fatalf("task %s: no commit to record, got %+v", id, record.Tree)
			}
			if record.Result != evidence.ResultPassed || !strings.HasPrefix(record.CodeIdentity, "sha256:") {
				t.Fatalf("task %s: the record must keep its result and identity: %+v", id, record)
			}
		}
	})

	t.Run("no-git", func(t *testing.T) {
		root := t.TempDir()
		t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
		t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
		// Keep git from finding a repository above the temporary directory.
		t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))
		factsFile(t, root, "main.go", "package main\n")
		writeVerifyFixture(t, root)

		completeBoth(t, root)
		ledger := factsLedger(t, root)
		for _, id := range ids {
			record := ledger.Tasks[id]
			if record.Tree != nil || record.CodeIdentity != "" {
				t.Fatalf("task %s: without git the record carries neither fact: %+v", id, record)
			}
			if record.Result != evidence.ResultPassed {
				t.Fatalf("task %s: completion must succeed as today: %+v", id, record)
			}
		}
	})
}

func treePaths(tree *evidence.TreeFacts) map[string]string {
	if tree == nil {
		return nil
	}
	return tree.Paths
}

func treeOmitted(tree *evidence.TreeFacts) int {
	if tree == nil {
		return -1
	}
	return tree.PathsOmitted
}

// writeStatementFixture is a feature whose requirements define statements
// and whose tasks name them in the two places a task can: its Requirements
// line and the covers of a proof step.
func writeStatementFixture(t *testing.T, root string) {
	t.Helper()
	writeFreshFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document

### R1 Parser

#### Acceptance Criteria

1. `+"`R1.AC1`"+` WHEN input arrives, the system SHALL parse it.
   - Acceptance check: a parsed tree is returned.
2. `+"`R1.AC2`"+` IF input is empty, THEN the system SHALL reject it.
   - Acceptance check: an error names the empty input.

## Non-Functional Requirements

- `+"`NFR1`"+` Parsing stays deterministic.
`)
	writeFreshFeatureDoc(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-21T14:10:00Z
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at: 2026-03-21T14:00:00Z
---

# Feature Design
`)
	writeFreshFeatureDoc(t, root, "todo-app-demo", "tasks.md", `---
status: approved
approved_at: 2026-03-21T14:20:00Z
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at: 2026-03-21T14:10:00Z
---

# Implementation Plan

- [ ] 1. Build parser
  - [ ] 1.1 Implement parser
    - Requirements: `+"`R1`"+`, `+"`R1.AC1`"+`
    - Design: Task Store
    - Verification:
      - command: ["go", "test", "./internal/spec"]
        covers: ["R1.AC2"]
  - [ ] 1.2 Implement readiness
    - Requirements: `+"`NFR1`"+`
    - Design: Execution Service
    - Verification:
      - command: ["go", "test", "./internal/workflow"]
`)
}

func TestRecordStatementFingerprints(t *testing.T) {
	root := t.TempDir()
	writeStatementFixture(t, root)
	overrideIdentityRunner(t, identityYielding("100644 blob aaa\tmain.go\n"))

	// The blocks are written out here, not extracted: what a fingerprint
	// covers is stated by the test.
	want := map[string]map[string]string{
		"1.1": {
			"R1.AC1": spec.StatementFingerprint("WHEN input arrives, the system SHALL parse it.\n- Acceptance check: a parsed tree is returned."),
			"R1.AC2": spec.StatementFingerprint("IF input is empty, THEN the system SHALL reject it.\n- Acceptance check: an error names the empty input."),
		},
		"1.2": {
			"NFR1": spec.StatementFingerprint("Parsing stays deterministic."),
		},
	}

	check := func(t *testing.T, origin string) {
		t.Helper()
		ledger := factsLedger(t, root)
		for id, fingerprints := range want {
			record := ledger.Tasks[id]
			if record.Execution == nil || record.Execution.Origin != origin {
				t.Fatalf("task %s: not a %s record: %+v", id, origin, record.Execution)
			}
			if record.Statements == nil {
				t.Fatalf("task %s: the %s record carries no statement fingerprints", id, origin)
			}
			if record.Statements.Scheme != spec.StatementScheme {
				t.Fatalf("task %s: scheme %q, want %q", id, record.Statements.Scheme, spec.StatementScheme)
			}
			if !reflect.DeepEqual(record.Statements.Fingerprints, fingerprints) {
				t.Fatalf("task %s: fingerprints %v, want %v", id, record.Statements.Fingerprints, fingerprints)
			}
		}
	}

	t.Run("requirements-line-and-covers", func(t *testing.T) {
		// 1.1 names R1.AC1 in its Requirements line and R1.AC2 only in a
		// covers attribute; the heading R1 names no statement.
		completeBoth(t, root)
		check(t, "complete")
	})

	t.Run("verify", func(t *testing.T) {
		factsVerifyAll(t, root)
		check(t, "verify")
	})
}

// gitCallRecorder runs real git and keeps the arguments of each call after
// the prefix every call carries (--no-optional-locks -C <root>).
type gitCallRecorder struct{ calls [][]string }

func (r *gitCallRecorder) Run(ctx context.Context, name string, args ...string) (shell.Response, error) {
	if len(args) > 3 {
		r.calls = append(r.calls, append([]string(nil), args[3:]...))
	}
	return shell.NewExecRunner().Run(ctx, name, args...)
}

// listings returns the revision of every tree listing, in call order.
func (r *gitCallRecorder) listings() []string {
	var revisions []string
	for _, call := range r.calls {
		if call[0] == "ls-tree" {
			revisions = append(revisions, call[len(call)-1])
		}
	}
	return revisions
}

// The explanations ask git for the tree of each distinct recorded commit
// once, however many tasks were recorded at it.
func TestEvidenceReportListsEachRecordedCommitOnce(t *testing.T) {
	ctx := context.Background()
	root, first := factsCommittedRepo(t)
	completeBoth(t, root)

	explained := func(t *testing.T, reading EvidenceReading) {
		t.Helper()
		for _, entry := range reading.Entries {
			if entry.State != evidence.StateStaleCode || entry.Explanation == nil || entry.Explanation.Code == nil || entry.Explanation.Code.Unavailable != "" {
				t.Fatalf("task %s (%s) is not explained: %+v", entry.TaskID, entry.State, entry.Explanation)
			}
		}
	}

	t.Run("two-tasks-one-commit", func(t *testing.T) {
		factsFile(t, root, "notes.txt", "notes\n")
		recorder := &gitCallRecorder{}
		overrideIdentityRunner(t, recorder)

		reading, err := ReadEvidence(ctx, root, "todo-app-demo")
		if err != nil {
			t.Fatal(err)
		}
		explained(t, reading)
		// One listing for the current tree, one for the commit both tasks
		// were recorded at.
		if want := []string{first, first}; !reflect.DeepEqual(recorder.listings(), want) {
			t.Fatalf("listings = %v, want %v", recorder.listings(), want)
		}
	})

	t.Run("two-tasks-two-commits", func(t *testing.T) {
		// Task 1.2 is recorded again at a second commit; 1.1 stays at the first.
		factsGit(t, root, "add", "-A")
		factsGit(t, root, "commit", "-qm", "second")
		second := factsGit(t, root, "rev-parse", "HEAD")
		if _, err := VerifyTask(ctx, root, "todo-app-demo", "1.2", false, testutil.NewFakeRunner(testutil.Response{Stdout: "ok"})); err != nil {
			t.Fatal(err)
		}
		factsFile(t, root, "more.txt", "more\n")

		recorder := &gitCallRecorder{}
		overrideIdentityRunner(t, recorder)
		reading, err := ReadEvidence(ctx, root, "todo-app-demo")
		if err != nil {
			t.Fatal(err)
		}
		explained(t, reading)
		// One listing for the current tree and one for each of the two
		// recorded commits.
		if want := []string{second, first, second}; !reflect.DeepEqual(recorder.listings(), want) {
			t.Fatalf("listings = %v, want %v", recorder.listings(), want)
		}
	})
}
