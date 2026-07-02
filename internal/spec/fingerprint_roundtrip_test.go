package spec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFingerprintFieldsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "design.md")

	body := "# Feature Design\n\nContent under review.\n"
	fingerprint := Fingerprint(body)

	document := Document{
		Path:   path,
		Status: "approved",
		Fields: map[string]string{
			"status":                          "approved",
			"approved_at":                     "2026-07-02T08:00:00Z",
			"last_modified":                   "2026-07-02T08:00:00Z",
			"approved_fingerprint":            fingerprint,
			"source_requirements_approved_at": "2026-07-02T07:45:06Z",
			"source_requirements_fingerprint": fingerprint,
		},
		Exists: true,
		Body:   body,
	}

	if err := SaveDocument(document); err != nil {
		t.Fatalf("SaveDocument: %v", err)
	}

	loaded, err := loadDocument(path)
	if err != nil {
		t.Fatalf("loadDocument: %v", err)
	}

	if loaded.ApprovedFingerprint != fingerprint {
		t.Fatalf("ApprovedFingerprint = %q, want %q", loaded.ApprovedFingerprint, fingerprint)
	}
	if loaded.SourceRequirementsFingerprint != fingerprint {
		t.Fatalf("SourceRequirementsFingerprint = %q, want %q", loaded.SourceRequirementsFingerprint, fingerprint)
	}
	if got := Fingerprint(loaded.Body); got != fingerprint {
		t.Fatalf("body fingerprint changed across save/load round-trip: %q != %q", got, fingerprint)
	}
}

func TestFingerprintUnchangedByFrontmatterOnlyEdit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "requirements.md")

	body := "# Requirements Document\n\n1. `R1.AC1` WHEN x, the system SHALL y.\n"
	fingerprint := Fingerprint(body)

	document := Document{
		Path:   path,
		Status: "approved",
		Fields: map[string]string{
			"status":               "approved",
			"approved_at":          "2026-07-02T08:00:00Z",
			"last_modified":        "2026-07-02T08:00:00Z",
			"approved_fingerprint": fingerprint,
		},
		Exists: true,
		Body:   body,
	}
	if err := SaveDocument(document); err != nil {
		t.Fatalf("SaveDocument: %v", err)
	}

	// Frontmatter-only edit: change workflow metadata, leave the body alone.
	document.Fields["status"] = "in-review"
	document.Fields["last_modified"] = "2026-07-03T09:00:00Z"
	if err := SaveDocument(document); err != nil {
		t.Fatalf("SaveDocument after frontmatter edit: %v", err)
	}

	loaded, err := loadDocument(path)
	if err != nil {
		t.Fatalf("loadDocument: %v", err)
	}
	if got := Fingerprint(loaded.Body); got != fingerprint {
		t.Fatalf("frontmatter-only edit changed the body fingerprint: %q != %q", got, fingerprint)
	}
}

func TestFingerprintResetClearsFields(t *testing.T) {
	body := "# Implementation Plan\n\n- [ ] 1. Objective\n"
	document := Document{
		Path:                    "tasks.md",
		Status:                  "approved",
		ApprovedAt:              "2026-07-02T08:00:00Z",
		LastModified:            "2026-07-02T08:00:00Z",
		ApprovedFingerprint:     Fingerprint(body),
		SourceDesignApprovedAt:  "2026-07-02T07:50:00Z",
		SourceDesignFingerprint: Fingerprint("design body"),
		Fields: map[string]string{
			"status":                    "approved",
			"approved_at":               "2026-07-02T08:00:00Z",
			"last_modified":             "2026-07-02T08:00:00Z",
			"approved_fingerprint":      Fingerprint(body),
			"source_design_approved_at": "2026-07-02T07:50:00Z",
			"source_design_fingerprint": Fingerprint("design body"),
		},
		Exists: true,
		Body:   body,
	}

	reset, err := ResetDocumentToDraft(document, "2026-07-03T09:00:00Z")
	if err != nil {
		t.Fatalf("ResetDocumentToDraft: %v", err)
	}

	if reset.ApprovedFingerprint != "" || reset.Fields["approved_fingerprint"] != "" {
		t.Fatalf("approved fingerprint not cleared: %q / %q", reset.ApprovedFingerprint, reset.Fields["approved_fingerprint"])
	}
	if reset.SourceDesignFingerprint != "" || reset.Fields["source_design_fingerprint"] != "" {
		t.Fatalf("source design fingerprint not cleared: %q / %q", reset.SourceDesignFingerprint, reset.Fields["source_design_fingerprint"])
	}
	if reset.Status != "draft" {
		t.Fatalf("status = %q, want draft", reset.Status)
	}
}

func TestFingerprintLegacyDocumentLoadsWithoutFingerprint(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "requirements.md")

	legacy := "---\nstatus: approved\napproved_at: 2026-04-01T10:00:00Z\nlast_modified: 2026-04-01T10:00:00Z\n---\n\n# Requirements Document\n"
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatalf("write legacy document: %v", err)
	}

	loaded, err := loadDocument(path)
	if err != nil {
		t.Fatalf("loadDocument: %v", err)
	}
	if loaded.ApprovedFingerprint != "" {
		t.Fatalf("legacy document should have no fingerprint, got %q", loaded.ApprovedFingerprint)
	}
}
