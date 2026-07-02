package spec

import "testing"

func TestResetDocumentToDraftClearsFingerprintFields(t *testing.T) {
	document := Document{
		Path:                          "design.md",
		Status:                        "approved",
		ApprovedAt:                    "2026-03-21T14:10:00Z",
		LastModified:                  "2026-03-21T14:10:00Z",
		ApprovedFingerprint:           Fingerprint("# Feature Design\n"),
		SourceRequirementsApprovedAt:  "2026-03-21T14:00:00Z",
		SourceRequirementsFingerprint: Fingerprint("# Requirements Document\n"),
		Exists:                        true,
		Body:                          "# Feature Design\n",
		Fields: map[string]string{
			"status":                          "approved",
			"approved_at":                     "2026-03-21T14:10:00Z",
			"last_modified":                   "2026-03-21T14:10:00Z",
			"approved_fingerprint":            Fingerprint("# Feature Design\n"),
			"source_requirements_approved_at": "2026-03-21T14:00:00Z",
			"source_requirements_fingerprint": Fingerprint("# Requirements Document\n"),
		},
	}

	updated, err := ResetDocumentToDraft(document, "2026-03-21T18:50:00Z")
	if err != nil {
		t.Fatalf("expected reset to draft to succeed, got %v", err)
	}

	if updated.ApprovedFingerprint != "" || updated.Fields["approved_fingerprint"] != "" {
		t.Fatalf("expected approval fingerprint to be cleared, got %q / %q", updated.ApprovedFingerprint, updated.Fields["approved_fingerprint"])
	}
	if updated.SourceRequirementsFingerprint != "" || updated.Fields["source_requirements_fingerprint"] != "" {
		t.Fatalf("expected source fingerprint to be cleared, got %q / %q", updated.SourceRequirementsFingerprint, updated.Fields["source_requirements_fingerprint"])
	}
}

func TestResetDocumentToDraftClearsInvalidApprovalMetadata(t *testing.T) {
	tests := []struct {
		name        string
		document    Document
		wantSource  string
		sourceField string
	}{
		{
			name: "design",
			document: Document{
				Path:                         "design.md",
				Status:                       "approved",
				ApprovedAt:                   "2026-03-21T14:10:00Z",
				LastModified:                 "2026-03-21T14:10:00Z",
				SourceRequirementsApprovedAt: "2026-03-21T14:00:00Z",
				Exists:                       true,
				Body:                         "# Feature Design\n",
				Fields: map[string]string{
					"status":                          "approved",
					"approved_at":                     "2026-03-21T14:10:00Z",
					"last_modified":                   "2026-03-21T14:10:00Z",
					"source_requirements_approved_at": "2026-03-21T14:00:00Z",
				},
			},
			wantSource:  "",
			sourceField: "source_requirements_approved_at",
		},
		{
			name: "tasks",
			document: Document{
				Path:                   "tasks.md",
				Status:                 "approved",
				ApprovedAt:             "2026-03-21T14:20:00Z",
				LastModified:           "2026-03-21T14:20:00Z",
				SourceDesignApprovedAt: "2026-03-21T14:10:00Z",
				Exists:                 true,
				Body:                   "# Implementation Plan\n",
				Fields: map[string]string{
					"status":                    "approved",
					"approved_at":               "2026-03-21T14:20:00Z",
					"last_modified":             "2026-03-21T14:20:00Z",
					"source_design_approved_at": "2026-03-21T14:10:00Z",
				},
			},
			wantSource:  "",
			sourceField: "source_design_approved_at",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			updated, err := ResetDocumentToDraft(test.document, "2026-03-21T18:50:00Z")
			if err != nil {
				t.Fatalf("expected reset to draft to succeed, got %v", err)
			}

			if updated.Status != "draft" {
				t.Fatalf("expected draft status, got %q", updated.Status)
			}
			if updated.ApprovedAt != "" {
				t.Fatalf("expected cleared approved_at, got %q", updated.ApprovedAt)
			}
			if updated.LastModified != "2026-03-21T18:50:00Z" {
				t.Fatalf("unexpected last_modified: %q", updated.LastModified)
			}
			if updated.Fields["status"] != "draft" {
				t.Fatalf("unexpected status field: %q", updated.Fields["status"])
			}
			if updated.Fields["approved_at"] != "" {
				t.Fatalf("expected empty approved_at field, got %q", updated.Fields["approved_at"])
			}
			if updated.Fields[test.sourceField] != test.wantSource {
				t.Fatalf("unexpected source field %q: %q", test.sourceField, updated.Fields[test.sourceField])
			}
		})
	}
}
