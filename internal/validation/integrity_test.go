package validation

import (
	"os"
	"testing"

	"github.com/andrearaponi/walden/internal/spec"
)

func TestEvidenceIntegrityCapturedDocumentValidation(t *testing.T) {
	root := t.TempDir()
	writeValidFeature(t, root, "captured")
	feature, err := spec.LoadFeature(root, "captured")
	if err != nil {
		t.Fatal(err)
	}
	if result, err := ValidateLoadedFeature(feature, ScopeFullSpec); err != nil || !result.Valid {
		t.Fatalf("initial fixture not valid: %+v %v", result, err)
	}
	if err := os.WriteFile(feature.Requirements.Path, []byte("unrelated replacement"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := ValidateLoadedFeature(feature, ScopeFullSpec)
	if err != nil || !result.Valid {
		t.Fatalf("validator re-read a different version instead of its captured documents: %+v %v", result, err)
	}
	feature.Requirements.Body += "\nchanged captured contract\n"
	if result, err := ValidateLoadedFeature(feature, ScopeFullSpec); err == nil && result.Valid {
		t.Fatal("snapshot validator stopped checking its own approval seal")
	}
}
