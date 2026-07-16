package repo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/spec"
)

func TestScaffoldedDocumentsCarrySchemaVersion(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, ".git"))
	if _, err := Init(root, "v0.9.1"); err != nil {
		t.Fatalf("repo init: %v", err)
	}
	if _, err := InitFeature(root, "demo"); err != nil {
		t.Fatalf("feature init: %v", err)
	}

	for _, name := range []string{"requirements.md", "design.md", "tasks.md"} {
		content, err := os.ReadFile(filepath.Join(root, ".walden", "specs", "demo", name))
		if err != nil {
			t.Fatalf("read scaffolded %s: %v", name, err)
		}
		want := "walden_schema_version: " + spec.DocumentSchemaVersion + "\n"
		if !strings.Contains(string(content), want) {
			t.Fatalf("%s scaffold lacks the schema version field:\n%s", name, content)
		}
	}
}
