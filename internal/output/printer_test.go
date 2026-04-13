package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestPrintTextIncludesSummaryAndDetails(t *testing.T) {
	result := Result{
		Summary:         "repository initialized",
		ChangedFiles:    []string{".walden/lessons.md", ".github/workflows/validate-walden.yml"},
		SkippedFiles:    []string{"README.md"},
		CompletedTasks:  []string{"1.1", "1.2"},
		AutoCompleted:   []string{"1"},
		ValidatedPhases: []string{"requirements", "design"},
		SkippedPhases:   []string{"tasks"},
		GitInitialized:  true,
		Documents: []DocumentStatus{
			{Name: "requirements.md", Status: "approved", Fresh: true, ApprovedAt: "2026-03-21T14:00:00Z"},
		},
		Task: &TaskStatus{
			ID:           "1.2",
			Title:        "Implement readiness resolution",
			ParentID:     "1",
			Requirements: []string{"R1", "NFR3"},
			DesignRefs:   []string{"Execution Service"},
			Verification: "`go test ./internal/workflow ./internal/app`",
		},
		Blockers:   []string{"design.md is stale relative to requirements.md"},
		NextAction: "Run walden feature init todo-app-demo",
		Warnings:   []string{"git remote not configured"},
		ExitCode:   0,
	}

	var out bytes.Buffer
	PrintText(&out, result)

	rendered := out.String()
	for _, want := range []string{
		"repository initialized",
		".walden/lessons.md",
		"README.md",
		"Completed tasks: 1.1, 1.2",
		"Auto-completed tasks: 1",
		"Git: initialized new repository",
		"Validated phases: requirements, design",
		"Skipped phases: tasks",
		"requirements.md: status=approved fresh=true approved_at=2026-03-21T14:00:00Z",
		"Task: 1.2 Implement readiness resolution",
		"Task parent: 1",
		"Task requirements: R1, NFR3",
		"Task design refs: Execution Service",
		"Task verification: `go test ./internal/workflow ./internal/app`",
		"design.md is stale relative to requirements.md",
		"Run walden feature init todo-app-demo",
		"git remote not configured",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected output to contain %q, got %q", want, rendered)
		}
	}
}

func TestPrintJSONProducesVersionedEnvelope(t *testing.T) {
	result := Result{
		Summary:               "validation failed",
		ChangedFiles:          []string{},
		SkippedFiles:          []string{},
		CompletedTasks:        []string{"1.1"},
		AutoCompleted:         []string{"1"},
		ValidatedPhases:       []string{"requirements", "design", "tasks"},
		GitAlreadyInitialized: true,
		Documents: []DocumentStatus{
			{Name: "tasks.md", Status: "approved", Fresh: false, ApprovedAt: "2026-03-21T14:20:00Z"},
		},
		Task: &TaskStatus{
			ID:           "2.1",
			Title:        "Implement lesson service",
			Requirements: []string{"R5"},
			DesignRefs:   []string{"Lesson Service"},
			Verification: "`go test ./internal/spec`",
		},
		Blockers:   []string{"tasks.md is stale relative to design.md"},
		NextAction: "Fix requirements.md and rerun validation",
		Warnings:   []string{"design.md is stale"},
		ExitCode:   2,
	}

	var out bytes.Buffer
	if err := PrintJSON(&out, "validate", result); err != nil {
		t.Fatalf("expected json output to succeed, got %v", err)
	}

	var envelope Envelope
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("expected valid json, got %v", err)
	}

	if envelope.SchemaVersion != "v0alpha1" {
		t.Fatalf("expected schema_version %q, got %q", "v0alpha1", envelope.SchemaVersion)
	}
	if envelope.Command != "validate" {
		t.Fatalf("expected command %q, got %q", "validate", envelope.Command)
	}
	if envelope.OK {
		t.Fatalf("expected ok=false for exit code 2, got true")
	}

	decoded := envelope.Result
	if decoded.Summary != result.Summary {
		t.Fatalf("expected summary %q, got %q", result.Summary, decoded.Summary)
	}
	if decoded.ExitCode != result.ExitCode {
		t.Fatalf("expected exit code %d, got %d", result.ExitCode, decoded.ExitCode)
	}
	if len(decoded.Warnings) != 1 || decoded.Warnings[0] != "design.md is stale" {
		t.Fatalf("expected warnings to round-trip, got %#v", decoded.Warnings)
	}
	if len(decoded.ValidatedPhases) != 3 || decoded.ValidatedPhases[2] != "tasks" {
		t.Fatalf("expected validated phases to round-trip, got %#v", decoded.ValidatedPhases)
	}
	if !decoded.GitAlreadyInitialized {
		t.Fatalf("expected git bootstrap metadata to round-trip, got %#v", decoded)
	}
	if len(decoded.CompletedTasks) != 1 || decoded.CompletedTasks[0] != "1.1" {
		t.Fatalf("expected completed tasks to round-trip, got %#v", decoded.CompletedTasks)
	}
	if len(decoded.AutoCompleted) != 1 || decoded.AutoCompleted[0] != "1" {
		t.Fatalf("expected auto-completed tasks to round-trip, got %#v", decoded.AutoCompleted)
	}
	if len(decoded.Documents) != 1 || decoded.Documents[0].Name != "tasks.md" || decoded.Documents[0].Fresh {
		t.Fatalf("expected documents to round-trip, got %#v", decoded.Documents)
	}
	if decoded.Task == nil || decoded.Task.ID != "2.1" || decoded.Task.Title != "Implement lesson service" {
		t.Fatalf("expected task to round-trip, got %#v", decoded.Task)
	}
	if len(decoded.Blockers) != 1 || decoded.Blockers[0] != "tasks.md is stale relative to design.md" {
		t.Fatalf("expected blockers to round-trip, got %#v", decoded.Blockers)
	}
}

func TestPrintJSONEnvelopeOKTrueOnSuccess(t *testing.T) {
	result := Result{
		Summary:  "status ok",
		ExitCode: 0,
	}

	var out bytes.Buffer
	if err := PrintJSON(&out, "status", result); err != nil {
		t.Fatalf("expected json output to succeed, got %v", err)
	}

	var envelope Envelope
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("expected valid json, got %v", err)
	}

	if !envelope.OK {
		t.Fatalf("expected ok=true for exit code 0, got false")
	}
	if envelope.Command != "status" {
		t.Fatalf("expected command %q, got %q", "status", envelope.Command)
	}
}
