package validation

import (
	"reflect"
	"testing"

	"github.com/andrearaponi/walden/internal/spec"
)

// The coverage fixtures hold four criteria. R1.AC10 is there for the order of
// the validator: it sorts identifiers as text, so R1.AC10 comes before R1.AC2.
const coverageRequirements = `---
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

1. __BT__R1.AC1__BT__ WHEN the user submits a todo, the system SHALL create it.
2. __BT__R1.AC2__BT__ WHEN the user completes a todo, the system SHALL mark it.
10. __BT__R1.AC10__BT__ WHEN the user reopens a todo, the system SHALL unmark it.

### R2 Reject bad input

**User Story:** As a user, I want bad input rejected, so that the list stays clean.

#### Acceptance Criteria

1. __BT__R2.AC1__BT__ IF the title is empty, THEN the system SHALL reject the todo.

## Non-Functional Requirements

- __BT__NFR1__BT__ The system SHALL remain deterministic.
`

const coverageDesign = `---
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
| __BT__R1__BT__ | Todo flow |
| __BT__R2__BT__ | Todo flow |
| __BT__NFR1__BT__ | Deterministic checks |
`

const coverageTasksHeader = `---
status: approved
approved_at: 2026-03-21T14:20:00Z
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at: 2026-03-21T14:10:00Z
---

# Implementation Plan

`

// coverageAcceptedPlan names every criterion and covers two of them.
const coverageAcceptedPlan = coverageTasksHeader + `- [ ] 1. Build feature
  - [ ] 1.1 Create and reject
    - Requirements: __BT__R1.AC1__BT__, __BT__R1.AC2__BT__, __BT__R2.AC1__BT__
    - Design: Architecture
    - Verification:
      - command: ["go", "test", "./..."]
        covers: ["R1.AC1", "R2.AC1"]
  - [ ] 1.2 Reopen
    - Requirements: __BT__R1.AC10__BT__, __BT__NFR1__BT__
    - Design: Architecture
    - Verification:
      - command: ["go", "vet", "./..."]
`

// TestCoverageReportPinned fixes what the validator reports about coverage
// for a plan it accepts: which criteria it lists, and in which order. It was
// written against the validator as it stood before the report was computed by
// an exported function, and it passes unchanged after.
func TestCoverageReportPinned(t *testing.T) {
	root := t.TempDir()
	writeFeatureFile(t, root, "cov-pinned", "requirements.md", coverageRequirements)
	writeFeatureFile(t, root, "cov-pinned", "design.md", coverageDesign)
	writeFeatureFile(t, root, "cov-pinned", "tasks.md", coverageAcceptedPlan)

	result, err := ValidateFeatureWithScope(root, "cov-pinned", ScopeFullSpec)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !result.Valid {
		t.Fatalf("the fixture is not valid at full scope: %s", result.Message)
	}
	if result.Coverage == nil {
		t.Fatal("the result holds no coverage report")
	}

	task := result.Coverage.TaskReferenceCoverage
	if !task.Complete || len(task.Missing) != 0 {
		t.Errorf("task reference coverage = %+v, want complete with nothing missing", task)
	}
	proof := result.Coverage.ProofReferenceCoverage
	if proof.Complete {
		t.Errorf("proof reference coverage is complete, want two criteria missing")
	}
	// The order of the validator, not the numeric one.
	if want := []string{"R1.AC10", "R1.AC2"}; !reflect.DeepEqual(proof.Missing, want) {
		t.Errorf("criteria without a proof = %v, want %v", proof.Missing, want)
	}
}

// coverageRejectedPlan leaves R1.AC2 without a Requirements line and names
// R2.AC1 without backticks, which the validator does not count. Its first
// leaf names the heading R2, so that the validator reaches the criteria.
const coverageRejectedPlan = coverageTasksHeader + `- [ ] 1. Build feature
  - [ ] 1.1 Create and reopen
    - Requirements: __BT__R1.AC1__BT__, __BT__R1.AC10__BT__, __BT__R2__BT__
    - Design: Architecture
    - Verification:
      - command: ["go", "test", "./..."]
        covers: ["R1.AC1"]
  - [ ] 1.2 Reject
    - Requirements: R2.AC1
    - Design: Architecture
    - Verification:
      - command: ["go", "vet", "./..."]
        covers: ["R2.AC1"]
`

func sameList(got, want []string) bool {
	return (len(got) == 0 && len(want) == 0) || reflect.DeepEqual(got, want)
}

// TestCoverage: the exported reading returns, for a plan the validator
// accepts and for one it rejects, the criteria no Requirements line names and
// the criteria no proof step covers, in the order of the validator. For the
// first plan it is the report the validator carries; for the second, its
// first list is the one the validator fails on.
func TestCoverage(t *testing.T) {
	cases := []struct {
		name      string
		plan      string
		unnamed   []string
		uncovered []string
		failsOn   string
	}{
		{
			name:      "a plan the validator accepts",
			plan:      coverageAcceptedPlan,
			uncovered: []string{"R1.AC10", "R1.AC2"},
		},
		{
			name:      "a plan the validator rejects",
			plan:      coverageRejectedPlan,
			unnamed:   []string{"R1.AC2", "R2.AC1"},
			uncovered: []string{"R1.AC10", "R1.AC2"},
			failsOn:   "tasks.md missing coverage for acceptance criteria: R1.AC2, R2.AC1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFeatureFile(t, root, "cov", "requirements.md", coverageRequirements)
			writeFeatureFile(t, root, "cov", "design.md", coverageDesign)
			writeFeatureFile(t, root, "cov", "tasks.md", tc.plan)
			feature, err := spec.LoadFeature(root, "cov")
			if err != nil {
				t.Fatalf("load the fixture: %v", err)
			}

			report := Coverage(feature)
			if task := report.TaskReferenceCoverage; !sameList(task.Missing, tc.unnamed) || task.Complete != (len(tc.unnamed) == 0) {
				t.Errorf("criteria no Requirements line names = %+v, want %v", task, tc.unnamed)
			}
			if proof := report.ProofReferenceCoverage; !sameList(proof.Missing, tc.uncovered) || proof.Complete != (len(tc.uncovered) == 0) {
				t.Errorf("criteria no proof step covers = %+v, want %v", proof, tc.uncovered)
			}

			// What the validator says of the same documents.
			result, err := ValidateFeatureWithScope(root, "cov", ScopeFullSpec)
			if err != nil {
				t.Fatalf("validate: %v", err)
			}
			if tc.failsOn == "" {
				if !result.Valid || result.Coverage == nil {
					t.Fatalf("the validator does not accept the plan: %s", result.Message)
				}
				if !reflect.DeepEqual(*result.Coverage, report) {
					t.Errorf("the report of the validator = %+v, the reading = %+v", *result.Coverage, report)
				}
				return
			}
			if result.Valid || result.Message != "INVALID: "+tc.failsOn {
				t.Errorf("the validator says %q, want it to fail on %q", result.Message, tc.failsOn)
			}
			if result.Coverage != nil {
				t.Errorf("the validator carries a coverage report for a plan it rejects: %+v", *result.Coverage)
			}
		})
	}
}
