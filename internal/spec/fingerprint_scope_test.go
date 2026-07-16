package spec

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestFingerprintScopesCheckboxNormalization(t *testing.T) {
	checked := "# Document\n\n- [x] 1. Item\n"
	unchecked := "# Document\n\n- [ ] 1. Item\n"

	// tasks.md: checkbox state is execution progress — identical fingerprints.
	if Fingerprint("specs/f/tasks.md", checked) != Fingerprint("specs/f/tasks.md", unchecked) {
		t.Fatal("tasks.md checkbox state moved the fingerprint")
	}

	// requirements.md and design.md: a checkbox is content — state distinguishes.
	if Fingerprint("specs/f/requirements.md", checked) == Fingerprint("specs/f/requirements.md", unchecked) {
		t.Fatal("requirements.md checkbox state did not move the fingerprint")
	}
	if Fingerprint("specs/f/design.md", checked) == Fingerprint("specs/f/design.md", unchecked) {
		t.Fatal("design.md checkbox state did not move the fingerprint")
	}

	// Marker-free bodies keep the pre-scoping digest in every document type:
	// the normalization pipeline reduces to line endings plus trim, so the
	// digest equals the direct hash of the trimmed body.
	body := "# Requirements Document\n\nNo markers here.\n"
	sum := sha256.Sum256([]byte(strings.TrimSpace(body)))
	expected := "sha256:" + hex.EncodeToString(sum[:])
	for _, documentPath := range []string{"requirements.md", "design.md", "tasks.md"} {
		if got := Fingerprint(documentPath, body); got != expected {
			t.Fatalf("marker-free fingerprint moved for %s: %s != %s", documentPath, got, expected)
		}
	}
}
