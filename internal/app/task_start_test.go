package app

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/output"
)

func TestRunTaskStartPrintsHumanReadableExecutionContext(t *testing.T) {
	root := t.TempDir()
	writeStatusFeatureFile(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeStatusFeatureFile(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-21T14:10:00Z
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at: 2026-03-21T14:00:00Z
---

# Feature Design
`)
	writeStatusFeatureFile(t, root, "todo-app-demo", "tasks.md", `---
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

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"task", "start", "todo-app-demo"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected task start to succeed, got %d and stderr %q", exitCode, stderr.String())
	}

	rendered := stdout.String()
	for _, want := range []string{
		"Summary: task start context for todo-app-demo",
		"Current phase: tasks",
		"Task: 1.1 Implement parser",
		"Task requirements: R1, R2",
		"Task design refs: Task Store",
		"Task verification: `go test ./internal/spec`",
		"Next action: Implement the task, run `go test ./internal/spec`, then complete task 1.1",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected output to contain %q, got %q", want, rendered)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunTaskStartPrintsJSONForExplicitTask(t *testing.T) {
	root := t.TempDir()
	writeStatusFeatureFile(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeStatusFeatureFile(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-21T14:10:00Z
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at: 2026-03-21T14:00:00Z
---

# Feature Design
`)
	writeStatusFeatureFile(t, root, "todo-app-demo", "tasks.md", `---
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

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"task", "start", "todo-app-demo", "1.1", "--json"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected explicit task start to succeed, got %d and stderr %q", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr for json mode, got %q", stderr.String())
	}

	var envelope output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("expected valid json output, got %v", err)
	}
	result := envelope.Result
	if result.Task == nil || result.Task.ID != "1.1" {
		t.Fatalf("unexpected task payload: %#v", result.Task)
	}
	if result.NextAction != "Implement the task, run `go test ./internal/spec`, then complete task 1.1" {
		t.Fatalf("unexpected next action: %q", result.NextAction)
	}
}

func TestRunTaskStartFailsForBlockedTask(t *testing.T) {
	root := t.TempDir()
	writeStatusFeatureFile(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeStatusFeatureFile(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-21T14:10:00Z
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at: 2026-03-21T14:00:00Z
---

# Feature Design
`)
	writeStatusFeatureFile(t, root, "todo-app-demo", "tasks.md", `---
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

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"task", "start", "todo-app-demo", "1.2"}, &stdout, &stderr)

	if exitCode != 1 {
		t.Fatalf("expected blocked task start to fail, got %d", exitCode)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout on failure, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), `task "1.2" is blocked by incomplete prerequisite task "1.1"`) {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}
