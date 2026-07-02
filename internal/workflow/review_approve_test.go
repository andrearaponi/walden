package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/spec"
)

func TestApproveReviewRequirementsSetsApprovedTimestampAndFingerprint(t *testing.T) {
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
	if !spec.ValidFingerprint(feature.Requirements.ApprovedFingerprint) {
		t.Fatalf("expected a valid approval fingerprint, got %q", feature.Requirements.ApprovedFingerprint)
	}
	if !spec.BodyMatchesFingerprint(feature.Requirements.Body, feature.Requirements.ApprovedFingerprint) {
		t.Fatal("recorded approval fingerprint does not match the approved body")
	}
	if result.Document != ".walden/specs/todo-app-demo/requirements.md" {
		t.Fatalf("unexpected approved document %q", result.Document)
	}
	if result.NextAction != "Create design.md" {
		t.Fatalf("unexpected next action %q", result.NextAction)
	}
}

func TestApproveReviewDesignCopiesUpstreamApprovalTimestampAndFingerprint(t *testing.T) {
	root := t.TempDir()
	writeApproveFeatureDoc(t, root, "todo-app-demo", "requirements.md", approvedRequirementsContent(approveReqBody, "2026-03-21T14:00:00Z"))
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
	if feature.Design.SourceRequirementsFingerprint != spec.Fingerprint(approveReqBody) {
		t.Fatalf("source requirements fingerprint %q does not match upstream approval fingerprint", feature.Design.SourceRequirementsFingerprint)
	}
	if !spec.BodyMatchesFingerprint(feature.Design.Body, feature.Design.ApprovedFingerprint) {
		t.Fatal("recorded design approval fingerprint does not match the approved body")
	}
	if !timestampLike(feature.Design.ApprovedAt) {
		t.Fatalf("expected approved_at timestamp, got %q", feature.Design.ApprovedAt)
	}
	if result.NextAction != "Create tasks.md" {
		t.Fatalf("unexpected next action %q", result.NextAction)
	}
}

func TestApproveReviewTasksCopiesUpstreamApprovalTimestampAndFingerprint(t *testing.T) {
	root := t.TempDir()
	writeApproveFeatureDoc(t, root, "todo-app-demo", "requirements.md", approvedRequirementsContent(approveReqBody, "2026-03-21T14:00:00Z"))
	writeApproveFeatureDoc(t, root, "todo-app-demo", "design.md", approvedDesignContent(approveDesignBody, "2026-03-21T14:10:00Z", "2026-03-21T14:00:00Z", spec.Fingerprint(approveReqBody)))
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
	if feature.Tasks.SourceDesignFingerprint != spec.Fingerprint(approveDesignBody) {
		t.Fatalf("source design fingerprint %q does not match upstream approval fingerprint", feature.Tasks.SourceDesignFingerprint)
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
	writeApproveFeatureDoc(t, root, "todo-app-demo", "requirements.md", approvedRequirementsContent(approveReqBody, "2026-03-21T15:00:00Z"))
	// Design was approved against different requirements content: chain mismatch.
	writeApproveFeatureDoc(t, root, "todo-app-demo", "design.md", approvedDesignContent(approveDesignBody, "2026-03-21T14:10:00Z", "2026-03-21T14:00:00Z", spec.Fingerprint("some earlier requirements content")))
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
	if !strings.Contains(err.Error(), spec.CauseSourceMismatch) {
		t.Fatalf("expected cause %q in error, got %v", spec.CauseSourceMismatch, err)
	}
}

func TestApproveReviewBlocksTamperedUpstream(t *testing.T) {
	root := t.TempDir()
	// Approved requirements whose body was edited after approval: the
	// recorded fingerprint no longer matches the content.
	tampered := fmt.Sprintf(`---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
approved_fingerprint: %s
---

# Requirements Document

Injected after approval.
`, spec.Fingerprint(approveReqBody))
	writeApproveFeatureDoc(t, root, "todo-app-demo", "requirements.md", tampered)
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
		t.Fatal("expected design approval over a tampered upstream to fail")
	}
	if !strings.Contains(err.Error(), spec.CauseContentMismatch) {
		t.Fatalf("expected cause %q in error, got %v", spec.CauseContentMismatch, err)
	}
	if !strings.Contains(err.Error(), "walden reconcile") {
		t.Fatalf("expected reconcile hint in error, got %v", err)
	}
}

func TestApproveReviewBlocksLegacyUpstreamWithoutFingerprint(t *testing.T) {
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

	_, err := ApproveReview(root, "todo-app-demo", PhaseDesign)
	if err == nil {
		t.Fatal("expected design approval over a legacy upstream to fail")
	}
	if !strings.Contains(err.Error(), spec.CauseFingerprintMissing) {
		t.Fatalf("expected cause %q in error, got %v", spec.CauseFingerprintMissing, err)
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
