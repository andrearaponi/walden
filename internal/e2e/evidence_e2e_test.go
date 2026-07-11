// Package e2e replays the evidence-ledger evaluation battery as compiled
// tests: real git repositories, the real process runner, every step driven
// through app.Run exactly as a user or agent would. The feature that
// promises proofs is itself pinned by executable ones.
package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/app"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
}

// cli runs one walden invocation from dir, returning combined output and exit code.
func cli(t *testing.T, dir string, args ...string) (string, int) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := app.Run(args, &stdout, &stderr)
	_ = os.Chdir(previous)
	return stdout.String() + stderr.String(), code
}

func mustCLI(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, code := cli(t, dir, args...)
	if code != 0 {
		t.Fatalf("walden %s failed (%d): %s", strings.Join(args, " "), code, out)
	}
	return out
}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", name, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// fixtureRepo bootstraps a real git repo with an approved, completed
// two-task feature whose proofs grep markers out of src.txt.
func fixtureRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q", "-b", "main")
	gitIn(t, dir, "config", "user.email", "e2e@walden.dev")
	gitIn(t, dir, "config", "user.name", "E2E")

	mustCLI(t, dir, "repo", "init")
	mustCLI(t, dir, "feature", "init", "ledger-demo")

	writeFile(t, dir, ".walden/specs/ledger-demo/requirements.md", `---
status: draft
approved_at:
last_modified: 2026-07-11T10:00:00Z
approved_fingerprint:
---

# Requirements Document

## Introduction

End-to-end evidence fixture.

## Requirements

### R1 Markers

**User Story:** As a tester, I want provable markers, so that evidence binds to code.

#### Acceptance Criteria

1. `+"`R1.AC1`"+` WHEN the first proof runs, the system SHALL find marker A in the source.
2. `+"`R1.AC2`"+` WHEN the second proof runs, the system SHALL find marker B in the source.
`)
	writeFile(t, dir, ".walden/specs/ledger-demo/design.md", `---
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

## Requirement Coverage

| Requirement | Covered By |
| --- | --- |
| `+"`R1`"+` | src.txt |
`)
	writeFile(t, dir, ".walden/specs/ledger-demo/tasks.md", `---
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
		mustCLI(t, dir, "review", "open", "ledger-demo", "--phase", phase)
		mustCLI(t, dir, "review", "approve", "ledger-demo", "--phase", phase)
	}

	writeFile(t, dir, "src.txt", "MARKER-A\nMARKER-B\n")
	mustCLI(t, dir, "task", "complete-all", "ledger-demo")
	return dir
}

func evidenceSummary(t *testing.T, dir string) string {
	t.Helper()
	out, _ := cli(t, dir, "evidence", "status", "ledger-demo")
	return out
}

func TestE2ESerpentCommitTransitions(t *testing.T) {
	requireGit(t)
	dir := fixtureRepo(t)

	if err := os.Symlink("src.txt", filepath.Join(dir, "link.txt")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if err := os.Chmod(filepath.Join(dir, "src.txt"), 0o755); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	mustCLI(t, dir, "verify", "ledger-demo", "--all")

	if out := evidenceSummary(t, dir); !strings.Contains(out, "2 verified") {
		t.Fatalf("pre-commit baseline not verified: %s", out)
	}

	// The serpent: committing everything — code, symlink, executable bit,
	// specs, evidence — must not move the identity.
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-qm", "everything, evidence included")

	if out := evidenceSummary(t, dir); !strings.Contains(out, "2 verified") {
		t.Fatalf("committing the evidence invalidated it: %s", out)
	}

	// A mode-only flip is a code change and must surface.
	if err := os.Chmod(filepath.Join(dir, "src.txt"), 0o644); err != nil {
		t.Fatalf("chmod back: %v", err)
	}
	if out := evidenceSummary(t, dir); !strings.Contains(out, "stale-code") {
		t.Fatalf("executable-bit flip stayed invisible: %s", out)
	}
}

func TestE2EBrokenImplementationSurfaces(t *testing.T) {
	requireGit(t)
	dir := fixtureRepo(t)

	// WDN-001 replay: the implementation regresses after completion.
	writeFile(t, dir, "src.txt", "MARKER-A\n")

	if out := evidenceSummary(t, dir); !strings.Contains(out, "stale-code") {
		t.Fatalf("regression invisible to evidence status: %s", out)
	}
	out, code := cli(t, dir, "task", "status", "ledger-demo")
	if code != 0 || !strings.Contains(out, "evidence") {
		t.Fatalf("task status hides the regression: %s", out)
	}

	out, code = cli(t, dir, "verify", "ledger-demo")
	if code == 0 {
		t.Fatalf("verify passed on broken code: %s", out)
	}
	if !strings.Contains(out, "1.2") {
		t.Fatalf("verify does not name the failing task: %s", out)
	}
	if summary := evidenceSummary(t, dir); !strings.Contains(summary, "failed") {
		t.Fatalf("failure not recorded: %s", summary)
	}
}

func TestE2ESpecRevivalSurfaces(t *testing.T) {
	requireGit(t)
	dir := fixtureRepo(t)

	// WDN-002 replay: requirements change, the chain is reconciled and
	// re-approved, and no proof reruns — the checkbox must not lie.
	requirements := filepath.Join(dir, ".walden/specs/ledger-demo/requirements.md")
	content, err := os.ReadFile(requirements)
	if err != nil {
		t.Fatalf("read requirements: %v", err)
	}
	if err := os.WriteFile(requirements, []byte(strings.Replace(string(content), "find marker A", "find the renamed marker A", 1)), 0o644); err != nil {
		t.Fatalf("edit requirements: %v", err)
	}

	mustCLI(t, dir, "reconcile", "ledger-demo")
	for _, phase := range []string{"requirements", "design", "tasks"} {
		mustCLI(t, dir, "review", "open", "ledger-demo", "--phase", phase)
		mustCLI(t, dir, "review", "approve", "ledger-demo", "--phase", phase)
	}

	out, code := cli(t, dir, "task", "status", "ledger-demo")
	if code != 0 || !strings.Contains(out, "stale-spec") {
		t.Fatalf("re-approved plan hides stale evidence: %s", out)
	}

	mustCLI(t, dir, "verify", "ledger-demo")
	if out := evidenceSummary(t, dir); !strings.Contains(out, "2 verified") {
		t.Fatalf("verify did not heal the chain: %s", out)
	}
}

func TestE2ECorruptedLedgerRecovery(t *testing.T) {
	requireGit(t)
	dir := fixtureRepo(t)

	ledger := filepath.Join(dir, ".walden/evidence/ledger-demo.json")
	if err := os.WriteFile(ledger, []byte("{truncated"), 0o644); err != nil {
		t.Fatalf("corrupt ledger: %v", err)
	}

	out, code := cli(t, dir, "task", "status", "ledger-demo")
	if code != 0 || !strings.Contains(out, "unreadable") {
		t.Fatalf("corruption invisible on task status: %s", out)
	}
	if _, code := cli(t, dir, "verify", "ledger-demo", "--all"); code == 0 {
		t.Fatal("verify accepted a corrupt ledger")
	}

	if err := os.Remove(ledger); err != nil {
		t.Fatalf("remove ledger: %v", err)
	}
	mustCLI(t, dir, "verify", "ledger-demo", "--all")
	if out := evidenceSummary(t, dir); !strings.Contains(out, "2 verified") {
		t.Fatalf("recovery did not regenerate the truth: %s", out)
	}
}

func TestE2EOrphanPruning(t *testing.T) {
	requireGit(t)
	dir := fixtureRepo(t)

	tasks := filepath.Join(dir, ".walden/specs/ledger-demo/tasks.md")
	content, err := os.ReadFile(tasks)
	if err != nil {
		t.Fatalf("read tasks: %v", err)
	}
	renumbered := strings.Replace(string(content), "1.2 Place marker B", "1.3 Place marker B", 1)
	if err := os.WriteFile(tasks, []byte(renumbered), 0o644); err != nil {
		t.Fatalf("renumber: %v", err)
	}

	mustCLI(t, dir, "reconcile", "ledger-demo")
	for _, phase := range []string{"requirements", "design", "tasks"} {
		mustCLI(t, dir, "review", "open", "ledger-demo", "--phase", phase)
		mustCLI(t, dir, "review", "approve", "ledger-demo", "--phase", phase)
	}

	out := mustCLI(t, dir, "verify", "ledger-demo")
	if !strings.Contains(out, "pruned") || !strings.Contains(out, "1.2") {
		t.Fatalf("orphan pruning not reported: %s", out)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".walden/evidence/ledger-demo.json"))
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if strings.Contains(string(data), "\"1.2\"") {
		t.Fatal("orphan entry survived in the ledger")
	}
	if summary := evidenceSummary(t, dir); !strings.Contains(summary, "2 verified") {
		t.Fatalf("final state incoherent: %s", summary)
	}
}

func TestE2ELegacyProofVerify(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q", "-b", "main")
	gitIn(t, dir, "config", "user.email", "e2e@walden.dev")
	gitIn(t, dir, "config", "user.name", "E2E")

	mustCLI(t, dir, "repo", "init")
	mustCLI(t, dir, "feature", "init", "legacy-demo")

	writeFile(t, dir, ".walden/specs/legacy-demo/requirements.md", `---
status: draft
approved_at:
last_modified: 2026-07-11T10:00:00Z
approved_fingerprint:
---

# Requirements Document

## Introduction

Legacy proof fixture.

## Requirements

### R1 Marker

**User Story:** As a tester, I want legacy proofs exercised, so that old plans keep verifying.

#### Acceptance Criteria

1. `+"`R1.AC1`"+` WHEN the legacy proof runs, the system SHALL find the marker.
`)
	writeFile(t, dir, ".walden/specs/legacy-demo/design.md", `---
status: draft
approved_at:
last_modified: 2026-07-11T10:00:00Z
approved_fingerprint:
source_requirements_approved_at:
source_requirements_fingerprint:
---

# Feature Design

## Overview

One legacy single-line proof.

## Requirement Coverage

| Requirement | Covered By |
| --- | --- |
| `+"`R1`"+` | src.txt |
`)
	writeFile(t, dir, ".walden/specs/legacy-demo/tasks.md", `---
status: draft
approved_at:
last_modified: 2026-07-11T10:00:00Z
approved_fingerprint:
source_design_approved_at:
source_design_fingerprint:
---

# Implementation Plan

- [ ] 1. Marker
  - [ ] 1.1 Place the marker
    - Requirements: `+"`R1.AC1`"+`
    - Design: Overview
    - Verification: `+"`grep -q MARKER src.txt`"+`
`)

	for _, phase := range []string{"requirements", "design", "tasks"} {
		mustCLI(t, dir, "review", "open", "legacy-demo", "--phase", phase)
		mustCLI(t, dir, "review", "approve", "legacy-demo", "--phase", phase)
	}
	writeFile(t, dir, "src.txt", "MARKER\n")
	mustCLI(t, dir, "task", "complete-all", "legacy-demo")

	out, _ := cli(t, dir, "evidence", "status", "legacy-demo")
	if !strings.Contains(out, "1 verified") {
		t.Fatalf("legacy completion not recorded as verified: %s", out)
	}

	// The legacy path must survive re-verification too.
	mustCLI(t, dir, "verify", "legacy-demo", "--all")
	out, _ = cli(t, dir, "evidence", "status", "legacy-demo")
	if !strings.Contains(out, "1 verified") {
		t.Fatalf("legacy verify broke the state: %s", out)
	}

	// And a regression is still caught through the legacy proof.
	writeFile(t, dir, "src.txt", "nothing here\n")
	if _, code := cli(t, dir, "verify", "legacy-demo"); code == 0 {
		t.Fatal("legacy verify passed on broken code")
	}
}
