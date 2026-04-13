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

func TestRunTaskCompletePrintsHumanReadableSuccess(t *testing.T) {
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

	previousRunner := commandRunner
	commandRunner = testutil.NewFakeRunner(testutil.Response{Stdout: "ok", ExitCode: 0})
	t.Cleanup(func() {
		commandRunner = previousRunner
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"task", "complete", "todo-app-demo", "1.1"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected task complete to succeed, got %d and stderr %q", exitCode, stderr.String())
	}

	rendered := stdout.String()
	for _, want := range []string{
		"Summary: task completed for todo-app-demo",
		"Task: 1.1 Implement parser",
		"Changed files:",
		".walden/specs/todo-app-demo/tasks.md",
		"Next action: No runnable tasks remain; implementation plan is complete",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected output to contain %q, got %q", want, rendered)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunTaskCompletePrintsJSONFailureWhenProofFails(t *testing.T) {
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

	previousRunner := commandRunner
	commandRunner = testutil.NewFakeRunner(testutil.Response{Stderr: "proof failed", ExitCode: 1})
	t.Cleanup(func() {
		commandRunner = previousRunner
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"task", "complete", "todo-app-demo", "1.1", "--json"}, &stdout, &stderr)

	if exitCode != 1 {
		t.Fatalf("expected task complete to fail, got %d", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr for json mode, got %q", stderr.String())
	}

	var envelope output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("expected valid json output, got %v", err)
	}
	result := envelope.Result
	if result.ExitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", result.ExitCode)
	}
	if !strings.Contains(result.Summary, `verification failed for task "1.1"`) {
		t.Fatalf("unexpected summary: %q", result.Summary)
	}
}
