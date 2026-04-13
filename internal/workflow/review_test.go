package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenReviewTransitionsDocumentToInReviewAndReturnsContext(t *testing.T) {
	root := t.TempDir()
	writeReviewFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeReviewFeatureDoc(t, root, "todo-app-demo", "design.md", `---
status: draft
approved_at:
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at:
---

# Feature Design
`)

	context, err := OpenReview(root, "todo-app-demo", PhaseDesign)
	if err != nil {
		t.Fatalf("expected review open to succeed, got %v", err)
	}

	if context.BranchName != "design/todo-app-demo" {
		t.Fatalf("unexpected branch name: %q", context.BranchName)
	}
	if context.Document != ".walden/specs/todo-app-demo/design.md" {
		t.Fatalf("unexpected review document: %q", context.Document)
	}

	feature, err := loadFeatureStateForReview(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("expected state reload to succeed, got %v", err)
	}
	if feature.Design.Status != "in-review" {
		t.Fatalf("expected design to move to in-review, got %q", feature.Design.Status)
	}
}

func TestOpenReviewBlocksUnapprovedUpstreamPhase(t *testing.T) {
	root := t.TempDir()
	writeReviewFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: draft
approved_at:
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeReviewFeatureDoc(t, root, "todo-app-demo", "design.md", `---
status: draft
approved_at:
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at:
---

# Feature Design
`)

	_, err := OpenReview(root, "todo-app-demo", PhaseDesign)
	if err == nil {
		t.Fatal("expected review open to fail")
	}
	if !strings.Contains(err.Error(), "requirements.md must be approved before opening design review") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOpenReviewBlocksStaleUpstreamArtifact(t *testing.T) {
	root := t.TempDir()
	writeReviewFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T15:00:00Z
last_modified: 2026-03-21T15:00:00Z
---

# Requirements Document
`)
	writeReviewFeatureDoc(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-21T14:10:00Z
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at: 2026-03-21T14:00:00Z
---

# Feature Design
`)
	writeReviewFeatureDoc(t, root, "todo-app-demo", "tasks.md", `---
status: draft
approved_at:
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at:
---

# Implementation Plan
`)

	_, err := OpenReview(root, "todo-app-demo", PhaseTasks)
	if err == nil {
		t.Fatal("expected stale review open to fail")
	}
	if !strings.Contains(err.Error(), "design.md is stale relative to requirements.md") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func loadFeatureStateForReview(root, feature string) (FeatureState, error) {
	return LoadFeatureState(root, feature)
}

func writeReviewFeatureDoc(t *testing.T, root, feature, name, content string) {
	t.Helper()

	featureDir := filepath.Join(root, ".walden", "specs", feature)
	if err := os.MkdirAll(featureDir, 0o755); err != nil {
		t.Fatalf("expected feature directory creation to succeed, got %v", err)
	}
	if err := os.WriteFile(filepath.Join(featureDir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("expected write for %q to succeed, got %v", name, err)
	}
}
