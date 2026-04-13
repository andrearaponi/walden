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

func TestRunReviewOpenPrintsReviewContext(t *testing.T) {
	root := t.TempDir()
	writeReviewCommandFile(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeReviewCommandFile(t, root, "todo-app-demo", "design.md", `---
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

	exitCode := Run([]string{"review", "open", "todo-app-demo", "--phase", "design"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected review open to succeed, got %d and stderr %q", exitCode, stderr.String())
	}

	rendered := stdout.String()
	for _, want := range []string{
		"review gate opened for design.md",
		"Branch: design/todo-app-demo",
		"Document: .walden/specs/todo-app-demo/design.md",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected output to contain %q, got %q", want, rendered)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunReviewOpenPrintsJSON(t *testing.T) {
	root := t.TempDir()
	writeReviewCommandFile(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeReviewCommandFile(t, root, "todo-app-demo", "design.md", `---
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

	exitCode := Run([]string{"review", "open", "todo-app-demo", "--phase", "design", "--json"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected review open --json to succeed, got %d and stderr %q", exitCode, stderr.String())
	}

	var envelope output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("expected valid json, got %v", err)
	}
	if envelope.Command != "review-open" {
		t.Fatalf("expected command review-open, got %q", envelope.Command)
	}
	if !envelope.OK {
		t.Fatalf("expected ok=true, got false")
	}
}

func writeReviewCommandFile(t *testing.T, root, feature, name, content string) {
	t.Helper()

	featureDir := filepath.Join(root, ".walden", "specs", feature)
	if err := os.MkdirAll(featureDir, 0o755); err != nil {
		t.Fatalf("expected feature directory creation to succeed, got %v", err)
	}
	if err := os.WriteFile(filepath.Join(featureDir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("expected write for %q to succeed, got %v", name, err)
	}
}
