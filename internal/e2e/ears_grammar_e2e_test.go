package e2e

import (
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/output"
)

// TestE2EEARSCanonicalGrammarCycle walks a feature whose criteria use the
// EARS clause grammar through the whole cycle: validation, the three approvals
// and task completion. Every relaxation of the validator travels the cycle.
func TestE2EEARSCanonicalGrammarCycle(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q", "-b", "main")
	gitIn(t, dir, "config", "user.email", "e2e@walden.dev")
	gitIn(t, dir, "config", "user.name", "E2E")

	mustCLI(t, dir, "repo", "init")
	mustCLI(t, dir, "feature", "init", "grammar-demo")

	writeFile(t, dir, ".walden/specs/grammar-demo/requirements.md", `---
status: draft
approved_at:
last_modified: 2026-10-10T16:00:00Z
approved_fingerprint:
---

# Requirements Document

## Introduction

End-to-end fixture for the EARS clause grammar.

## Requirements

### R1 Replies

**User Story:** As an operator, I want every reply handled in every condition, so that none is lost.

#### Acceptance Criteria

1. `+"`R1.AC1`"+` WHERE voice replies are enabled, WHEN a turn ends, the system SHALL speak the reply
2. `+"`R1.AC2`"+` WHILE a turn is running, IF the container exits, THEN the system SHALL report the exit
3. `+"`R1.AC3`"+` WHEN a message arrives, WHILE the user is offline, the system SHALL store the message
4. `+"`R1.AC4`"+` WHEN the system assembles the export, it SHALL order the sections
5. `+"`R1.AC5`"+` WHEN the user asks if the file exists, the system SHALL answer with its path
`)

	result, code := integrityResult(t, dir, "validate", "grammar-demo")
	if code != 0 {
		t.Fatalf("validate: exit %d: %s", code, result.Summary)
	}
	criteria := map[string]output.EARSCriterion{}
	for _, criterion := range result.EARSValidation {
		criteria[criterion.ID] = criterion
	}
	for _, want := range []struct {
		id, form, warning string
	}{
		{"R1.AC1", "complex", ""},
		{"R1.AC2", "complex", ""},
		{"R1.AC3", "complex", "out of EARS order"},
		{"R1.AC4", "event-driven", "is a pronoun"},
		{"R1.AC5", "event-driven", ""},
	} {
		got, ok := criteria[want.id]
		if !ok || !got.Valid || got.Form != want.form {
			t.Fatalf("%s: want valid %s, got %+v", want.id, want.form, got)
		}
		if want.warning == "" && len(got.Warnings) != 0 {
			t.Fatalf("%s: want no warning, got %v", want.id, got.Warnings)
		}
		if want.warning != "" && (len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], want.warning)) {
			t.Fatalf("%s: want one warning containing %q, got %v", want.id, want.warning, got.Warnings)
		}
	}
	for _, warning := range result.Warnings {
		if strings.Contains(warning, "no unwanted-behavior") {
			t.Fatalf("the complex criterion with IF and THEN is failure handling, got %q", warning)
		}
	}

	writeFile(t, dir, ".walden/specs/grammar-demo/design.md", `---
status: draft
approved_at:
last_modified: 2026-10-10T16:00:00Z
approved_fingerprint:
source_requirements_approved_at:
source_requirements_fingerprint:
---

# Feature Design

## Architecture

One reply log, one grep proof.

## Options Considered

- A grep proof over the log; no meaningful alternative for a fixture.

## Simplicity And Elegance Review

- One file, one proof.

## Failure Modes And Tradeoffs

- None relevant for a fixture.

## Verification Plan

- The proof greps the reply log.

## Requirement Coverage

| Requirement | Covered By |
| --- | --- |
| `+"`R1`"+` | replies.txt |
`)
	writeFile(t, dir, ".walden/specs/grammar-demo/tasks.md", `---
status: draft
approved_at:
last_modified: 2026-10-10T16:00:00Z
approved_fingerprint:
source_design_approved_at:
source_design_fingerprint:
---

# Implementation Plan

- [ ] 1. Log every reply
  - Requirements: `+"`R1.AC1`, `R1.AC2`, `R1.AC3`, `R1.AC4`, `R1.AC5`"+`
  - Design: Architecture
  - Verification:
    - command: ["sh", "-c", "grep -q SPOKEN replies.txt && grep -q ANSWERED replies.txt"]
      covers: ["R1.AC1", "R1.AC2", "R1.AC3", "R1.AC4", "R1.AC5"]
`)
	for _, phase := range []string{"requirements", "design", "tasks"} {
		mustCLI(t, dir, "review", "open", "grammar-demo", "--phase", phase)
		mustCLI(t, dir, "review", "approve", "grammar-demo", "--phase", phase)
	}

	writeFile(t, dir, "replies.txt", "SPOKEN\nREPORTED\nSTORED\nORDERED\nANSWERED\n")
	mustCLI(t, dir, "task", "complete-all", "grammar-demo")

	if final, code := integrityResult(t, dir, "validate", "grammar-demo", "--all"); code != 0 {
		t.Fatalf("validate --all after completion: exit %d: %s", code, final.Summary)
	}
}
