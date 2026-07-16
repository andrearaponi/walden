package validation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/spec"
)

func TestValidateFeatureReturnsValidResultForRepresentativeSpec(t *testing.T) {
	root := t.TempDir()
	writeValidFeature(t, root, "todo-app-demo")

	result, err := ValidateFeature(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("expected validation to succeed, got %v", err)
	}

	if !result.Valid {
		t.Fatalf("expected validation success, got %#v", result)
	}
	if result.Message != "VALID: .walden/specs/todo-app-demo" {
		t.Fatalf("unexpected success message: %q", result.Message)
	}
	if result.Scope != ScopeCurrentPhase {
		t.Fatalf("expected current-phase scope, got %q", result.Scope)
	}
	if got, want := strings.Join(result.ValidatedPhases, ","), "requirements,design,tasks"; got != want {
		t.Fatalf("expected validated phases %q, got %q", want, got)
	}
	if len(result.SkippedPhases) != 0 {
		t.Fatalf("expected no skipped phases, got %v", result.SkippedPhases)
	}
}

func TestValidateFeatureFailsOnMissingTaskCoverage(t *testing.T) {
	root := t.TempDir()
	writeFeatureFile(t, root, "todo-app-demo", "requirements.md", validRequirements)
	writeFeatureFile(t, root, "todo-app-demo", "design.md", validDesign)
	writeFeatureFile(t, root, "todo-app-demo", "tasks.md", `---
status: approved
approved_at: 2026-03-21T14:20:00Z
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at: 2026-03-21T14:10:00Z
---

# Implementation Plan

- [ ] 1. Build feature
  - [ ] 1.1 Add implementation
    - Requirements: __BT__NFR1__BT__
    - Design: Components
    - Verification: __BT__go test ./...__BT__
`)

	result, err := ValidateFeature(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("expected deterministic validation failure, got unexpected error %v", err)
	}

	if result.Valid {
		t.Fatalf("expected validation failure, got %#v", result)
	}
	if !strings.Contains(result.Message, "tasks.md missing task coverage for requirement IDs: R1") {
		t.Fatalf("unexpected validation message: %q", result.Message)
	}
}

func TestValidateFeatureFailsOnInvalidDesignStructure(t *testing.T) {
	root := t.TempDir()
	writeFeatureFile(t, root, "todo-app-demo", "requirements.md", validRequirements)
	writeFeatureFile(t, root, "todo-app-demo", "design.md", `---
status: draft
approved_at:
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at:
---

# Feature Design

## Overview

Only overview.
`)

	result, err := ValidateFeature(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("expected deterministic validation failure, got unexpected error %v", err)
	}

	if result.Valid {
		t.Fatalf("expected validation failure, got %#v", result)
	}
	if !strings.Contains(result.Message, "design.md: missing required sections") {
		t.Fatalf("unexpected validation message: %q", result.Message)
	}
}

func TestValidateFeatureSkipsDownstreamTasksForCurrentDesignPhase(t *testing.T) {
	root := t.TempDir()
	writeFeatureFile(t, root, "todo-app-demo", "requirements.md", validRequirements)
	writeFeatureFile(t, root, "todo-app-demo", "design.md", validDraftDesign)
	writeFeatureFile(t, root, "todo-app-demo", "tasks.md", invalidDraftTasks)

	result, err := ValidateFeature(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("expected validation to succeed, got %v", err)
	}

	if !result.Valid {
		t.Fatalf("expected current-phase validation success, got %#v", result)
	}
	if got, want := strings.Join(result.ValidatedPhases, ","), "requirements,design"; got != want {
		t.Fatalf("expected validated phases %q, got %q", want, got)
	}
	if got, want := strings.Join(result.SkippedPhases, ","), "tasks"; got != want {
		t.Fatalf("expected skipped phases %q, got %q", want, got)
	}
}

func TestValidateFeatureWithFullSpecFailsOnInvalidDownstreamTasks(t *testing.T) {
	root := t.TempDir()
	writeFeatureFile(t, root, "todo-app-demo", "requirements.md", validRequirements)
	writeFeatureFile(t, root, "todo-app-demo", "design.md", validDraftDesign)
	writeFeatureFile(t, root, "todo-app-demo", "tasks.md", invalidDraftTasks)

	result, err := ValidateFeatureWithScope(root, "todo-app-demo", ScopeFullSpec)
	if err != nil {
		t.Fatalf("expected deterministic validation failure, got unexpected error %v", err)
	}

	if result.Valid {
		t.Fatalf("expected full-spec validation failure, got %#v", result)
	}
	if !strings.Contains(result.Message, "tasks.md missing task coverage for requirement IDs: R1") {
		t.Fatalf("unexpected validation message: %q", result.Message)
	}
	if got, want := strings.Join(result.ValidatedPhases, ","), "requirements,design,tasks"; got != want {
		t.Fatalf("expected validated phases %q, got %q", want, got)
	}
}

const validRequirements = `---
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

## Non-Functional Requirements

- __BT__NFR1__BT__ The system SHALL remain deterministic.

## Constraints And Dependencies

- __BT__C1__BT__ Local filesystem only.

## Out Of Scope

- Remote sync.
`

const validDesign = `---
status: approved
approved_at: 2026-03-21T14:10:00Z
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at: 2026-03-21T14:00:00Z
---

# Feature Design

## Overview

Design for __BT__R1__BT__.

## Architecture

Simple architecture.

## Options Considered

### Option A

- Summary: Preferred.
- Why chosen: Simpler.

### Option B

- Summary: Alternative.
- Why rejected: More moving parts.

## Simplicity And Elegance Review

- Simplest viable shape: Single flow.
- Coupling check: Low coupling.
- Future-proofing: Deferred.

## Components And Interfaces

### Todo flow

- Purpose: Handle __BT__R1__BT__.
- Inputs/Outputs: Local input and output.
- Dependencies: Filesystem.
- Requirements: __BT__R1__BT__

## Data Models

Local model.

## Error Handling

Deterministic failures.

## Security Considerations

No secrets.

## Failure Modes And Tradeoffs

- Failure mode: User error.
- Mitigation: Validation.
- Tradeoff: Minimalism.

## Testing Strategy

Unit tests.

## Verification Plan

- Requirement proof: Exercise __BT__R1__BT__.
- Test evidence: __BT__go test ./...__BT__
- Operational evidence: Command output.

## Requirement Coverage

| Requirement | Covered By |
| --- | --- |
| __BT__R1__BT__ | Todo flow |
| __BT__NFR1__BT__ | Validation tests |
`

const validDraftDesign = `---
status: draft
approved_at:
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at:
---

# Feature Design

## Overview

Design for __BT__R1__BT__.

## Architecture

Simple architecture.

## Options Considered

### Option A

- Summary: Preferred.
- Why chosen: Simpler.

### Option B

- Summary: Alternative.
- Why rejected: More moving parts.

## Simplicity And Elegance Review

- Simplest viable shape: Single flow.
- Coupling check: Low coupling.
- Future-proofing: Deferred.

## Components And Interfaces

### Todo flow

- Purpose: Handle __BT__R1__BT__.
- Inputs/Outputs: Local input and output.
- Dependencies: Filesystem.
- Requirements: __BT__R1__BT__

## Data Models

Local model.

## Error Handling

Deterministic failures.

## Security Considerations

No secrets.

## Failure Modes And Tradeoffs

- Failure mode: User error.
- Mitigation: Validation.
- Tradeoff: Minimalism.

## Testing Strategy

Unit tests.

## Verification Plan

- Requirement proof: Exercise __BT__R1__BT__.
- Test evidence: __BT__go test ./...__BT__
- Operational evidence: Command output.

## Requirement Coverage

| Requirement | Covered By |
| --- | --- |
| __BT__R1__BT__ | Todo flow |
| __BT__NFR1__BT__ | Validation tests |
`

const validTasks = `---
status: approved
approved_at: 2026-03-21T14:20:00Z
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at: 2026-03-21T14:10:00Z
---

# Implementation Plan

- [ ] 1. Build feature
  - [ ] 1.1 Add implementation
    - Requirements: __BT__R1.AC1__BT__, __BT__NFR1__BT__
    - Design: Todo flow
    - Verification: __BT__go test ./...__BT__
`

const invalidDraftTasks = `---
status: draft
approved_at:
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at:
---

# Implementation Plan

- [ ] 1. Build feature
  - [ ] 1.1 Add implementation
    - Requirements: __BT__NFR1__BT__
    - Design: Todo flow
    - Verification: __BT__go test ./...__BT__
`

func TestValidateFeatureACCoverage(t *testing.T) {
	requirementsWithMultipleACs := `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document

## Introduction

Scope.

## Requirements

### R1 Feature

**User Story:** As a user, I want a feature.

#### Acceptance Criteria

1. __BT__R1.AC1__BT__ WHEN triggered, the system SHALL respond.
2. __BT__R1.AC2__BT__ WHEN failed, the system SHALL recover.
3. __BT__R1.AC3__BT__ WHEN idle, the system SHALL wait.

## Non-Functional Requirements

- __BT__NFR1__BT__ The system SHALL remain deterministic.

## Constraints And Dependencies

- __BT__C1__BT__ Local only.

## Out Of Scope

- None.
`

	tests := []struct {
		name         string
		requirements string
		tasks        string
		wantValid    bool
		wantContains string
	}{
		{
			name:         "full AC coverage passes",
			requirements: requirementsWithMultipleACs,
			tasks: `---
status: approved
approved_at: 2026-03-21T14:20:00Z
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at: 2026-03-21T14:10:00Z
---

# Implementation Plan

- [ ] 1. Build feature
  - [ ] 1.1 Add implementation
    - Requirements: __BT__R1.AC1__BT__, __BT__R1.AC2__BT__, __BT__R1.AC3__BT__, __BT__NFR1__BT__
    - Design: Components
    - Verification: __BT__go test ./...__BT__
`,
			wantValid: true,
		},
		{
			name:         "partial AC coverage fails",
			requirements: requirementsWithMultipleACs,
			tasks: `---
status: approved
approved_at: 2026-03-21T14:20:00Z
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at: 2026-03-21T14:10:00Z
---

# Implementation Plan

- [ ] 1. Build feature
  - [ ] 1.1 Add implementation
    - Requirements: __BT__R1.AC1__BT__, __BT__NFR1__BT__
    - Design: Components
    - Verification: __BT__go test ./...__BT__
`,
			wantValid:    false,
			wantContains: "tasks.md missing coverage for acceptance criteria: R1.AC2, R1.AC3",
		},
		{
			name:         "single AC fully covered passes",
			requirements: validRequirements,
			tasks: `---
status: approved
approved_at: 2026-03-21T14:20:00Z
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at: 2026-03-21T14:10:00Z
---

# Implementation Plan

- [ ] 1. Build feature
  - [ ] 1.1 Add implementation
    - Requirements: __BT__R1.AC1__BT__, __BT__NFR1__BT__
    - Design: Components
    - Verification: __BT__go test ./...__BT__
`,
			wantValid: true,
		},
		{
			name:         "mixed AC and NFR passes when all covered",
			requirements: requirementsWithMultipleACs,
			tasks: `---
status: approved
approved_at: 2026-03-21T14:20:00Z
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at: 2026-03-21T14:10:00Z
---

# Implementation Plan

- [ ] 1. Build feature
  - [ ] 1.1 Core logic
    - Requirements: __BT__R1.AC1__BT__, __BT__R1.AC2__BT__
    - Design: Components
    - Verification: __BT__go test ./...__BT__
  - [ ] 1.2 Edge cases
    - Requirements: __BT__R1.AC3__BT__, __BT__NFR1__BT__
    - Design: Components
    - Verification: __BT__go test ./...__BT__
`,
			wantValid: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFeatureFile(t, root, "test-feature", "requirements.md", tc.requirements)
			writeFeatureFile(t, root, "test-feature", "design.md", validDesign)
			writeFeatureFile(t, root, "test-feature", "tasks.md", tc.tasks)

			result, err := ValidateFeature(root, "test-feature")
			if err != nil {
				t.Fatalf("expected deterministic validation result, got error: %v", err)
			}

			if result.Valid != tc.wantValid {
				t.Fatalf("expected valid=%v, got valid=%v with message: %q", tc.wantValid, result.Valid, result.Message)
			}

			if !tc.wantValid && tc.wantContains != "" {
				if !strings.Contains(result.Message, tc.wantContains) {
					t.Fatalf("expected message to contain %q, got %q", tc.wantContains, result.Message)
				}
			}
		})
	}
}

func TestMissingFailureModeSignalWhenNoUnwanted(t *testing.T) {
	root := t.TempDir()
	writeFeatureFile(t, root, "fm-test", "requirements.md", `---
status: draft
approved_at:
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document

## Requirements

### R1 Feature

#### Acceptance Criteria

1. __BT__R1.AC1__BT__ WHEN triggered, the system SHALL respond

## Non-Functional Requirements

- __BT__NFR1__BT__ Fast.

## Constraints And Dependencies

- __BT__C1__BT__ Local storage only.
- __BT__C2__BT__ No backend.

## Out Of Scope

- None.
`)

	result, err := ValidateFeature(root, "fm-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid, got: %s", result.Message)
	}
	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "no unwanted-behavior") && strings.Contains(w, "C1") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected missing failure mode warning, got warnings: %v", result.Warnings)
	}
}

func TestMissingFailureModeSignalNotEmittedWhenUnwantedExists(t *testing.T) {
	root := t.TempDir()
	writeFeatureFile(t, root, "fm-ok", "requirements.md", `---
status: draft
approved_at:
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document

## Requirements

### R1 Feature

#### Acceptance Criteria

1. __BT__R1.AC1__BT__ WHEN triggered, the system SHALL respond
2. __BT__R1.AC2__BT__ IF failure, THEN the system SHALL recover

## Non-Functional Requirements

- __BT__NFR1__BT__ Fast.

## Constraints And Dependencies

- __BT__C1__BT__ Local.

## Out Of Scope

- None.
`)

	result, err := ValidateFeature(root, "fm-ok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, w := range result.Warnings {
		if strings.Contains(w, "no unwanted-behavior") {
			t.Fatalf("expected no missing failure mode warning when unwanted exists, got: %s", w)
		}
	}
}

func TestEARSDistributionCounts(t *testing.T) {
	root := t.TempDir()
	writeFeatureFile(t, root, "dist-test", "requirements.md", `---
status: draft
approved_at:
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document

## Requirements

### R1 Feature

#### Acceptance Criteria

1. __BT__R1.AC1__BT__ The system SHALL work
2. __BT__R1.AC2__BT__ WHEN triggered, the system SHALL respond
3. __BT__R1.AC3__BT__ WHILE active, the system SHALL monitor
4. __BT__R1.AC4__BT__ IF failure, THEN the system SHALL recover

## Non-Functional Requirements

- __BT__NFR1__BT__ The system SHALL be fast.

## Constraints And Dependencies

- __BT__C1__BT__ Local only.

## Out Of Scope

- None.
`)

	result, err := ValidateFeature(root, "dist-test")
	if err != nil {
		t.Fatalf("expected validation to succeed, got %v", err)
	}
	if result.EARSDistribution == nil {
		t.Fatal("expected EARS distribution to be populated")
	}
	d := result.EARSDistribution
	if d.Total != 4 {
		t.Fatalf("expected total 4, got %d", d.Total)
	}
	if d.Ubiquitous != 1 {
		t.Fatalf("expected 1 ubiquitous, got %d", d.Ubiquitous)
	}
	if d.EventDriven != 1 {
		t.Fatalf("expected 1 event-driven, got %d", d.EventDriven)
	}
	if d.StateDriven != 1 {
		t.Fatalf("expected 1 state-driven, got %d", d.StateDriven)
	}
	if d.Unwanted != 1 {
		t.Fatalf("expected 1 unwanted, got %d", d.Unwanted)
	}
}

func TestEARSDistributionZeroWhenNoCriteria(t *testing.T) {
	root := t.TempDir()
	writeFeatureFile(t, root, "empty-test", "requirements.md", `---
status: draft
approved_at:
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document

## Requirements

### R1 Feature

#### Acceptance Criteria

1. __BT__R1.AC1__BT__ The system SHALL work

## Non-Functional Requirements

- __BT__NFR1__BT__ Fast.

## Constraints And Dependencies

- __BT__C1__BT__ Local.

## Out Of Scope

- None.
`)

	result, err := ValidateFeature(root, "empty-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EARSDistribution == nil {
		t.Fatal("expected EARS distribution even for minimal spec")
	}
	if result.EARSDistribution.Total != 1 {
		t.Fatalf("expected total 1, got %d", result.EARSDistribution.Total)
	}
}

func TestProofCoverageReportedSeparately(t *testing.T) {
	root := t.TempDir()
	writeFeatureFile(t, root, "cov-test", "requirements.md", validRequirements)
	writeFeatureFile(t, root, "cov-test", "design.md", validDesign)
	writeFeatureFile(t, root, "cov-test", "tasks.md", `---
status: approved
approved_at: 2026-03-21T14:20:00Z
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at: 2026-03-21T14:10:00Z
---

# Implementation Plan

- [ ] 1. Build feature
  - [ ] 1.1 Add implementation
    - Requirements: __BT__R1.AC1__BT__, __BT__NFR1__BT__
    - Design: Todo flow
    - Verification:
      - command: ["go", "test", "./..."]
        covers: ["R1.AC1"]
`)

	result, err := ValidateFeatureWithScope(root, "cov-test", ScopeFullSpec)
	if err != nil {
		t.Fatalf("expected validation to succeed, got %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid, got: %s", result.Message)
	}
	if result.Coverage == nil {
		t.Fatal("expected coverage report to be populated")
	}
	if !result.Coverage.TaskReferenceCoverage.Complete {
		t.Fatalf("expected task reference coverage complete, missing: %v", result.Coverage.TaskReferenceCoverage.Missing)
	}
	if !result.Coverage.ProofReferenceCoverage.Complete {
		t.Fatalf("expected proof reference coverage complete, missing: %v", result.Coverage.ProofReferenceCoverage.Missing)
	}
}

func TestProofCoverageReportsMissingWhenNoCoversField(t *testing.T) {
	root := t.TempDir()
	writeFeatureFile(t, root, "cov-miss", "requirements.md", validRequirements)
	writeFeatureFile(t, root, "cov-miss", "design.md", validDesign)
	writeFeatureFile(t, root, "cov-miss", "tasks.md", `---
status: approved
approved_at: 2026-03-21T14:20:00Z
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at: 2026-03-21T14:10:00Z
---

# Implementation Plan

- [ ] 1. Build feature
  - [ ] 1.1 Add implementation
    - Requirements: __BT__R1.AC1__BT__, __BT__NFR1__BT__
    - Design: Todo flow
    - Verification:
      - command: ["go", "test", "./..."]
`)

	result, err := ValidateFeatureWithScope(root, "cov-miss", ScopeFullSpec)
	if err != nil {
		t.Fatalf("expected validation to succeed, got %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid, got: %s", result.Message)
	}
	if result.Coverage == nil {
		t.Fatal("expected coverage report to be populated")
	}
	if !result.Coverage.TaskReferenceCoverage.Complete {
		t.Fatalf("expected task reference coverage complete, missing: %v", result.Coverage.TaskReferenceCoverage.Missing)
	}
	if result.Coverage.ProofReferenceCoverage.Complete {
		t.Fatal("expected proof reference coverage incomplete when no covers field")
	}
	if len(result.Coverage.ProofReferenceCoverage.Missing) == 0 {
		t.Fatal("expected missing proof coverage entries")
	}
}

func TestProofCoverageRejectsUnknownCoversID(t *testing.T) {
	root := t.TempDir()
	writeFeatureFile(t, root, "cov-bad", "requirements.md", validRequirements)
	writeFeatureFile(t, root, "cov-bad", "design.md", validDesign)
	writeFeatureFile(t, root, "cov-bad", "tasks.md", `---
status: approved
approved_at: 2026-03-21T14:20:00Z
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at: 2026-03-21T14:10:00Z
---

# Implementation Plan

- [ ] 1. Build feature
  - [ ] 1.1 Add implementation
    - Requirements: __BT__R1.AC1__BT__, __BT__NFR1__BT__
    - Design: Todo flow
    - Verification:
      - command: ["go", "test", "./..."]
        covers: ["R99.AC99"]
`)

	result, err := ValidateFeatureWithScope(root, "cov-bad", ScopeFullSpec)
	if err != nil {
		t.Fatalf("expected deterministic result, got error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected validation failure for unknown covers ID")
	}
	if !strings.Contains(result.Message, "covers unknown ID") {
		t.Fatalf("expected covers error, got: %s", result.Message)
	}
}

func TestValidateEARSPassesForValidCriteria(t *testing.T) {
	root := t.TempDir()
	writeFeatureFile(t, root, "ears-test", "requirements.md", validRequirements)

	result, err := ValidateFeature(root, "ears-test")
	if err != nil {
		t.Fatalf("expected validation to succeed, got %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid, got: %s", result.Message)
	}
	if len(result.EARSResults) == 0 {
		t.Fatal("expected EARS results to be populated")
	}
	if result.EARSResults[0].ID != "R1.AC1" {
		t.Fatalf("expected first EARS result to be R1.AC1, got %s", result.EARSResults[0].ID)
	}
	if result.EARSResults[0].Form != "event-driven" {
		t.Fatalf("expected event-driven form, got %s", result.EARSResults[0].Form)
	}
}

func TestValidateEARSRejectsMissingSHALL(t *testing.T) {
	root := t.TempDir()
	writeFeatureFile(t, root, "ears-fail", "requirements.md", `---
status: draft
approved_at:
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document

## Requirements

### R1 Feature

#### Acceptance Criteria

1. __BT__R1.AC1__BT__ WHEN triggered, the system creates a todo

## Non-Functional Requirements

- __BT__NFR1__BT__ Fast.

## Constraints And Dependencies

- __BT__C1__BT__ Local.

## Out Of Scope

- None.
`)

	result, err := ValidateFeature(root, "ears-fail")
	if err != nil {
		t.Fatalf("expected deterministic result, got error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected validation failure for missing SHALL")
	}
	if !strings.Contains(result.Message, "invalid EARS syntax") {
		t.Fatalf("expected EARS error in message, got: %s", result.Message)
	}
}

func TestLegacyProofWarningInValidation(t *testing.T) {
	root := t.TempDir()
	writeFeatureFile(t, root, "warn-test", "requirements.md", validRequirements)
	writeFeatureFile(t, root, "warn-test", "design.md", validDesign)
	writeFeatureFile(t, root, "warn-test", "tasks.md", validTasks)

	result, err := ValidateFeature(root, "warn-test")
	if err != nil {
		t.Fatalf("expected validation to succeed, got %v", err)
	}

	if !result.Valid {
		t.Fatalf("expected validation success, got: %s", result.Message)
	}

	if len(result.Warnings) == 0 {
		t.Fatal("expected deprecation warning for legacy proof format")
	}

	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "deprecated legacy verification format") && strings.Contains(w, "1.1") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected legacy proof warning for task 1.1, got warnings: %v", result.Warnings)
	}
}

func TestNoLegacyProofWarningForStructuredProofs(t *testing.T) {
	root := t.TempDir()
	writeFeatureFile(t, root, "nowarn-test", "requirements.md", validRequirements)
	writeFeatureFile(t, root, "nowarn-test", "design.md", validDesign)
	writeFeatureFile(t, root, "nowarn-test", "tasks.md", `---
status: approved
approved_at: 2026-03-21T14:20:00Z
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at: 2026-03-21T14:10:00Z
---

# Implementation Plan

- [ ] 1. Build feature
  - [ ] 1.1 Add implementation
    - Requirements: __BT__R1.AC1__BT__, __BT__NFR1__BT__
    - Design: Todo flow
    - Verification:
      - command: ["go", "test", "./..."]
`)

	result, err := ValidateFeature(root, "nowarn-test")
	if err != nil {
		t.Fatalf("expected validation to succeed, got %v", err)
	}

	if !result.Valid {
		t.Fatalf("expected validation success, got: %s", result.Message)
	}

	for _, w := range result.Warnings {
		if strings.Contains(w, "deprecated legacy verification format") {
			t.Fatalf("expected no legacy proof warnings for structured proofs, got: %s", w)
		}
	}
}

func TestValidateProofSyntaxAtValidateTime(t *testing.T) {
	tests := []struct {
		name         string
		tasks        string
		wantValid    bool
		wantContains string
	}{
		{
			name: "malformed JSON command array fails validation",
			tasks: `---
status: in-review
approved_at:
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at:
---

# Implementation Plan

- [ ] 1. Build feature
  - [ ] 1.1 Add implementation
    - Requirements: __BT__R1.AC1__BT__, __BT__NFR1__BT__
    - Design: Todo flow
    - Verification:
      - command: [not valid json]
`,
			wantValid:    false,
			wantContains: "invalid argv verification step",
		},
		{
			name: "structured block with no command field fails validation",
			tasks: `---
status: in-review
approved_at:
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at:
---

# Implementation Plan

- [ ] 1. Build feature
  - [ ] 1.1 Add implementation
    - Requirements: __BT__R1.AC1__BT__, __BT__NFR1__BT__
    - Design: Todo flow
    - Verification:
      - something: not a command
`,
			wantValid:    false,
			wantContains: "must include at least one command step",
		},
		{
			name: "valid structured proof passes validation",
			tasks: `---
status: in-review
approved_at:
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at:
---

# Implementation Plan

- [ ] 1. Build feature
  - [ ] 1.1 Add implementation
    - Requirements: __BT__R1.AC1__BT__, __BT__NFR1__BT__
    - Design: Todo flow
    - Verification:
      - command: ["go", "test", "./..."]
`,
			wantValid: true,
		},
		{
			name: "draft document with incomplete proof is allowed",
			tasks: `---
status: draft
approved_at:
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at:
---

# Implementation Plan

- [ ] 1. Build feature
  - [ ] 1.1 Add implementation
    - Requirements: __BT__R1.AC1__BT__, __BT__NFR1__BT__
    - Design: Todo flow
    - Verification: TODO
`,
			wantValid: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFeatureFile(t, root, "proof-test", "requirements.md", validRequirements)
			writeFeatureFile(t, root, "proof-test", "design.md", validDraftDesign)
			writeFeatureFile(t, root, "proof-test", "tasks.md", tc.tasks)

			result, err := ValidateFeatureWithScope(root, "proof-test", ScopeFullSpec)
			if err != nil {
				t.Fatalf("expected deterministic result, got error: %v", err)
			}

			if result.Valid != tc.wantValid {
				t.Fatalf("expected valid=%v, got valid=%v with message: %q", tc.wantValid, result.Valid, result.Message)
			}

			if !tc.wantValid && tc.wantContains != "" {
				if !strings.Contains(result.Message, tc.wantContains) {
					t.Fatalf("expected message to contain %q, got %q", tc.wantContains, result.Message)
				}
			}
		})
	}
}

// TestValidateTasksDraftTopLevelLeaf pins draft leaf detection to the full
// parser's semantics: a top-level task without dotted children is a leaf and
// subject to the draft metadata checks; one with children is a container.
func TestValidateTasksDraftTopLevelLeaf(t *testing.T) {
	const draftHeader = `---
status: draft
approved_at:
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at:
---

# Implementation Plan

`

	tests := []struct {
		name         string
		tasks        string
		wantValid    bool
		wantContains string
	}{
		{
			name: "top-level childless task with metadata is a valid leaf",
			tasks: draftHeader + `- [ ] 1. Ship the fix
  - Requirements: __BT__R1.AC1__BT__, __BT__NFR1__BT__
  - Design: Todo flow
  - Verification: TODO
`,
			wantValid: true,
		},
		{
			name: "top-level task with dotted children checks only the children",
			tasks: draftHeader + `- [ ] 1. Container objective
  - [ ] 1.1 Real step
    - Requirements: __BT__R1.AC1__BT__, __BT__NFR1__BT__
    - Design: Todo flow
    - Verification: TODO

- [ ] 2. Standalone leaf
  - Requirements: __BT__R1.AC1__BT__
  - Design: Todo flow
  - Verification: TODO
`,
			wantValid: true,
		},
		{
			name: "top-level leaf missing metadata fails like any leaf",
			tasks: draftHeader + `- [ ] 1. Ship the fix
  - Design: Todo flow
  - Verification: TODO
`,
			wantValid:    false,
			wantContains: "task 1 is missing Requirements",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFeatureFile(t, root, "leaf-test", "requirements.md", validRequirements)
			writeFeatureFile(t, root, "leaf-test", "design.md", validDraftDesign)
			writeFeatureFile(t, root, "leaf-test", "tasks.md", tc.tasks)

			result, err := ValidateFeatureWithScope(root, "leaf-test", ScopeFullSpec)
			if err != nil {
				t.Fatalf("expected deterministic result, got error: %v", err)
			}
			if result.Valid != tc.wantValid {
				t.Fatalf("expected valid=%v, got valid=%v with message: %q", tc.wantValid, result.Valid, result.Message)
			}
			if !tc.wantValid && !strings.Contains(result.Message, tc.wantContains) {
				t.Fatalf("expected message to contain %q, got %q", tc.wantContains, result.Message)
			}
		})
	}
}

func TestValidateFreshnessReportsTamperedRequirements(t *testing.T) {
	root := t.TempDir()
	writeValidFeature(t, root, "tamper-test")

	// Edit the approved requirements body without re-approval: the recorded
	// fingerprint no longer matches.
	path := filepath.Join(root, ".walden", "specs", "tamper-test", "requirements.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected requirements read to succeed, got %v", err)
	}
	if err := os.WriteFile(path, append(content, []byte("\nInjected after approval.\n")...), 0o644); err != nil {
		t.Fatalf("expected tampered write to succeed, got %v", err)
	}

	result, err := ValidateFeatureWithScope(root, "tamper-test", ScopeFullSpec)
	if err != nil {
		t.Fatalf("expected validation to run, got %v", err)
	}
	if result.Valid {
		t.Fatalf("expected validation failure for tampered requirements, got %#v", result)
	}
	if !strings.Contains(result.Message, "requirements.md is stale: content does not match approval fingerprint") {
		t.Fatalf("unexpected validation message: %q", result.Message)
	}
}

func TestValidateFreshnessReportsMissingFingerprintCause(t *testing.T) {
	root := t.TempDir()
	// Legacy fixture: approved before fingerprints existed.
	writeFeatureFileRaw(t, root, "legacy-test", "requirements.md", validRequirements)

	result, err := ValidateFeatureWithScope(root, "legacy-test", ScopeFullSpec)
	if err != nil {
		t.Fatalf("expected validation to run, got %v", err)
	}
	if result.Valid {
		t.Fatalf("expected validation failure for legacy approved document, got %#v", result)
	}
	if !strings.Contains(result.Message, "requirements.md is stale: approval fingerprint missing") {
		t.Fatalf("unexpected validation message: %q", result.Message)
	}
}

func TestValidateFreshnessFingerprintDecidesDespiteTimestampDivergence(t *testing.T) {
	root := t.TempDir()
	writeFeatureFile(t, root, "ts-test", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document

## Requirements

### R1 Feature

#### Acceptance Criteria

1. __BT__R1.AC1__BT__ The system SHALL work.

## Non-Functional Requirements

- __BT__NFR1__BT__ Fast.

## Constraints And Dependencies

- __BT__C1__BT__ Local.

## Out Of Scope

- None.
`)
	writeFeatureFile(t, root, "ts-test", "design.md", `---
status: approved
approved_at: 2026-03-21T14:10:00Z
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at: 2026-03-20T09:00:00Z
---

# Feature Design

## Overview

Design for __BT__R1__BT__.

## Architecture

Simple.

## Options Considered

### Option A

- Summary: Preferred.
- Why chosen: Simpler.

### Option B

- Summary: Alt.
- Why rejected: Complex.

## Simplicity And Elegance Review

- Simplest viable shape: Minimal.
- Coupling check: Low.
- Future-proofing: Deferred.

## Components And Interfaces

### Flow

- Purpose: __BT__R1__BT__.
- Inputs/Outputs: Local.
- Dependencies: None.
- Requirements: __BT__R1__BT__

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
- Test evidence: __BT__go test ./...__BT__
- Operational evidence: None.

## Requirement Coverage

| Requirement | Covered By |
| --- | --- |
| __BT__R1__BT__ | Flow |
| __BT__NFR1__BT__ | Tests |
`)

	result, err := ValidateFeatureWithScope(root, "ts-test", ScopeFullSpec)
	if err != nil {
		t.Fatalf("expected validation to succeed, got %v", err)
	}

	if !result.Valid {
		t.Fatalf("expected validation success with equivalent offset timestamp, got: %s", result.Message)
	}
}

func writeValidFeature(t *testing.T, root, feature string) {
	t.Helper()

	writeFeatureFile(t, root, feature, "requirements.md", validRequirements)
	writeFeatureFile(t, root, feature, "design.md", validDesign)
	writeFeatureFile(t, root, feature, "tasks.md", validTasks)
}

func writeFeatureFile(t *testing.T, root, feature, name, content string) {
	t.Helper()

	writeFeatureFileRaw(t, root, feature, name, content)
	stampFixtureFingerprint(t, root, feature, name)
}

func writeFeatureFileRaw(t *testing.T, root, feature, name, content string) {
	t.Helper()

	featureDir := filepath.Join(root, ".walden", "specs", feature)
	if err := os.MkdirAll(featureDir, 0o755); err != nil {
		t.Fatalf("expected feature directory creation to succeed, got %v", err)
	}
	content = strings.ReplaceAll(content, "__BT__", "`")
	if err := os.WriteFile(filepath.Join(featureDir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("expected %s write to succeed, got %v", name, err)
	}
}

// stampFixtureFingerprint keeps approved fixtures fingerprint-fresh: it
// records the document's own approval fingerprint and binds it to the
// upstream's, mirroring what `review approve` does in production.
func stampFixtureFingerprint(t *testing.T, root, featureName, name string) {
	t.Helper()

	feature, err := spec.LoadFeature(root, featureName)
	if err != nil {
		t.Fatalf("expected fixture feature load to succeed, got %v", err)
	}

	var document *spec.Document
	switch name {
	case "requirements.md":
		document = &feature.Requirements
	case "design.md":
		document = &feature.Design
	case "tasks.md":
		document = &feature.Tasks
	default:
		return
	}

	if !document.Exists || document.Status != "approved" {
		return
	}

	document.ApprovedFingerprint = spec.Fingerprint(document.Path, document.Body)
	document.Fields["approved_fingerprint"] = document.ApprovedFingerprint

	switch name {
	case "design.md":
		if feature.Requirements.Status == "approved" {
			fingerprint := feature.Requirements.ApprovedFingerprint
			if fingerprint == "" {
				fingerprint = spec.Fingerprint(feature.Requirements.Path, feature.Requirements.Body)
			}
			document.SourceRequirementsFingerprint = fingerprint
			document.Fields["source_requirements_fingerprint"] = fingerprint
		}
	case "tasks.md":
		if feature.Design.Status == "approved" {
			fingerprint := feature.Design.ApprovedFingerprint
			if fingerprint == "" {
				fingerprint = spec.Fingerprint(feature.Design.Path, feature.Design.Body)
			}
			document.SourceDesignFingerprint = fingerprint
			document.Fields["source_design_fingerprint"] = fingerprint
		}
	}

	if err := spec.SaveDocument(*document); err != nil {
		t.Fatalf("expected fixture fingerprint stamp to succeed, got %v", err)
	}
}

func TestValidateTasksDraftRejectsExecutorIllegalLayout(t *testing.T) {
	const draftHeader = `---
status: draft
approved_at:
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at:
---

# Implementation Plan

`

	tests := []struct {
		name         string
		tasks        string
		wantContains string
	}{
		{
			name: "top-level metadata at six spaces",
			tasks: draftHeader + `- [ ] 1. Flat task
      - Requirements: __BT__R1.AC1__BT__
      - Design: Todo flow
      - Verification: TODO
`,
			wantContains: `invalid metadata indentation for task "1": expected 2 or 4 spaces`,
		},
		{
			name: "child metadata at parent offset",
			tasks: draftHeader + `- [ ] 1. Container
  - [ ] 1.1 Child
  - Requirements: __BT__R1.AC1__BT__
  - Design: Todo flow
  - Verification: TODO
`,
			wantContains: `invalid metadata indentation for task "1.1": expected 4 spaces`,
		},
		{
			name: "proof step deeper than its verification block",
			tasks: draftHeader + `- [ ] 1. Flat task
  - Requirements: __BT__R1.AC1__BT__
  - Design: Todo flow
  - Verification:
      - command: ["go", "test", "./..."]
`,
			wantContains: `invalid proof step indentation for task "1": expected 4 spaces`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFeatureFile(t, root, "layout-test", "requirements.md", validRequirements)
			writeFeatureFile(t, root, "layout-test", "design.md", validDraftDesign)
			writeFeatureFile(t, root, "layout-test", "tasks.md", tc.tasks)

			result, err := ValidateFeatureWithScope(root, "layout-test", ScopeFullSpec)
			if err != nil {
				t.Fatalf("expected deterministic result, got error: %v", err)
			}
			if result.Valid {
				t.Fatal("expected draft validation to reject executor-illegal layout")
			}
			if !strings.Contains(result.Message, tc.wantContains) {
				t.Fatalf("expected message to contain %q, got %q", tc.wantContains, result.Message)
			}
		})
	}
}

func TestValidateTasksDraftAcceptsIncompleteProof(t *testing.T) {
	const draftTasks = `---
status: draft
approved_at:
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at:
---

# Implementation Plan

- [ ] 1. Flat task drafted incrementally
  - Requirements: __BT__R1.AC1__BT__
  - Design: Todo flow
  - Verification:

- [ ] 2. Container
  - [ ] 2.1 Child without steps yet
    - Requirements: __BT__R1.AC1__BT__
    - Design: Todo flow
    - Verification:
`

	root := t.TempDir()
	writeFeatureFile(t, root, "incomplete-test", "requirements.md", validRequirements)
	writeFeatureFile(t, root, "incomplete-test", "design.md", validDraftDesign)
	writeFeatureFile(t, root, "incomplete-test", "tasks.md", draftTasks)

	result, err := ValidateFeatureWithScope(root, "incomplete-test", ScopeFullSpec)
	if err != nil {
		t.Fatalf("expected deterministic result, got error: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected incomplete proofs to stay legal in draft, got: %q", result.Message)
	}
}
