package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// releasableRepo bootstraps a real git repository holding one feature that
// passes every certification criterion: approved fresh chain, full-spec-valid
// documents, no decision markers, completed tasks verified on the committed
// tree, clean worktree.
func releasableRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q", "-b", "main")
	gitIn(t, dir, "config", "user.email", "e2e@walden.dev")
	gitIn(t, dir, "config", "user.name", "E2E")

	mustCLI(t, dir, "repo", "init")
	mustCLI(t, dir, "feature", "init", "gate-demo")

	writeFile(t, dir, ".walden/specs/gate-demo/requirements.md", `---
status: draft
approved_at:
last_modified: 2026-07-11T10:00:00Z
approved_fingerprint:
---

# Requirements Document

## Introduction

Certification fixture.

## Requirements

### R1 Markers

**User Story:** As a tester, I want provable markers, so that certification judges real evidence.

#### Acceptance Criteria

1. `+"`R1.AC1`"+` WHEN the first proof runs, the system SHALL find marker A in the source.
2. `+"`R1.AC2`"+` WHEN the second proof runs, the system SHALL find marker B in the source.
`)
	writeFile(t, dir, ".walden/specs/gate-demo/design.md", `---
status: draft
approved_at:
last_modified: 2026-07-11T10:00:00Z
approved_fingerprint:
source_requirements_approved_at:
source_requirements_fingerprint:
---

# Feature Design

## Overview

A source file carries two grep-provable markers.

## Architecture

One shell proof per task.

## Options Considered

### Option A

- Summary: grep proofs.
- Why chosen: trivial and real.

### Option B

- Summary: none.
- Why rejected: n/a.

## Simplicity And Elegance Review

- Simplest viable shape: two commands.
- Coupling check: none.
- Future-proofing: none.

## Failure Modes And Tradeoffs

- Failure mode: none.
- Mitigation: none.
- Tradeoff: none.

## Testing Strategy

Shell exit codes.

## Verification Plan

- Requirement proof: grep exits.
- Test evidence: exit codes.

## Requirement Coverage

| Requirement | Covered By |
| --- | --- |
| `+"`R1`"+` | src.txt |
`)
	writeFile(t, dir, ".walden/specs/gate-demo/tasks.md", `---
status: draft
approved_at:
last_modified: 2026-07-11T10:00:00Z
approved_fingerprint:
source_design_approved_at:
source_design_fingerprint:
---

# Implementation Plan

- [ ] 1. Markers
  - [ ] 1.1 Place marker A
    - Requirements: `+"`R1.AC1`"+`
    - Design: Overview
    - Verification:
      - command: ["sh", "-c", "grep -q MARKER-A src.txt"]
        covers: ["R1.AC1"]
  - [ ] 1.2 Place marker B
    - Requirements: `+"`R1.AC2`"+`
    - Design: Overview
    - Verification:
      - command: ["sh", "-c", "grep -q MARKER-B src.txt"]
        covers: ["R1.AC2"]
`)

	for _, phase := range []string{"requirements", "design", "tasks"} {
		mustCLI(t, dir, "review", "open", "gate-demo", "--phase", phase)
		mustCLI(t, dir, "review", "approve", "gate-demo", "--phase", phase)
	}

	writeFile(t, dir, "src.txt", "MARKER-A\nMARKER-B\n")
	mustCLI(t, dir, "task", "complete-all", "gate-demo")

	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-qm", "gate-demo: code, specs, evidence")
	return dir
}

func TestE2EReleaseCheckCertifiesGreenRepository(t *testing.T) {
	requireGit(t)
	dir := releasableRepo(t)

	out := mustCLI(t, dir, "release", "check")
	if !strings.Contains(out, "RELEASABLE") || !strings.Contains(out, "worktree: clean") {
		t.Fatalf("green repository not certified: %s", out)
	}
	// The full CI recipe: verify re-proves, release check certifies. On an
	// unchanged tree verify rewrites identical evidence bytes, so the
	// worktree stays clean and certification holds with no commit between.
	mustCLI(t, dir, "verify", "gate-demo", "--all")
	mustCLI(t, dir, "release", "check")
}

func TestE2EReleaseCheckCodeEditBlocks(t *testing.T) {
	requireGit(t)
	dir := releasableRepo(t)

	writeFile(t, dir, "src.txt", "MARKER-A\nMARKER-B\ndrifted\n")

	out, code := cli(t, dir, "release", "check")
	if code == 0 {
		t.Fatalf("edited code certified as releasable: %s", out)
	}
	if !strings.Contains(out, "stale-code") || !strings.Contains(out, "walden verify gate-demo") {
		t.Fatalf("stale-code blocker missing its remedy: %s", out)
	}
	if !strings.Contains(out, "uncommitted: src.txt") {
		t.Fatalf("dirty worktree not reported alongside: %s", out)
	}
}

func TestE2EReleaseCheckSpecRevivalBlocks(t *testing.T) {
	requireGit(t)
	dir := releasableRepo(t)

	// The spec moves after execution: evidence recorded under the old chain
	// must read stale-spec even once the chain is re-approved and committed.
	path := filepath.Join(dir, ".walden/specs/gate-demo/requirements.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read requirements: %v", err)
	}
	writeFile(t, dir, ".walden/specs/gate-demo/requirements.md", string(content)+"\nRevived scope note.\n")
	mustCLI(t, dir, "reconcile", "gate-demo")
	for _, phase := range []string{"requirements", "design", "tasks"} {
		mustCLI(t, dir, "review", "open", "gate-demo", "--phase", phase)
		mustCLI(t, dir, "review", "approve", "gate-demo", "--phase", phase)
	}
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-qm", "revive the spec")

	out, code := cli(t, dir, "release", "check")
	if code == 0 {
		t.Fatalf("revived spec certified against old evidence: %s", out)
	}
	if !strings.Contains(out, "stale-spec") || !strings.Contains(out, "walden verify gate-demo") {
		t.Fatalf("stale-spec blocker missing its remedy: %s", out)
	}
}

func TestE2EReleaseCheckDecisionMarkerBlocks(t *testing.T) {
	requireGit(t)
	dir := releasableRepo(t)

	path := filepath.Join(dir, ".walden/specs/gate-demo/design.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read design: %v", err)
	}
	writeFile(t, dir, ".walden/specs/gate-demo/design.md", string(content)+"\n[decision: which backend stays?]\n")
	mustCLI(t, dir, "reconcile", "gate-demo")
	for _, phase := range []string{"design", "tasks"} {
		mustCLI(t, dir, "review", "open", "gate-demo", "--phase", phase)
		mustCLI(t, dir, "review", "approve", "gate-demo", "--phase", phase)
	}
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-qm", "approve a design carrying an open checkpoint")

	out, code := cli(t, dir, "release", "check")
	if code == 0 {
		t.Fatalf("open decision checkpoint certified: %s", out)
	}
	if !strings.Contains(out, "[decision:] marker in design.md") || !strings.Contains(out, "resolve") {
		t.Fatalf("decision blocker missing document or remedy: %s", out)
	}
}

func TestE2EReleaseCheckUncommittedFileBlocks(t *testing.T) {
	requireGit(t)
	dir := releasableRepo(t)

	writeFile(t, dir, "notes.txt", "wip\n")

	out, code := cli(t, dir, "release", "check")
	if code == 0 {
		t.Fatalf("dirty worktree certified: %s", out)
	}
	if !strings.Contains(out, "uncommitted: notes.txt — commit it") {
		t.Fatalf("worktree blocker missing its remedy: %s", out)
	}

	// The same dirt under .walden/ warns without blocking.
	if err := os.Remove(filepath.Join(dir, "notes.txt")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	writeFile(t, dir, ".walden/scratch.txt", "local\n")
	out = mustCLI(t, dir, "release", "check")
	if !strings.Contains(out, ".walden/scratch.txt") || !strings.Contains(out, "not blocking") {
		t.Fatalf(".walden dirt not surfaced as a warning: %s", out)
	}
}
