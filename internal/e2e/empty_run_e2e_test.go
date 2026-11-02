package e2e

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/output"
)

// emptyRunPlan declares go test proofs that name a test and assert nothing on
// its output, the shape of a real plan whose selector can stop matching.
const emptyRunPlan = `---
status: draft
approved_at:
last_modified: 2026-07-11T10:00:00Z
approved_fingerprint:
source_design_approved_at:
source_design_fingerprint:
---

# Implementation Plan

- [ ] 1. Tests
  - [ ] 1.1 Prove the test that gets renamed
    - Requirements: ` + "`R1.AC1`" + `
    - Design: Overview
    - Verification:
      - command: ["go", "test", "-count=1", "-run", "^TestFirst$", "."]
        covers: ["R1.AC1"]
  - [ ] 1.2 Prove the test that stays
    - Requirements: ` + "`R1.AC2`" + `
    - Design: Overview
    - Verification:
      - command: ["go", "test", "-count=1", "-run", "^TestKept$", "."]
        covers: ["R1.AC2"]
`

const emptyRunWarning = `task 1.1: command ["go","test","-count=1","-run","^TestFirst$","."] passed without running any test: check its selector and assert the test by name with -v and expect_output`

func emptyRunTests(first string) string {
	return "package lab\n\nimport \"testing\"\n\nfunc " + first + "(t *testing.T) {}\n\nfunc TestKept(t *testing.T) {}\n"
}

// nativeResult runs the built binary with --json and returns its exit code and
// result.
func nativeResult(t *testing.T, binary, dir string, args ...string) (output.Result, int) {
	t.Helper()
	stdout, stderr, code := diagnosticsNativeRun(t, binary, dir, append(args, "--json")...)
	var envelope output.Envelope
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("decode %v: %v\n%s%s", args, err, stdout, stderr)
	}
	return envelope.Result, code
}

func TestE2EEmptyRunCycle(t *testing.T) {
	binary := diagnosticsNativeCLI(t)
	// The proofs run in the fixture module, whatever the host has configured.
	t.Setenv("GOFLAGS", "")
	t.Setenv("GOWORK", "off")

	dir := approvedRepo(t, emptyRunPlan)
	writeFile(t, dir, "go.mod", "module lab\n\ngo 1.25.0\n")
	writeFile(t, dir, "lab_test.go", emptyRunTests("TestFirst"))
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-qm", "code and specs")

	// Proofs that run a test complete with nothing to report.
	completed, code := nativeResult(t, binary, dir, "task", "complete-all", "gate-demo")
	if code != 0 || len(completed.CompletedTasks) != 2 {
		t.Fatalf("complete-all exited %d: %+v", code, completed)
	}
	if len(completed.Warnings) != 0 {
		t.Fatalf("a proof that ran a test warned: %q", completed.Warnings)
	}
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-qm", "evidence")

	// Renaming the test leaves the selector of task 1.1 matching nothing.
	writeFile(t, dir, "lab_test.go", emptyRunTests("TestRenamed"))
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-qm", "rename the test")

	// The empty proof passes, warns, and only its own step is named.
	stdout, stderr, code := diagnosticsNativeRun(t, binary, dir, "verify", "gate-demo", "--check")
	if code != 0 {
		t.Fatalf("verify --check exited %d: %s%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "Warnings:\n- "+emptyRunWarning+"\n") {
		t.Fatalf("verify --check does not hold the warning:\n%s", stdout)
	}
	if strings.Contains(stdout, "task 1.2:") {
		t.Fatalf("the proof that ran a test warned:\n%s", stdout)
	}

	verified, code := nativeResult(t, binary, dir, "verify", "gate-demo")
	if code != 0 {
		t.Fatalf("verify exited %d: %+v", code, verified)
	}
	if len(verified.Warnings) != 1 || verified.Warnings[0] != emptyRunWarning {
		t.Fatalf("warnings = %q, want the warning of task 1.1 alone", verified.Warnings)
	}
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-qm", "evidence")

	// The warning decides nothing: the task reads verified and the feature
	// certifies as any other.
	status, code := nativeResult(t, binary, dir, "evidence", "status", "gate-demo")
	if code != 0 || len(status.Evidence) != 2 {
		t.Fatalf("evidence status exited %d: %+v", code, status)
	}
	for _, entry := range status.Evidence {
		if entry.State != "verified" {
			t.Fatalf("task %s reads %s, want verified: %+v", entry.TaskID, entry.State, status)
		}
	}

	release, code := nativeResult(t, binary, dir, "release", "check", "gate-demo")
	if code != 0 || release.Release == nil || !release.Release.Releasable {
		t.Fatalf("release check exited %d: %+v", code, release.Release)
	}
}
