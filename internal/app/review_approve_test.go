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

// approveGateRequirements is a minimal requirements document that passes full
// validation, so the pre-approval gate lets the fixture through.
const approveGateRequirements = `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document

## Introduction

Approve fixture.

## Requirements

### R1 Gate

**User Story:** As a tester, I want a valid chain, so that approval can proceed.

#### Acceptance Criteria

1. ` + "`R1.AC1`" + ` WHEN the reviewer approves, the system SHALL record the approval.
`

// approveGateDesignInReview carries every section the validator requires.
const approveGateDesignInReview = `---
status: in-review
approved_at:
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at:
---

# Feature Design

## Overview

Valid minimal design.

## Architecture

One component.

## Options Considered

### Option A

- Summary: minimal.
- Why chosen: simplest.

### Option B

- Summary: none.
- Why rejected: n/a.

## Simplicity And Elegance Review

- Simplest viable shape: minimal.
- Coupling check: none.
- Future-proofing: none.

## Failure Modes And Tradeoffs

- Failure mode: none.
- Mitigation: none.
- Tradeoff: none.

## Verification Plan

- Requirement proof: tests.
- Test evidence: go test.

## Requirement Coverage

| Requirement | Covered By |
| --- | --- |
| ` + "`R1`" + ` | Gate |
`

func TestRunReviewApprovePrintsNextAction(t *testing.T) {
	root := t.TempDir()
	writeReviewApproveCommandFile(t, root, "todo-app-demo", "requirements.md", approveGateRequirements)
	writeReviewApproveCommandFile(t, root, "todo-app-demo", "design.md", approveGateDesignInReview)

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

	exitCode := Run([]string{"review", "approve", "todo-app-demo", "--phase", "design"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected review approve to succeed, got %d and stderr %q", exitCode, stderr.String())
	}

	rendered := stdout.String()
	for _, want := range []string{
		"review gate approved for design.md",
		"Document: .walden/specs/todo-app-demo/design.md",
		"Current phase: tasks",
		"Next action: Create tasks.md",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected output to contain %q, got %q", want, rendered)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunReviewApprovePrintsJSON(t *testing.T) {
	root := t.TempDir()
	writeReviewApproveCommandFile(t, root, "todo-app-demo", "requirements.md", approveGateRequirements)
	writeReviewApproveCommandFile(t, root, "todo-app-demo", "design.md", approveGateDesignInReview)

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

	exitCode := Run([]string{"review", "approve", "todo-app-demo", "--phase", "design", "--json"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected review approve --json to succeed, got %d and stderr %q", exitCode, stderr.String())
	}

	var envelope output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("expected valid json, got %v", err)
	}
	if envelope.Command != "review-approve" {
		t.Fatalf("expected command review-approve, got %q", envelope.Command)
	}
	if !envelope.OK {
		t.Fatalf("expected ok=true, got false")
	}
}

func TestRunReviewApproveRefusesInvalidDocument(t *testing.T) {
	root := t.TempDir()
	writeReviewApproveCommandFile(t, root, "todo-app-demo", "requirements.md", approveGateRequirements)
	approvedDesign := strings.Replace(approveGateDesignInReview, "status: in-review", "status: approved", 1)
	approvedDesign = strings.Replace(approvedDesign, "approved_at:\n", "approved_at: 2026-03-21T14:20:00Z\n", 1)
	approvedDesign = strings.Replace(approvedDesign, "source_requirements_approved_at:\n", "source_requirements_approved_at: 2026-03-21T14:00:00Z\n", 1)
	writeReviewApproveCommandFile(t, root, "todo-app-demo", "design.md", approvedDesign)
	writeReviewApproveCommandFile(t, root, "todo-app-demo", "tasks.md", `---
status: in-review
approved_at:
last_modified: 2026-03-21T14:30:00Z
source_design_approved_at:
---

# Implementation Plan

- [ ] 1. Objective
  - [ ] 1.1 Step
      - Requirements: `+"`R1.AC1`"+`
      - Design: Overview
      - Verification:
        - command: ["go", "version"]
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

	exitCode := Run([]string{"review", "approve", "todo-app-demo", "--phase", "tasks"}, &stdout, &stderr)

	if exitCode != 1 {
		t.Fatalf("expected review approve to refuse with exit 1, got %d (stdout %q, stderr %q)", exitCode, stdout.String(), stderr.String())
	}
	rendered := stderr.String()
	if !strings.Contains(rendered, "approval refused") {
		t.Fatalf("expected refusal to say 'approval refused', got %q", rendered)
	}
	if !strings.Contains(rendered, "invalid metadata indentation") {
		t.Fatalf("expected refusal to carry the validation defect, got %q", rendered)
	}

	tasksRaw, err := os.ReadFile(filepath.Join(root, ".walden", "specs", "todo-app-demo", "tasks.md"))
	if err != nil {
		t.Fatalf("expected tasks read-back to succeed, got %v", err)
	}
	if !strings.Contains(string(tasksRaw), "status: in-review") {
		t.Fatalf("expected document status to remain in-review, got %q", string(tasksRaw))
	}
}

func writeReviewApproveCommandFile(t *testing.T, root, feature, name, content string) {
	t.Helper()

	featureDir := filepath.Join(root, ".walden", "specs", feature)
	if err := os.MkdirAll(featureDir, 0o755); err != nil {
		t.Fatalf("expected feature directory creation to succeed, got %v", err)
	}
	if err := os.WriteFile(filepath.Join(featureDir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("expected write for %q to succeed, got %v", name, err)
	}

	stampSpecFingerprint(t, root, feature, name)
}
