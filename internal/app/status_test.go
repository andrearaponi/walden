package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/output"
)

func TestRunStatusPrintsHumanReadableState(t *testing.T) {
	root := t.TempDir()
	writeStatusFeatureFile(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeStatusFeatureFile(t, root, "todo-app-demo", "design.md", `---
status: draft
approved_at:
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at:
---

# Feature Design
`)
	writeStatusFeatureFile(t, root, "todo-app-demo", "tasks.md", `---
status: draft
approved_at:
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at:
---

# Implementation Plan
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

	exitCode := Run([]string{"status", "todo-app-demo"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected status to succeed, got %d and stderr %q", exitCode, stderr.String())
	}

	rendered := stdout.String()
	for _, want := range []string{
		"Summary: workflow status for todo-app-demo",
		"Current phase: design",
		"requirements.md: status=approved fresh=true approved_at=2026-03-21T14:00:00Z",
		"design.md: status=draft fresh=true",
		"tasks.md: status=draft fresh=true",
		"Next action: Edit design.md and move it to in-review",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected output to contain %q, got %q", want, rendered)
		}
	}
	if strings.Contains(rendered, "Blockers:") {
		t.Fatalf("expected no blockers for valid feature state, got %q", rendered)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunStatusPrintsHumanReadableBlockers(t *testing.T) {
	root := t.TempDir()
	writeStatusFeatureFile(t, root, "todo-app-demo", "design.md", `---
status: draft
approved_at:
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at:
---

# Feature Design
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

	exitCode := Run([]string{"status", "todo-app-demo"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected blocked status to succeed, got %d and stderr %q", exitCode, stderr.String())
	}

	rendered := stdout.String()
	for _, want := range []string{
		"Current phase: requirements",
		"requirements.md: status=missing fresh=false",
		"design.md exists without requirements.md",
		"Next action: Create requirements.md",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected blocked output to contain %q, got %q", want, rendered)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunStatusPrintsJSONForStaleFeature(t *testing.T) {
	root := t.TempDir()
	writeStatusFeatureFile(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:30:00Z
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

	exitCode := Run([]string{"status", "todo-app-demo", "--json"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected stale status to succeed, got %d and stderr %q", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr for json mode, got %q", stderr.String())
	}

	var envelope output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("expected valid json output, got %v", err)
	}
	result := envelope.Result
	if result.CurrentPhase != "design" {
		t.Fatalf("expected current phase design, got %q", result.CurrentPhase)
	}
	if result.NextAction != "Update design.md to match requirements.md and return it to in-review" {
		t.Fatalf("unexpected next action: %q", result.NextAction)
	}
	if len(result.Blockers) != 2 {
		t.Fatalf("expected two blockers for stale feature, got %#v", result.Blockers)
	}
	if result.Documents[1].Name != "design.md" || result.Documents[1].Fresh {
		t.Fatalf("expected stale design document in json output, got %#v", result.Documents)
	}
	if result.Documents[2].Name != "tasks.md" || result.Documents[2].Fresh {
		t.Fatalf("expected stale tasks document in json output, got %#v", result.Documents)
	}
}

func TestRunStatusPrintsJSONFailureForMissingFeature(t *testing.T) {
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

	exitCode := Run([]string{"status", "Todo App Demo", "--json"}, &stdout, &stderr)

	if exitCode != 1 {
		t.Fatalf("expected missing feature status to fail, got %d", exitCode)
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
		t.Fatalf("expected exit code 1 in json output, got %d", result.ExitCode)
	}
}

func TestRunStatusPrintsCompletedExecutionPlanNextAction(t *testing.T) {
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

	exitCode := Run([]string{"status", "todo-app-demo"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected completed status to succeed, got %d and stderr %q", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}

	rendered := stdout.String()
	if !strings.Contains(rendered, "Next action: No runnable tasks remain; implementation plan is complete") {
		t.Fatalf("expected completed execution next action, got %q", rendered)
	}
}

func writeStatusFeatureFile(t *testing.T, root, feature, name, content string) {
	t.Helper()

	featureDir := filepath.Join(root, ".walden", "specs", feature)
	if err := os.MkdirAll(featureDir, 0o755); err != nil {
		t.Fatalf("expected feature directory creation to succeed, got %v", err)
	}
	if err := os.WriteFile(filepath.Join(featureDir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("expected write for %q to succeed, got %v", name, err)
	}
}
