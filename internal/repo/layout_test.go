package repo

import "testing"

func TestDefaultManagedPathsIncludesCoreRoots(t *testing.T) {
	paths := DefaultManagedPaths()

	expected := []string{
		".walden",
		"cmd/walden",
		"internal/app",
		"internal/workflow",
	}

	for _, want := range expected {
		if _, ok := paths[want]; !ok {
			t.Fatalf("expected managed paths to include %q", want)
		}
	}
}
