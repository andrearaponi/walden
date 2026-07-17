package spec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListFeatures(t *testing.T) {
	root := t.TempDir()
	specs := filepath.Join(root, ".walden", "specs")
	for _, name := range []string{"zeta-feature", "alpha-feature", "mid-feature"} {
		if err := os.MkdirAll(filepath.Join(specs, name), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", name, err)
		}
	}
	// A stray plain file is not a feature.
	if err := os.WriteFile(filepath.Join(specs, "README.md"), []byte("notes"), 0o644); err != nil {
		t.Fatalf("write stray file: %v", err)
	}

	features, err := ListFeatures(root)
	if err != nil {
		t.Fatalf("ListFeatures: %v", err)
	}
	if strings.Join(features, ",") != "alpha-feature,mid-feature,zeta-feature" {
		t.Fatalf("expected sorted directories only, got %v", features)
	}

	if _, err := ListFeatures(t.TempDir()); err == nil {
		t.Fatal("expected an error for a repository without .walden/specs")
	}
}
