package workflow

import (
	"fmt"
	"testing"

	"github.com/andrearaponi/walden/internal/spec"
)

const (
	approveReqBody    = "# Requirements Document\n"
	approveDesignBody = "# Feature Design\n"
	approveTasksBody  = "# Implementation Plan\n"
)

func approvedRequirementsContent(body, approvedAt string) string {
	return fmt.Sprintf(`---
status: approved
approved_at: %s
last_modified: %s
approved_fingerprint: %s
---

%s`, approvedAt, approvedAt, spec.Fingerprint(body), body)
}

func approvedDesignContent(body, approvedAt, sourceApprovedAt, sourceFingerprint string) string {
	return fmt.Sprintf(`---
status: approved
approved_at: %s
last_modified: %s
approved_fingerprint: %s
source_requirements_approved_at: %s
source_requirements_fingerprint: %s
---

%s`, approvedAt, approvedAt, spec.Fingerprint(body), sourceApprovedAt, sourceFingerprint, body)
}

func approvedTasksContent(body, approvedAt, sourceApprovedAt, sourceFingerprint string) string {
	return fmt.Sprintf(`---
status: approved
approved_at: %s
last_modified: %s
approved_fingerprint: %s
source_design_approved_at: %s
source_design_fingerprint: %s
---

%s`, approvedAt, approvedAt, spec.Fingerprint(body), sourceApprovedAt, sourceFingerprint, body)
}

// writeFreshFeatureDoc writes a fixture document and, when it is approved,
// stamps a matching approval fingerprint plus the source fingerprint of its
// upstream, keeping legacy fixtures fingerprint-fresh. Tests that need
// staleness inject a mismatch afterwards via overrideFrontmatterField.
func writeFreshFeatureDoc(t *testing.T, root, featureName, name, content string) {
	t.Helper()

	writeFeatureDoc(t, root, featureName, name, content)

	feature, err := spec.LoadFeature(root, featureName)
	if err != nil {
		t.Fatalf("expected fixture feature load to succeed, got %v", err)
	}

	document := fixtureDocumentByName(&feature, name)
	if document == nil || !document.Exists || document.Status != "approved" {
		return
	}

	document.ApprovedFingerprint = spec.Fingerprint(document.Body)
	document.Fields["approved_fingerprint"] = document.ApprovedFingerprint

	switch name {
	case "design.md":
		if feature.Requirements.Status == "approved" {
			fingerprint := feature.Requirements.ApprovedFingerprint
			if fingerprint == "" {
				fingerprint = spec.Fingerprint(feature.Requirements.Body)
			}
			document.SourceRequirementsFingerprint = fingerprint
			document.Fields["source_requirements_fingerprint"] = fingerprint
		}
	case "tasks.md":
		if feature.Design.Status == "approved" {
			fingerprint := feature.Design.ApprovedFingerprint
			if fingerprint == "" {
				fingerprint = spec.Fingerprint(feature.Design.Body)
			}
			document.SourceDesignFingerprint = fingerprint
			document.Fields["source_design_fingerprint"] = fingerprint
		}
	}

	if err := spec.SaveDocument(*document); err != nil {
		t.Fatalf("expected fixture fingerprint stamp to succeed, got %v", err)
	}
}

func overrideFrontmatterField(t *testing.T, root, featureName, name, key, value string) {
	t.Helper()

	feature, err := spec.LoadFeature(root, featureName)
	if err != nil {
		t.Fatalf("expected fixture feature load to succeed, got %v", err)
	}

	document := fixtureDocumentByName(&feature, name)
	if document == nil || !document.Exists {
		t.Fatalf("fixture document %q does not exist", name)
	}

	document.Fields[key] = value
	switch key {
	case "approved_fingerprint":
		document.ApprovedFingerprint = value
	case "source_requirements_fingerprint":
		document.SourceRequirementsFingerprint = value
	case "source_design_fingerprint":
		document.SourceDesignFingerprint = value
	}

	if err := spec.SaveDocument(*document); err != nil {
		t.Fatalf("expected fixture override save to succeed, got %v", err)
	}
}

func fixtureDocumentByName(feature *spec.Feature, name string) *spec.Document {
	switch name {
	case "requirements.md":
		return &feature.Requirements
	case "design.md":
		return &feature.Design
	case "tasks.md":
		return &feature.Tasks
	default:
		return nil
	}
}
