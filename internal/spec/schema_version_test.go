package spec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoaderRefusesUnsupportedSchemaVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "requirements.md")
	content := "---\nwalden_schema_version: v9alpha9\nstatus: draft\napproved_at: \nlast_modified: 2026-07-16T10:00:00Z\napproved_fingerprint: \n---\n\n# Requirements Document\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write document: %v", err)
	}

	_, err := loadDocument(path)
	if err == nil {
		t.Fatal("expected the loader to refuse an unsupported schema version")
	}
	for _, fragment := range []string{`"v9alpha9"`, `"v1alpha1"`, "upgrade the walden CLI or migrate the document", path} {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("refusal missing %q: %v", fragment, err)
		}
	}
}

func TestLoaderAcceptsCurrentSchemaVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "requirements.md")
	content := "---\nwalden_schema_version: " + DocumentSchemaVersion + "\nstatus: draft\napproved_at: \nlast_modified: 2026-07-16T10:00:00Z\napproved_fingerprint: \n---\n\n# Requirements Document\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write document: %v", err)
	}

	if _, err := loadDocument(path); err != nil {
		t.Fatalf("current schema version refused: %v", err)
	}
}

func TestLegacyDocumentsLoadAndStampOnSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "requirements.md")
	legacy := "---\nstatus: draft\napproved_at: \nlast_modified: 2026-07-16T10:00:00Z\napproved_fingerprint: \n---\n\n# Requirements Document\n"
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatalf("write legacy document: %v", err)
	}

	document, err := loadDocument(path)
	if err != nil {
		t.Fatalf("legacy document refused: %v", err)
	}

	if err := SaveDocument(document); err != nil {
		t.Fatalf("SaveDocument: %v", err)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved document: %v", err)
	}
	if !strings.HasPrefix(string(saved), "---\nwalden_schema_version: "+DocumentSchemaVersion+"\n") {
		t.Fatalf("save did not stamp the schema version first:\n%s", saved)
	}

	reloaded, err := loadDocument(path)
	if err != nil {
		t.Fatalf("reload stamped document: %v", err)
	}
	if reloaded.Fields["walden_schema_version"] != DocumentSchemaVersion {
		t.Fatalf("stamped version not visible after reload: %q", reloaded.Fields["walden_schema_version"])
	}
}
