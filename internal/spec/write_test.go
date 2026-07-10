package spec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func requirementsDocument(dir, body string) Document {
	return Document{
		Path: filepath.Join(dir, "requirements.md"),
		Fields: map[string]string{
			"status":               "draft",
			"approved_at":          "",
			"last_modified":        "2026-07-10T00:00:00Z",
			"approved_fingerprint": "",
		},
		Body: body,
	}
}

func TestSaveDocumentWritesContentWithoutStagingLeftovers(t *testing.T) {
	dir := t.TempDir()
	document := requirementsDocument(dir, "# Requirements Document\n")

	if err := SaveDocument(document); err != nil {
		t.Fatalf("SaveDocument returned error: %v", err)
	}

	content, err := os.ReadFile(document.Path)
	if err != nil {
		t.Fatalf("read saved document: %v", err)
	}
	if !strings.Contains(string(content), "# Requirements Document") {
		t.Fatalf("saved content = %q, want the document body", content)
	}
	if !strings.HasPrefix(string(content), "---\n") {
		t.Fatalf("saved content does not start with frontmatter: %q", content)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read document dir: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".walden-doc-") {
			t.Fatalf("staging leftover %s after a successful save", entry.Name())
		}
	}
	if len(entries) != 1 {
		t.Fatalf("directory holds %d entries, want only the document", len(entries))
	}
}

func TestSaveDocumentFailureLeavesExistingDocumentUntouched(t *testing.T) {
	dir := t.TempDir()
	document := requirementsDocument(dir, "ORIGINAL BODY\n")

	if err := SaveDocument(document); err != nil {
		t.Fatalf("seed original document: %v", err)
	}
	original, err := os.ReadFile(document.Path)
	if err != nil {
		t.Fatalf("read original: %v", err)
	}

	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("make directory read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	document.Body = "REPLACEMENT BODY\n"
	saveErr := SaveDocument(document)
	if saveErr == nil {
		t.Fatal("SaveDocument succeeded in a read-only directory")
	}
	if !strings.Contains(saveErr.Error(), document.Path) && !strings.Contains(saveErr.Error(), "requirements.md") {
		t.Fatalf("error %q does not name the affected document", saveErr)
	}

	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatalf("restore directory mode: %v", err)
	}
	after, err := os.ReadFile(document.Path)
	if err != nil {
		t.Fatalf("read document after failed save: %v", err)
	}
	if string(after) != string(original) {
		t.Fatalf("document changed by a failed save:\nbefore: %q\nafter:  %q", original, after)
	}

	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".walden-doc-") {
			t.Fatalf("staging leftover %s after a failed save", entry.Name())
		}
	}
}

func TestSaveDocumentReplacesExistingContentAtomically(t *testing.T) {
	dir := t.TempDir()
	document := requirementsDocument(dir, "FIRST\n")

	if err := SaveDocument(document); err != nil {
		t.Fatalf("first save: %v", err)
	}
	document.Body = "SECOND\n"
	if err := SaveDocument(document); err != nil {
		t.Fatalf("second save: %v", err)
	}

	content, err := os.ReadFile(document.Path)
	if err != nil {
		t.Fatalf("read document: %v", err)
	}
	if !strings.Contains(string(content), "SECOND") || strings.Contains(string(content), "FIRST") {
		t.Fatalf("content = %q, want the replacement body only", content)
	}

	info, err := os.Stat(document.Path)
	if err != nil {
		t.Fatalf("stat document: %v", err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("document mode = %v, want 0644", info.Mode().Perm())
	}
}
