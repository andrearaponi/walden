package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var traceFormats = [][]string{{verifyTaskFeature}, {verifyTaskFeature, "--markdown"}, {verifyTaskFeature, "--json"}}

// TestTraceLeavesRepositoryUntouched: every file under the root, the git
// index among them, has the same bytes and the same mode before and after a
// trace in each format, on stale evidence and a tree that git has work to do
// on: a changed, an added, a removed and a staged path, and a changed
// symbolic link, the one path on which the identity capture writes a file,
// outside the repository.
func TestTraceLeavesRepositoryUntouched(t *testing.T) {
	// deliveryStatusFixture sets the git configuration aside before any git
	// runs: on a machine with a file-system monitor, git would rewrite the
	// index on a read and the comparison would fail for that.
	root := deliveryStatusFixture(t, deliveryStatusTasks)
	link := filepath.Join(root, "link")
	if err := os.Symlink("main.go", link); err != nil {
		t.Skipf("symbolic links are not available: %v", err)
	}
	gitify(t, root)
	completeStatusTask(t, "1.1")
	completeStatusTask(t, "1.2")

	diagnosticsFile(t, root, "main.go", "package main // changed\n")
	diagnosticsFile(t, root, "added.go", "package main\n")
	if err := os.Remove(filepath.Join(root, "notes.txt")); err != nil {
		t.Fatal(err)
	}
	diagnosticsFile(t, root, "staged.go", "package main\n")
	integrityGit(t, root, "add", "staged.go")
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(".gitignore", link); err != nil {
		t.Fatal(err)
	}

	before := diagnosticsTreeBytes(t, root)
	for _, args := range traceFormats {
		stdout, stderr, code := runTraceCLI(t, args...)
		if code != 0 {
			t.Fatalf("trace %v exited %d: %s", args[1:], code, stderr)
		}
		if after := diagnosticsTreeBytes(t, root); !reflect.DeepEqual(before, after) {
			for path, digest := range after {
				if before[path] != digest {
					t.Errorf("trace %v changed or created %s", args[1:], path)
				}
			}
			for path := range before {
				if _, kept := after[path]; !kept {
					t.Errorf("trace %v removed %s", args[1:], path)
				}
			}
		}
		// A guard: the run read stale evidence, so the comparison is not of
		// a trace that had nothing to derive.
		if len(args) == 1 && !strings.Contains(stdout, "delivered (delivery proof)") {
			t.Fatalf("the trace does not report the delivered leaf:\n%s", stdout)
		}
	}
	_, block := traceBlock(t)
	states := map[string]any{}
	for _, entry := range block["leaves"].([]any) {
		leaf := entry.(map[string]any)
		states[leaf["id"].(string)] = leaf["state"]
	}
	if want := map[string]any{"1.1": "delivered", "1.2": "stale-code"}; !reflect.DeepEqual(states, want) {
		t.Fatalf("the trace read the states %v, want a stale one: %v", states, want)
	}
}

// TestTraceRunsNoProofAndNoProbe: a pending proof step and a declared
// environment probe that would each create a file create none in any format.
// Each file is then created by the command that does run its step or its
// probe, so the absence is not that of a file nothing could create.
func TestTraceRunsNoProofAndNoProbe(t *testing.T) {
	proofFile := filepath.Join(t.TempDir(), "proof-ran")
	probeFile := filepath.Join(t.TempDir(), "probe-ran")
	quote := func(path string) string {
		quoted, err := json.Marshal(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(quoted)
	}
	plan := `---
status: approved
approved_at: 2026-03-22T07:20:00Z
last_modified: 2026-03-22T07:20:00Z
source_design_approved_at: 2026-03-22T07:10:00Z
---

# Implementation Plan

- [ ] 1. Build parser
  - [ ] 1.1 Leave a file behind
    - Requirements: ` + "`R1.AC1`, `R2.AC1`" + `
    - Design: Task Store
    - Verification:
      - command: ["touch", ` + quote(proofFile) + `]
        covers: ["R1.AC1", "R2.AC1"]
  - [ ] 1.2 Keep it deterministic
    - Requirements: ` + "`NFR1`" + `
    - Design: Task Store
    - Verification:
      - command: ["go", "test", "./internal/workflow"]
`
	// No task is completed and no command has run on this root: the process
	// holds no captured profile for it, so a probe declared here does run
	// for a command that captures the profile.
	root := deliveryStatusFixture(t, plan)
	gitify(t, root)
	probes := fmt.Sprintf("- sentinel: [\"touch\", %s]\n", quote(probeFile))
	if err := os.WriteFile(filepath.Join(root, ".walden", "environment.md"), []byte(probes), 0o644); err != nil {
		t.Fatalf("declare the probe: %v", err)
	}
	exists := func(path string) bool {
		_, err := os.Stat(path)
		return err == nil
	}

	for _, args := range traceFormats {
		stdout, stderr, code := runTraceCLI(t, args...)
		if code != 0 {
			t.Fatalf("trace %v exited %d: %s", args[1:], code, stderr)
		}
		// A guard: the trace did read the step it must not run.
		if len(args) == 1 && !strings.Contains(stdout, "1.1 step 1, pending: touch ") {
			t.Fatalf("the trace does not list the pending step:\n%s", stdout)
		}
		if exists(proofFile) {
			t.Fatalf("trace %v executed the proof step", args[1:])
		}
		if exists(probeFile) {
			t.Fatalf("trace %v executed the environment probe", args[1:])
		}
	}

	// The positive controls. evidence status captures the profile, and the
	// probe creates its file; task complete runs the step with the real
	// runner, and the step creates its file.
	integrityMustCLI(t, "evidence", "status", verifyTaskFeature)
	if !exists(probeFile) {
		t.Fatal("evidence status did not run the probe: the sentinel proves nothing on this root")
	}
	integrityMustCLI(t, "task", "complete", verifyTaskFeature, "1.1")
	if !exists(proofFile) {
		t.Fatal("task complete did not create the file: the proof step proves nothing")
	}
}
