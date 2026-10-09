package spec

import "testing"

// v0130WellFormedPlan exercises every proof form that v0.13.0 reads in full:
// top-level leaves with two- and four-space metadata, a child task, every
// attribute with quoted and unquoted values, an argv step, blank lines inside
// a block, a legacy one-line proof and free text outside the blocks.
const v0130WellFormedPlan = `# Implementation Plan

Free text between the heading and the first task.

- [ ] 1. Top-level leaf with natural offsets
  - Requirements: ` + "`R1.AC1`, `NFR1`" + `
  - Design: Parser
  - Verification:
    - command: ["go", "test", "-v", "-count=1", "-run", "^TestA$", "./internal/a"]
      expect_output: "--- PASS: TestA"
      timeout: 30m
      covers: ["R1.AC1"]

    - command: ["sh", "-c", "test -f go.mod"]
      expect_exit: 0
      expect_output: go.mod
      timeout: "90s"
      covers: ["R1.AC1", "NFR1"]

- [x] 2. Top-level leaf with legacy offsets
    - Requirements: ` + "`R1.AC2`" + `
    - Design: Parser, Layout
    - Verification:
      - argv: ["go", "vet", "./..."]
        expect_exit: 1

- [ ] 3. Container
  - [ ] 3.1 Child with every attribute
    - Requirements: ` + "`R2.AC1`" + `
    - Design: Executor
    - Verification:
      - command: ["go", "build", "./..."]
      - command: ["go", "test", "./internal/b"]
        expect_exit: 2
        expect_output: "  padded output  "
        covers: ["R2.AC1"]
        timeout: 2m
  - [ ] 3.2 Child with a legacy one-line proof
    - Requirements: ` + "`R2.AC2`" + `
    - Design: Executor
    - Verification: go test ./...

## Notes

Prose after the plan stays free text.
`

// emptyCoversPlan declares covers: [] on a first step that a second step
// follows.
const emptyCoversPlan = `# Implementation Plan

- [ ] 1. Empty coverage before a second step
  - Requirements: ` + "`R1.AC1`" + `
  - Design: Parser
  - Verification:
    - command: ["go", "test", "./internal/spec"]
      covers: []
    - command: ["go", "vet", "./internal/spec"]
      expect_exit: 0
`

// v0130EmptyCoversFingerprint is the definition fingerprint v0.13.0 derived
// for task 1 of emptyCoversPlan, having read only its first step.
const v0130EmptyCoversFingerprint = "sha256:7c096bf235f017888e93c2af7b7ca3b0a3a2d0813c5b8ef50d77e045b84f6030"

// TestParseTaskTreeKeepsV0130Reading pins what v0.13.0 read from a plan it
// reads in full: the leaves, their steps and their definition fingerprints.
// The golden values were printed by the v0.13.0 parser.
func TestParseTaskTreeKeepsV0130Reading(t *testing.T) {
	tree, err := ParseTaskTree(Document{Exists: true, Path: "tasks.md", Body: v0130WellFormedPlan})
	if err != nil {
		t.Fatalf("parse the well-formed plan: %v", err)
	}

	want := []struct {
		id           string
		steps        int
		verification string
		fingerprint  string
	}{
		{
			id:           "1",
			steps:        2,
			verification: `command ["go","test","-v","-count=1","-run","^TestA$","./internal/a"] timeout=30m; command ["sh","-c","test -f go.mod"] timeout=90s`,
			fingerprint:  "sha256:022136de6cb89b51427daac3b1f4d72559c68837ae4237dcc69927de0af57646",
		},
		{
			id:           "2",
			steps:        1,
			verification: `command ["go","vet","./..."]`,
			fingerprint:  "sha256:72b753e82a296f6bf0ab08d50b030e65d49d4d1e3e41c45d9d81a051d43a73c5",
		},
		{
			id:           "3.1",
			steps:        2,
			verification: `command ["go","build","./..."]; command ["go","test","./internal/b"] timeout=2m`,
			fingerprint:  "sha256:7d5cc3fb3efe725a78bf923ce33d009ceb18c48107e53861878c8dfd812da791",
		},
		{
			id:           "3.2",
			steps:        0,
			verification: "go test ./...",
			fingerprint:  "sha256:2bc589875d39b172e82d3e7559b029439821f8a529928c108cb07dc522fd5913",
		},
	}

	leaves := tree.LeafTasks()
	if len(leaves) != len(want) {
		t.Fatalf("read %d leaves, v0.13.0 read %d", len(leaves), len(want))
	}
	for i, leaf := range leaves {
		w := want[i]
		if leaf.ID != w.id || len(leaf.Proof.Steps) != w.steps || leaf.Verification != w.verification {
			t.Errorf("leaf %d is %s with %d steps, verification %q; v0.13.0 read %s with %d steps, verification %q",
				i, leaf.ID, len(leaf.Proof.Steps), leaf.Verification, w.id, w.steps, w.verification)
		}
		if got := TaskDefinitionFingerprint(leaf); got != w.fingerprint {
			t.Errorf("task %s fingerprint is %s; v0.13.0 derived %s", leaf.ID, got, w.fingerprint)
		}
	}
}
