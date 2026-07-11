package spec

import (
	"strings"
	"testing"
)

const fingerprintPlan = `# Implementation Plan

- [%s] 1. Objective
  - [%s] 1.1 First step
    - Requirements: ` + "`R1.AC1`" + `
    - Design: Section A
    - Verification:
      - command: ["go", "test", "./..."]
  - [ ] 1.2 Sibling step
    - Requirements: ` + "`R2.AC1`" + `
    - Design: Section B
    - Verification:
      - command: ["go", "vet", "./..."]
`

func parsePlanTask(t *testing.T, body, id string) *Task {
	t.Helper()
	tree, err := ParseTaskTree(Document{Exists: true, Path: "tasks.md", Body: body})
	if err != nil {
		t.Fatalf("parse plan: %v", err)
	}
	for _, task := range tree.LeafTasks() {
		if task.ID == id {
			return task
		}
	}
	t.Fatalf("task %s not found", id)
	return nil
}

func planWith(checkTop, checkLeaf string) string {
	return strings.Replace(strings.Replace(fingerprintPlan, "%s", checkTop, 1), "%s", checkLeaf, 1)
}

func TestTaskDefinitionFingerprintIgnoresProgressAndSiblings(t *testing.T) {
	unchecked := TaskDefinitionFingerprint(parsePlanTask(t, planWith(" ", " "), "1.1"))
	checked := TaskDefinitionFingerprint(parsePlanTask(t, planWith(" ", "x"), "1.1"))
	if unchecked != checked {
		t.Fatal("checkbox state moved the task definition fingerprint")
	}

	siblingEdited := strings.Replace(planWith(" ", " "), "Sibling step", "Renamed sibling", 1)
	afterSibling := TaskDefinitionFingerprint(parsePlanTask(t, siblingEdited, "1.1"))
	if unchecked != afterSibling {
		t.Fatal("a sibling edit moved the task definition fingerprint")
	}
}

func TestTaskDefinitionFingerprintTracksTheContract(t *testing.T) {
	base := TaskDefinitionFingerprint(parsePlanTask(t, planWith(" ", " "), "1.1"))

	verificationEdited := strings.Replace(planWith(" ", " "), `["go", "test", "./..."]`, `["go", "test", "-count=2", "./..."]`, 1)
	moved := TaskDefinitionFingerprint(parsePlanTask(t, verificationEdited, "1.1"))
	if base == moved {
		t.Fatal("a verification edit did not move the task definition fingerprint")
	}

	titleEdited := strings.Replace(planWith(" ", " "), "First step", "First step, renamed", 1)
	movedTitle := TaskDefinitionFingerprint(parsePlanTask(t, titleEdited, "1.1"))
	if base == movedTitle {
		t.Fatal("a title edit did not move the task definition fingerprint")
	}
}
