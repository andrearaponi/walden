package workflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/testutil"
)

func writeVerifyFixture(t *testing.T, root string) {
	t.Helper()
	writeFreshFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
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
    - Requirements: `+"`R1`"+`
    - Design: Task Store
    - Verification:
      - command: ["go", "test", "./internal/spec"]
  - [ ] 1.2 Implement readiness
    - Requirements: `+"`R1`"+`
    - Design: Execution Service
    - Verification:
      - command: ["go", "test", "./internal/workflow"]
`)
}

func identityYielding(listing string) shell.Runner {
	return &scriptedIdentityRunner{
		statusResponse: shell.Response{ExitCode: 0},
		lsTreeResponse: shell.Response{ExitCode: 0, Stdout: listing},
	}
}

// completeBoth completes both fixture tasks under the current identity runner.
func completeBoth(t *testing.T, root string) {
	t.Helper()
	runner := testutil.NewFakeRunner(
		testutil.Response{Stdout: "ok", ExitCode: 0},
		testutil.Response{Stdout: "ok", ExitCode: 0},
	)
	if _, err := CompleteAllTasks(context.Background(), root, "todo-app-demo", runner); err != nil {
		t.Fatalf("complete fixture tasks: %v", err)
	}
}

func TestVerifyDefaultSkipsVerifiedTasks(t *testing.T) {
	root := t.TempDir()
	writeVerifyFixture(t, root)
	overrideIdentityRunner(t, identityYielding("100644 blob aaa\tmain.go\n"))
	completeBoth(t, root)

	runner := testutil.NewFakeRunner()
	result, err := Verify(context.Background(), root, "todo-app-demo", false, false, runner)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if len(runner.Calls()) != 0 {
		t.Fatalf("verified tasks were re-executed: %v", runner.Calls())
	}
	if len(result.Skipped) != 2 || len(result.Outcomes) != 0 {
		t.Fatalf("result = %+v, want both tasks skipped", result)
	}
}

func TestVerifyRefreshesStaleCode(t *testing.T) {
	root := t.TempDir()
	writeVerifyFixture(t, root)
	overrideIdentityRunner(t, identityYielding("100644 blob aaa\tmain.go\n"))
	completeBoth(t, root)

	// The code moves: identity changes, both tasks go stale-code.
	overrideIdentityRunner(t, identityYielding("100644 blob bbb\tmain.go\n"))

	runner := testutil.NewFakeRunner(
		testutil.Response{Stdout: "ok", ExitCode: 0},
		testutil.Response{Stdout: "ok", ExitCode: 0},
	)
	result, err := Verify(context.Background(), root, "todo-app-demo", false, false, runner)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if len(result.Outcomes) != 2 || len(result.Failed) != 0 {
		t.Fatalf("result = %+v, want two passing re-executions", result)
	}

	ledger, err := evidence.Load(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("load evidence: %v", err)
	}
	for _, id := range []string{"1.1", "1.2"} {
		if !strings.HasPrefix(ledger.Tasks[id].CodeIdentity, "sha256:") {
			t.Fatalf("task %s identity not refreshed", id)
		}
	}

	// A second default verify now has nothing to do.
	idle := testutil.NewFakeRunner()
	again, err := Verify(context.Background(), root, "todo-app-demo", false, false, idle)
	if err != nil {
		t.Fatalf("second Verify: %v", err)
	}
	if len(idle.Calls()) != 0 || len(again.Skipped) != 2 {
		t.Fatalf("refresh did not restore verified: %+v", again)
	}
}

func TestVerifyAllReexecutesVerifiedTasks(t *testing.T) {
	root := t.TempDir()
	writeVerifyFixture(t, root)
	overrideIdentityRunner(t, identityYielding("100644 blob aaa\tmain.go\n"))
	completeBoth(t, root)

	runner := testutil.NewFakeRunner(
		testutil.Response{Stdout: "ok", ExitCode: 0},
		testutil.Response{Stdout: "ok", ExitCode: 0},
	)
	result, err := Verify(context.Background(), root, "todo-app-demo", true, false, runner)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if len(runner.Calls()) != 2 || len(result.Outcomes) != 2 {
		t.Fatalf("--all did not re-execute everything: %+v", result)
	}
}

func TestVerifyCheckPersistsNothing(t *testing.T) {
	root := t.TempDir()
	writeVerifyFixture(t, root)
	overrideIdentityRunner(t, identityYielding("100644 blob aaa\tmain.go\n"))
	completeBoth(t, root)
	overrideIdentityRunner(t, identityYielding("100644 blob bbb\tmain.go\n"))

	before, err := os.ReadFile(evidence.DocumentPath(root, "todo-app-demo"))
	if err != nil {
		t.Fatalf("read ledger before: %v", err)
	}

	runner := testutil.NewFakeRunner(
		testutil.Response{Stdout: "ok", ExitCode: 0},
		testutil.Response{Stdout: "ok", ExitCode: 0},
	)
	result, err := Verify(context.Background(), root, "todo-app-demo", false, true, runner)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if !result.Checked || len(result.Outcomes) != 2 {
		t.Fatalf("check mode outcome = %+v", result)
	}

	after, err := os.ReadFile(evidence.DocumentPath(root, "todo-app-demo"))
	if err != nil {
		t.Fatalf("read ledger after: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("check mode modified the evidence document")
	}
}

func TestVerifyCollectsFailuresAcrossTasks(t *testing.T) {
	root := t.TempDir()
	writeVerifyFixture(t, root)
	overrideIdentityRunner(t, identityYielding("100644 blob aaa\tmain.go\n"))
	completeBoth(t, root)
	overrideIdentityRunner(t, identityYielding("100644 blob bbb\tmain.go\n"))

	runner := testutil.NewFakeRunner(
		testutil.Response{Stdout: "ok", ExitCode: 0},
		testutil.Response{Stderr: "assertion blew up", ExitCode: 1},
	)
	result, err := Verify(context.Background(), root, "todo-app-demo", false, false, runner)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if len(result.Failed) != 1 || result.Failed[0] != "1.2" {
		t.Fatalf("failed = %v, want [1.2]", result.Failed)
	}

	ledger, err := evidence.Load(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("load evidence: %v", err)
	}
	if ledger.Tasks["1.1"].Result != evidence.ResultPassed {
		t.Fatal("passing task not refreshed as passed")
	}
	if ledger.Tasks["1.2"].Result != evidence.ResultFailed {
		t.Fatal("failing task not recorded as failed")
	}
}

func TestVerifyBlockedByStaleChain(t *testing.T) {
	root := t.TempDir()
	writeVerifyFixture(t, root)
	overrideIdentityRunner(t, identityYielding("100644 blob aaa\tmain.go\n"))
	completeBoth(t, root)

	overrideFrontmatterField(t, root, "todo-app-demo", "requirements.md", "approved_fingerprint", "sha256:0000000000000000000000000000000000000000000000000000000000000000")

	_, err := Verify(context.Background(), root, "todo-app-demo", false, false, testutil.NewFakeRunner())
	if err == nil {
		t.Fatal("Verify accepted a stale chain")
	}
}

// sideEffectRunner passes every proof while mutating a tracked-visible file,
// simulating build/generate proofs that regenerate artifacts.
type sideEffectRunner struct {
	root  string
	inner shell.Runner
}

func (r *sideEffectRunner) Run(ctx context.Context, name string, args ...string) (shell.Response, error) {
	_ = os.WriteFile(r.root+"/generated.txt", []byte(name+strings.Join(args, " ")), 0o644)
	return shell.Response{Stdout: "ok", ExitCode: 0}, nil
}

// dynamicIdentityRunner derives porcelain output from the real filesystem so
// the identity truly changes when a proof regenerates a file.
type dynamicIdentityRunner struct{ root string }

func (r *dynamicIdentityRunner) Run(_ context.Context, _ string, args ...string) (shell.Response, error) {
	if len(args) >= 3 && args[2] == "ls-tree" {
		return shell.Response{ExitCode: 0, Stdout: "100644 blob aaa\tmain.go\n"}, nil
	}
	if len(args) >= 3 && args[2] == "hash-object" {
		content, err := os.ReadFile(r.root + "/" + args[len(args)-1])
		if err != nil {
			return shell.Response{ExitCode: 128, Stderr: err.Error()}, nil
		}
		return shell.Response{ExitCode: 0, Stdout: testDigest(content) + "\n"}, nil
	}
	// status: report generated.txt as untracked when it exists.
	if _, err := os.Stat(r.root + "/generated.txt"); err == nil {
		return shell.Response{ExitCode: 0, Stdout: "?? generated.txt\x00"}, nil
	}
	return shell.Response{ExitCode: 0}, nil
}

func testDigest(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func TestVerifyRecordsPostProofIdentity(t *testing.T) {
	root := t.TempDir()
	writeVerifyFixture(t, root)
	overrideIdentityRunner(t, &dynamicIdentityRunner{root: root})
	completeBoth(t, root)

	// The proofs regenerate a tracked-visible artifact: the identity at the
	// start of the run is not the identity of the tree the proofs leave.
	if _, err := Verify(context.Background(), root, "todo-app-demo", true, false, &sideEffectRunner{root: root}); err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}

	_, entries, err := EvidenceReport(context.Background(), root, "todo-app-demo")
	if err != nil {
		t.Fatalf("EvidenceReport: %v", err)
	}
	for _, entry := range entries {
		if entry.State != evidence.StateVerified {
			t.Fatalf("task %s = %s immediately after verify, want verified (identity recorded pre-proof?)", entry.TaskID, entry.State)
		}
	}
}

func TestVerifyPrunesOrphanedEntries(t *testing.T) {
	root := t.TempDir()
	writeVerifyFixture(t, root)
	overrideIdentityRunner(t, identityYielding("100644 blob aaa\tmain.go\n"))
	completeBoth(t, root)

	ledger, err := evidence.Load(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("load ledger: %v", err)
	}
	orphan := ledger.Tasks["1.1"]
	ledger.Tasks["9.9"] = orphan
	if err := evidence.Save(root, ledger); err != nil {
		t.Fatalf("seed orphan: %v", err)
	}

	// Check mode reports without touching the orphan.
	checked, err := Verify(context.Background(), root, "todo-app-demo", false, true, testutil.NewFakeRunner())
	if err != nil {
		t.Fatalf("Verify --check: %v", err)
	}
	if len(checked.Pruned) != 1 || checked.Pruned[0] != "9.9" {
		t.Fatalf("check mode pruned report = %v, want [9.9]", checked.Pruned)
	}
	after, _ := evidence.Load(root, "todo-app-demo")
	if _, exists := after.Tasks["9.9"]; !exists {
		t.Fatal("check mode removed the orphan")
	}

	// A real run prunes even when nothing needs re-proving.
	result, err := Verify(context.Background(), root, "todo-app-demo", false, false, testutil.NewFakeRunner())
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if len(result.Pruned) != 1 || result.Pruned[0] != "9.9" {
		t.Fatalf("pruned = %v, want [9.9]", result.Pruned)
	}
	final, _ := evidence.Load(root, "todo-app-demo")
	if _, exists := final.Tasks["9.9"]; exists {
		t.Fatal("orphan survived a persisting verify")
	}
	if len(final.Tasks) != 2 {
		t.Fatalf("ledger holds %d entries, want the 2 plan tasks", len(final.Tasks))
	}
}
