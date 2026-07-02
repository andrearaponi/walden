package skill

import (
	"bytes"
	"os"
	"testing"
)

func TestContentMatchesCanonicalFile(t *testing.T) {
	onDisk, err := os.ReadFile("walden/SKILL.md")
	if err != nil {
		t.Fatalf("read canonical skill: %v", err)
	}
	if len(onDisk) == 0 {
		t.Fatal("canonical skill file is empty")
	}
	if !bytes.Equal(Content(), onDisk) {
		t.Fatal("embedded content differs from skill/walden/SKILL.md")
	}
}

func TestContentReturnsACopy(t *testing.T) {
	first := Content()
	first[0] = '!'
	if bytes.Equal(first[:1], Content()[:1]) {
		t.Fatal("mutating the returned slice must not affect the embedded content")
	}
}
