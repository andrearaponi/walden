package app

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

// The coverage fixtures hold four criteria. R1.AC10 is there for the order of
// the validator: it sorts identifiers as text, so R1.AC10 comes before R1.AC2.
const coverageBlockRequirements = `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document

## Introduction

Scope.

## Requirements

### R1 Create todos

**User Story:** As a user, I want to create todos, so that I can track work.

#### Acceptance Criteria

1. ` + "`R1.AC1`" + ` WHEN the user submits a todo, the system SHALL create it.
2. ` + "`R1.AC2`" + ` WHEN the user completes a todo, the system SHALL mark it.
10. ` + "`R1.AC10`" + ` WHEN the user reopens a todo, the system SHALL unmark it.

### R2 Reject bad input

**User Story:** As a user, I want bad input rejected, so that the list stays clean.

#### Acceptance Criteria

1. ` + "`R2.AC1`" + ` IF the title is empty, THEN the system SHALL reject the todo.

## Non-Functional Requirements

- ` + "`NFR1`" + ` The system SHALL remain deterministic.
`

const coverageBlockDesign = `---
status: approved
approved_at: 2026-03-21T14:10:00Z
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at: 2026-03-21T14:00:00Z
---

# Feature Design

## Architecture

Simple architecture.

## Options Considered

One flow; no alternative for a fixture.

## Simplicity And Elegance Review

Single flow.

## Failure Modes And Tradeoffs

None for a fixture.

## Verification Plan

Unit tests.

## Requirement Coverage

| Requirement | Covered By |
| --- | --- |
| ` + "`R1`" + ` | Todo flow |
| ` + "`R2`" + ` | Todo flow |
| ` + "`NFR1`" + ` | Deterministic checks |
`

// coverageBlockTasks names every criterion and covers two of them.
const coverageBlockTasks = `---
status: approved
approved_at: 2026-03-21T14:20:00Z
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at: 2026-03-21T14:10:00Z
---

# Implementation Plan

- [ ] 1. Build feature
  - [ ] 1.1 Create and reject
    - Requirements: ` + "`R1.AC1`, `R1.AC2`, `R2.AC1`" + `
    - Design: Architecture
    - Verification:
      - command: ["go", "test", "./..."]
        covers: ["R1.AC1", "R2.AC1"]
  - [ ] 1.2 Reopen
    - Requirements: ` + "`R1.AC10`, `NFR1`" + `
    - Design: Architecture
    - Verification:
      - command: ["go", "vet", "./..."]
`

// TestValidateCoverageBlockPinned fixes the coverage block that
// `validate --all --json` carries for a plan the validator accepts: its keys,
// the criteria it lists and their order, read from the wire as untyped maps.
// It was written against the command as it stood before the report was
// computed by an exported function, and it passes unchanged after.
func TestValidateCoverageBlockPinned(t *testing.T) {
	root := chdirContract(t)
	const feature = "cov-block"
	writeStatusFeatureFile(t, root, feature, "requirements.md", coverageBlockRequirements)
	writeStatusFeatureFile(t, root, feature, "design.md", coverageBlockDesign)
	writeStatusFeatureFile(t, root, feature, "tasks.md", coverageBlockTasks)

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"validate", feature, "--all", "--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("validate --all exited %d: %s%s", code, stdout.String(), stderr.String())
	}
	var envelope struct {
		Result struct {
			Coverage map[string]any `json:"coverage"`
		} `json:"result"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal the envelope: %v", err)
	}

	want := map[string]any{
		// A complete list is carried without its missing key.
		"task_reference_coverage": map[string]any{"complete": true},
		// The order of the validator, not the numeric one.
		"proof_reference_coverage": map[string]any{
			"complete": false,
			"missing":  []any{"R1.AC10", "R1.AC2"},
		},
	}
	if !reflect.DeepEqual(envelope.Result.Coverage, want) {
		t.Errorf("the coverage block = %v, want %v", envelope.Result.Coverage, want)
	}
}
