package app

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/spec"
)

func TestVersionReportsDocumentSchema(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if exitCode := Run([]string{"version", "--json"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr: %s)", exitCode, stderr.String())
	}

	var envelope struct {
		Result struct {
			Summary               string `json:"summary"`
			DocumentSchemaVersion string `json:"document_schema_version"`
		} `json:"result"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("parse version envelope: %v", err)
	}
	if envelope.Result.DocumentSchemaVersion != spec.DocumentSchemaVersion {
		t.Fatalf("document_schema_version = %q, want %q", envelope.Result.DocumentSchemaVersion, spec.DocumentSchemaVersion)
	}
	if !strings.Contains(envelope.Result.Summary, spec.DocumentSchemaVersion) {
		t.Fatalf("summary does not mention the document schema: %q", envelope.Result.Summary)
	}
}
