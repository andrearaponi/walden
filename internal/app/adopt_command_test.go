package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/andrearaponi/walden/internal/output"
	"github.com/andrearaponi/walden/internal/testutil"
)

func adoptFixture(t *testing.T) string {
	t.Helper()
	root := chdirContract(t)

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"repo", "init"}, &stdout, &stderr); code != 0 {
		t.Fatalf("repo init failed: %s", stderr.String())
	}
	if code := Run([]string{"feature", "init", "Old Era"}, &stdout, &stderr); code != 0 {
		t.Fatalf("feature init failed: %s", stderr.String())
	}

	// Pre-fingerprint era: approved, no fingerprints, one completed task —
	// written raw so no helper stamps a seal.
	writeRawFeatureFile(t, root, "old-era", "requirements.md", "---\nstatus: approved\napproved_at: 2026-06-09T14:00:00Z\nlast_modified: 2026-06-09T14:00:00Z\n---\n\n# Requirements Document\n")
	writeRawFeatureFile(t, root, "old-era", "design.md", "---\nstatus: approved\napproved_at: 2026-06-09T14:10:00Z\nlast_modified: 2026-06-09T14:10:00Z\nsource_requirements_approved_at: 2026-06-09T14:00:00Z\n---\n\n# Feature Design\n")
	writeRawFeatureFile(t, root, "old-era", "tasks.md", "---\nstatus: approved\napproved_at: 2026-06-09T14:20:00Z\nlast_modified: 2026-06-09T14:20:00Z\nsource_design_approved_at: 2026-06-09T14:10:00Z\n---\n\n# Implementation Plan\n\n- [x] 1. Build it\n  - [x] 1.1 Implement it\n    - Requirements: `R1`\n    - Design: Overview\n    - Verification:\n      - command: [\"go\", \"test\", \"./...\"]\n")
	return root
}

func writeRawFeatureFile(t *testing.T, root, feature, name, content string) {
	t.Helper()
	dir := filepath.Join(root, ".walden", "specs", feature)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir feature dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestAdoptCommandEnvelope(t *testing.T) {
	adoptFixture(t)

	// Plan: read-only classification in the envelope.
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"adopt", "--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("adopt plan exited %d: %s", code, stderr.String())
	}
	var envelope output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal plan envelope: %v", err)
	}
	if envelope.Result.Adoption == nil || len(envelope.Result.Adoption.Features) != 1 {
		t.Fatalf("adoption block missing: %+v", envelope.Result.Adoption)
	}
	planned := envelope.Result.Adoption.Features[0]
	if planned.Class != "backfill" || len(planned.SealableDocs) != 3 || planned.ReproveCount != 1 {
		t.Fatalf("plan = %+v", planned)
	}

	// Apply with a passing proof: sealed and verified, exit 0.
	overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: "ok", ExitCode: 0}))
	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"adopt", "--apply", "--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("adopt --apply exited %d: %s", code, stderr.String())
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal apply envelope: %v", err)
	}
	applied := envelope.Result.Adoption.Features[0]
	if len(applied.SealedDocs) != 3 || len(applied.Verified) != 1 {
		t.Fatalf("apply = %+v", applied)
	}

	// Text-mode progress streams name and position.
	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"adopt", "--apply"}, &stdout, &stderr); code != 0 {
		t.Fatalf("adopt --apply text exited %d: %s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("adopting old-era (1/1)")) {
		t.Fatalf("progress line missing:\n%s", stdout.String())
	}
}

func TestAdoptCommandExitsOneOnFailures(t *testing.T) {
	adoptFixture(t)

	overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stderr: "broken", ExitCode: 1}))
	var stdout, stderr bytes.Buffer
	code := Run([]string{"adopt", "--apply", "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1 on a failed partition, got %d", code)
	}
	var envelope output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if len(envelope.Result.Adoption.Features[0].Failed) != 1 {
		t.Fatalf("failed partition missing: %+v", envelope.Result.Adoption.Features[0])
	}
}
