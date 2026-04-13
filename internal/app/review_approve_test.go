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

func TestRunReviewApprovePrintsNextAction(t *testing.T) {
	root := t.TempDir()
	writeReviewApproveCommandFile(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeReviewApproveCommandFile(t, root, "todo-app-demo", "design.md", `---
status: in-review
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

	exitCode := Run([]string{"review", "approve", "todo-app-demo", "--phase", "design"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected review approve to succeed, got %d and stderr %q", exitCode, stderr.String())
	}

	rendered := stdout.String()
	for _, want := range []string{
		"review gate approved for design.md",
		"Document: .walden/specs/todo-app-demo/design.md",
		"Current phase: tasks",
		"Next action: Create tasks.md",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected output to contain %q, got %q", want, rendered)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunReviewApprovePrintsJSON(t *testing.T) {
	root := t.TempDir()
	writeReviewApproveCommandFile(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeReviewApproveCommandFile(t, root, "todo-app-demo", "design.md", `---
status: in-review
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

	exitCode := Run([]string{"review", "approve", "todo-app-demo", "--phase", "design", "--json"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected review approve --json to succeed, got %d and stderr %q", exitCode, stderr.String())
	}

	var envelope output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("expected valid json, got %v", err)
	}
	if envelope.Command != "review-approve" {
		t.Fatalf("expected command review-approve, got %q", envelope.Command)
	}
	if !envelope.OK {
		t.Fatalf("expected ok=true, got false")
	}
}

func writeReviewApproveCommandFile(t *testing.T, root, feature, name, content string) {
	t.Helper()

	featureDir := filepath.Join(root, ".walden", "specs", feature)
	if err := os.MkdirAll(featureDir, 0o755); err != nil {
		t.Fatalf("expected feature directory creation to succeed, got %v", err)
	}
	if err := os.WriteFile(filepath.Join(featureDir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("expected write for %q to succeed, got %v", name, err)
	}
}
