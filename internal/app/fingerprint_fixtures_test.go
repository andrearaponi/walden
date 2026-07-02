package app

import (
	"testing"

	"github.com/andrearaponi/walden/internal/spec"
)

// stampSpecFingerprint keeps approved fixtures fingerprint-fresh: it records
// the document's own approval fingerprint and binds it to the upstream's,
// mirroring what `review approve` does in production. Tests that need
// staleness rewrite the file afterwards or corrupt the fingerprint fields.
func stampSpecFingerprint(t *testing.T, root, featureName, name string) {
	t.Helper()

	feature, err := spec.LoadFeature(root, featureName)
	if err != nil {
		t.Fatalf("expected fixture feature load to succeed, got %v", err)
	}

	var document *spec.Document
	switch name {
	case "requirements.md":
		document = &feature.Requirements
	case "design.md":
		document = &feature.Design
	case "tasks.md":
		document = &feature.Tasks
	default:
		return
	}

	if !document.Exists || document.Status != "approved" {
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

// overrideSpecFingerprintField injects a fingerprint mismatch into a fixture,
// for tests that need a stale chain.
func overrideSpecFingerprintField(t *testing.T, root, featureName, name, key, value string) {
	t.Helper()

	feature, err := spec.LoadFeature(root, featureName)
	if err != nil {
		t.Fatalf("expected fixture feature load to succeed, got %v", err)
	}

	var document *spec.Document
	switch name {
	case "requirements.md":
		document = &feature.Requirements
	case "design.md":
		document = &feature.Design
	case "tasks.md":
		document = &feature.Tasks
	}
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
