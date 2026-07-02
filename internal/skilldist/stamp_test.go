package skilldist

import (
	"bytes"
	"testing"
)

func TestStampStripRoundTrip(t *testing.T) {
	original := []byte("---\nname: walden\n---\n\n# Walden\n\nBody text.\n")
	stamped := Stamp(original, "v1.2.3")

	body, version := Strip(stamped)
	if version != "v1.2.3" {
		t.Fatalf("expected version v1.2.3, got %q", version)
	}
	if !bytes.Equal(body, original) {
		t.Fatalf("round trip must restore the original content\noriginal: %q\nbody:     %q", original, body)
	}
}

func TestStampOnlyAppends(t *testing.T) {
	original := []byte("---\nname: walden\ndescription: test\n---\nbody\n")
	stamped := Stamp(original, "v0.5.0")
	if !bytes.HasPrefix(stamped, original) {
		t.Fatal("stamping must only append: the original content, frontmatter included, must be an untouched prefix")
	}
}

func TestStampContentWithoutTrailingNewline(t *testing.T) {
	stamped := Stamp([]byte("no newline"), "v1")
	body, version := Strip(stamped)
	if version != "v1" {
		t.Fatalf("expected version v1, got %q", version)
	}
	if !bytes.Equal(body, []byte("no newline\n")) {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestStripLegacyContentWithoutMarker(t *testing.T) {
	legacy := []byte("---\nname: walden\n---\nbody\n")
	body, version := Strip(legacy)
	if version != "" {
		t.Fatalf("legacy content must report an empty version, got %q", version)
	}
	if !bytes.Equal(body, legacy) {
		t.Fatal("legacy content must be returned unchanged")
	}
}

func TestStripMarkerOnlyContent(t *testing.T) {
	stamped := Stamp(nil, "v2")
	body, version := Strip(stamped)
	if version != "v2" {
		t.Fatalf("expected version v2, got %q", version)
	}
	if len(body) != 0 {
		t.Fatalf("expected empty body, got %q", body)
	}
}
