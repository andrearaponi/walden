package formattools

import "testing"

func TestNormalize(t *testing.T) {
	for input, want := range map[string]string{"  HELLO WORLD  ": "hello world", "Hello": "hello", " ": ""} {
		if got := Normalize(input); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSlug(t *testing.T) {
	for input, want := range map[string]string{"  HELLO  WORLD  ": "hello-world", "one\ttwo": "one-two", " ": ""} {
		if got := Slug(input); got != want {
			t.Errorf("Slug(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestLabel(t *testing.T) {
	for input, want := range map[string]string{"  HELLO  WORLD  ": "note:hello-world", " ": "note:"} {
		if got := Label(input); got != want {
			t.Errorf("Label(%q) = %q, want %q", input, got, want)
		}
	}
}
