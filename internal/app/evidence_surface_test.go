package app

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/andrearaponi/walden/internal/output"
	"github.com/andrearaponi/walden/internal/testutil"
)

// TestEvidenceStatusJSONCarriesPassed locks the populated union on the
// status side: recorded tasks report the stored result under the same
// `passed` field verify uses; tasks without a record omit it.
func TestEvidenceStatusJSONCarriesPassed(t *testing.T) {
	root := setupEvidenceFeature(t)

	// A second, never-completed task joins the plan; 1.1 keeps its exact
	// definition so its record stays verified.
	writeStatusFeatureFile(t, root, "todo-app-demo", "tasks.md", `---
status: approved
approved_at: 2026-03-22T07:20:00Z
last_modified: 2026-03-22T07:20:00Z
source_design_approved_at: 2026-03-22T07:10:00Z
---

# Implementation Plan

- [x] 1. Build parser
  - [x] 1.1 Implement parser
    - Requirements: `+"`R1`"+`
    - Design: Task Store
    - Verification:
      - command: ["go", "test", "./internal/spec"]
  - [ ] 1.2 Implement readiness
    - Requirements: `+"`R1`"+`
    - Design: Task Store
    - Verification:
      - command: ["go", "test", "./internal/workflow"]
`)

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"evidence", "status", "todo-app-demo", "--json"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("evidence status exited %d: %s", exitCode, stderr.String())
	}

	var envelope output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if len(envelope.Result.Evidence) != 2 {
		t.Fatalf("evidence = %+v, want two entries", envelope.Result.Evidence)
	}
	for _, entry := range envelope.Result.Evidence {
		switch entry.TaskID {
		case "1.1":
			if entry.Passed == nil || !*entry.Passed {
				t.Fatalf("recorded task 1.1 passed = %v, want true", entry.Passed)
			}
		case "1.2":
			if entry.Passed != nil {
				t.Fatalf("task without a record carries passed = %v, want omitted", *entry.Passed)
			}
		default:
			t.Fatalf("unexpected task %s", entry.TaskID)
		}
	}
}

// TestVerifyJSONCarriesIdentityAndProfile locks the populated union on the
// verify side: entries carry the identities and the profile under the field
// names evidence status uses.
func TestVerifyJSONCarriesIdentityAndProfile(t *testing.T) {
	setupEvidenceFeature(t)
	overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: "ok", ExitCode: 0}))

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"verify", "todo-app-demo", "--all", "--json"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("verify --all exited %d: %s", exitCode, stderr.String())
	}

	var envelope output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if len(envelope.Result.Evidence) != 1 {
		t.Fatalf("evidence = %+v, want one entry", envelope.Result.Evidence)
	}
	entry := envelope.Result.Evidence[0]
	if entry.RecordedIdentity == "" {
		t.Fatal("verify entry lacks recorded_identity")
	}
	if entry.CurrentIdentity != entry.RecordedIdentity {
		t.Fatalf("pure run identities diverge: recorded %q, current %q", entry.RecordedIdentity, entry.CurrentIdentity)
	}
	if entry.Profile["platform"] == "" || entry.Profile["walden"] == "" {
		t.Fatalf("verify entry profile = %v, want the reserved keys", entry.Profile)
	}
}
