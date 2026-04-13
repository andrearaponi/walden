package validation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestParseWaldenTimestampValidation(t *testing.T) {
	tests := []struct {
		name    string
		a, b    string
		wantEq  bool
		wantErr bool
	}{
		{"canonical equal", "2026-03-21T14:00:00Z", "2026-03-21T14:00:00Z", true, false},
		{"canonical different", "2026-03-21T14:00:00Z", "2026-03-21T15:00:00Z", false, false},
		{"offset equivalent", "2026-03-21T14:00:00Z", "2026-03-21T15:00:00+01:00", true, false},
		{"fractional seconds", "2026-03-21T14:00:00Z", "2026-03-21T14:00:00.000Z", true, false},
		{"malformed left", "not-a-timestamp", "2026-03-21T14:00:00Z", false, true},
		{"malformed right", "2026-03-21T14:00:00Z", "not-a-timestamp", false, true},
		{"both empty", "", "", true, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := timestampsEqual(tc.a, tc.b)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantEq {
				t.Fatalf("expected equal=%v, got %v", tc.wantEq, got)
			}
		})
	}
}

func TestValidateFreshnessWithEquivalentTimestampFormats(t *testing.T) {
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
source_requirements_approved_at: 2026-03-21T15:00:00+01:00
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

	featureDir := filepath.Join(root, ".walden", "specs", feature)
	if err := os.MkdirAll(featureDir, 0o755); err != nil {
		t.Fatalf("expected feature directory creation to succeed, got %v", err)
	}
	content = strings.ReplaceAll(content, "__BT__", "`")
	if err := os.WriteFile(filepath.Join(featureDir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("expected %s write to succeed, got %v", name, err)
	}
}
