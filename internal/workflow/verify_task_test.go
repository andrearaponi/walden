package workflow

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/testutil"
)

const (
	verifyTaskFeature = "todo-app-demo"
	identityBefore    = "100644 blob aaa\tmain.go\n"
	identityAfter     = "100644 blob bbb\tmain.go\n"
	identityLater     = "100644 blob ccc\tmain.go\n"
)

// writeLeafPlan writes an approved plan of count top-level leaves, each with a
// command of its own, so a test can tell whose proof ran.
func writeLeafPlan(t *testing.T, root string, count int) {
	t.Helper()
	writeVerifyFixture(t, root)

	var plan strings.Builder
	plan.WriteString(`---
status: approved
approved_at: 2026-03-21T14:20:00Z
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at: 2026-03-21T14:10:00Z
---

# Implementation Plan

`)
	for id := 1; id <= count; id++ {
		fmt.Fprintf(&plan, "- [ ] %d. Task %d\n  - Requirements: `R1`\n  - Design: Task Store\n  - Verification:\n    - command: [\"go\", \"test\", \"./pkg%d\"]\n", id, id, id)
	}
	writeFreshFeatureDoc(t, root, verifyTaskFeature, "tasks.md", plan.String())
}

func passingRunner(count int) *testutil.FakeRunner {
	return testutil.NewFakeRunner(slices.Repeat([]testutil.Response{{Stdout: "ok", ExitCode: 0}}, count)...)
}

// completedLeaves returns a repository whose count tasks were completed under
// identityBefore. The identity runner stays at identityBefore.
func completedLeaves(t *testing.T, count int) string {
	t.Helper()
	root := t.TempDir()
	writeLeafPlan(t, root, count)
	overrideIdentityRunner(t, identityYielding(identityBefore))
	if _, err := CompleteAllTasks(context.Background(), root, verifyTaskFeature, passingRunner(count)); err != nil {
		t.Fatalf("complete fixture tasks: %v", err)
	}
	return root
}

// staleLeaves returns a repository of count completed tasks, all stale-code.
func staleLeaves(t *testing.T, count int) string {
	t.Helper()
	root := completedLeaves(t, count)
	overrideIdentityRunner(t, identityYielding(identityAfter))
	return root
}

func loadLeafLedger(t *testing.T, root string) evidence.Document {
	t.Helper()
	ledger, err := evidence.Load(root, verifyTaskFeature)
	if err != nil {
		t.Fatalf("load ledger: %v", err)
	}
	return ledger
}

// proofTargets lists the package each recorded proof ran, in order.
func proofTargets(runner *testutil.FakeRunner) []string {
	targets := []string{}
	for _, call := range runner.Calls() {
		targets = append(targets, call.Args[len(call.Args)-1])
	}
	return targets
}

func assertOneOutcome(t *testing.T, result VerifyResult, id string) VerifyOutcome {
	t.Helper()
	if len(result.Outcomes) != 1 || result.Outcomes[0].TaskID != id {
		t.Fatalf("outcomes = %+v, want task %s alone", result.Outcomes, id)
	}
	return result.Outcomes[0]
}

func TestVerifyTaskRunsOnlySelectedProof(t *testing.T) {
	root := staleLeaves(t, 3)
	runner := passingRunner(1)

	result, err := VerifyTask(context.Background(), root, verifyTaskFeature, "2", false, runner)
	if err != nil {
		t.Fatalf("VerifyTask: %v", err)
	}

	if targets := proofTargets(runner); !slices.Equal(targets, []string{"./pkg2"}) {
		t.Fatalf("proofs run = %v, want only ./pkg2", targets)
	}
	if result.Task != "2" {
		t.Fatalf("result.Task = %q, want 2", result.Task)
	}
	outcome := assertOneOutcome(t, result, "2")
	if !outcome.Passed || outcome.State != evidence.StateVerified {
		t.Fatalf("outcome = %+v, want passed and verified", outcome)
	}

	// Control: the same feature under a plain verify runs every stale proof.
	control := staleLeaves(t, 3)
	plain := passingRunner(3)
	if _, err := Verify(context.Background(), control, verifyTaskFeature, false, false, plain); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if targets := proofTargets(plain); !slices.Equal(targets, []string{"./pkg1", "./pkg2", "./pkg3"}) {
		t.Fatalf("plain verify ran %v, want all three", targets)
	}
}

func TestVerifyTaskRunsVerifiedTask(t *testing.T) {
	root := completedLeaves(t, 3)

	// Control: every task is verified, so a plain verify runs nothing.
	plain := testutil.NewFakeRunner()
	if _, err := Verify(context.Background(), root, verifyTaskFeature, false, false, plain); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if calls := plain.Calls(); len(calls) != 0 {
		t.Fatalf("plain verify ran %v on verified tasks", calls)
	}

	runner := passingRunner(1)
	result, err := VerifyTask(context.Background(), root, verifyTaskFeature, "2", false, runner)
	if err != nil {
		t.Fatalf("VerifyTask: %v", err)
	}
	if targets := proofTargets(runner); !slices.Equal(targets, []string{"./pkg2"}) {
		t.Fatalf("proofs run = %v, want ./pkg2", targets)
	}
	if outcome := assertOneOutcome(t, result, "2"); outcome.State != evidence.StateVerified {
		t.Fatalf("state = %s, want verified", outcome.State)
	}
}

func TestVerifyTaskCheckPersistsNothing(t *testing.T) {
	root := staleLeaves(t, 3)
	path := evidence.DocumentPath(root, verifyTaskFeature)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}

	result, err := VerifyTask(context.Background(), root, verifyTaskFeature, "2", true, passingRunner(1))
	if err != nil {
		t.Fatalf("VerifyTask: %v", err)
	}

	if !result.Checked {
		t.Fatal("result does not report check mode")
	}
	if outcome := assertOneOutcome(t, result, "2"); !outcome.Passed {
		t.Fatalf("outcome = %+v, want passed", outcome)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("check mode changed the ledger:\nbefore: %s\n after: %s", before, after)
	}
}

func TestVerifyTaskLeavesOtherRecords(t *testing.T) {
	root := completedLeaves(t, 3)

	// Task 1 ends failed, then a later change leaves tasks 2 and 3 stale.
	overrideIdentityRunner(t, identityYielding(identityAfter))
	mixed := testutil.NewFakeRunner(
		testutil.Response{Stderr: "boom", ExitCode: 1},
		testutil.Response{Stdout: "ok", ExitCode: 0},
		testutil.Response{Stdout: "ok", ExitCode: 0},
	)
	if _, err := Verify(context.Background(), root, verifyTaskFeature, false, false, mixed); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	overrideIdentityRunner(t, identityYielding(identityLater))

	before := loadLeafLedger(t, root)
	if before.Tasks["1"].Result != evidence.ResultFailed {
		t.Fatalf("fixture task 1 = %s, want failed", before.Tasks["1"].Result)
	}

	if _, err := VerifyTask(context.Background(), root, verifyTaskFeature, "2", false, passingRunner(1)); err != nil {
		t.Fatalf("VerifyTask: %v", err)
	}

	after := loadLeafLedger(t, root)
	for _, id := range []string{"1", "3"} {
		if !reflect.DeepEqual(before.Tasks[id], after.Tasks[id]) {
			t.Fatalf("record %s changed:\nbefore: %+v\n after: %+v", id, before.Tasks[id], after.Tasks[id])
		}
	}
	if before.Tasks["2"].CodeIdentity == after.Tasks["2"].CodeIdentity {
		t.Fatal("the selected record was not refreshed, so this test observed nothing")
	}
}

func TestVerifyTaskKeepsOrphans(t *testing.T) {
	root := staleLeaves(t, 3)
	ledger := loadLeafLedger(t, root)
	ledger.Tasks["9.9"] = ledger.Tasks["1"]
	if err := evidence.Save(root, ledger); err != nil {
		t.Fatalf("seed orphan: %v", err)
	}

	result, err := VerifyTask(context.Background(), root, verifyTaskFeature, "2", false, passingRunner(1))
	if err != nil {
		t.Fatalf("VerifyTask: %v", err)
	}
	if len(result.Pruned) != 0 {
		t.Fatalf("pruned = %v, want none", result.Pruned)
	}
	if _, exists := loadLeafLedger(t, root).Tasks["9.9"]; !exists {
		t.Fatal("a selected run removed the orphan")
	}

	// A feature-level run keeps pruning.
	if _, err := Verify(context.Background(), root, verifyTaskFeature, false, false, passingRunner(2)); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if _, exists := loadLeafLedger(t, root).Tasks["9.9"]; exists {
		t.Fatal("the orphan survived a feature-level run")
	}
}

func TestVerifyTaskMutatingProofFails(t *testing.T) {
	root := t.TempDir()
	writeLeafPlan(t, root, 3)
	overrideIdentityRunner(t, &dynamicIdentityRunner{root: root})
	if _, err := CompleteAllTasks(context.Background(), root, verifyTaskFeature, passingRunner(3)); err != nil {
		t.Fatalf("complete fixture tasks: %v", err)
	}

	result, err := VerifyTask(context.Background(), root, verifyTaskFeature, "2", false, &sideEffectRunner{root: root})
	if err != nil {
		t.Fatalf("VerifyTask: %v", err)
	}

	outcome := assertOneOutcome(t, result, "2")
	if outcome.Passed || outcome.State != evidence.StateFailed {
		t.Fatalf("outcome = %+v, want a failure", outcome)
	}
	for _, want := range []string{"working tree changed while task 2 proof ran", "generated.txt"} {
		if !strings.Contains(outcome.Failure, want) {
			t.Fatalf("failure %q does not hold %q", outcome.Failure, want)
		}
	}
	if record := loadLeafLedger(t, root).Tasks["2"]; record.Result != evidence.ResultFailed {
		t.Fatalf("recorded %s, want failed", record.Result)
	}
}

func TestVerifyTaskRecordMatchesVerify(t *testing.T) {
	selectedRoot := completedLeaves(t, 3)
	plainRoot := completedLeaves(t, 3)
	overrideIdentityRunner(t, identityYielding(identityAfter))

	if _, err := VerifyTask(context.Background(), selectedRoot, verifyTaskFeature, "2", false, passingRunner(1)); err != nil {
		t.Fatalf("VerifyTask: %v", err)
	}
	if _, err := Verify(context.Background(), plainRoot, verifyTaskFeature, false, false, passingRunner(3)); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	selected, plain := loadLeafLedger(t, selectedRoot).Tasks["2"], loadLeafLedger(t, plainRoot).Tasks["2"]
	selected.VerifiedAt, plain.VerifiedAt = "", ""
	if !reflect.DeepEqual(selected, plain) {
		t.Fatalf("records differ:\nselected: %+v\n   plain: %+v", selected, plain)
	}
}

func TestVerifyTaskReportsOpenTasks(t *testing.T) {
	t.Run("twelve other stale tasks", func(t *testing.T) {
		root := staleLeaves(t, 13)

		result, err := VerifyTask(context.Background(), root, verifyTaskFeature, "1", false, passingRunner(1))
		if err != nil {
			t.Fatalf("VerifyTask: %v", err)
		}

		want := []string{"2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13"}
		if !slices.Equal(result.Open, want) {
			t.Fatalf("open = %v, want %v in plan order", result.Open, want)
		}
		warning := "12 other completed task(s) not verified: 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, +2 more"
		if !slices.Contains(result.Warnings, warning) {
			t.Fatalf("warnings = %q, want %q", result.Warnings, warning)
		}
	})

	t.Run("no other open task", func(t *testing.T) {
		root := completedLeaves(t, 3)

		result, err := VerifyTask(context.Background(), root, verifyTaskFeature, "2", false, passingRunner(1))
		if err != nil {
			t.Fatalf("VerifyTask: %v", err)
		}
		if len(result.Open) != 0 || len(result.Warnings) != 0 {
			t.Fatalf("open = %v, warnings = %q, want neither", result.Open, result.Warnings)
		}
	})

	t.Run("tasks a mutating proof staled", func(t *testing.T) {
		root := t.TempDir()
		writeLeafPlan(t, root, 3)
		overrideIdentityRunner(t, &dynamicIdentityRunner{root: root})
		if _, err := CompleteAllTasks(context.Background(), root, verifyTaskFeature, passingRunner(3)); err != nil {
			t.Fatalf("complete fixture tasks: %v", err)
		}

		result, err := VerifyTask(context.Background(), root, verifyTaskFeature, "2", false, &sideEffectRunner{root: root})
		if err != nil {
			t.Fatalf("VerifyTask: %v", err)
		}
		if want := []string{"1", "3"}; !slices.Equal(result.Open, want) {
			t.Fatalf("open = %v, want %v", result.Open, want)
		}
	})

	t.Run("plain verify reports none", func(t *testing.T) {
		root := staleLeaves(t, 3)
		failing := testutil.NewFakeRunner(testutil.Response{Stderr: "boom", ExitCode: 1}, testutil.Response{Stdout: "ok"}, testutil.Response{Stdout: "ok"})

		result, err := Verify(context.Background(), root, verifyTaskFeature, false, false, failing)
		if err != nil {
			t.Fatalf("Verify: %v", err)
		}
		if result.Task != "" || len(result.Open) != 0 {
			t.Fatalf("task = %q, open = %v, want neither", result.Task, result.Open)
		}
	})
}
