package spec

import (
	"strings"
	"testing"
)

func expectOutputPlan(field string) string {
	return `# Implementation Plan

- [ ] 1. Objective
  - [ ] 1.1 Step
    - Requirements: ` + "`R1.AC1`" + `
    - Design: Section A
    - Verification:
` + field
}

func TestExpectOutputParsingVariants(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "bare content",
			body: expectOutputPlan("      - command: [\"go\", \"test\", \"./...\"]\n        expect_output: ok  \n"),
			want: "ok",
		},
		{
			name: "quoted content stripped",
			body: expectOutputPlan("      - command: [\"go\", \"test\", \"./...\"]\n        expect_output: \"PASS: TestAuth\"\n"),
			want: "PASS: TestAuth",
		},
		{
			name: "combined with expect_exit and covers",
			body: expectOutputPlan("      - command: [\"go\", \"test\", \"./...\"]\n        expect_exit: 0\n        expect_output: ok\n        covers: [\"R1.AC1\"]\n"),
			want: "ok",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree, err := ParseTaskTree(Document{Exists: true, Path: "tasks.md", Body: tc.body})
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			steps := tree.LeafTasks()[0].Proof.Steps
			if len(steps) != 1 || steps[0].ExpectOutput == nil {
				t.Fatalf("expect_output not parsed: %+v", steps)
			}
			if *steps[0].ExpectOutput != tc.want {
				t.Fatalf("expect_output = %q, want %q", *steps[0].ExpectOutput, tc.want)
			}
		})
	}
}

func TestExpectOutputParsingRejectsOrphanField(t *testing.T) {
	body := expectOutputPlan("      expect_output: ok\n")
	_, err := ParseTaskTree(Document{Exists: true, Path: "tasks.md", Body: body})
	if err == nil {
		t.Fatal("orphan expect_output accepted")
	}
}

func TestExpectOutputParsingLeavesPlainStepsUntouched(t *testing.T) {
	body := expectOutputPlan("      - command: [\"go\", \"test\", \"./...\"]\n")
	tree, err := ParseTaskTree(Document{Exists: true, Path: "tasks.md", Body: body})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if tree.LeafTasks()[0].Proof.Steps[0].ExpectOutput != nil {
		t.Fatal("plain step gained an unexpected expect_output")
	}
	if !strings.Contains(tree.LeafTasks()[0].Verification, "go") {
		t.Fatal("verification display broken")
	}
}
