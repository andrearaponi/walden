package workflow

import (
	"context"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/spec"
	"github.com/andrearaponi/walden/internal/testutil"
)

// TestCompleteTaskRunsStepsAfterEmptyCovers completes a task whose first step
// declares covers: [] and whose second step fails: the second step must run
// and the task must stay unchecked.
func TestCompleteTaskRunsStepsAfterEmptyCovers(t *testing.T) {
	root := t.TempDir()
	writeFreshFeatureDoc(t, root, "empty-covers", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeFreshFeatureDoc(t, root, "empty-covers", "design.md", `---
status: approved
approved_at: 2026-03-21T14:10:00Z
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at: 2026-03-21T14:00:00Z
---

# Feature Design
`)
	writeFreshFeatureDoc(t, root, "empty-covers", "tasks.md", `---
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
        covers: []
      - command: ["go", "vet", "./internal/spec"]
`)

	runner := testutil.NewFakeRunner(
		testutil.Response{Stdout: "ok", ExitCode: 0},
		testutil.Response{Stderr: "vet failed", ExitCode: 1},
	)

	if _, err := CompleteTask(context.Background(), root, "empty-covers", "1.1", runner); err == nil {
		t.Fatal("completion passed although the step after covers: [] exits 1")
	}

	calls := runner.Calls()
	if len(calls) != 2 {
		t.Fatalf("the runner saw %d steps; the plan declares 2", len(calls))
	}
	if got := strings.Join(append([]string{calls[1].Name}, calls[1].Args...), " "); got != "go vet ./internal/spec" {
		t.Fatalf("second step ran as %q", got)
	}

	tree, err := spec.LoadTaskTree(root, "empty-covers")
	if err != nil {
		t.Fatalf("reload the plan: %v", err)
	}
	task, ok := tree.FindTask("1.1")
	if !ok {
		t.Fatal("task 1.1 is missing")
	}
	if task.Completed {
		t.Fatal("the checkbox flipped although the second step failed")
	}
}
