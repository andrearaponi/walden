package spec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFeatureReadsDocumentMetadataAndMissingDocuments(t *testing.T) {
	root := t.TempDir()
	featureDir := filepath.Join(root, ".walden", "specs", "todo-app-demo")
	if err := os.MkdirAll(featureDir, 0o755); err != nil {
		t.Fatalf("expected feature directory creation to succeed, got %v", err)
	}

	requirements := `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`
	if err := os.WriteFile(filepath.Join(featureDir, "requirements.md"), []byte(requirements), 0o644); err != nil {
		t.Fatalf("expected requirements write to succeed, got %v", err)
	}

	feature, err := LoadFeature(root, "Todo App Demo")
	if err != nil {
		t.Fatalf("expected feature load to succeed, got %v", err)
	}

	if feature.Name != "todo-app-demo" {
		t.Fatalf("expected normalized feature name, got %q", feature.Name)
	}
	if !feature.Requirements.Exists {
		t.Fatal("expected requirements document to exist")
	}
	if feature.Requirements.Status != "approved" {
		t.Fatalf("expected requirements status approved, got %q", feature.Requirements.Status)
	}
	if feature.Requirements.ApprovedAt != "2026-03-21T14:00:00Z" {
		t.Fatalf("unexpected approved_at: %q", feature.Requirements.ApprovedAt)
	}
	if feature.Design.Exists {
		t.Fatal("expected design document to be reported as missing")
	}
	if feature.Tasks.Exists {
		t.Fatal("expected tasks document to be reported as missing")
	}
}

func TestLoadFeatureFailsWhenFeatureDirectoryDoesNotExist(t *testing.T) {
	root := t.TempDir()

	if _, err := LoadFeature(root, "missing-feature"); err == nil {
		t.Fatal("expected missing feature to fail")
	}
}
