package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/spec"
	"github.com/andrearaponi/walden/internal/testutil"
)

func TestEndToEndLocalWorkflowSequence(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("expected git dir creation to succeed, got %v", err)
	}

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("expected working directory lookup to succeed, got %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWD)
	})
	if err := os.Chdir(root); err != nil {
		t.Fatalf("expected chdir to succeed, got %v", err)
	}

	assertCommandSuccess(t, []string{"repo", "init"}, "repository initialized")
	assertCommandSuccess(t, []string{"feature", "init", "Todo App Demo"}, "feature scaffold initialized for todo-app-demo")
	assertCommandSuccess(t, []string{"status", "todo-app-demo"}, "workflow status for todo-app-demo")
	assertCommandSuccess(t, []string{"validate", "todo-app-demo"}, "VALID: .walden/specs/todo-app-demo")
	assertCommandSuccess(t, []string{"review", "open", "todo-app-demo", "--phase", "requirements"}, "review gate opened for requirements.md")
	assertCommandSuccess(t, []string{"review", "approve", "todo-app-demo", "--phase", "requirements"}, "review gate approved for requirements.md")

	feature, err := spec.LoadFeature(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("expected feature reload to succeed, got %v", err)
	}

	if feature.Requirements.Status != "approved" {
		t.Fatalf("expected requirements to be approved, got %q", feature.Requirements.Status)
	}
	if feature.Design.Exists != true {
		t.Fatal("expected design scaffold to exist")
	}
	if feature.Design.Status != "draft" {
		t.Fatalf("expected design to remain draft after requirements approval, got %q", feature.Design.Status)
	}
}

func TestEndToEndSecondDeterministicSliceWorkflowSequence(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("expected git dir creation to succeed, got %v", err)
	}

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("expected working directory lookup to succeed, got %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWD)
	})
	if err := os.Chdir(root); err != nil {
		t.Fatalf("expected chdir to succeed, got %v", err)
	}

	assertCommandSuccess(t, []string{"repo", "init"}, "repository initialized")
	assertCommandSuccess(t, []string{"feature", "init", "Todo App Demo"}, "feature scaffold initialized for todo-app-demo")

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

- [ ] 1. Implement deterministic lesson logging
  - [ ] 1.1 Log lessons through the CLI
    - Requirements: `+"`R5`"+`
    - Design: Lesson Service
    - Verification: `+"`go test ./internal/spec`"+`
`)

	assertCommandSuccess(t, []string{"task", "status", "todo-app-demo"}, "execution readiness for todo-app-demo")
	assertCommandSuccess(t, []string{"task", "start", "todo-app-demo"}, "task start context for todo-app-demo")

	previousRunner := commandRunner
	fakeRunner := testutil.NewFakeRunner(testutil.Response{Stdout: "ok", ExitCode: 0})
	commandRunner = fakeRunner
	t.Cleanup(func() {
		commandRunner = previousRunner
	})

	assertCommandSuccess(t, []string{"task", "complete", "todo-app-demo", "1.1"}, "task completed for todo-app-demo")

	calls := fakeRunner.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected one proof invocation, got %#v", calls)
	}
	if calls[0].Name != "go" || strings.Join(calls[0].Args, " ") != "test ./internal/spec" {
		t.Fatalf("unexpected proof invocation: %#v", calls[0])
	}

	feature, err := spec.LoadFeature(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("expected feature reload to succeed, got %v", err)
	}
	tree, err := spec.ParseTaskTree(feature.Tasks)
	if err != nil {
		t.Fatalf("expected task tree parse to succeed, got %v", err)
	}
	if task, ok := tree.FindTask("1.1"); !ok || !task.Completed {
		t.Fatalf("expected task 1.1 to be completed, got %#v", task)
	}
	if task, ok := tree.FindTask("1"); !ok || !task.Completed {
		t.Fatalf("expected parent task 1 to auto-complete, got %#v", task)
	}

	writeStatusFeatureFile(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-22T07:00:00Z
last_modified: 2026-03-22T07:45:00Z
---

# Requirements Document
`)

	assertCommandSuccess(t, []string{"reconcile", "todo-app-demo"}, "reconciliation completed for todo-app-demo")

	feature, err = spec.LoadFeature(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("expected feature reload after reconcile to succeed, got %v", err)
	}
	if feature.Requirements.Status != "in-review" {
		t.Fatalf("expected requirements to move to in-review, got %q", feature.Requirements.Status)
	}
	if feature.Design.Status != "draft" {
		t.Fatalf("expected design to reset to draft, got %q", feature.Design.Status)
	}
	if feature.Tasks.Status != "draft" {
		t.Fatalf("expected tasks to reset to draft, got %q", feature.Tasks.Status)
	}

	assertCommandSuccess(t, []string{
		"lesson", "log",
		"--feature", "todo-app-demo",
		"--phase", "execute",
		"--trigger", "end-to-end coverage completed",
		"--lesson", "compose deterministic CLI coverage around one realistic repository sequence",
		"--guardrail", "prefer one high-signal e2e flow over many overlapping command permutations",
	}, "lesson logged for todo-app-demo")

	lessonsText, err := os.ReadFile(filepath.Join(root, ".walden", "lessons.md"))
	if err != nil {
		t.Fatalf("expected lessons file read to succeed, got %v", err)
	}
	for _, want := range []string{
		"# Walden Lessons",
		"| todo-app-demo | execute",
		"- Trigger: end-to-end coverage completed",
		"- Guardrail: prefer one high-signal e2e flow over many overlapping command permutations",
	} {
		if !strings.Contains(string(lessonsText), want) {
			t.Fatalf("expected lessons file to contain %q, got %q", want, string(lessonsText))
		}
	}
}

func TestEndToEndWorkflowUXAndVerificationHardening(t *testing.T) {
	root := t.TempDir()

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("expected working directory lookup to succeed, got %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWD)
	})
	if err := os.Chdir(root); err != nil {
		t.Fatalf("expected chdir to succeed, got %v", err)
	}

	stdout, stderr, exitCode := runCommand(t, []string{"repo", "init"})
	if exitCode != 0 {
		t.Fatalf("expected repo init to succeed, got exit code %d and stderr %q", exitCode, stderr)
	}
	for _, want := range []string{
		"repository initialized",
		"Git: initialized new repository",
		".walden/lessons.md",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected repo init output to contain %q, got %q", want, stdout)
		}
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr for repo init, got %q", stderr)
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		t.Fatalf("expected repo init to create git metadata, got %v", err)
	}

	assertCommandSuccess(t, []string{"feature", "init", "Todo App Demo"}, "feature scaffold initialized for todo-app-demo")

	writeValidateFeatureFile(t, root, "todo-app-demo", "requirements.md", validRequirementsForValidateCommand)
	writeValidateFeatureFile(t, root, "todo-app-demo", "design.md", validDraftDesignForValidateCommand)
	writeValidateFeatureFile(t, root, "todo-app-demo", "tasks.md", invalidDraftTasksForValidateCommand)

	stdout, stderr, exitCode = runCommand(t, []string{"validate", "todo-app-demo"})
	if exitCode != 0 {
		t.Fatalf("expected phase-aware validate to succeed, got exit code %d and stderr %q", exitCode, stderr)
	}
	for _, want := range []string{
		"VALID: .walden/specs/todo-app-demo",
		"Validated phases: requirements, design",
		"Skipped phases: tasks",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected validate output to contain %q, got %q", want, stdout)
		}
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr for phase-aware validate, got %q", stderr)
	}

	stdout, stderr, exitCode = runCommand(t, []string{"validate", "todo-app-demo", "--all"})
	if exitCode != 1 {
		t.Fatalf("expected full-spec validate to fail, got exit code %d and stdout %q", exitCode, stdout)
	}
	if !strings.Contains(stderr, "INVALID: tasks.md missing task coverage for requirement IDs: R1") {
		t.Fatalf("expected downstream validation failure, got %q", stderr)
	}

	writeValidateFeatureFile(t, root, "todo-app-demo", "requirements.md", validRequirementsForValidateCommand)
	writeValidateFeatureFile(t, root, "todo-app-demo", "design.md", validDesignForValidateCommand)
	writeStatusFeatureFile(t, root, "todo-app-demo", "tasks.md", `---
status: approved
approved_at: 2026-03-22T07:20:00Z
last_modified: 2026-03-22T07:20:00Z
source_design_approved_at: 2026-03-21T14:10:00Z
---

# Implementation Plan

- [ ] 1. Build feature
  - [ ] 1.1 Add parser proof
    - Requirements: `+"`R1`"+`
    - Design: Todo flow
    - Verification:
      - argv: ["go", "test", "./internal/spec"]
  - [ ] 1.2 Add workflow proof
    - Requirements: `+"`NFR1`"+`
    - Design: Todo flow
    - Verification:
      - argv: ["go", "test", "./internal/workflow"]
`)

	previousRunner := commandRunner
	fakeRunner := testutil.NewFakeRunner(
		testutil.Response{Stdout: "spec ok", ExitCode: 0},
		testutil.Response{Stdout: "workflow ok", ExitCode: 0},
	)
	commandRunner = fakeRunner
	t.Cleanup(func() {
		commandRunner = previousRunner
	})

	stdout, stderr, exitCode = runCommand(t, []string{"task", "complete-all", "todo-app-demo"})
	if exitCode != 0 {
		t.Fatalf("expected batch completion to succeed, got exit code %d and stderr %q", exitCode, stderr)
	}
	for _, want := range []string{
		"batch task completion finished for todo-app-demo",
		"Completed tasks: 1.1, 1.2",
		"Auto-completed tasks: 1",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected batch completion output to contain %q, got %q", want, stdout)
		}
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr for batch completion, got %q", stderr)
	}

	calls := fakeRunner.Calls()
	if len(calls) != 2 {
		t.Fatalf("expected two proof invocations, got %#v", calls)
	}
	if calls[0].Name != "go" || strings.Join(calls[0].Args, " ") != "test ./internal/spec" {
		t.Fatalf("unexpected first proof invocation: %#v", calls[0])
	}
	if calls[1].Name != "go" || strings.Join(calls[1].Args, " ") != "test ./internal/workflow" {
		t.Fatalf("unexpected second proof invocation: %#v", calls[1])
	}

	feature, err := spec.LoadFeature(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("expected feature reload to succeed, got %v", err)
	}
	tree, err := spec.ParseTaskTree(feature.Tasks)
	if err != nil {
		t.Fatalf("expected task tree parse to succeed, got %v", err)
	}
	for _, id := range []string{"1.1", "1.2", "1"} {
		task, ok := tree.FindTask(id)
		if !ok || !task.Completed {
			t.Fatalf("expected task %s to be completed, got %#v", id, task)
		}
	}
}

func assertCommandSuccess(t *testing.T, args []string, summaryContains string) {
	t.Helper()

	stdout, stderr, exitCode := runCommand(t, args)
	if exitCode != 0 {
		t.Fatalf("expected %v to succeed, got exit code %d and stderr %q", args, exitCode, stderr)
	}
	if !strings.Contains(stdout, summaryContains) {
		t.Fatalf("expected stdout for %v to contain %q, got %q", args, summaryContains, stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr for %v, got %q", args, stderr)
	}
}

func runCommand(t *testing.T, args []string) (string, string, int) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run(args, &stdout, &stderr)
	return stdout.String(), stderr.String(), exitCode
}
