package spec

import (
	"strings"
	"testing"
)

func TestParseTaskTreeParsesHierarchyAndMetadata(t *testing.T) {
	tree, err := ParseTaskTree(Document{
		Path:   "tasks.md",
		Exists: true,
		Body: `# Implementation Plan

- [ ] 1. Build the task parsing and execution-readiness core
  - [ ] 1.1 Implement the parser
    - Requirements: ` + "`R1`, `R2`, `NFR2`" + `
    - Design: Task Store, Executable Task
    - Verification: ` + "`go test ./internal/spec ./internal/workflow`" + `
  - [x] 1.2 Implement readiness resolution
    - Requirements: ` + "`R1`, `NFR3`" + `
    - Design: Execution Service
    - Verification: ` + "`go test ./internal/workflow ./internal/app`" + `

- [ ] 2. Implement deterministic lesson logging
  - [ ] 2.1 Implement lesson formatting
    - Requirements: ` + "`R5`, `NFR1`" + `
    - Design: Lesson Service
    - Verification: ` + "`go test ./internal/spec`" + `
`,
	})
	if err != nil {
		t.Fatalf("expected parse to succeed, got %v", err)
	}

	if len(tree.Tasks) != 2 {
		t.Fatalf("expected 2 top-level tasks, got %d", len(tree.Tasks))
	}

	firstParent := tree.Tasks[0]
	if firstParent.ID != "1" {
		t.Fatalf("expected first parent ID 1, got %q", firstParent.ID)
	}
	if firstParent.Title != "Build the task parsing and execution-readiness core" {
		t.Fatalf("unexpected parent title: %q", firstParent.Title)
	}
	if len(firstParent.Children) != 2 {
		t.Fatalf("expected 2 child tasks under parent 1, got %d", len(firstParent.Children))
	}

	firstLeaf := firstParent.Children[0]
	if firstLeaf.ID != "1.1" {
		t.Fatalf("expected first leaf ID 1.1, got %q", firstLeaf.ID)
	}
	if firstLeaf.ParentID != "1" {
		t.Fatalf("expected first leaf parent ID 1, got %q", firstLeaf.ParentID)
	}
	if firstLeaf.Completed {
		t.Fatal("expected first leaf to be incomplete")
	}
	if firstLeaf.Level != 2 {
		t.Fatalf("expected first leaf level 2, got %d", firstLeaf.Level)
	}
	if strings.Join(firstLeaf.Requirements, ",") != "R1,R2,NFR2" {
		t.Fatalf("unexpected requirements: %#v", firstLeaf.Requirements)
	}
	if strings.Join(firstLeaf.DesignRefs, ",") != "Task Store,Executable Task" {
		t.Fatalf("unexpected design refs: %#v", firstLeaf.DesignRefs)
	}
	if firstLeaf.Verification != "`go test ./internal/spec ./internal/workflow`" {
		t.Fatalf("unexpected verification: %q", firstLeaf.Verification)
	}
	if firstLeaf.Proof.LegacyCommand != "`go test ./internal/spec ./internal/workflow`" {
		t.Fatalf("unexpected legacy proof command: %q", firstLeaf.Proof.LegacyCommand)
	}

	secondLeaf := firstParent.Children[1]
	if !secondLeaf.Completed {
		t.Fatal("expected second leaf to be completed")
	}

	leafTasks := tree.LeafTasks()
	if len(leafTasks) != 3 {
		t.Fatalf("expected 3 leaf tasks, got %d", len(leafTasks))
	}

	found, ok := tree.FindTask("2.1")
	if !ok {
		t.Fatal("expected to find task 2.1")
	}
	if found.Title != "Implement lesson formatting" {
		t.Fatalf("unexpected found task title: %q", found.Title)
	}
}

func TestParseTaskTreeParsesStructuredVerificationSteps(t *testing.T) {
	tree, err := ParseTaskTree(Document{
		Path:   "tasks.md",
		Exists: true,
		Body: `# Implementation Plan

- [ ] 1. Harden verification parsing
  - [ ] 1.1 Add structured verification support
    - Requirements: ` + "`R2`" + `
    - Design: Task parser, Data Models
    - Verification:
      - argv: ["go", "test", "./internal/spec"]
      - argv: ["grep", "-c", "id=\"hero\"", "index.html"]
`,
	})
	if err != nil {
		t.Fatalf("expected parse to succeed, got %v", err)
	}

	task, ok := tree.FindTask("1.1")
	if !ok {
		t.Fatal("expected task 1.1 to exist")
	}
	if task.Proof.LegacyCommand != "" {
		t.Fatalf("expected no legacy proof command, got %q", task.Proof.LegacyCommand)
	}
	if len(task.Proof.Steps) != 2 {
		t.Fatalf("expected 2 structured proof steps, got %d", len(task.Proof.Steps))
	}
	if got, want := strings.Join(task.Proof.Steps[0].Argv, ","), "go,test,./internal/spec"; got != want {
		t.Fatalf("unexpected first argv step: %q", got)
	}
	if got, want := strings.Join(task.Proof.Steps[1].Argv, ","), "grep,-c,id=\"hero\",index.html"; got != want {
		t.Fatalf("unexpected second argv step: %q", got)
	}
	if !strings.Contains(task.Verification, `command ["go","test","./internal/spec"]`) {
		t.Fatalf("expected display verification to describe structured proof, got %q", task.Verification)
	}
}

func TestParseTaskTreeRejectsLeafWithoutVerification(t *testing.T) {
	_, err := ParseTaskTree(Document{
		Path:   "tasks.md",
		Exists: true,
		Body: `# Implementation Plan

- [ ] 1. Build parser
  - [ ] 1.1 Implement parser
    - Requirements: ` + "`R1`" + `
    - Design: Task Store
`,
	})
	if err == nil {
		t.Fatal("expected parse to fail without verification")
	}
	if !strings.Contains(err.Error(), `task "1.1" is missing Verification metadata`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseTaskTreeRejectsStructuredVerificationWithoutSteps(t *testing.T) {
	_, err := ParseTaskTree(Document{
		Path:   "tasks.md",
		Exists: true,
		Body: `# Implementation Plan

- [ ] 1. Harden verification parsing
  - [ ] 1.1 Add structured verification support
    - Requirements: ` + "`R2`" + `
    - Design: Task parser
    - Verification:
`,
	})
	if err == nil {
		t.Fatal("expected parse to fail without argv steps")
	}
	if !strings.Contains(err.Error(), `structured Verification for task "1.1" must include at least one command step`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseTaskTreeRejectsInvalidStructuredVerificationArgv(t *testing.T) {
	_, err := ParseTaskTree(Document{
		Path:   "tasks.md",
		Exists: true,
		Body: `# Implementation Plan

- [ ] 1. Harden verification parsing
  - [ ] 1.1 Add structured verification support
    - Requirements: ` + "`R2`" + `
    - Design: Task parser
    - Verification:
      - argv: ["go", 5]
`,
	})
	if err == nil {
		t.Fatal("expected parse to fail on invalid argv step")
	}
	if !strings.Contains(err.Error(), "invalid argv verification step") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseTaskTreeRejectsInvalidIndentation(t *testing.T) {
	_, err := ParseTaskTree(Document{
		Path:   "tasks.md",
		Exists: true,
		Body: `# Implementation Plan

- [ ] 1. Build parser
    - [ ] 1.1 Implement parser
      - Requirements: ` + "`R1`" + `
      - Design: Task Store
      - Verification: ` + "`go test ./internal/spec`" + `
`,
	})
	if err == nil {
		t.Fatal("expected parse to fail on invalid task indentation")
	}
	if !strings.Contains(err.Error(), "invalid task indentation") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseTaskTreeRejectsInvalidParentChildNumbering(t *testing.T) {
	_, err := ParseTaskTree(Document{
		Path:   "tasks.md",
		Exists: true,
		Body: `# Implementation Plan

- [ ] 1. Build parser
  - [ ] 2.1 Implement parser
    - Requirements: ` + "`R1`" + `
    - Design: Task Store
    - Verification: ` + "`go test ./internal/spec`" + `
`,
	})
	if err == nil {
		t.Fatal("expected parse to fail on invalid parent-child numbering")
	}
	if !strings.Contains(err.Error(), `task "2.1" does not belong to parent "1"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseTaskTreeRejectsParentTaskWithLeafMetadata(t *testing.T) {
	_, err := ParseTaskTree(Document{
		Path:   "tasks.md",
		Exists: true,
		Body: `# Implementation Plan

- [ ] 1. Build parser
    - Requirements: ` + "`R1`" + `
    - Design: Task Store
    - Verification: ` + "`go test ./internal/spec`" + `
  - [ ] 1.1 Implement parser
    - Requirements: ` + "`R1`" + `
    - Design: Task Store
    - Verification: ` + "`go test ./internal/spec`" + `
`,
	})
	if err == nil {
		t.Fatal("expected parse to fail when a parent task has executable metadata")
	}
	if !strings.Contains(err.Error(), `non-leaf task "1" cannot declare executable metadata`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMarkTaskCompleteAutoCompletesParentWhenLastChildCloses(t *testing.T) {
	document := Document{
		Path:   "tasks.md",
		Exists: true,
		Fields: map[string]string{
			"status":                    "approved",
			"approved_at":               "2026-03-21T14:20:00Z",
			"last_modified":             "2026-03-21T14:20:00Z",
			"source_design_approved_at": "2026-03-21T14:10:00Z",
		},
		Body: `# Implementation Plan

- [ ] 1. Build parser
  - [x] 1.1 Implement parser
    - Requirements: ` + "`R1`" + `
    - Design: Task Store
    - Verification: ` + "`go test ./internal/spec`" + `
  - [ ] 1.2 Implement readiness
    - Requirements: ` + "`R1`" + `
    - Design: Execution Service
    - Verification: ` + "`go test ./internal/workflow`" + `
`,
	}

	updated, err := MarkTaskComplete(document, "1.2", "2026-03-21T18:40:00Z")
	if err != nil {
		t.Fatalf("expected mark task complete to succeed, got %v", err)
	}

	tree, err := ParseTaskTree(updated)
	if err != nil {
		t.Fatalf("expected updated tree parse to succeed, got %v", err)
	}

	parent, ok := tree.FindTask("1")
	if !ok {
		t.Fatal("expected parent task to exist")
	}
	if !parent.Completed {
		t.Fatal("expected parent task to auto-complete when all children are complete")
	}

	child, ok := tree.FindTask("1.2")
	if !ok {
		t.Fatal("expected child task to exist")
	}
	if !child.Completed {
		t.Fatal("expected completed child task to be marked done")
	}
	if updated.LastModified != "2026-03-21T18:40:00Z" {
		t.Fatalf("unexpected last_modified: %q", updated.LastModified)
	}
	if updated.Fields["last_modified"] != "2026-03-21T18:40:00Z" {
		t.Fatalf("unexpected last_modified field: %q", updated.Fields["last_modified"])
	}
}

func TestParseTaskTreeParsesCommandKeyword(t *testing.T) {
	tree, err := ParseTaskTree(Document{
		Path:   "tasks.md",
		Exists: true,
		Body: `# Implementation Plan

- [ ] 1. Verify rename
  - [ ] 1.1 Check module path
    - Requirements: ` + "`R1`" + `
    - Design: Module rename
    - Verification:
      - command: ["grep", "-q", "walden", "go.mod"]
`,
	})
	if err != nil {
		t.Fatalf("expected parse to succeed, got %v", err)
	}

	task, ok := tree.FindTask("1.1")
	if !ok {
		t.Fatal("expected task 1.1 to exist")
	}
	if len(task.Proof.Steps) != 1 {
		t.Fatalf("expected 1 proof step, got %d", len(task.Proof.Steps))
	}
	if got := strings.Join(task.Proof.Steps[0].Argv, ","); got != "grep,-q,walden,go.mod" {
		t.Fatalf("unexpected command step: %q", got)
	}
	if task.Proof.Steps[0].ExpectExit != nil {
		t.Fatalf("expected nil ExpectExit, got %d", *task.Proof.Steps[0].ExpectExit)
	}
}

func TestParseTaskTreeParsesExpectExit(t *testing.T) {
	tree, err := ParseTaskTree(Document{
		Path:   "tasks.md",
		Exists: true,
		Body: `# Implementation Plan

- [ ] 1. Verify no residual
  - [ ] 1.1 Assert no andyarch references
    - Requirements: ` + "`R1`" + `
    - Design: Zero residual
    - Verification:
      - command: ["grep", "-rq", "andyarch", "."]
        expect_exit: 1
`,
	})
	if err != nil {
		t.Fatalf("expected parse to succeed, got %v", err)
	}

	task, _ := tree.FindTask("1.1")
	if len(task.Proof.Steps) != 1 {
		t.Fatalf("expected 1 proof step, got %d", len(task.Proof.Steps))
	}
	step := task.Proof.Steps[0]
	if step.ExpectExit == nil {
		t.Fatal("expected ExpectExit to be set")
	}
	if *step.ExpectExit != 1 {
		t.Fatalf("expected ExpectExit=1, got %d", *step.ExpectExit)
	}
}

func TestParseTaskTreeRejectsExpectExitWithoutPrecedingStep(t *testing.T) {
	_, err := ParseTaskTree(Document{
		Path:   "tasks.md",
		Exists: true,
		Body: `# Implementation Plan

- [ ] 1. Bad verification
  - [ ] 1.1 Orphan expect_exit
    - Requirements: ` + "`R1`" + `
    - Design: Bad
    - Verification:
        expect_exit: 1
`,
	})
	if err == nil {
		t.Fatal("expected parse to fail for orphan expect_exit")
	}
	if !strings.Contains(err.Error(), "expect_exit declared before any command step") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseTaskTreeAcceptsArgvAsBackwardCompatAlias(t *testing.T) {
	tree, err := ParseTaskTree(Document{
		Path:   "tasks.md",
		Exists: true,
		Body: `# Implementation Plan

- [ ] 1. Legacy format
  - [ ] 1.1 Use argv keyword
    - Requirements: ` + "`R1`" + `
    - Design: Backward compat
    - Verification:
      - argv: ["go", "test", "./..."]
`,
	})
	if err != nil {
		t.Fatalf("expected parse to succeed with argv alias, got %v", err)
	}

	task, _ := tree.FindTask("1.1")
	if len(task.Proof.Steps) != 1 {
		t.Fatalf("expected 1 proof step, got %d", len(task.Proof.Steps))
	}
}

func TestCoversFieldParsedFromProofStep(t *testing.T) {
	tree, err := ParseTaskTree(Document{
		Path:   "tasks.md",
		Exists: true,
		Body: `# Implementation Plan

- [ ] 1. Build feature
  - [ ] 1.1 Add implementation
    - Requirements: ` + "`R1.AC1`, `R1.AC2`" + `
    - Design: Components
    - Verification:
      - command: ["go", "test", "./..."]
        covers: ["R1.AC1", "R1.AC2"]
`,
	})
	if err != nil {
		t.Fatalf("expected parse to succeed with covers field, got %v", err)
	}

	task, _ := tree.FindTask("1.1")
	if len(task.Proof.Steps) != 1 {
		t.Fatalf("expected 1 proof step, got %d", len(task.Proof.Steps))
	}
	if len(task.Proof.Steps[0].Covers) != 2 {
		t.Fatalf("expected 2 covers entries, got %d", len(task.Proof.Steps[0].Covers))
	}
	if task.Proof.Steps[0].Covers[0] != "R1.AC1" || task.Proof.Steps[0].Covers[1] != "R1.AC2" {
		t.Fatalf("unexpected covers: %v", task.Proof.Steps[0].Covers)
	}
}

func TestCoversFieldOptionalOnProofStep(t *testing.T) {
	tree, err := ParseTaskTree(Document{
		Path:   "tasks.md",
		Exists: true,
		Body: `# Implementation Plan

- [ ] 1. Build feature
  - [ ] 1.1 Add implementation
    - Requirements: ` + "`R1.AC1`" + `
    - Design: Components
    - Verification:
      - command: ["go", "test", "./..."]
`,
	})
	if err != nil {
		t.Fatalf("expected parse to succeed without covers field, got %v", err)
	}

	task, _ := tree.FindTask("1.1")
	if len(task.Proof.Steps[0].Covers) != 0 {
		t.Fatalf("expected no covers when field is absent, got %v", task.Proof.Steps[0].Covers)
	}
}

func TestCoversFieldBeforeCommandStepFails(t *testing.T) {
	_, err := ParseTaskTree(Document{
		Path:   "tasks.md",
		Exists: true,
		Body: `# Implementation Plan

- [ ] 1. Build feature
  - [ ] 1.1 Add implementation
    - Requirements: ` + "`R1.AC1`" + `
    - Design: Components
    - Verification:
        covers: ["R1.AC1"]
      - command: ["go", "test", "./..."]
`,
	})
	if err == nil {
		t.Fatal("expected parse to fail when covers appears before command step")
	}
	if !strings.Contains(err.Error(), "covers declared before any command step") {
		t.Fatalf("unexpected error: %v", err)
	}
}
