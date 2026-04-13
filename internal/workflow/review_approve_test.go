package workflow

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/spec"
)

func TestApproveReviewRequirementsSetsApprovedTimestamp(t *testing.T) {
	root := t.TempDir()
	writeApproveFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: in-review
approved_at:
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)

	result, err := ApproveReview(root, "todo-app-demo", PhaseRequirements)
	if err != nil {
		t.Fatalf("expected requirements approval to succeed, got %v", err)
	}

	feature, err := spec.LoadFeature(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("expected feature reload to succeed, got %v", err)
	}

	if feature.Requirements.Status != "approved" {
		t.Fatalf("expected requirements status approved, got %q", feature.Requirements.Status)
	}
	if !timestampLike(feature.Requirements.ApprovedAt) {
		t.Fatalf("expected approved_at timestamp, got %q", feature.Requirements.ApprovedAt)
	}
	if feature.Requirements.LastModified != feature.Requirements.ApprovedAt {
		t.Fatalf("expected last_modified to match approved_at, got %q vs %q", feature.Requirements.LastModified, feature.Requirements.ApprovedAt)
	}
	if result.Document != ".walden/specs/todo-app-demo/requirements.md" {
		t.Fatalf("unexpected approved document %q", result.Document)
	}
	if result.NextAction != "Create design.md" {
		t.Fatalf("unexpected next action %q", result.NextAction)
	}
}

func TestApproveReviewDesignCopiesUpstreamApprovalTimestamp(t *testing.T) {
	root := t.TempDir()
	writeApproveFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeApproveFeatureDoc(t, root, "todo-app-demo", "design.md", `---
status: in-review
approved_at:
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at:
---

# Feature Design
`)

	result, err := ApproveReview(root, "todo-app-demo", PhaseDesign)
	if err != nil {
		t.Fatalf("expected design approval to succeed, got %v", err)
	}

	feature, err := spec.LoadFeature(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("expected feature reload to succeed, got %v", err)
	}

	if feature.Design.Status != "approved" {
		t.Fatalf("expected design status approved, got %q", feature.Design.Status)
	}
	if feature.Design.SourceRequirementsApprovedAt != "2026-03-21T14:00:00Z" {
		t.Fatalf("unexpected source requirements approval timestamp %q", feature.Design.SourceRequirementsApprovedAt)
	}
	if !timestampLike(feature.Design.ApprovedAt) {
		t.Fatalf("expected approved_at timestamp, got %q", feature.Design.ApprovedAt)
	}
	if result.NextAction != "Create tasks.md" {
		t.Fatalf("unexpected next action %q", result.NextAction)
	}
}

func TestApproveReviewTasksCopiesUpstreamApprovalTimestamp(t *testing.T) {
	root := t.TempDir()
	writeApproveFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeApproveFeatureDoc(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-21T14:10:00Z
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at: 2026-03-21T14:00:00Z
---

# Feature Design
`)
	writeApproveFeatureDoc(t, root, "todo-app-demo", "tasks.md", `---
status: in-review
approved_at:
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at:
---

# Implementation Plan
`)

	result, err := ApproveReview(root, "todo-app-demo", PhaseTasks)
	if err != nil {
		t.Fatalf("expected tasks approval to succeed, got %v", err)
	}

	feature, err := spec.LoadFeature(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("expected feature reload to succeed, got %v", err)
	}

	if feature.Tasks.Status != "approved" {
		t.Fatalf("expected tasks status approved, got %q", feature.Tasks.Status)
	}
	if feature.Tasks.SourceDesignApprovedAt != "2026-03-21T14:10:00Z" {
		t.Fatalf("unexpected source design approval timestamp %q", feature.Tasks.SourceDesignApprovedAt)
	}
	if result.NextAction != "Start execution from the next unchecked task" {
		t.Fatalf("unexpected next action %q", result.NextAction)
	}
}

func TestApproveReviewBlocksWhenPrerequisitesAreNotApproved(t *testing.T) {
	root := t.TempDir()
	writeApproveFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: draft
approved_at:
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeApproveFeatureDoc(t, root, "todo-app-demo", "design.md", `---
status: in-review
approved_at:
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at:
---

# Feature Design
`)

	_, err := ApproveReview(root, "todo-app-demo", PhaseDesign)
	if err == nil {
		t.Fatal("expected design approval to fail")
	}
	if !strings.Contains(err.Error(), "requirements.md must be approved before approving design review") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestApproveReviewBlocksWhenDesignIsStaleForTasksApproval(t *testing.T) {
	root := t.TempDir()
	writeApproveFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T15:00:00Z
last_modified: 2026-03-21T15:00:00Z
---

# Requirements Document
`)
	writeApproveFeatureDoc(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-21T14:10:00Z
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at: 2026-03-21T14:00:00Z
---

# Feature Design
`)
	writeApproveFeatureDoc(t, root, "todo-app-demo", "tasks.md", `---
status: in-review
approved_at:
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at:
---

# Implementation Plan
`)

	_, err := ApproveReview(root, "todo-app-demo", PhaseTasks)
	if err == nil {
		t.Fatal("expected tasks approval to fail")
	}
	if !strings.Contains(err.Error(), "design.md is stale relative to requirements.md") {
		t.Fatalf("unexpected error %v", err)
	}
}

func writeApproveFeatureDoc(t *testing.T, root, feature, name, content string) {
	t.Helper()

	featureDir := filepath.Join(root, ".walden", "specs", feature)
	if err := os.MkdirAll(featureDir, 0o755); err != nil {
		t.Fatalf("expected feature directory creation to succeed, got %v", err)
	}
	if err := os.WriteFile(filepath.Join(featureDir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("expected write for %q to succeed, got %v", name, err)
	}
}

func timestampLike(value string) bool {
	return regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`).MatchString(value)
}
