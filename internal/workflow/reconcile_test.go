package workflow

import (
	"testing"

	"github.com/andrearaponi/walden/internal/spec"
)

func TestReconcileFeatureDowngradesModifiedRequirementsAndResetsDownstream(t *testing.T) {
	root := t.TempDir()
	writeFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:30:00Z
---

# Requirements Document
`)
	writeFeatureDoc(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-21T14:10:00Z
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at: 2026-03-21T14:00:00Z
---

# Feature Design
`)
	writeFeatureDoc(t, root, "todo-app-demo", "tasks.md", `---
status: approved
approved_at: 2026-03-21T14:20:00Z
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at: 2026-03-21T14:10:00Z
---

# Implementation Plan
`)

	result, err := reconcileFeatureAt(root, "todo-app-demo", "2026-03-21T18:55:00Z")
	if err != nil {
		t.Fatalf("expected reconcile to succeed, got %v", err)
	}

	if result.CurrentPhase != PhaseRequirements {
		t.Fatalf("expected requirements phase after reconcile, got %q", result.CurrentPhase)
	}
	if result.NextAction != "Approve requirements.md" {
		t.Fatalf("unexpected next action: %q", result.NextAction)
	}
	if len(result.ChangedDocs) != 3 {
		t.Fatalf("expected 3 changed docs, got %#v", result.ChangedDocs)
	}

	feature, err := spec.LoadFeature(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("expected feature reload to succeed, got %v", err)
	}

	if feature.Requirements.Status != "in-review" {
		t.Fatalf("expected requirements to move to in-review, got %q", feature.Requirements.Status)
	}
	if feature.Requirements.ApprovedAt != "" {
		t.Fatalf("expected requirements approved_at to be cleared, got %q", feature.Requirements.ApprovedAt)
	}
	if feature.Design.Status != "draft" {
		t.Fatalf("expected design to reset to draft, got %q", feature.Design.Status)
	}
	if feature.Design.ApprovedAt != "" || feature.Design.SourceRequirementsApprovedAt != "" {
		t.Fatalf("expected design approval metadata to be cleared, got approved_at=%q source=%q", feature.Design.ApprovedAt, feature.Design.SourceRequirementsApprovedAt)
	}
	if feature.Tasks.Status != "draft" {
		t.Fatalf("expected tasks to reset to draft, got %q", feature.Tasks.Status)
	}
	if feature.Tasks.ApprovedAt != "" || feature.Tasks.SourceDesignApprovedAt != "" {
		t.Fatalf("expected tasks approval metadata to be cleared, got approved_at=%q source=%q", feature.Tasks.ApprovedAt, feature.Tasks.SourceDesignApprovedAt)
	}
}

func TestReconcileFeatureResetsDesignAndTasksWhenRequirementsApprovalTimestampMismatch(t *testing.T) {
	root := t.TempDir()
	writeFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:30:00Z
last_modified: 2026-03-21T14:30:00Z
---

# Requirements Document
`)
	writeFeatureDoc(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-21T14:40:00Z
last_modified: 2026-03-21T14:40:00Z
source_requirements_approved_at: 2026-03-21T14:00:00Z
---

# Feature Design
`)
	writeFeatureDoc(t, root, "todo-app-demo", "tasks.md", `---
status: approved
approved_at: 2026-03-21T14:50:00Z
last_modified: 2026-03-21T14:50:00Z
source_design_approved_at: 2026-03-21T14:40:00Z
---

# Implementation Plan
`)

	result, err := reconcileFeatureAt(root, "todo-app-demo", "2026-03-21T18:55:00Z")
	if err != nil {
		t.Fatalf("expected reconcile to succeed, got %v", err)
	}

	if result.CurrentPhase != PhaseDesign {
		t.Fatalf("expected design phase after reconcile, got %q", result.CurrentPhase)
	}
	if result.NextAction != "Edit design.md and move it to in-review" {
		t.Fatalf("unexpected next action: %q", result.NextAction)
	}

	feature, err := spec.LoadFeature(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("expected feature reload to succeed, got %v", err)
	}

	if feature.Requirements.Status != "approved" {
		t.Fatalf("expected requirements to remain approved, got %q", feature.Requirements.Status)
	}
	if feature.Design.Status != "draft" {
		t.Fatalf("expected design to reset to draft, got %q", feature.Design.Status)
	}
	if feature.Design.ApprovedAt != "" || feature.Design.SourceRequirementsApprovedAt != "" {
		t.Fatalf("expected design approval metadata to be cleared, got approved_at=%q source=%q", feature.Design.ApprovedAt, feature.Design.SourceRequirementsApprovedAt)
	}
	if feature.Tasks.Status != "draft" {
		t.Fatalf("expected tasks to reset to draft, got %q", feature.Tasks.Status)
	}
	if feature.Tasks.ApprovedAt != "" || feature.Tasks.SourceDesignApprovedAt != "" {
		t.Fatalf("expected tasks approval metadata to be cleared, got approved_at=%q source=%q", feature.Tasks.ApprovedAt, feature.Tasks.SourceDesignApprovedAt)
	}
}

func TestReconcileFeatureResetsTasksWhenDesignApprovalTimestampMismatch(t *testing.T) {
	root := t.TempDir()
	writeFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeFeatureDoc(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-21T14:30:00Z
last_modified: 2026-03-21T14:30:00Z
source_requirements_approved_at: 2026-03-21T14:00:00Z
---

# Feature Design
`)
	writeFeatureDoc(t, root, "todo-app-demo", "tasks.md", `---
status: approved
approved_at: 2026-03-21T14:50:00Z
last_modified: 2026-03-21T14:50:00Z
source_design_approved_at: 2026-03-21T14:10:00Z
---

# Implementation Plan
`)

	result, err := reconcileFeatureAt(root, "todo-app-demo", "2026-03-21T18:55:00Z")
	if err != nil {
		t.Fatalf("expected reconcile to succeed, got %v", err)
	}

	if result.CurrentPhase != PhaseTasks {
		t.Fatalf("expected tasks phase after reconcile, got %q", result.CurrentPhase)
	}
	if result.NextAction != "Edit tasks.md and move it to in-review" {
		t.Fatalf("unexpected next action: %q", result.NextAction)
	}

	feature, err := spec.LoadFeature(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("expected feature reload to succeed, got %v", err)
	}

	if feature.Design.Status != "approved" {
		t.Fatalf("expected design to remain approved, got %q", feature.Design.Status)
	}
	if feature.Tasks.Status != "draft" {
		t.Fatalf("expected tasks to reset to draft, got %q", feature.Tasks.Status)
	}
	if feature.Tasks.ApprovedAt != "" || feature.Tasks.SourceDesignApprovedAt != "" {
		t.Fatalf("expected tasks approval metadata to be cleared, got approved_at=%q source=%q", feature.Tasks.ApprovedAt, feature.Tasks.SourceDesignApprovedAt)
	}
}
