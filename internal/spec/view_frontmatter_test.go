package spec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The current-contract view is a repository-level document with the same
// CLI-owned review frontmatter as a requirements document.
func TestViewFrontmatterOrder(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".walden", "contracts.md")
	document := Document{Path: path, Body: "# Current Contracts\n", Fields: map[string]string{
		"approved_fingerprint": "", "last_modified": "2026-10-04T08:00:00Z", "approved_at": "", "status": "draft",
	}}
	if err := SaveDocument(document); err != nil {
		t.Fatalf("save view: %v", err)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	wantHead := "---\nwalden_schema_version: " + DocumentSchemaVersion + "\nstatus: draft\napproved_at: \nlast_modified: 2026-10-04T08:00:00Z\napproved_fingerprint: \n---\n\n# Current Contracts\n"
	if string(written) != wantHead {
		t.Fatalf("view bytes =\n%q\nwant\n%q", written, wantHead)
	}
	parsed, err := ParseDocument(path, written)
	if err != nil || parsed.Status != "draft" || strings.TrimSpace(parsed.Body) != "# Current Contracts" {
		t.Fatalf("parsed view = %+v, %v", parsed, err)
	}
	if _, err := ParseDocument(path, []byte("---\nstatus: draft\nsurprise: 1\n---\n\nbody\n")); err == nil {
		t.Fatal("unknown view frontmatter field accepted")
	}
}
