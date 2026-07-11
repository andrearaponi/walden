package app

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/output"
	"github.com/andrearaponi/walden/internal/testutil"
)

const releasableRequirements = `---
status: draft
approved_at:
last_modified: 2026-07-11T13:00:00Z
approved_fingerprint:
---

# Requirements Document

## Introduction

Gate fixture.

## Requirements

### R1 Markers

**User Story:** As a tester, I want markers, so that the gate has proofs to judge.

#### Acceptance Criteria

1. ` + "`R1.AC1`" + ` WHEN the first proof runs, the system SHALL succeed.
2. ` + "`R1.AC2`" + ` WHEN the second proof runs, the system SHALL succeed.
`

const releasableDesign = `---
status: draft
approved_at:
last_modified: 2026-07-11T13:00:00Z
approved_fingerprint:
source_requirements_approved_at:
source_requirements_fingerprint:
---

# Feature Design

## Overview

Two trivially provable markers.

## Architecture

One shell proof per task.

## Options Considered

### Option A

- Summary: shell proofs.
- Why chosen: trivial.

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

- Requirement proof: shell exits.
- Test evidence: exit codes.

## Requirement Coverage

| Requirement | Covered By |
| --- | --- |
| ` + "`R1`" + ` | markers |
`

const releasableTasks = `---
status: draft
approved_at:
last_modified: 2026-07-11T13:00:00Z
approved_fingerprint:
source_design_approved_at:
source_design_fingerprint:
---

# Implementation Plan

- [ ] 1. Markers
  - [ ] 1.1 First marker
    - Requirements: ` + "`R1.AC1`" + `
    - Design: Overview
    - Verification:
      - command: ["sh", "-c", "true"]
        covers: ["R1.AC1"]
  - [ ] 1.2 Second marker
    - Requirements: ` + "`R1.AC2`" + `
    - Design: Overview
    - Verification:
      - command: ["sh", "-c", "true"]
        covers: ["R1.AC2"]
`

// setupReleasableFeature writes a fully certifiable feature in a temp cwd and
// approves its chain through the CLI. Task completion is left to each test.
func setupReleasableFeature(t *testing.T, name string) string {
	t.Helper()
	root := chdirContract(t)
	addReleasableFeature(t, root, name)
	return root
}

func addReleasableFeature(t *testing.T, root, name string) {
	t.Helper()
	writeStatusFeatureFile(t, root, name, "requirements.md", releasableRequirements)
	writeStatusFeatureFile(t, root, name, "design.md", releasableDesign)
	writeStatusFeatureFile(t, root, name, "tasks.md", releasableTasks)

	var stdout, stderr bytes.Buffer
	for _, phase := range []string{"requirements", "design", "tasks"} {
		if code := Run([]string{"review", "open", name, "--phase", phase}, &stdout, &stderr); code != 0 {
			t.Fatalf("review open %s failed: %s", phase, stderr.String())
		}
		if code := Run([]string{"review", "approve", name, "--phase", phase}, &stdout, &stderr); code != 0 {
			t.Fatalf("review approve %s failed: %s", phase, stderr.String())
		}
	}
}

func completeReleasableFeature(t *testing.T, name string) {
	t.Helper()
	overrideCommandRunner(t, testutil.NewFakeRunner(
		testutil.Response{Stdout: "ok", ExitCode: 0},
		testutil.Response{Stdout: "ok", ExitCode: 0},
	))
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"task", "complete-all", name}, &stdout, &stderr); code != 0 {
		t.Fatalf("task complete-all failed: %s", stderr.String())
	}
}

func decodeReleaseEnvelope(t *testing.T, stdout *bytes.Buffer) output.Envelope {
	t.Helper()
	var envelope output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not a JSON envelope: %v (%q)", err, stdout.String())
	}
	if envelope.Command != "release-check" {
		t.Fatalf("command = %q, want release-check", envelope.Command)
	}
	return envelope
}

func TestRunReleaseCheckReleasableJSON(t *testing.T) {
	setupReleasableFeature(t, "gate-demo")
	completeReleasableFeature(t, "gate-demo")

	var stdout, stderr bytes.Buffer
	code := Run([]string{"release", "check", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stdout %s, stderr %s)", code, stdout.String(), stderr.String())
	}

	envelope := decodeReleaseEnvelope(t, &stdout)
	if !envelope.OK || !strings.Contains(envelope.Result.Summary, "RELEASABLE") {
		t.Fatalf("green run not marked releasable: %+v", envelope.Result)
	}
	releaseBlock := envelope.Result.Release
	if releaseBlock == nil || !releaseBlock.Releasable || releaseBlock.Strict {
		t.Fatalf("release block wrong: %+v", releaseBlock)
	}
	if len(releaseBlock.Features) != 1 || len(releaseBlock.Features[0].Criteria) != 4 {
		t.Fatalf("expected 1 feature with 4 criteria: %+v", releaseBlock.Features)
	}
	// chdirContract roots are not git repositories: the worktree criterion
	// must report itself skipped rather than inventing a verdict.
	if !releaseBlock.Worktree.GitSkipped {
		t.Fatalf("worktree.git_skipped = false in a git-less root: %+v", releaseBlock.Worktree)
	}
}

func TestRunReleaseCheckNotReleasableTextReport(t *testing.T) {
	root := chdirContract(t)
	// Approved chain, but the design misses required sections: full-spec
	// validation blocks certification.
	writeStatusFeatureFile(t, root, "gate-demo", "requirements.md", releasableRequirements)
	writeStatusFeatureFile(t, root, "gate-demo", "design.md", `---
status: draft
approved_at:
last_modified: 2026-07-11T13:00:00Z
approved_fingerprint:
source_requirements_approved_at:
source_requirements_fingerprint:
---

# Feature Design

## Overview

Bare.

## Requirement Coverage

| Requirement | Covered By |
| --- | --- |
| `+"`R1`"+` | markers |
`)
	writeStatusFeatureFile(t, root, "gate-demo", "tasks.md", releasableTasks)
	var buffer bytes.Buffer
	for _, phase := range []string{"requirements", "design", "tasks"} {
		if code := Run([]string{"review", "open", "gate-demo", "--phase", phase}, &buffer, &buffer); code != 0 {
			t.Fatalf("review open %s failed: %s", phase, buffer.String())
		}
		if code := Run([]string{"review", "approve", "gate-demo", "--phase", phase}, &buffer, &buffer); code != 0 {
			t.Fatalf("review approve %s failed: %s", phase, buffer.String())
		}
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"release", "check"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("blocked repository exited zero: %s", stdout.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("failed certification wrote to stdout: %q", stdout.String())
	}
	report := stderr.String()
	if !strings.Contains(report, "NOT RELEASABLE") || !strings.Contains(report, "Blockers:") || !strings.Contains(report, "gate-demo") {
		t.Fatalf("text report incomplete: %q", report)
	}
}

func TestRunReleaseCheckStrictPromotesPending(t *testing.T) {
	setupReleasableFeature(t, "gate-demo")

	var stdout, stderr bytes.Buffer
	code := Run([]string{"release", "check", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("pending-only plan blocked a non-strict run: %s", stdout.String())
	}
	envelope := decodeReleaseEnvelope(t, &stdout)
	if pending := envelope.Result.Release.Features[0].Pending; len(pending) != 2 {
		t.Fatalf("pending = %v, want both planned tasks", pending)
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"release", "check", "--strict", "--json"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("--strict did not block pending work")
	}
	envelope = decodeReleaseEnvelope(t, &stdout)
	if envelope.OK || !envelope.Result.Release.Strict {
		t.Fatalf("strict envelope wrong: %+v", envelope.Result.Release)
	}
}

func TestRunReleaseCheckRejectsBypassFlags(t *testing.T) {
	setupReleasableFeature(t, "gate-demo")

	var stdout, stderr bytes.Buffer
	code := Run([]string{"release", "check", "--allow-dirty", "--json"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("--allow-dirty accepted")
	}
	envelope := decodeReleaseEnvelope(t, &stdout)
	if envelope.OK || !strings.Contains(envelope.Result.Summary, "--allow-dirty") {
		t.Fatalf("bypass flag not named in the error: %+v", envelope.Result)
	}

	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"release", "check", "--force"}, &stdout, &stderr); code == 0 {
		t.Fatal("--force accepted")
	}
	if !strings.Contains(stderr.String(), "unknown flag --force") {
		t.Fatalf("text-mode flag error missing: %q", stderr.String())
	}
}

func TestRunReleaseCheckScopesToOneFeature(t *testing.T) {
	root := setupReleasableFeature(t, "gate-demo")
	addReleasableFeature(t, root, "second-feature")
	completeReleasableFeature(t, "gate-demo")

	var stdout, stderr bytes.Buffer
	code := Run([]string{"release", "check", "gate-demo", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("scoped run failed: %s (stderr %s)", stdout.String(), stderr.String())
	}
	envelope := decodeReleaseEnvelope(t, &stdout)
	features := envelope.Result.Release.Features
	if len(features) != 1 || features[0].Feature != "gate-demo" {
		t.Fatalf("scoped run evaluated %+v, want only gate-demo", features)
	}
}

func TestRunReleaseCheckUsageListed(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("--help failed: %s", stderr.String())
	}
	usage := stdout.String()
	if !strings.Contains(usage, "release check [<feature>] [--strict] [--json]") {
		t.Fatalf("usage does not list release check: %q", usage)
	}
}
