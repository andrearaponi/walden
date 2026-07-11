package repo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitWritesWaldenGitignore(t *testing.T) {
	root := t.TempDir()

	report, err := Init(root, "v0.8.0")
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	created := false
	for _, path := range report.CreatedFiles {
		if path == ".walden/.gitignore" {
			created = true
		}
	}
	if !created {
		t.Fatalf("Init did not report .walden/.gitignore as created: %+v", report)
	}

	content, err := os.ReadFile(filepath.Join(root, ".walden", ".gitignore"))
	if err != nil {
		t.Fatalf("read .walden/.gitignore: %v", err)
	}
	if !strings.Contains(string(content), ".walden-doc-*") {
		t.Fatalf("gitignore does not exclude staging artifacts:\n%s", content)
	}
	if strings.Contains(string(content), "\nevidence") {
		t.Fatalf("gitignore excludes the evidence directory:\n%s", content)
	}
}
