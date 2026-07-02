package workflow

import (
	"testing"

	"github.com/andrearaponi/walden/internal/spec"
)

func TestReconcileFeatureResetsTamperedRequirementsAndDownstream(t *testing.T) {
	root := t.TempDir()
	// Approved requirements whose body was edited after approval: the
	// recorded fingerprint no longer matches the content.
	writeFeatureDoc(t, root, "todo-app-demo", "requirements.md",
		approvedRequirementsContent(approveReqBody, "2026-03-21T14:00:00Z")+"\nInjected after approval.\n")
	writeFeatureDoc(t, root, "todo-app-demo", "design.md",
		approvedDesignContent(approveDesignBody, "2026-03-21T14:10:00Z", "2026-03-21T14:00:00Z", spec.Fingerprint(approveReqBody)))
	writeFeatureDoc(t, root, "todo-app-demo", "tasks.md",
		approvedTasksContent(approveTasksBody, "2026-03-21T14:20:00Z", "2026-03-21T14:10:00Z", spec.Fingerprint(approveDesignBody)))

	result, err := reconcileFeatureAt(root, "todo-app-demo", "2026-03-21T18:55:00Z")
	if err != nil {
		t.Fatalf("expected reconcile to succeed, got %v", err)
	}

	if result.CurrentPhase != PhaseRequirements {
		t.Fatalf("expected requirements phase after reconcile, got %q", result.CurrentPhase)
	}
	if result.NextAction != "Edit requirements.md and move it to in-review" {
		t.Fatalf("unexpected next action: %q", result.NextAction)
	}
	if len(result.ChangedDocs) != 3 {
		t.Fatalf("expected 3 changed docs, got %#v", result.ChangedDocs)
	}

	feature, err := spec.LoadFeature(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("expected feature reload to succeed, got %v", err)
	}

	if feature.Requirements.Status != "draft" {
		t.Fatalf("expected tampered requirements to reset to draft, got %q", feature.Requirements.Status)
	}
	if feature.Requirements.ApprovedAt != "" || feature.Requirements.ApprovedFingerprint != "" {
		t.Fatalf("expected requirements approval metadata to be cleared, got approved_at=%q fingerprint=%q", feature.Requirements.ApprovedAt, feature.Requirements.ApprovedFingerprint)
	}
	if feature.Design.Status != "draft" {
		t.Fatalf("expected design to reset to draft, got %q", feature.Design.Status)
	}
	if feature.Design.ApprovedFingerprint != "" || feature.Design.SourceRequirementsFingerprint != "" {
		t.Fatalf("expected design fingerprints to be cleared, got own=%q source=%q", feature.Design.ApprovedFingerprint, feature.Design.SourceRequirementsFingerprint)
	}
	if feature.Tasks.Status != "draft" {
		t.Fatalf("expected tasks to reset to draft, got %q", feature.Tasks.Status)
	}
	if feature.Tasks.ApprovedFingerprint != "" || feature.Tasks.SourceDesignFingerprint != "" {
		t.Fatalf("expected tasks fingerprints to be cleared, got own=%q source=%q", feature.Tasks.ApprovedFingerprint, feature.Tasks.SourceDesignFingerprint)
	}
}

func TestReconcileFeatureResetsDesignAndTasksOnSourceFingerprintMismatch(t *testing.T) {
	root := t.TempDir()
	writeFeatureDoc(t, root, "todo-app-demo", "requirements.md",
		approvedRequirementsContent(approveReqBody, "2026-03-21T14:30:00Z"))
	// Design was approved against different requirements content.
	writeFeatureDoc(t, root, "todo-app-demo", "design.md",
		approvedDesignContent(approveDesignBody, "2026-03-21T14:40:00Z", "2026-03-21T14:00:00Z", spec.Fingerprint("some earlier requirements content")))
	writeFeatureDoc(t, root, "todo-app-demo", "tasks.md",
		approvedTasksContent(approveTasksBody, "2026-03-21T14:50:00Z", "2026-03-21T14:40:00Z", spec.Fingerprint(approveDesignBody)))

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
	if feature.Design.ApprovedAt != "" || feature.Design.SourceRequirementsFingerprint != "" {
		t.Fatalf("expected design approval metadata to be cleared, got approved_at=%q source=%q", feature.Design.ApprovedAt, feature.Design.SourceRequirementsFingerprint)
	}
	if feature.Tasks.Status != "draft" {
		t.Fatalf("expected tasks to reset to draft, got %q", feature.Tasks.Status)
	}
}

func TestReconcileFeatureResetsTasksOnSourceFingerprintMismatch(t *testing.T) {
	root := t.TempDir()
	writeFeatureDoc(t, root, "todo-app-demo", "requirements.md",
		approvedRequirementsContent(approveReqBody, "2026-03-21T14:00:00Z"))
	writeFeatureDoc(t, root, "todo-app-demo", "design.md",
		approvedDesignContent(approveDesignBody, "2026-03-21T14:30:00Z", "2026-03-21T14:00:00Z", spec.Fingerprint(approveReqBody)))
	// Tasks were approved against different design content.
	writeFeatureDoc(t, root, "todo-app-demo", "tasks.md",
		approvedTasksContent(approveTasksBody, "2026-03-21T14:50:00Z", "2026-03-21T14:10:00Z", spec.Fingerprint("some earlier design content")))

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
	if feature.Tasks.ApprovedAt != "" || feature.Tasks.SourceDesignFingerprint != "" {
		t.Fatalf("expected tasks approval metadata to be cleared, got approved_at=%q source=%q", feature.Tasks.ApprovedAt, feature.Tasks.SourceDesignFingerprint)
	}
}

func TestReconcileFeatureMigratesLegacyChainToDraft(t *testing.T) {
	root := t.TempDir()
	// A pre-fingerprint feature: everything approved, no fingerprints anywhere.
	writeFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
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

	if len(result.ChangedDocs) != 3 {
		t.Fatalf("expected the whole legacy chain to reset, got %#v", result.ChangedDocs)
	}

	feature, err := spec.LoadFeature(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("expected feature reload to succeed, got %v", err)
	}

	for name, status := range map[string]string{
		"requirements.md": feature.Requirements.Status,
		"design.md":       feature.Design.Status,
		"tasks.md":        feature.Tasks.Status,
	} {
		if status != "draft" {
			t.Fatalf("%s: expected legacy approved document to reset to draft, got %q", name, status)
		}
	}
}

func TestReconcileFeatureLeavesFreshChainUntouched(t *testing.T) {
	root := t.TempDir()
	writeFeatureDoc(t, root, "todo-app-demo", "requirements.md",
		approvedRequirementsContent(approveReqBody, "2026-03-21T14:00:00Z"))
	writeFeatureDoc(t, root, "todo-app-demo", "design.md",
		approvedDesignContent(approveDesignBody, "2026-03-21T14:10:00Z", "2026-03-21T14:00:00Z", spec.Fingerprint(approveReqBody)))
	writeFeatureDoc(t, root, "todo-app-demo", "tasks.md",
		approvedTasksContent(approveTasksBody, "2026-03-21T14:20:00Z", "2026-03-21T14:10:00Z", spec.Fingerprint(approveDesignBody)))

	result, err := reconcileFeatureAt(root, "todo-app-demo", "2026-03-21T18:55:00Z")
	if err != nil {
		t.Fatalf("expected reconcile to succeed, got %v", err)
	}

	if len(result.ChangedDocs) != 0 {
		t.Fatalf("expected no changes on a fresh chain, got %#v", result.ChangedDocs)
	}
	if result.CurrentPhase != PhaseTasks {
		t.Fatalf("expected tasks phase on a fresh chain, got %q", result.CurrentPhase)
	}
}
