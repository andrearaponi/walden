package app

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/output"
	"github.com/andrearaponi/walden/internal/spec"
)

func TestRunReconcilePrintsHumanReadableChangedFilesAndNextGate(t *testing.T) {
	root := t.TempDir()
	writeStatusFeatureFile(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:30:00Z
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

- [ ] 1. Restore freshness
  - [ ] 1.1 Reconcile workflow state
    - Requirements: `+"`R4`"+`
    - Design: Reconciliation Service
    - Verification: `+"`go test ./internal/app ./internal/output`"+`
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

	exitCode := Run([]string{"reconcile", "todo-app-demo"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected reconcile to succeed, got %d and stderr %q", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}

	rendered := stdout.String()
	for _, want := range []string{
		"Summary: reconciliation completed for todo-app-demo",
		"Changed files:",
		".walden/specs/todo-app-demo/requirements.md",
		".walden/specs/todo-app-demo/design.md",
		".walden/specs/todo-app-demo/tasks.md",
		"Current phase: requirements",
		"Next action: Approve requirements.md",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected output to contain %q, got %q", want, rendered)
		}
	}

	feature, err := spec.LoadFeature(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("expected feature reload to succeed, got %v", err)
	}
	if feature.Requirements.Status != "in-review" {
		t.Fatalf("expected requirements to be in-review, got %q", feature.Requirements.Status)
	}
	if feature.Design.Status != "draft" {
		t.Fatalf("expected design to be draft, got %q", feature.Design.Status)
	}
	if feature.Tasks.Status != "draft" {
		t.Fatalf("expected tasks to be draft, got %q", feature.Tasks.Status)
	}
}

func TestRunReconcilePrintsJSONWhenNoChangesAreNeeded(t *testing.T) {
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

- [ ] 1. Reconcile nothing
  - [ ] 1.1 Stay normalized
    - Requirements: `+"`R4`"+`
    - Design: Reconciliation Service
    - Verification: `+"`go test ./internal/app ./internal/output`"+`
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

	exitCode := Run([]string{"reconcile", "todo-app-demo", "--json"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected reconcile json mode to succeed, got %d and stderr %q", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr for json mode, got %q", stderr.String())
	}

	var envelope output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("expected valid json output, got %v", err)
	}
	result := envelope.Result
	if result.Summary != "workflow state already normalized for todo-app-demo" {
		t.Fatalf("unexpected summary: %q", result.Summary)
	}
	if len(result.ChangedFiles) != 0 {
		t.Fatalf("expected no changed files, got %#v", result.ChangedFiles)
	}
	if result.CurrentPhase != "tasks" {
		t.Fatalf("expected tasks phase, got %q", result.CurrentPhase)
	}
	if result.NextAction != "Start execution from the next unchecked task" {
		t.Fatalf("unexpected next action: %q", result.NextAction)
	}
}

func TestRunReconcilePrintsJSONFailureForMissingFeature(t *testing.T) {
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

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"reconcile", "Todo App Demo", "--json"}, &stdout, &stderr)

	if exitCode != 1 {
		t.Fatalf("expected missing feature reconcile to fail, got %d", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr for json mode, got %q", stderr.String())
	}

	var envelope output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("expected valid json output, got %v", err)
	}
	result := envelope.Result
	if !strings.Contains(result.Summary, `feature "todo-app-demo" does not exist`) {
		t.Fatalf("unexpected summary: %q", result.Summary)
	}
	if result.NextAction != "Run walden feature init todo-app-demo" {
		t.Fatalf("unexpected next action: %q", result.NextAction)
	}
	if result.ExitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", result.ExitCode)
	}
}
