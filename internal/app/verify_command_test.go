package app

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/output"
	"github.com/andrearaponi/walden/internal/testutil"
)

// setupEvidenceFeature bootstraps a real repository in a temp cwd, approves a
// one-task feature, and completes it through the CLI so evidence exists.
func setupEvidenceFeature(t *testing.T) string {
	t.Helper()
	root := chdirContract(t)

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"repo", "init"}, &stdout, &stderr); code != 0 {
		t.Fatalf("repo init failed: %s", stderr.String())
	}
	if code := Run([]string{"feature", "init", "Todo App Demo"}, &stdout, &stderr); code != 0 {
		t.Fatalf("feature init failed: %s", stderr.String())
	}

	writeStatusFeatureFile(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-22T07:00:00Z
last_modified: 2026-03-22T07:00:00Z
---

# Requirements Document
`)
	writeStatusFeatureFile(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-22T07:10:00Z
last_modified: 2026-03-22T07:10:00Z
source_requirements_approved_at: 2026-03-22T07:00:00Z
---

# Feature Design
`)
	writeStatusFeatureFile(t, root, "todo-app-demo", "tasks.md", `---
status: approved
approved_at: 2026-03-22T07:20:00Z
last_modified: 2026-03-22T07:20:00Z
source_design_approved_at: 2026-03-22T07:10:00Z
---

# Implementation Plan

- [ ] 1. Build parser
  - [ ] 1.1 Implement parser
    - Requirements: `+"`R1`"+`
    - Design: Task Store
    - Verification:
      - command: ["go", "test", "./internal/spec"]
`)

	overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: "ok", ExitCode: 0}))
	if code := Run([]string{"task", "complete", "todo-app-demo", "1.1"}, &stdout, &stderr); code != 0 {
		t.Fatalf("task complete failed: %s", stderr.String())
	}
	return root
}

func overrideCommandRunner(t *testing.T, runner *testutil.FakeRunner) *testutil.FakeRunner {
	t.Helper()
	previous := commandRunner
	commandRunner = runner
	t.Cleanup(func() { commandRunner = previous })
	return runner
}

func TestRunVerifyDefaultSkipsVerifiedTasks(t *testing.T) {
	setupEvidenceFeature(t)
	runner := overrideCommandRunner(t, testutil.NewFakeRunner())

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"verify", "todo-app-demo"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("verify exited %d: %s", exitCode, stderr.String())
	}
	if len(runner.Calls()) != 0 {
		t.Fatalf("verified task re-executed: %v", runner.Calls())
	}
	if !strings.Contains(stdout.String(), "already verified") {
		t.Fatalf("summary does not report the skip: %s", stdout.String())
	}
}

func TestRunVerifyAllReprovesAndReportsJSON(t *testing.T) {
	setupEvidenceFeature(t)
	overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: "ok", ExitCode: 0}))

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"verify", "todo-app-demo", "--all", "--json"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("verify --all exited %d: %s", exitCode, stderr.String())
	}

	var envelope output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if envelope.Command != "verify" || !envelope.OK {
		t.Fatalf("envelope = %s ok=%t", envelope.Command, envelope.OK)
	}
	if len(envelope.Result.Evidence) != 1 {
		t.Fatalf("evidence entries = %d, want 1", len(envelope.Result.Evidence))
	}
	entry := envelope.Result.Evidence[0]
	if entry.TaskID != "1.1" || entry.State != "verified" || entry.Passed == nil || !*entry.Passed {
		t.Fatalf("entry = %+v, want 1.1 verified passed", entry)
	}
}

func TestRunVerifyFailureExitsNonZeroNamingTasks(t *testing.T) {
	setupEvidenceFeature(t)
	overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stderr: "boom", ExitCode: 1}))

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"verify", "todo-app-demo", "--all"}, &stdout, &stderr)

	if exitCode == 0 {
		t.Fatal("failing verify exited zero")
	}
	if !strings.Contains(stderr.String(), "verification failed") || !strings.Contains(stderr.String(), "1.1") {
		t.Fatalf("stderr %q does not name the failed task", stderr.String())
	}
}

func TestRunVerifyCheckPersistsNothing(t *testing.T) {
	root := setupEvidenceFeature(t)
	overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: "ok", ExitCode: 0}))

	ledgerPath := root + "/.walden/evidence/todo-app-demo.json"
	before, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"verify", "todo-app-demo", "--all", "--check"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("verify --check exited %d: %s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "check mode") {
		t.Fatalf("summary does not note check mode: %s", stdout.String())
	}

	after, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatalf("read ledger after: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("check mode modified the ledger")
	}
}
