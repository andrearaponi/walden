package app

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/output"
)

func TestEvidenceStatusCommandReportsVerified(t *testing.T) {
	setupEvidenceFeature(t)

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"evidence", "status", "todo-app-demo", "--json"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("evidence status exited %d: %s", exitCode, stderr.String())
	}

	var envelope output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if envelope.Command != "evidence-status" || !envelope.OK {
		t.Fatalf("envelope = %s ok=%t", envelope.Command, envelope.OK)
	}
	if len(envelope.Result.Evidence) != 1 || envelope.Result.Evidence[0].State != "verified" {
		t.Fatalf("evidence = %+v, want one verified entry", envelope.Result.Evidence)
	}
	if !strings.Contains(envelope.Result.Summary, "1 verified") {
		t.Fatalf("summary %q lacks state counts", envelope.Result.Summary)
	}
}

func TestEvidenceStatusCommandStaysZeroOnStaleSpec(t *testing.T) {
	root := setupEvidenceFeature(t)

	// Requirements change and the chain is re-approved with fresh
	// fingerprints — the WDN-002 revival — while proofs never rerun.
	writeStatusFeatureFile(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-22T09:00:00Z
last_modified: 2026-03-22T09:00:00Z
---

# Requirements Document

## R2 Brand new behavior
`)
	writeStatusFeatureFile(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-22T09:10:00Z
last_modified: 2026-03-22T09:10:00Z
source_requirements_approved_at: 2026-03-22T09:00:00Z
---

# Feature Design
`)

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"evidence", "status", "todo-app-demo"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("report-not-gate violated: exit %d (%s)", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "stale-spec") {
		t.Fatalf("output does not surface stale-spec: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "walden verify") {
		t.Fatalf("output does not point at the remedy: %s", stdout.String())
	}
}

func TestEvidenceStatusCommandTaskStatusWarningPassthrough(t *testing.T) {
	root := setupEvidenceFeature(t)

	writeStatusFeatureFile(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-22T09:00:00Z
last_modified: 2026-03-22T09:00:00Z
---

# Requirements Document

## R2 Brand new behavior
`)
	writeStatusFeatureFile(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-22T09:10:00Z
last_modified: 2026-03-22T09:10:00Z
source_requirements_approved_at: 2026-03-22T09:00:00Z
---

# Feature Design
`)

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"task", "status", "todo-app-demo"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("task status exited %d: %s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "stale-spec") || !strings.Contains(stdout.String(), "evidence") {
		t.Fatalf("task status does not surface the evidence warning: %s", stdout.String())
	}
}
