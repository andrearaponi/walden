package app

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/output"
)

func TestVersionPrintsVersionAndSchema(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"version"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "walden") {
		t.Fatalf("expected output to contain binary name, got %q", out)
	}
	if !strings.Contains(out, "v0alpha1") {
		t.Fatalf("expected output to contain schema version, got %q", out)
	}
}

func TestVersionPrintsJSON(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"version", "--json"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	var envelope output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("expected valid json, got %v", err)
	}
	if envelope.SchemaVersion != "v0alpha1" {
		t.Fatalf("expected schema_version v0alpha1, got %q", envelope.SchemaVersion)
	}
	if envelope.Command != "version" {
		t.Fatalf("expected command version, got %q", envelope.Command)
	}
	if !envelope.OK {
		t.Fatalf("expected ok=true, got false")
	}
}
