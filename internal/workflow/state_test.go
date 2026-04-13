package workflow

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFeatureStateReturnsRequirementsPhaseForDraftRequirements(t *testing.T) {
	root := t.TempDir()
	writeFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: draft
approved_at:
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)

	state, err := LoadFeatureState(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("expected feature state load to succeed, got %v", err)
	}

	if state.CurrentPhase != PhaseRequirements {
		t.Fatalf("expected requirements phase, got %q", state.CurrentPhase)
	}
	if state.NextAction != "Edit requirements.md and move it to in-review" {
		t.Fatalf("unexpected next action: %q", state.NextAction)
	}
	if state.IsStale {
		t.Fatal("expected draft requirements not to be marked stale")
	}
	if len(state.Blockers) != 0 {
		t.Fatalf("expected no blockers, got %#v", state.Blockers)
	}
}

func TestLoadFeatureStateMarksDesignAndTasksStaleAfterRequirementsChange(t *testing.T) {
	root := t.TempDir()
	writeFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T15:00:00Z
last_modified: 2026-03-21T15:00:00Z
---

# Requirements Document
`)
	writeFeatureDoc(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-21T14:30:00Z
last_modified: 2026-03-21T14:30:00Z
source_requirements_approved_at: 2026-03-21T14:00:00Z
---

# Feature Design
`)
	writeFeatureDoc(t, root, "todo-app-demo", "tasks.md", `---
status: approved
approved_at: 2026-03-21T14:40:00Z
last_modified: 2026-03-21T14:40:00Z
source_design_approved_at: 2026-03-21T14:30:00Z
---

# Implementation Plan
`)

	state, err := LoadFeatureState(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("expected feature state load to succeed, got %v", err)
	}

	if !state.IsStale {
		t.Fatal("expected state to be stale")
	}
	if state.CurrentPhase != PhaseDesign {
		t.Fatalf("expected design to become current phase, got %q", state.CurrentPhase)
	}
	if state.Design.Fresh {
		t.Fatal("expected design to be stale")
	}
	if state.Tasks.Fresh {
		t.Fatal("expected tasks to inherit staleness from stale design")
	}
	if state.NextAction != "Update design.md to match requirements.md and return it to in-review" {
		t.Fatalf("unexpected next action: %q", state.NextAction)
	}
	assertContains(t, state.Blockers, "design.md is stale relative to requirements.md")
	assertContains(t, state.Blockers, "tasks.md is stale because design.md is stale")
}

func TestLoadFeatureStateReportsInvalidTopologyBlockers(t *testing.T) {
	root := t.TempDir()
	writeFeatureDoc(t, root, "todo-app-demo", "design.md", `---
status: draft
approved_at:
last_modified: 2026-03-21T14:00:00Z
source_requirements_approved_at:
---

# Feature Design
`)

	state, err := LoadFeatureState(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("expected feature state load to succeed, got %v", err)
	}

	if state.CurrentPhase != PhaseRequirements {
		t.Fatalf("expected requirements phase for invalid topology, got %q", state.CurrentPhase)
	}
	assertContains(t, state.Blockers, "design.md exists without requirements.md")
	if state.NextAction != "Create requirements.md" {
		t.Fatalf("unexpected next action: %q", state.NextAction)
	}
}

func TestLoadFeatureStateReturnsExecutionNextActionWhenAllSpecsAreApproved(t *testing.T) {
	root := t.TempDir()
	writeFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeFeatureDoc(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-21T14:10:00Z
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at: 2026-03-21T14:00:00Z
---

# Feature Design
`)
	writeFeatureDoc(t, root, "todo-app-demo", "tasks.md", `---
status: approved
approved_at: 2026-03-21T14:20:00Z
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at: 2026-03-21T14:10:00Z
---

# Implementation Plan
`)

	state, err := LoadFeatureState(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("expected feature state load to succeed, got %v", err)
	}

	if state.CurrentPhase != PhaseTasks {
		t.Fatalf("expected tasks phase when all specs are approved, got %q", state.CurrentPhase)
	}
	if state.NextAction != "Start execution from the next unchecked task" {
		t.Fatalf("unexpected next action: %q", state.NextAction)
	}
	if state.IsStale {
		t.Fatal("expected fresh approved state")
	}
	if len(state.Blockers) != 0 {
		t.Fatalf("expected no blockers, got %#v", state.Blockers)
	}
}

func writeFeatureDoc(t *testing.T, root, feature, name, content string) {
	t.Helper()

	featureDir := filepath.Join(root, ".walden", "specs", feature)
	if err := os.MkdirAll(featureDir, 0o755); err != nil {
		t.Fatalf("expected feature directory creation to succeed, got %v", err)
	}
	if err := os.WriteFile(filepath.Join(featureDir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("expected write for %q to succeed, got %v", name, err)
	}
}

func assertContains(t *testing.T, values []string, want string) {
	t.Helper()

	for _, value := range values {
		if value == want {
			return
		}
	}

	t.Fatalf("expected %#v to contain %q", values, want)
}

func TestTimestampFreshnessWithEquivalentFormats(t *testing.T) {
	root := t.TempDir()
	writeFeatureDoc(t, root, "ts-test", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeFeatureDoc(t, root, "ts-test", "design.md", `---
status: approved
approved_at: 2026-03-21T14:10:00Z
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at: 2026-03-21T15:00:00+01:00
---

# Feature Design
`)

	state, err := LoadFeatureState(root, "ts-test")
	if err != nil {
		t.Fatalf("expected state load to succeed, got %v", err)
	}

	if !state.Design.Fresh {
		t.Fatal("expected design to be fresh when source_requirements_approved_at is equivalent offset timestamp")
	}
	if state.IsStale {
		t.Fatal("expected state not to be stale with equivalent timestamps")
	}
}

func TestLoadFeatureStateReportsCompletedImplementationPlan(t *testing.T) {
	root := t.TempDir()
	writeFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeFeatureDoc(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-21T14:10:00Z
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at: 2026-03-21T14:00:00Z
---

# Feature Design
`)
	writeFeatureDoc(t, root, "todo-app-demo", "tasks.md", `---
status: approved
approved_at: 2026-03-21T14:20:00Z
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at: 2026-03-21T14:10:00Z
---

# Implementation Plan

- [x] 1. Build parser
  - [x] 1.1 Implement parser
    - Requirements: `+"`R1`"+`
    - Design: Task Store
    - Verification: `+"`go test ./internal/spec`"+`
`)

	state, err := LoadFeatureState(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("expected feature state load to succeed, got %v", err)
	}

	if state.CurrentPhase != PhaseTasks {
		t.Fatalf("expected tasks phase when all specs are approved, got %q", state.CurrentPhase)
	}
	if state.NextAction != "No runnable tasks remain; implementation plan is complete" {
		t.Fatalf("unexpected next action: %q", state.NextAction)
	}
	if state.IsStale {
		t.Fatal("expected fresh approved state")
	}
	if len(state.Blockers) != 0 {
		t.Fatalf("expected no blockers, got %#v", state.Blockers)
	}
}
