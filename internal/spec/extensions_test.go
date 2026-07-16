package spec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func extensionDocument(path string) Document {
	return Document{
		Path:   path,
		Status: "draft",
		Fields: map[string]string{
			"status":               "draft",
			"approved_at":          "",
			"last_modified":        "2026-07-16T10:00:00Z",
			"approved_fingerprint": "",
			"x-review-url":         "https://example.test/pr/42",
			"x-approver":           "team-platform",
		},
		Exists: true,
		Body:   "# Requirements Document\n\nBody.\n",
	}
}

func TestSaveDocumentPreservesExtensions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "requirements.md")

	if err := SaveDocument(extensionDocument(path)); err != nil {
		t.Fatalf("SaveDocument: %v", err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved document: %v", err)
	}

	// Extensions come after the core keys, lexicographically ordered.
	content := string(first)
	frontmatter := content[:strings.Index(content, "\n---\n")]
	approverIndex := strings.Index(frontmatter, "x-approver: team-platform")
	reviewIndex := strings.Index(frontmatter, "x-review-url: https://example.test/pr/42")
	coreIndex := strings.Index(frontmatter, "approved_fingerprint:")
	if approverIndex < 0 || reviewIndex < 0 {
		t.Fatalf("extensions missing from serialized frontmatter:\n%s", frontmatter)
	}
	if !(coreIndex < approverIndex && approverIndex < reviewIndex) {
		t.Fatalf("extension ordering wrong (core=%d, approver=%d, review=%d):\n%s", coreIndex, approverIndex, reviewIndex, frontmatter)
	}

	// Round trip: values verbatim after reload.
	loaded, err := loadDocument(path)
	if err != nil {
		t.Fatalf("loadDocument: %v", err)
	}
	if loaded.Fields["x-review-url"] != "https://example.test/pr/42" || loaded.Fields["x-approver"] != "team-platform" {
		t.Fatalf("extension values not preserved verbatim: %v", loaded.Fields)
	}

	// Determinism: saving the reloaded document produces identical bytes.
	if err := SaveDocument(loaded); err != nil {
		t.Fatalf("second SaveDocument: %v", err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read re-saved document: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("serialization is not byte-deterministic:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

func TestLoaderRejectsUnknownField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "requirements.md")
	content := "---\nstatus: draft\napproved_at: \nlast_modified: 2026-07-16T10:00:00Z\napproved_fingerprint: \naproved_at: 2026-07-16T10:00:00Z\n---\n\n# Requirements Document\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write document: %v", err)
	}

	_, err := loadDocument(path)
	if err == nil {
		t.Fatal("expected the loader to reject the unknown field")
	}
	for _, fragment := range []string{`"aproved_at"`, `"x-aproved_at"`, "or remove it", path} {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("rejection missing %q: %v", fragment, err)
		}
	}
}

func TestVersionErrorPrecedesUnknownField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "requirements.md")
	content := "---\nwalden_schema_version: v9alpha9\nstatus: draft\napproved_at: \nlast_modified: 2026-07-16T10:00:00Z\napproved_fingerprint: \nstray_field: junk\n---\n\n# Requirements Document\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write document: %v", err)
	}

	_, err := loadDocument(path)
	if err == nil {
		t.Fatal("expected a refusal")
	}
	if !strings.Contains(err.Error(), "walden_schema_version") {
		t.Fatalf("expected the schema-version error first, got %v", err)
	}
	if strings.Contains(err.Error(), "unknown frontmatter field") {
		t.Fatalf("unknown-field error must not precede the version error: %v", err)
	}
}

func TestApprovalFingerprintIgnoresFrontmatter(t *testing.T) {
	dir := t.TempDir()
	plainPath := filepath.Join(dir, "plain", "requirements.md")
	extendedPath := filepath.Join(dir, "extended", "requirements.md")

	plain := extensionDocument(plainPath)
	delete(plain.Fields, "x-review-url")
	delete(plain.Fields, "x-approver")
	extended := extensionDocument(extendedPath)

	for _, document := range []Document{plain, extended} {
		if err := SaveDocument(document); err != nil {
			t.Fatalf("SaveDocument: %v", err)
		}
	}
	loadedPlain, err := loadDocument(plainPath)
	if err != nil {
		t.Fatalf("load plain: %v", err)
	}
	loadedExtended, err := loadDocument(extendedPath)
	if err != nil {
		t.Fatalf("load extended: %v", err)
	}

	if Fingerprint(loadedPlain.Path, loadedPlain.Body) != Fingerprint(loadedExtended.Path, loadedExtended.Body) {
		t.Fatal("frontmatter extensions leaked into the body fingerprint")
	}
}
