package workflow

import (
	"context"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/spec"
	"github.com/andrearaponi/walden/internal/testutil"
)

// The document-evolution contract, end to end: an x- extension declared on
// tasks.md survives the real approval gate, a task completion that rewrites
// the document, and a persisting verify — byte-identical value, schema
// version stamped, chain still fresh.
func TestExtensionFieldsSurviveFullCycle(t *testing.T) {
	root := t.TempDir()
	overrideIdentityRunner(t, identityYielding("100644 blob aaa\tmain.go\n"))

	writeApproveFeatureDoc(t, root, "todo-app-demo", "requirements.md", approvedRequirementsContent(approveReqBody, "2026-03-21T14:00:00Z"))
	writeApproveFeatureDoc(t, root, "todo-app-demo", "design.md", approvedDesignContent(approveDesignBody, "2026-03-21T14:10:00Z", "2026-03-21T14:00:00Z", spec.Fingerprint("requirements.md", approveReqBody)))
	writeApproveFeatureDoc(t, root, "todo-app-demo", "tasks.md", `---
status: in-review
approved_at:
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at:
x-tracking: JIRA-123
---

# Implementation Plan

- [ ] 1. Build parser
  - [ ] 1.1 Implement parser
    - Requirements: `+"`R1`"+`
    - Design: Task Store
    - Verification:
      - command: ["go", "test", "./internal/spec"]
`)

	// The real approval gate rewrites the document: the extension survives it.
	if _, err := ApproveReview(root, "todo-app-demo", PhaseTasks); err != nil {
		t.Fatalf("ApproveReview: %v", err)
	}

	// Task completion mutates the checkbox and rewrites the document again.
	runner := testutil.NewFakeRunner(testutil.Response{Stdout: "ok", ExitCode: 0})
	if _, err := CompleteTask(context.Background(), root, "todo-app-demo", "1.1", runner); err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}

	// A persisting verify refreshes evidence without touching the documents.
	verifyRunner := testutil.NewFakeRunner(testutil.Response{Stdout: "ok", ExitCode: 0})
	if _, err := Verify(context.Background(), root, "todo-app-demo", true, false, verifyRunner); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	feature, err := spec.LoadFeature(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("LoadFeature: %v", err)
	}
	tasks := feature.Tasks
	if tasks.Fields["x-tracking"] != "JIRA-123" {
		t.Fatalf("extension lost across the cycle: %v", tasks.Fields)
	}
	if tasks.Fields["walden_schema_version"] != spec.DocumentSchemaVersion {
		t.Fatalf("mutations did not stamp the schema version, got %q", tasks.Fields["walden_schema_version"])
	}
	if !strings.Contains(tasks.Body, "- [x] 1.1") {
		t.Fatal("completion did not mutate the plan")
	}
	if state := ResolveFeatureState(feature); !state.Tasks.Fresh {
		t.Fatal("the cycle left the approved tasks.md stale")
	}
}
