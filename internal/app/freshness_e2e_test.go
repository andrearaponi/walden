package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/spec"
)

// TestEndToEndFingerprintHardeningLifecycle walks the full hardened chain:
// approve -> tamper -> blocked with causes -> reconcile -> re-approve ->
// execution restored. Every step drives the real CLI entrypoint.
func TestEndToEndFingerprintHardeningLifecycle(t *testing.T) {
	root := t.TempDir()

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("expected working directory lookup to succeed, got %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWD)
	})
	if err := os.Chdir(root); err != nil {
		t.Fatalf("expected chdir to succeed, got %v", err)
	}

	writeDraft := func(name, body string) {
		t.Helper()
		sourceKey := ""
		switch name {
		case "design.md":
			sourceKey = "source_requirements_approved_at:\n"
		case "tasks.md":
			sourceKey = "source_design_approved_at:\n"
		}
		content := "---\nstatus: draft\napproved_at:\nlast_modified: 2026-07-02T08:00:00Z\n" + sourceKey + "---\n\n" + body
		dir := filepath.Join(root, ".walden", "specs", "hardening-demo")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("expected feature directory creation to succeed, got %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("expected %s write to succeed, got %v", name, err)
		}
	}

	run := func(args ...string) (int, string, string) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		code := Run(args, &stdout, &stderr)
		return code, stdout.String(), stderr.String()
	}

	mustSucceed := func(args ...string) string {
		t.Helper()
		code, stdout, stderr := run(args...)
		if code != 0 {
			t.Fatalf("expected %v to succeed, got %d with stderr %q", args, code, stderr)
		}
		return stdout
	}

	mustFailWith := func(want string, args ...string) {
		t.Helper()
		code, stdout, stderr := run(args...)
		if code == 0 {
			t.Fatalf("expected %v to fail, got success with stdout %q", args, stdout)
		}
		if !strings.Contains(stdout+stderr, want) {
			t.Fatalf("expected %v output to contain %q, got stdout %q stderr %q", args, want, stdout, stderr)
		}
	}

	requirementsBody := `# Requirements Document

## Introduction

Demo feature for the hardened chain.

## Requirements

### R1 Deterministic gate

**User Story:** As a developer, I want blocking gates, so that drift stops.

#### Acceptance Criteria

1. ` + "`R1.AC1`" + ` WHEN a proof passes, the system SHALL mark the task complete.

## Non-Functional Requirements

- ` + "`NFR1`" + ` The system SHALL respond within one second.

## Constraints And Dependencies

- ` + "`C1`" + ` Local execution only.

## Out Of Scope

- None.
`

	designBody := `# Feature Design

## Overview

Design for ` + "`R1`" + `.

## Architecture

Single component.

## Options Considered

### Option A

- Summary: Preferred.
- Why chosen: Simpler.

### Option B

- Summary: Alternative.
- Why rejected: More moving parts.

## Simplicity And Elegance Review

- Simplest viable shape: Minimal.
- Coupling check: Low.
- Future-proofing: Deferred.

## Components And Interfaces

### Gate

- Purpose: Enforce ` + "`R1`" + `.
- Inputs/Outputs: CLI.
- Dependencies: None.
- Requirements: ` + "`R1`" + `

## Data Models

None.

## Error Handling

Fail fast.

## Security Considerations

None.

## Failure Modes And Tradeoffs

- Failure mode: None.
- Mitigation: None.
- Tradeoff: None.

## Testing Strategy

Unit tests.

## Verification Plan

- Requirement proof: Tests.
- Test evidence: ` + "`go test ./...`" + `
- Operational evidence: None.

## Requirement Coverage

| Requirement | Covered By |
| --- | --- |
| ` + "`R1`" + ` | Gate |
| ` + "`NFR1`" + ` | Tests |
`

	tasksBody := `# Implementation Plan

- [ ] 1. Objective
  - [ ] 1.1 Step
    - Requirements: ` + "`R1.AC1`" + `, ` + "`NFR1`" + `
    - Design: Gate
    - Verification:
      - command: ["go", "version"]
        covers: ["R1.AC1"]
`

	// 1. Build a genuinely approved chain through the real approve path.
	writeDraft("requirements.md", requirementsBody)
	mustSucceed("review", "open", "hardening-demo", "--phase", "requirements")
	mustSucceed("review", "approve", "hardening-demo", "--phase", "requirements")

	writeDraft("design.md", designBody)
	mustSucceed("review", "open", "hardening-demo", "--phase", "design")
	mustSucceed("review", "approve", "hardening-demo", "--phase", "design")

	writeDraft("tasks.md", tasksBody)
	mustSucceed("review", "open", "hardening-demo", "--phase", "tasks")
	mustSucceed("review", "approve", "hardening-demo", "--phase", "tasks")

	feature, err := spec.LoadFeature(root, "hardening-demo")
	if err != nil {
		t.Fatalf("expected feature load to succeed, got %v", err)
	}
	if !spec.ValidFingerprint(feature.Requirements.ApprovedFingerprint) ||
		!spec.ValidFingerprint(feature.Design.ApprovedFingerprint) ||
		!spec.ValidFingerprint(feature.Tasks.ApprovedFingerprint) {
		t.Fatal("expected the approve path to record valid fingerprints on the whole chain")
	}
	if feature.Design.SourceRequirementsFingerprint != feature.Requirements.ApprovedFingerprint {
		t.Fatal("expected design to be bound to the requirements approval fingerprint")
	}

	mustSucceed("task", "status", "hardening-demo")

	// 2. Tamper the approved requirements body without re-approval.
	requirementsPath := filepath.Join(root, ".walden", "specs", "hardening-demo", "requirements.md")
	content, err := os.ReadFile(requirementsPath)
	if err != nil {
		t.Fatalf("expected requirements read to succeed, got %v", err)
	}
	if err := os.WriteFile(requirementsPath, append(content, []byte("\nInjected after approval.\n")...), 0o644); err != nil {
		t.Fatalf("expected tampered write to succeed, got %v", err)
	}

	// 3. The gates close, with the precise cause everywhere.
	statusOut := mustSucceed("status", "hardening-demo")
	if !strings.Contains(statusOut, "requirements.md is stale: content does not match approval fingerprint") {
		t.Fatalf("expected status blockers to carry the tamper cause, got %q", statusOut)
	}
	mustFailWith("content does not match approval fingerprint", "validate", "hardening-demo", "--all")
	mustFailWith("requirements.md is stale", "task", "start", "hardening-demo")
	mustFailWith("requirements.md is stale", "task", "complete", "hardening-demo", "1.1")
	mustFailWith("requirements.md is stale", "task", "complete-all", "hardening-demo")

	// 4. Reconcile repairs deterministically: the tampered document and its
	// downstream lose approval.
	mustSucceed("reconcile", "hardening-demo")
	feature, err = spec.LoadFeature(root, "hardening-demo")
	if err != nil {
		t.Fatalf("expected feature reload to succeed, got %v", err)
	}
	if feature.Requirements.Status != "draft" || feature.Design.Status != "draft" || feature.Tasks.Status != "draft" {
		t.Fatalf("expected the whole chain to reset to draft, got %q/%q/%q",
			feature.Requirements.Status, feature.Design.Status, feature.Tasks.Status)
	}

	// 5. Re-approval restores execution.
	mustSucceed("review", "open", "hardening-demo", "--phase", "requirements")
	mustSucceed("review", "approve", "hardening-demo", "--phase", "requirements")
	mustSucceed("review", "open", "hardening-demo", "--phase", "design")
	mustSucceed("review", "approve", "hardening-demo", "--phase", "design")
	mustSucceed("review", "open", "hardening-demo", "--phase", "tasks")
	mustSucceed("review", "approve", "hardening-demo", "--phase", "tasks")

	startOut := mustSucceed("task", "start", "hardening-demo")
	if !strings.Contains(startOut, "task start context for hardening-demo") {
		t.Fatalf("expected execution to be restored, got %q", startOut)
	}
}
