package adopt

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/release"
	"github.com/andrearaponi/walden/internal/shell"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
}

func gitStatusOutsideWalden(t *testing.T, dir string) string {
	t.Helper()
	command := exec.Command("git", "status", "--porcelain", "--", ".", ":(exclude).walden")
	command.Dir = dir
	out, err := command.Output()
	if err != nil {
		t.Fatalf("git status: %v", err)
	}
	return string(out)
}

// certifiablePreFingerprintFeature writes a feature whose documents pass full
// validation but predate the fingerprint era: approved, sealless, executed.
func certifiablePreFingerprintFeature(t *testing.T, root, name string) {
	t.Helper()
	writeDoc(t, root, name, "requirements.md", `---
status: approved
approved_at: 2026-06-09T14:00:00Z
last_modified: 2026-06-09T14:00:00Z
---

# Requirements Document

## Introduction

Adoption fixture.

## Requirements

### R1 Markers

**User Story:** As a tester, I want markers, so that the lane has proofs to re-run.

#### Acceptance Criteria

1. `+"`R1.AC1`"+` WHEN the proof runs, the system SHALL succeed.
`)
	writeDoc(t, root, name, "design.md", `---
status: approved
approved_at: 2026-06-09T14:10:00Z
last_modified: 2026-06-09T14:10:00Z
source_requirements_approved_at: 2026-06-09T14:00:00Z
---

# Feature Design

## Overview

One trivially provable marker.

## Architecture

One shell proof.

## Options Considered

### Option A

- Summary: shell proof.
- Why chosen: trivial.

### Option B

- Summary: none.
- Why rejected: n/a.

## Simplicity And Elegance Review

- Simplest viable shape: one command.
- Coupling check: none.
- Future-proofing: none.

## Failure Modes And Tradeoffs

- Failure mode: none.
- Mitigation: none.
- Tradeoff: none.

## Testing Strategy

Shell exit codes.

## Verification Plan

- Requirement proof: shell exit.
- Test evidence: exit code.

## Requirement Coverage

| Requirement | Covered By |
| --- | --- |
| `+"`R1`"+` | marker |
`)
	writeDoc(t, root, name, "tasks.md", `---
status: approved
approved_at: 2026-06-09T14:20:00Z
last_modified: 2026-06-09T14:20:00Z
source_design_approved_at: 2026-06-09T14:10:00Z
---

# Implementation Plan

- [x] 1. Markers
  - [x] 1.1 First marker
    - Requirements: `+"`R1.AC1`"+`
    - Design: Overview
    - Verification:
      - command: ["sh", "-c", "true"]
        covers: ["R1.AC1"]
`)
}

// The lane's whole promise as one test: a committed pre-fingerprint
// repository goes from gate-blocked to RELEASABLE through adoption alone —
// with a read-only plan, an idempotent second run, and not one byte changed
// outside .walden/.
func TestAdoptEndToEndCertifiable(t *testing.T) {
	requireGit(t)
	root := t.TempDir()
	gitIn(t, root, "init", "-q", "-b", "main")
	gitIn(t, root, "config", "user.email", "adopt@walden.dev")
	gitIn(t, root, "config", "user.name", "Adopt")
	certifiablePreFingerprintFeature(t, root, "old-era")
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-qm", "pre-fingerprint era")

	// Before adoption: the feature cannot certify — the chain is stale.
	before, err := release.ReleaseCheck(context.Background(), root, "old-era", false)
	if err != nil {
		t.Fatalf("ReleaseCheck before: %v", err)
	}
	if before.Releasable() {
		t.Fatal("fixture must start blocked, or the lane proves nothing")
	}

	// The plan is read-only.
	planBefore := hashWaldenTree(t, root)
	plan, err := Plan(context.Background(), root, "")
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if plan.Features[0].Class != ClassBackfill {
		t.Fatalf("plan class = %s, want backfill", plan.Features[0].Class)
	}
	if hashWaldenTree(t, root) != planBefore {
		t.Fatal("the plan modified .walden/")
	}

	// Apply with the real runner: sh -c true is the committed proof.
	report, err := Apply(context.Background(), root, "", shell.NewExecRunner(), nil)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if report.Totals.SealedDocs != 3 || report.Totals.Verified != 1 || report.Totals.Failed != 0 {
		t.Fatalf("apply totals = %+v", report.Totals)
	}

	// After adoption: every criterion passes and the verdict is RELEASABLE —
	// adoption's writes live under .walden/, which only warns.
	after, err := release.ReleaseCheck(context.Background(), root, "old-era", false)
	if err != nil {
		t.Fatalf("ReleaseCheck after: %v", err)
	}
	if !after.Releasable() {
		t.Fatalf("adopted feature not releasable: %+v", after)
	}
	if after.Completion() != release.CompletionComplete {
		t.Fatalf("completion = %s, want complete", after.Completion())
	}

	// Nothing outside .walden/ moved.
	if dirt := gitStatusOutsideWalden(t, root); dirt != "" {
		t.Fatalf("adoption touched the code tree:\n%s", dirt)
	}

	// A second apply is a complete no-op.
	second, err := Apply(context.Background(), root, "", shell.NewExecRunner(), nil)
	if err != nil {
		t.Fatalf("second Apply: %v", err)
	}
	if second.Features[0].Class != ClassComplete || second.Totals.SealedDocs != 0 || second.Totals.Verified != 0 {
		t.Fatalf("second apply = %+v, want a complete no-op", second.Features[0])
	}
}
