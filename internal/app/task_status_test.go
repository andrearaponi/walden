package app

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/output"
)

func TestRunTaskStatusPrintsHumanReadableReadiness(t *testing.T) {
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

- [x] 1. Build parser
  - [x] 1.1 Implement parser
    - Requirements: `+"`R1`, `R2`"+`
    - Design: Task Store
    - Verification: `+"`go test ./internal/spec`"+`
  - [ ] 1.2 Implement readiness
    - Requirements: `+"`R1`, `NFR3`"+`
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

	exitCode := Run([]string{"task", "status", "todo-app-demo"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected task status to succeed, got %d and stderr %q", exitCode, stderr.String())
	}

	rendered := stdout.String()
	for _, want := range []string{
		"Summary: execution readiness for todo-app-demo",
		"Current phase: tasks",
		"Task: 1.2 Implement readiness",
		"Task requirements: R1, NFR3",
		"Task design refs: Execution Service",
		"Task verification: `go test ./internal/workflow ./internal/app`",
		"Next action: Start task 1.2",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected output to contain %q, got %q", want, rendered)
		}
	}
	if strings.Contains(rendered, "Blockers:") {
		t.Fatalf("expected no blockers for runnable state, got %q", rendered)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunTaskStatusPrintsJSONForBlockedFeature(t *testing.T) {
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

	exitCode := Run([]string{"task", "status", "todo-app-demo", "--json"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected blocked task status to succeed, got %d and stderr %q", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr for json mode, got %q", stderr.String())
	}

	var envelope output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("expected valid json output, got %v", err)
	}
	result := envelope.Result
	if result.Summary != "execution readiness for todo-app-demo" {
		t.Fatalf("unexpected summary: %q", result.Summary)
	}
	if result.CurrentPhase != "tasks" {
		t.Fatalf("expected tasks phase, got %q", result.CurrentPhase)
	}
	if result.Task != nil {
		t.Fatalf("expected no task in blocked state, got %#v", result.Task)
	}
	if len(result.Blockers) != 1 || result.Blockers[0] != "tasks.md must be approved and fresh before execution" {
		t.Fatalf("unexpected blockers: %#v", result.Blockers)
	}
	if result.NextAction != "Approve tasks.md" {
		t.Fatalf("unexpected next action: %q", result.NextAction)
	}
}

func TestRunTaskStatusPrintsHumanReadableWhenNoTasksRemain(t *testing.T) {
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

- [x] 1. Build parser
  - [x] 1.1 Implement parser
    - Requirements: `+"`R1`"+`
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

	exitCode := Run([]string{"task", "status", "todo-app-demo"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected exhausted task status to succeed, got %d and stderr %q", exitCode, stderr.String())
	}

	rendered := stdout.String()
	for _, want := range []string{
		"Summary: execution readiness for todo-app-demo",
		"Blockers:",
		"implementation plan has no remaining runnable leaf tasks",
		"Next action: No runnable tasks remain; implementation plan is complete",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected output to contain %q, got %q", want, rendered)
		}
	}
	if strings.Contains(rendered, "Task:") {
		t.Fatalf("expected no task context when work is exhausted, got %q", rendered)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}
