package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/output"
	"github.com/andrearaponi/walden/internal/testutil"
)

func TestEvidenceStatusReportsProfileDrift(t *testing.T) {
	root := chdirContract(t)

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"repo", "init"}, &stdout, &stderr); code != 0 {
		t.Fatalf("repo init failed: %s", stderr.String())
	}
	if code := Run([]string{"feature", "init", "Todo App Demo"}, &stdout, &stderr); code != 0 {
		t.Fatalf("feature init failed: %s", stderr.String())
	}

	// A real, portable probe: the app layer exercises the true capture path.
	if err := os.WriteFile(filepath.Join(root, ".walden", "environment.md"), []byte("- stamp: [\"echo\", \"probile-e2e\"]\n"), 0o644); err != nil {
		t.Fatalf("write environment.md: %v", err)
	}

	writeStatusFeatureFile(t, root, "todo-app-demo", "requirements.md", "---\nstatus: approved\napproved_at: 2026-03-22T07:00:00Z\nlast_modified: 2026-03-22T07:00:00Z\n---\n\n# Requirements Document\n")
	writeStatusFeatureFile(t, root, "todo-app-demo", "design.md", "---\nstatus: approved\napproved_at: 2026-03-22T07:10:00Z\nlast_modified: 2026-03-22T07:10:00Z\nsource_requirements_approved_at: 2026-03-22T07:00:00Z\n---\n\n# Feature Design\n")
	writeStatusFeatureFile(t, root, "todo-app-demo", "tasks.md", "---\nstatus: approved\napproved_at: 2026-03-22T07:20:00Z\nlast_modified: 2026-03-22T07:20:00Z\nsource_design_approved_at: 2026-03-22T07:10:00Z\n---\n\n# Implementation Plan\n\n- [ ] 1. Build parser\n  - [ ] 1.1 Implement parser\n    - Requirements: `R1`\n    - Design: Task Store\n    - Verification:\n      - command: [\"go\", \"test\", \"./internal/spec\"]\n")

	overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: "ok", ExitCode: 0}))
	if code := Run([]string{"task", "complete", "todo-app-demo", "1.1"}, &stdout, &stderr); code != 0 {
		t.Fatalf("task complete failed: %s", stderr.String())
	}

	// The recorded profile reaches the JSON envelope, additively.
	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"evidence", "status", "todo-app-demo", "--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("evidence status failed: %s", stderr.String())
	}
	var envelope output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if len(envelope.Result.Evidence) != 1 {
		t.Fatalf("expected one evidence entry, got %+v", envelope.Result.Evidence)
	}
	entry := envelope.Result.Evidence[0]
	if entry.Profile["stamp"] != "probile-e2e" {
		t.Fatalf("probe output missing from JSON profile: %v", entry.Profile)
	}
	if entry.Profile["platform"] == "" || entry.Profile["walden"] == "" {
		t.Fatalf("reserved keys missing from JSON profile: %v", entry.Profile)
	}
	if entry.ProfileLegacy {
		t.Fatal("fresh record misreported as legacy")
	}

	// Strip the profile, simulating a pre-binding ledger: legacy marker.
	ledger, err := evidence.Load(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("load ledger: %v", err)
	}
	record := ledger.Tasks["1.1"]
	record.Profile = nil
	ledger.Tasks["1.1"] = record
	if err := evidence.Save(root, ledger); err != nil {
		t.Fatalf("save stripped ledger: %v", err)
	}

	stdout.Reset()
	if code := Run([]string{"evidence", "status", "todo-app-demo"}, &stdout, &stderr); code != 0 {
		t.Fatalf("evidence status (legacy) failed: %s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "(legacy record: no profile)") {
		t.Fatalf("legacy marker missing from text output:\n%s", stdout.String())
	}
}
