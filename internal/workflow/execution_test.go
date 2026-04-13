package workflow

import (
	"context"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/spec"
	"github.com/andrearaponi/walden/internal/testutil"
)

func TestLoadExecutionReadinessReturnsNextUncheckedLeafTask(t *testing.T) {
	root := t.TempDir()
	writeFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeFeatureDoc(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-21T14:10:00Z
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at: 2026-03-21T14:00:00Z
---

# Feature Design
`)
	writeFeatureDoc(t, root, "todo-app-demo", "tasks.md", `---
status: approved
approved_at: 2026-03-21T14:20:00Z
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at: 2026-03-21T14:10:00Z
---

# Implementation Plan

- [x] 1. Build parser
  - [x] 1.1 Implement parser
    - Requirements: `+"`R1`, `R2`"+`
    - Design: Task Store
    - Verification: `+"`go test ./internal/spec`"+`
  - [ ] 1.2 Implement readiness
    - Requirements: `+"`R1`, `NFR3`"+`
    - Design: Execution Service
    - Verification: `+"`go test ./internal/workflow ./internal/app`"+`

- [ ] 2. Implement lesson log
  - [ ] 2.1 Implement lesson service
    - Requirements: `+"`R5`"+`
    - Design: Lesson Service
    - Verification: `+"`go test ./internal/spec`"+`
`)

	readiness, err := LoadExecutionReadiness(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("expected readiness load to succeed, got %v", err)
	}

	if !readiness.Runnable {
		t.Fatal("expected readiness to be runnable")
	}
	if readiness.CurrentPhase != PhaseTasks {
		t.Fatalf("expected tasks phase, got %q", readiness.CurrentPhase)
	}
	if readiness.NextTask == nil {
		t.Fatal("expected next task to be present")
	}
	if readiness.NextTask.ID != "1.2" {
		t.Fatalf("expected next task 1.2, got %q", readiness.NextTask.ID)
	}
	if readiness.NextTask.Title != "Implement readiness" {
		t.Fatalf("unexpected next task title: %q", readiness.NextTask.Title)
	}
	if readiness.NextTask.Verification != "`go test ./internal/workflow ./internal/app`" {
		t.Fatalf("unexpected verification: %q", readiness.NextTask.Verification)
	}
	if readiness.NextAction != "Start task 1.2" {
		t.Fatalf("unexpected next action: %q", readiness.NextAction)
	}
	if len(readiness.Blockers) != 0 {
		t.Fatalf("expected no blockers, got %#v", readiness.Blockers)
	}
}

func TestLoadExecutionReadinessBlocksWhenTasksAreNotApproved(t *testing.T) {
	root := t.TempDir()
	writeFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeFeatureDoc(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-21T14:10:00Z
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at: 2026-03-21T14:00:00Z
---

# Feature Design
`)
	writeFeatureDoc(t, root, "todo-app-demo", "tasks.md", `---
status: in-review
approved_at:
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at:
---

# Implementation Plan

- [ ] 1. Build parser
  - [ ] 1.1 Implement parser
    - Requirements: `+"`R1`"+`
    - Design: Task Store
    - Verification: `+"`go test ./internal/spec`"+`
`)

	readiness, err := LoadExecutionReadiness(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("expected readiness load to succeed, got %v", err)
	}

	if readiness.Runnable {
		t.Fatal("expected readiness to be blocked")
	}
	if readiness.BlockingDocument != "tasks.md" {
		t.Fatalf("expected tasks.md as blocking document, got %q", readiness.BlockingDocument)
	}
	assertContains(t, readiness.Blockers, "tasks.md must be approved and fresh before execution")
	if readiness.NextTask != nil {
		t.Fatalf("expected no next task when blocked, got %#v", readiness.NextTask)
	}
	if readiness.NextAction != "Approve tasks.md" {
		t.Fatalf("unexpected next action: %q", readiness.NextAction)
	}
}

func TestLoadExecutionReadinessBlocksWhenTasksAreStale(t *testing.T) {
	root := t.TempDir()
	writeFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeFeatureDoc(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-21T14:30:00Z
last_modified: 2026-03-21T14:30:00Z
source_requirements_approved_at: 2026-03-21T14:00:00Z
---

# Feature Design
`)
	writeFeatureDoc(t, root, "todo-app-demo", "tasks.md", `---
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
    - Verification: `+"`go test ./internal/spec`"+`
`)

	readiness, err := LoadExecutionReadiness(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("expected readiness load to succeed, got %v", err)
	}

	if readiness.Runnable {
		t.Fatal("expected readiness to be blocked by stale tasks")
	}
	if readiness.BlockingDocument != "tasks.md" {
		t.Fatalf("expected tasks.md as blocking document, got %q", readiness.BlockingDocument)
	}
	assertContains(t, readiness.Blockers, "tasks.md is stale relative to design.md")
	if readiness.NextAction != "Update tasks.md to match the latest approved design and return it to in-review" {
		t.Fatalf("unexpected next action: %q", readiness.NextAction)
	}
}

func TestLoadExecutionReadinessReportsNoRemainingRunnableTasks(t *testing.T) {
	root := t.TempDir()
	writeFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeFeatureDoc(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-21T14:10:00Z
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at: 2026-03-21T14:00:00Z
---

# Feature Design
`)
	writeFeatureDoc(t, root, "todo-app-demo", "tasks.md", `---
status: approved
approved_at: 2026-03-21T14:20:00Z
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at: 2026-03-21T14:10:00Z
---

# Implementation Plan

- [x] 1. Build parser
  - [x] 1.1 Implement parser
    - Requirements: `+"`R1`"+`
    - Design: Task Store
    - Verification: `+"`go test ./internal/spec`"+`
`)

	readiness, err := LoadExecutionReadiness(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("expected readiness load to succeed, got %v", err)
	}

	if readiness.Runnable {
		t.Fatal("expected readiness to be non-runnable when all tasks are complete")
	}
	if readiness.NextTask != nil {
		t.Fatalf("expected no next task, got %#v", readiness.NextTask)
	}
	if readiness.BlockingDocument != "tasks.md" {
		t.Fatalf("expected tasks.md as exhausted document, got %q", readiness.BlockingDocument)
	}
	assertContains(t, readiness.Blockers, "implementation plan has no remaining runnable leaf tasks")
	if readiness.NextAction != "No runnable tasks remain; implementation plan is complete" {
		t.Fatalf("unexpected next action: %q", readiness.NextAction)
	}
}

func TestStartTaskSelectsNextLeafTaskByDefault(t *testing.T) {
	root := t.TempDir()
	writeFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeFeatureDoc(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-21T14:10:00Z
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at: 2026-03-21T14:00:00Z
---

# Feature Design
`)
	writeFeatureDoc(t, root, "todo-app-demo", "tasks.md", `---
status: approved
approved_at: 2026-03-21T14:20:00Z
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at: 2026-03-21T14:10:00Z
---

# Implementation Plan

- [x] 1. Build parser
  - [x] 1.1 Implement parser
    - Requirements: `+"`R1`"+`
    - Design: Task Store
    - Verification: `+"`go test ./internal/spec`"+`
  - [ ] 1.2 Implement readiness
    - Requirements: `+"`R1`, `NFR3`"+`
    - Design: Execution Service
    - Verification: `+"`go test ./internal/workflow ./internal/app`"+`
`)

	context, err := StartTask(root, "todo-app-demo", "")
	if err != nil {
		t.Fatalf("expected start task to succeed, got %v", err)
	}

	if context.Task.ID != "1.2" {
		t.Fatalf("expected task 1.2, got %q", context.Task.ID)
	}
	if context.Task.Title != "Implement readiness" {
		t.Fatalf("unexpected task title: %q", context.Task.Title)
	}
	if context.NextAction != "Implement the task, run `go test ./internal/workflow ./internal/app`, then complete task 1.2" {
		t.Fatalf("unexpected next action: %q", context.NextAction)
	}
}

func TestStartTaskAllowsExplicitCurrentNextLeafTask(t *testing.T) {
	root := t.TempDir()
	writeFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeFeatureDoc(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-21T14:10:00Z
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at: 2026-03-21T14:00:00Z
---

# Feature Design
`)
	writeFeatureDoc(t, root, "todo-app-demo", "tasks.md", `---
status: approved
approved_at: 2026-03-21T14:20:00Z
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at: 2026-03-21T14:10:00Z
---

# Implementation Plan

- [ ] 1. Build parser
  - [ ] 1.1 Implement parser
    - Requirements: `+"`R1`, `R2`"+`
    - Design: Task Store
    - Verification: `+"`go test ./internal/spec`"+`
`)

	context, err := StartTask(root, "todo-app-demo", "1.1")
	if err != nil {
		t.Fatalf("expected explicit start task to succeed, got %v", err)
	}

	if context.Task.ID != "1.1" {
		t.Fatalf("expected task 1.1, got %q", context.Task.ID)
	}
}

func TestStartTaskRejectsMissingTask(t *testing.T) {
	root := t.TempDir()
	writeFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeFeatureDoc(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-21T14:10:00Z
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at: 2026-03-21T14:00:00Z
---

# Feature Design
`)
	writeFeatureDoc(t, root, "todo-app-demo", "tasks.md", `---
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
    - Verification: `+"`go test ./internal/spec`"+`
`)

	if _, err := StartTask(root, "todo-app-demo", "9.9"); err == nil {
		t.Fatal("expected missing task to fail")
	} else if err.Error() != `task "9.9" does not exist` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStartTaskRejectsCompletedLeafTask(t *testing.T) {
	root := t.TempDir()
	writeFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeFeatureDoc(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-21T14:10:00Z
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at: 2026-03-21T14:00:00Z
---

# Feature Design
`)
	writeFeatureDoc(t, root, "todo-app-demo", "tasks.md", `---
status: approved
approved_at: 2026-03-21T14:20:00Z
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at: 2026-03-21T14:10:00Z
---

# Implementation Plan

- [x] 1. Build parser
  - [x] 1.1 Implement parser
    - Requirements: `+"`R1`"+`
    - Design: Task Store
    - Verification: `+"`go test ./internal/spec`"+`
`)

	if _, err := StartTask(root, "todo-app-demo", "1.1"); err == nil {
		t.Fatal("expected completed task to fail")
	} else if err.Error() != `task "1.1" is already completed` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStartTaskRejectsParentTask(t *testing.T) {
	root := t.TempDir()
	writeFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeFeatureDoc(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-21T14:10:00Z
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at: 2026-03-21T14:00:00Z
---

# Feature Design
`)
	writeFeatureDoc(t, root, "todo-app-demo", "tasks.md", `---
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
    - Verification: `+"`go test ./internal/spec`"+`
`)

	if _, err := StartTask(root, "todo-app-demo", "1"); err == nil {
		t.Fatal("expected parent task to fail")
	} else if err.Error() != `task "1" is not an executable leaf task` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStartTaskRejectsBlockedExplicitTaskWhenEarlierLeafIsIncomplete(t *testing.T) {
	root := t.TempDir()
	writeFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeFeatureDoc(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-21T14:10:00Z
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at: 2026-03-21T14:00:00Z
---

# Feature Design
`)
	writeFeatureDoc(t, root, "todo-app-demo", "tasks.md", `---
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
    - Verification: `+"`go test ./internal/spec`"+`
  - [ ] 1.2 Implement readiness
    - Requirements: `+"`R1`"+`
    - Design: Execution Service
    - Verification: `+"`go test ./internal/workflow ./internal/app`"+`
`)

	if _, err := StartTask(root, "todo-app-demo", "1.2"); err == nil {
		t.Fatal("expected blocked explicit task to fail")
	} else if err.Error() != `task "1.2" is blocked by incomplete prerequisite task "1.1"` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCompleteTaskRunsProofAndMarksLeafTaskComplete(t *testing.T) {
	root := t.TempDir()
	writeFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeFeatureDoc(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-21T14:10:00Z
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at: 2026-03-21T14:00:00Z
---

# Feature Design
`)
	writeFeatureDoc(t, root, "todo-app-demo", "tasks.md", `---
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
    - Verification: `+"`go test ./internal/spec`"+`
`)

	runner := testutil.NewFakeRunner(testutil.Response{Stdout: "ok", ExitCode: 0})

	result, err := CompleteTask(context.Background(), root, "todo-app-demo", "1.1", runner)
	if err != nil {
		t.Fatalf("expected complete task to succeed, got %v", err)
	}

	if result.Task.ID != "1.1" {
		t.Fatalf("expected completed task 1.1, got %q", result.Task.ID)
	}
	if result.ProofCommand != "`go test ./internal/spec`" {
		t.Fatalf("unexpected proof command: %q", result.ProofCommand)
	}
	if len(result.CompletedTasks) != 2 || result.CompletedTasks[0] != "1.1" || result.CompletedTasks[1] != "1" {
		t.Fatalf("unexpected completed tasks: %#v", result.CompletedTasks)
	}
	if result.NextAction != "No runnable tasks remain; implementation plan is complete" {
		t.Fatalf("unexpected next action: %q", result.NextAction)
	}

	calls := runner.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected one proof command call, got %d", len(calls))
	}
	if calls[0].Name != "go" || len(calls[0].Args) != 2 || calls[0].Args[0] != "test" || calls[0].Args[1] != "./internal/spec" {
		t.Fatalf("unexpected runner call: %#v", calls[0])
	}

	feature, err := spec.LoadFeature(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("expected feature reload to succeed, got %v", err)
	}
	tree, err := spec.ParseTaskTree(feature.Tasks)
	if err != nil {
		t.Fatalf("expected task tree parse to succeed, got %v", err)
	}
	task, ok := tree.FindTask("1.1")
	if !ok {
		t.Fatal("expected completed task to be present")
	}
	if !task.Completed {
		t.Fatal("expected task 1.1 to be completed after successful proof")
	}
}

func TestCompleteTaskRunsStructuredProofStepsWithoutShellInterpolation(t *testing.T) {
	root := t.TempDir()
	writeFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeFeatureDoc(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-21T14:10:00Z
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at: 2026-03-21T14:00:00Z
---

# Feature Design
`)
	writeFeatureDoc(t, root, "todo-app-demo", "tasks.md", `---
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
      - argv: ["go", "test", "./internal/spec"]
      - argv: ["grep", "-c", "id=\"hero\"", "index.html"]
`)

	runner := testutil.NewFakeRunner(
		testutil.Response{Stdout: "ok", ExitCode: 0},
		testutil.Response{Stdout: "1", ExitCode: 0},
	)

	result, err := CompleteTask(context.Background(), root, "todo-app-demo", "1.1", runner)
	if err != nil {
		t.Fatalf("expected complete task to succeed, got %v", err)
	}

	if result.ProofCommand != "command [\"go\",\"test\",\"./internal/spec\"]; command [\"grep\",\"-c\",\"id=\\\"hero\\\"\",\"index.html\"]" {
		t.Fatalf("unexpected proof command display: %q", result.ProofCommand)
	}

	calls := runner.Calls()
	if len(calls) != 2 {
		t.Fatalf("expected two proof steps, got %d", len(calls))
	}
	if calls[0].Name != "go" || len(calls[0].Args) != 2 || calls[0].Args[0] != "test" || calls[0].Args[1] != "./internal/spec" {
		t.Fatalf("unexpected first runner call: %#v", calls[0])
	}
	if calls[1].Name != "grep" || len(calls[1].Args) != 3 || calls[1].Args[0] != "-c" || calls[1].Args[1] != "id=\"hero\"" || calls[1].Args[2] != "index.html" {
		t.Fatalf("unexpected second runner call: %#v", calls[1])
	}
}

func TestCompleteTaskLeavesTaskUncheckedWhenProofFails(t *testing.T) {
	root := t.TempDir()
	writeFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeFeatureDoc(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-21T14:10:00Z
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at: 2026-03-21T14:00:00Z
---

# Feature Design
`)
	writeFeatureDoc(t, root, "todo-app-demo", "tasks.md", `---
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
    - Verification: `+"`go test ./internal/spec`"+`
`)

	runner := testutil.NewFakeRunner(testutil.Response{Stderr: "proof failed", ExitCode: 1})

	if _, err := CompleteTask(context.Background(), root, "todo-app-demo", "1.1", runner); err == nil {
		t.Fatal("expected complete task to fail when proof fails")
	} else if err.Error() != "verification failed for task \"1.1\": command \"`go test ./internal/spec`\" exited with code 1 (expected 0): proof failed" {
		t.Fatalf("unexpected error: %v", err)
	}

	feature, err := spec.LoadFeature(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("expected feature reload to succeed, got %v", err)
	}
	tree, err := spec.ParseTaskTree(feature.Tasks)
	if err != nil {
		t.Fatalf("expected task tree parse to succeed, got %v", err)
	}
	task, ok := tree.FindTask("1.1")
	if !ok {
		t.Fatal("expected task 1.1 to exist")
	}
	if task.Completed {
		t.Fatal("expected task 1.1 to remain unchecked after failed proof")
	}
}

func TestCompleteTaskLeavesTaskUncheckedWhenStructuredProofFails(t *testing.T) {
	root := t.TempDir()
	writeFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeFeatureDoc(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-21T14:10:00Z
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at: 2026-03-21T14:00:00Z
---

# Feature Design
`)
	writeFeatureDoc(t, root, "todo-app-demo", "tasks.md", `---
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
      - argv: ["go", "test", "./internal/spec"]
      - argv: ["grep", "-c", "id=\"hero\"", "index.html"]
`)

	runner := testutil.NewFakeRunner(
		testutil.Response{Stdout: "ok", ExitCode: 0},
		testutil.Response{Stderr: "missing hero", ExitCode: 1},
	)

	if _, err := CompleteTask(context.Background(), root, "todo-app-demo", "1.1", runner); err == nil {
		t.Fatal("expected structured proof failure")
	} else if err.Error() != "verification failed for task \"1.1\": command \"command [\\\"grep\\\",\\\"-c\\\",\\\"id=\\\\\\\"hero\\\\\\\"\\\",\\\"index.html\\\"]\" exited with code 1 (expected 0): missing hero" {
		t.Fatalf("unexpected error: %v", err)
	}

	feature, err := spec.LoadFeature(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("expected feature reload to succeed, got %v", err)
	}
	tree, err := spec.ParseTaskTree(feature.Tasks)
	if err != nil {
		t.Fatalf("expected task tree parse to succeed, got %v", err)
	}
	task, ok := tree.FindTask("1.1")
	if !ok {
		t.Fatal("expected task 1.1 to exist")
	}
	if task.Completed {
		t.Fatal("expected task 1.1 to remain unchecked after failed structured proof")
	}
}

func TestCompleteTaskReturnsParentAutoCompletionWhenLastChildCloses(t *testing.T) {
	root := t.TempDir()
	writeFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeFeatureDoc(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-21T14:10:00Z
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at: 2026-03-21T14:00:00Z
---

# Feature Design
`)
	writeFeatureDoc(t, root, "todo-app-demo", "tasks.md", `---
status: approved
approved_at: 2026-03-21T14:20:00Z
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at: 2026-03-21T14:10:00Z
---

# Implementation Plan

- [ ] 1. Build parser
  - [x] 1.1 Implement parser
    - Requirements: `+"`R1`"+`
    - Design: Task Store
    - Verification: `+"`go test ./internal/spec`"+`
  - [ ] 1.2 Implement readiness
    - Requirements: `+"`R1`"+`
    - Design: Execution Service
    - Verification: `+"`go test ./internal/workflow`"+`
`)

	runner := testutil.NewFakeRunner(testutil.Response{Stdout: "ok", ExitCode: 0})

	result, err := CompleteTask(context.Background(), root, "todo-app-demo", "1.2", runner)
	if err != nil {
		t.Fatalf("expected complete task to succeed, got %v", err)
	}

	if len(result.CompletedTasks) != 2 {
		t.Fatalf("expected leaf and parent to be reported as completed, got %#v", result.CompletedTasks)
	}
	if result.CompletedTasks[0] != "1.2" || result.CompletedTasks[1] != "1" {
		t.Fatalf("unexpected completed tasks ordering: %#v", result.CompletedTasks)
	}

	feature, err := spec.LoadFeature(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("expected feature reload to succeed, got %v", err)
	}
	tree, err := spec.ParseTaskTree(feature.Tasks)
	if err != nil {
		t.Fatalf("expected task tree parse to succeed, got %v", err)
	}
	parent, ok := tree.FindTask("1")
	if !ok {
		t.Fatal("expected parent task to exist")
	}
	if !parent.Completed {
		t.Fatal("expected parent task to be completed after last child closes")
	}
}

func TestCompleteAllTasksRunsLeafTasksInOrderUntilPlanExhausts(t *testing.T) {
	root := t.TempDir()
	writeFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeFeatureDoc(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-21T14:10:00Z
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at: 2026-03-21T14:00:00Z
---

# Feature Design
`)
	writeFeatureDoc(t, root, "todo-app-demo", "tasks.md", `---
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
    - Verification: `+"`go test ./internal/spec`"+`
  - [ ] 1.2 Implement readiness
    - Requirements: `+"`R1`"+`
    - Design: Execution Service
    - Verification: `+"`go test ./internal/workflow`"+`
`)

	runner := testutil.NewFakeRunner(
		testutil.Response{Stdout: "ok", ExitCode: 0},
		testutil.Response{Stdout: "ok", ExitCode: 0},
	)

	result, err := CompleteAllTasks(context.Background(), root, "todo-app-demo", runner)
	if err != nil {
		t.Fatalf("expected batch completion to succeed, got %v", err)
	}

	if got, want := strings.Join(result.CompletedTasks, ","), "1.1,1.2,1"; got != want {
		t.Fatalf("unexpected completed tasks: %q", got)
	}
	if result.FailedTask != "" || result.Failure != "" {
		t.Fatalf("expected no failure details, got %#v", result)
	}
	if result.NextAction != "No runnable tasks remain; implementation plan is complete" {
		t.Fatalf("unexpected next action: %q", result.NextAction)
	}

	calls := runner.Calls()
	if len(calls) != 2 {
		t.Fatalf("expected two proof calls, got %d", len(calls))
	}
	if calls[0].Name != "go" || calls[0].Args[1] != "./internal/spec" {
		t.Fatalf("unexpected first proof call: %#v", calls[0])
	}
	if calls[1].Name != "go" || calls[1].Args[1] != "./internal/workflow" {
		t.Fatalf("unexpected second proof call: %#v", calls[1])
	}
}

func TestCompleteAllTasksStopsOnFirstFailedTaskAndPreservesEarlierCompletions(t *testing.T) {
	root := t.TempDir()
	writeFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeFeatureDoc(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-21T14:10:00Z
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at: 2026-03-21T14:00:00Z
---

# Feature Design
`)
	writeFeatureDoc(t, root, "todo-app-demo", "tasks.md", `---
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
    - Verification: `+"`go test ./internal/spec`"+`
  - [ ] 1.2 Implement readiness
    - Requirements: `+"`R1`"+`
    - Design: Execution Service
    - Verification: `+"`go test ./internal/workflow`"+`
`)

	runner := testutil.NewFakeRunner(
		testutil.Response{Stdout: "ok", ExitCode: 0},
		testutil.Response{Stderr: "proof failed", ExitCode: 1},
	)

	result, err := CompleteAllTasks(context.Background(), root, "todo-app-demo", runner)
	if err == nil {
		t.Fatal("expected batch completion to fail on second task")
	}

	if got, want := strings.Join(result.CompletedTasks, ","), "1.1"; got != want {
		t.Fatalf("unexpected completed tasks before failure: %q", got)
	}
	if result.FailedTask != "1.2" {
		t.Fatalf("expected failed task 1.2, got %q", result.FailedTask)
	}
	if !strings.Contains(result.Failure, `task "1.2"`) {
		t.Fatalf("expected failure to mention task 1.2, got %q", result.Failure)
	}

	feature, loadErr := spec.LoadFeature(root, "todo-app-demo")
	if loadErr != nil {
		t.Fatalf("expected feature reload to succeed, got %v", loadErr)
	}
	tree, parseErr := spec.ParseTaskTree(feature.Tasks)
	if parseErr != nil {
		t.Fatalf("expected task tree parse to succeed, got %v", parseErr)
	}
	first, ok := tree.FindTask("1.1")
	if !ok || !first.Completed {
		t.Fatalf("expected first task to stay completed, got %#v", first)
	}
	second, ok := tree.FindTask("1.2")
	if !ok {
		t.Fatal("expected second task to exist")
	}
	if second.Completed {
		t.Fatal("expected second task to remain unchecked after failure")
	}
}

func setupApprovedFeature(t *testing.T, root, feature, tasksBody string) {
	t.Helper()
	writeFeatureDoc(t, root, feature, "requirements.md", `---
status: approved
approved_at: 2026-03-23T09:00:00Z
last_modified: 2026-03-23T09:00:00Z
---

# Requirements Document
`)
	writeFeatureDoc(t, root, feature, "design.md", `---
status: approved
approved_at: 2026-03-23T09:30:00Z
last_modified: 2026-03-23T09:30:00Z
source_requirements_approved_at: 2026-03-23T09:00:00Z
---

# Feature Design
`)
	writeFeatureDoc(t, root, feature, "tasks.md", tasksBody)
}

func TestCompleteTaskSucceedsWhenExitCodeMatchesExpectExit(t *testing.T) {
	root := t.TempDir()
	feature := "expect-exit-demo"
	setupApprovedFeature(t, root, feature, `---
status: approved
approved_at: 2026-03-23T10:00:00Z
last_modified: 2026-03-23T10:00:00Z
source_design_approved_at: 2026-03-23T09:30:00Z
---

# Implementation Plan

- [ ] 1. Check no residual
  - [ ] 1.1 Grep must not find pattern
    - Requirements: `+"`R1.AC1`"+`
    - Design: Zero residual
    - Verification:
      - command: ["grep", "-rq", "andyarch", "."]
        expect_exit: 1
`)
	runner := testutil.NewFakeRunner(testutil.Response{ExitCode: 1})

	result, err := CompleteTask(context.Background(), root, feature, "1.1", runner)
	if err != nil {
		t.Fatalf("expected success when exit code matches expect_exit, got %v", err)
	}
	if result.Task.ID != "1.1" {
		t.Fatalf("unexpected task ID: %q", result.Task.ID)
	}

	calls := runner.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 runner call, got %d", len(calls))
	}
	if calls[0].Name != "grep" {
		t.Fatalf("expected command name 'grep', got %q", calls[0].Name)
	}
}

func TestCompleteTaskFailsWhenExitCodeDoesNotMatchExpectExit(t *testing.T) {
	root := t.TempDir()
	feature := "expect-exit-mismatch"
	setupApprovedFeature(t, root, feature, `---
status: approved
approved_at: 2026-03-23T10:00:00Z
last_modified: 2026-03-23T10:00:00Z
source_design_approved_at: 2026-03-23T09:30:00Z
---

# Implementation Plan

- [ ] 1. Check no residual
  - [ ] 1.1 Grep must not find pattern
    - Requirements: `+"`R1.AC1`"+`
    - Design: Zero residual
    - Verification:
      - command: ["grep", "-rq", "andyarch", "."]
        expect_exit: 1
`)
	runner := testutil.NewFakeRunner(testutil.Response{ExitCode: 0})

	_, err := CompleteTask(context.Background(), root, feature, "1.1", runner)
	if err == nil {
		t.Fatal("expected failure when exit code does not match expect_exit")
	}
	if !strings.Contains(err.Error(), "expected 1") {
		t.Fatalf("expected error to mention expected exit code, got: %v", err)
	}
}

func TestCompleteTaskRunsShellViaCommandPattern(t *testing.T) {
	root := t.TempDir()
	feature := "shell-demo"
	setupApprovedFeature(t, root, feature, `---
status: approved
approved_at: 2026-03-23T10:00:00Z
last_modified: 2026-03-23T10:00:00Z
source_design_approved_at: 2026-03-23T09:30:00Z
---

# Implementation Plan

- [ ] 1. Shell verification
  - [ ] 1.1 Run shell command
    - Requirements: `+"`R1.AC1`"+`
    - Design: Shell via K8s pattern
    - Verification:
      - command: ["sh", "-c", "test -d .walden && echo ok"]
`)
	runner := testutil.NewFakeRunner(testutil.Response{ExitCode: 0, Stdout: "ok\n"})

	result, err := CompleteTask(context.Background(), root, feature, "1.1", runner)
	if err != nil {
		t.Fatalf("expected success for sh -c command, got %v", err)
	}
	if result.Task.ID != "1.1" {
		t.Fatalf("unexpected task ID: %q", result.Task.ID)
	}

	calls := runner.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 runner call, got %d", len(calls))
	}
	if calls[0].Name != "sh" {
		t.Fatalf("expected command name 'sh', got %q", calls[0].Name)
	}
	if len(calls[0].Args) != 2 || calls[0].Args[0] != "-c" {
		t.Fatalf("expected args [\"-c\", \"...\"], got %v", calls[0].Args)
	}
}

func TestLegacyProofDeprecationWarningInExecutor(t *testing.T) {
	task := ExecutableTask{
		ID:           "1.1",
		Title:        "Test task",
		Verification: "go test ./...",
		Proof: spec.VerificationSpec{
			LegacyCommand: "go test ./...",
		},
	}

	commands, _, err := resolveProofCommands(task)
	if err != nil {
		t.Fatalf("expected resolve to succeed, got %v", err)
	}

	if len(commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(commands))
	}
	if commands[0].Name != "go" {
		t.Fatalf("expected command name 'go', got %q", commands[0].Name)
	}
}
