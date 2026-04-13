package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/output"
)

func TestRunValidatePrintsHumanReadableSuccess(t *testing.T) {
	root := t.TempDir()
	writeValidValidateFeature(t, root, "todo-app-demo")

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

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"validate", "todo-app-demo"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected validate to succeed, got %d and stderr %q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "VALID: .walden/specs/todo-app-demo") {
		t.Fatalf("expected success output, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunValidatePrintsJSONFailure(t *testing.T) {
	root := t.TempDir()
	writeValidateFeatureFile(t, root, "todo-app-demo", "requirements.md", validRequirementsForValidateCommand)
	writeValidateFeatureFile(t, root, "todo-app-demo", "design.md", validDesignForValidateCommand)
	writeValidateFeatureFile(t, root, "todo-app-demo", "tasks.md", `---
status: approved
approved_at: 2026-03-21T14:20:00Z
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at: 2026-03-21T14:10:00Z
---

# Implementation Plan

- [ ] 1. Build feature
  - [ ] 1.1 Add implementation
    - Requirements: __BT__NFR1__BT__
    - Design: Todo flow
    - Verification: __BT__go test ./...__BT__
`)

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

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"validate", "todo-app-demo", "--json"}, &stdout, &stderr)

	if exitCode != 1 {
		t.Fatalf("expected validate to fail with exit code 1, got %d", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr for json mode, got %q", stderr.String())
	}

	var envelope output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("expected valid json output, got %v", err)
	}
	result := envelope.Result
	if result.ExitCode != 1 {
		t.Fatalf("expected exit code 1 in JSON output, got %d", result.ExitCode)
	}
	if !strings.Contains(result.Summary, "INVALID: tasks.md missing task coverage for requirement IDs: R1") {
		t.Fatalf("unexpected json summary: %q", result.Summary)
	}
}

func TestRunValidateUsesCurrentPhaseScopeByDefault(t *testing.T) {
	root := t.TempDir()
	writeValidateFeatureFile(t, root, "todo-app-demo", "requirements.md", validRequirementsForValidateCommand)
	writeValidateFeatureFile(t, root, "todo-app-demo", "design.md", validDraftDesignForValidateCommand)
	writeValidateFeatureFile(t, root, "todo-app-demo", "tasks.md", invalidDraftTasksForValidateCommand)

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

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"validate", "todo-app-demo"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected current-phase validate to succeed, got %d and stderr %q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "VALID: .walden/specs/todo-app-demo") {
		t.Fatalf("expected success output, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Validated phases: requirements, design") {
		t.Fatalf("expected validated phases in output, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Skipped phases: tasks") {
		t.Fatalf("expected skipped phases in output, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunValidateAllFailsOnInvalidDownstreamTasks(t *testing.T) {
	root := t.TempDir()
	writeValidateFeatureFile(t, root, "todo-app-demo", "requirements.md", validRequirementsForValidateCommand)
	writeValidateFeatureFile(t, root, "todo-app-demo", "design.md", validDraftDesignForValidateCommand)
	writeValidateFeatureFile(t, root, "todo-app-demo", "tasks.md", invalidDraftTasksForValidateCommand)

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

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"validate", "todo-app-demo", "--all"}, &stdout, &stderr)

	if exitCode != 1 {
		t.Fatalf("expected full-spec validate to fail, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "INVALID: tasks.md missing task coverage for requirement IDs: R1") {
		t.Fatalf("expected downstream task coverage failure, got %q", stderr.String())
	}
}

const validRequirementsForValidateCommand = `---
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

const validDesignForValidateCommand = `---
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

const validTasksForValidateCommand = `---
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

const validDraftDesignForValidateCommand = `---
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

const invalidDraftTasksForValidateCommand = `---
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

func writeValidValidateFeature(t *testing.T, root, feature string) {
	t.Helper()

	writeValidateFeatureFile(t, root, feature, "requirements.md", validRequirementsForValidateCommand)
	writeValidateFeatureFile(t, root, feature, "design.md", validDesignForValidateCommand)
	writeValidateFeatureFile(t, root, feature, "tasks.md", validTasksForValidateCommand)
}

func writeValidateFeatureFile(t *testing.T, root, feature, name, content string) {
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
