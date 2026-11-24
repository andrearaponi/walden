package app

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// The plans below are read with the requirements of the coverage block: four
// criteria, R1.AC1, R1.AC2, R1.AC10 and R2.AC1.
const traceGatePlanHeader = `---
status: approved
approved_at: 2026-03-21T14:20:00Z
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at: 2026-03-21T14:10:00Z
---

# Implementation Plan

`

// traceRejectedPlan leaves R1.AC2 without a leaf and names R2.AC1 without
// backticks: the parser reads it on leaf 1.2 and the validator does not
// count it. Leaf 1.1 names the heading R2 so that the validator reaches the
// criteria, and no step covers R1.AC10.
const traceRejectedPlan = traceGatePlanHeader + `- [ ] 1. Build feature
  - [ ] 1.1 Create and reopen
    - Requirements: ` + "`R1.AC1`, `R1.AC10`, `R2`" + `
    - Design: Architecture
    - Verification:
      - command: ["go", "test", "./..."]
        covers: ["R1.AC1"]
  - [ ] 1.2 Reject
    - Requirements: R2.AC1
    - Design: Architecture
    - Verification:
      - command: ["go", "vet", "./..."]
        covers: ["R2.AC1"]
`

// traceAcceptedPlan names every criterion, R1.AC2 on two leaves, and covers
// R1.AC1 and R2.AC1. It also names R1.AC9, which the requirements of this
// fixture mention and do not define.
const traceAcceptedPlan = traceGatePlanHeader + `- [ ] 1. Build feature
  - [ ] 1.1 Create and reject
    - Requirements: ` + "`R1.AC1`, `R1.AC2`, `R2.AC1`, `R1.AC9`" + `
    - Design: Architecture
    - Verification:
      - command: ["go", "test", "./..."]
        covers: ["R1.AC1", "R2.AC1"]
  - [ ] 1.2 Reopen
    - Requirements: ` + "`R1.AC10`, `R1.AC2`, `NFR1`" + `
    - Design: Architecture
    - Verification:
      - command: ["go", "vet", "./..."]
`

// traceCoverageHoles returns the holes of the first two kinds, as the wire
// holds them.
func traceCoverageHoles(t *testing.T) []any {
	t.Helper()
	_, block := traceBlock(t)
	holes := []any{}
	for _, entry := range block["holes"].([]any) {
		if kind := entry.(map[string]any)["kind"]; kind == "unnamed" || kind == "uncovered" {
			holes = append(holes, entry)
		}
	}
	return holes
}

// validateAll runs `walden validate <feature> --all --json` and returns its
// result as the wire holds it, with its exit code.
func validateAll(t *testing.T) (map[string]any, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run([]string{"validate", verifyTaskFeature, "--all", "--json"}, &stdout, &stderr)
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("the output of validate is not JSON: %v\n%s%s", err, stdout.String(), stderr.String())
	}
	result, _ := envelope["result"].(map[string]any)
	return result, code
}

// TestTraceHolesFollowValidate: the criteria a trace reports without a leaf
// or without a proof are the ones written here by hand, and they are the ones
// walden validate judges. Both plans hold a corner in which the reading of
// the validator and the parsed plan differ: a join computed from the parsed
// plan would give other holes.
func TestTraceHolesFollowValidate(t *testing.T) {
	t.Run("a plan the validator rejects", func(t *testing.T) {
		traceFixture(t, coverageBlockRequirements, coverageBlockDesign, traceRejectedPlan)

		want := []any{
			map[string]any{"kind": "unnamed", "statement": "R1.AC2"},
			// The leaf the parser read, which the validator does not count.
			map[string]any{"kind": "unnamed", "statement": "R2.AC1", "named_by": anyList("1.2")},
			map[string]any{"kind": "uncovered", "statement": "R1.AC10", "named_by": anyList("1.1")},
		}
		if got := traceCoverageHoles(t); !reflect.DeepEqual(got, want) {
			t.Errorf("holes = %v, want %v", got, want)
		}
		text, _, _ := runTraceCLI(t, verifyTaskFeature)
		for _, line := range []string{
			"- R2.AC1: named by 1.2\n",
			"- R1.AC2: no leaf names it\n",
			"- R2.AC1: named by 1.2, which walden validate does not count\n",
			"- R1.AC10: named by 1.1, covered by no proof step\n",
		} {
			if !strings.Contains(text, line) {
				t.Errorf("the text does not hold %q:\n%s", line, text)
			}
		}

		// The validator fails on the same plan, naming the two criteria.
		result, code := validateAll(t)
		summary, _ := result["summary"].(string)
		if code == 0 || summary != "INVALID: tasks.md missing coverage for acceptance criteria: R1.AC2, R2.AC1" {
			t.Errorf("validate --all exited %d saying %q, want it to fail on the two criteria", code, summary)
		}
		if _, present := result["coverage"]; present {
			t.Errorf("validate carries a coverage block for a plan it rejects: %v", result["coverage"])
		}
	})

	t.Run("a plan the validator accepts", func(t *testing.T) {
		requirements := strings.Replace(coverageBlockRequirements, "Scope.", "Scope. A removed criterion is still mentioned: `R1.AC9`.", 1)
		traceFixture(t, requirements, coverageBlockDesign, traceAcceptedPlan)

		want := []any{
			map[string]any{"kind": "uncovered", "statement": "R1.AC2", "named_by": anyList("1.1", "1.2")},
			map[string]any{"kind": "uncovered", "statement": "R1.AC10", "named_by": anyList("1.2")},
			// Mentioned and not defined: no listed statement, and the
			// validator counts it.
			map[string]any{"kind": "uncovered", "statement": "R1.AC9", "named_by": anyList("1.1")},
		}
		got := traceCoverageHoles(t)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("holes = %v, want %v", got, want)
		}
		_, block := traceBlock(t)
		for _, entry := range block["statements"].([]any) {
			if entry.(map[string]any)["id"] == "R1.AC9" {
				t.Errorf("the trace lists R1.AC9, which the requirements do not define")
			}
		}

		// The validator accepts the plan and lists the same criteria as
		// missing a proof.
		result, code := validateAll(t)
		if code != 0 {
			t.Fatalf("validate --all rejects the plan: %v", result["summary"])
		}
		coverage, _ := result["coverage"].(map[string]any)
		proof, _ := coverage["proof_reference_coverage"].(map[string]any)
		missing, _ := proof["missing"].([]any)
		reported := []string{}
		for _, hole := range got {
			reported = append(reported, hole.(map[string]any)["statement"].(string))
		}
		listed := []string{}
		for _, id := range missing {
			listed = append(listed, id.(string))
		}
		sort.Strings(reported)
		sort.Strings(listed)
		if len(listed) != 3 || !reflect.DeepEqual(reported, listed) {
			t.Errorf("the trace reports %v without a proof, validate lists %v", reported, listed)
		}
		task, _ := coverage["task_reference_coverage"].(map[string]any)
		if task["complete"] != true {
			t.Errorf("validate lists criteria without a leaf on a plan it accepts: %v", task)
		}
	})
}

// TestTraceStatesFollowReleaseCheck: the state of each leaf in a trace is
// the one written here by hand for a repository that holds four states at
// once, each proof step carries the state of its leaf, the holes of the
// leaves that are not standing are the ones written here, and walden release
// check reports the same states on the same repository.
func TestTraceStatesFollowReleaseCheck(t *testing.T) {
	traceStatesRepo(t)

	want := map[string]string{"1.1": "delivered", "1.2": "stale-code", "1.3": "verified", "1.4": "pending"}
	_, block := traceBlock(t)
	traced := map[string]string{}
	for _, entry := range block["leaves"].([]any) {
		leaf := entry.(map[string]any)
		traced[leaf["id"].(string)] = leaf["state"].(string)
	}
	if !reflect.DeepEqual(traced, want) {
		t.Errorf("states of the trace = %v, want %v", traced, want)
	}
	for _, entry := range block["statements"].([]any) {
		statement := entry.(map[string]any)
		for _, step := range statement["proofs"].([]any) {
			proof := step.(map[string]any)
			if proof["state"] != want[proof["task_id"].(string)] {
				t.Errorf("%v: step %v of %v carries the state %v, want the one of its leaf", statement["id"], proof["step"], proof["task_id"], proof["state"])
			}
		}
	}

	// After the change to the code, each statement a step of the leaf that
	// was verified covers is a hole with the state stale-code; the delivered
	// leaf adds none; the pending leaf adds its own.
	wantHoles := []any{
		traceNotStanding("R2.AC1", "1.2", 1, "stale-code"),
		traceNotStanding("R1.AC2", "1.2", 2, "stale-code"),
		traceNotStanding("R1.AC2", "1.4", 1, "pending"),
		traceNotStanding("R1.AC3", "1.2", 2, "stale-code"),
		traceNotStanding("R3.AC2", "1.4", 1, "pending"),
		traceNotStanding("NFR1", "1.2", 1, "stale-code"),
		traceNotStanding("C1", "1.2", 2, "stale-code"),
	}
	notStanding := []any{}
	for _, entry := range block["holes"].([]any) {
		if entry.(map[string]any)["kind"] == "not-standing" {
			notStanding = append(notStanding, entry)
		}
	}
	if !reflect.DeepEqual(notStanding, wantHoles) {
		t.Errorf("holes of the leaves that are not standing = %v, want %v", notStanding, wantHoles)
	}

	// The gate, on the same repository.
	var stdout, stderr bytes.Buffer
	Run([]string{"release", "check", verifyTaskFeature, "--json"}, &stdout, &stderr)
	var envelope struct {
		Result struct {
			Release struct {
				Features []struct {
					Evidence []struct {
						TaskID string `json:"task_id"`
						State  string `json:"state"`
					} `json:"evidence"`
				} `json:"features"`
			} `json:"release"`
		} `json:"result"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil || len(envelope.Result.Release.Features) != 1 {
		t.Fatalf("the output of release check holds no feature: %v\n%s%s", err, stdout.String(), stderr.String())
	}
	gate := map[string]string{}
	for _, entry := range envelope.Result.Release.Features[0].Evidence {
		gate[entry.TaskID] = entry.State
	}
	if !reflect.DeepEqual(gate, want) {
		t.Errorf("states of release check = %v, want %v", gate, want)
	}
}

// TestOtherResultsCarryNoTrace: the trace is one new block of the result of
// its own command. The results of the other commands hold no such key.
func TestOtherResultsCarryNoTrace(t *testing.T) {
	traceStatesRepo(t)
	for _, args := range [][]string{
		{"validate", verifyTaskFeature, "--all", "--json"},
		{"release", "check", verifyTaskFeature, "--json"},
		{"status", verifyTaskFeature, "--json"},
		{"task", "status", verifyTaskFeature, "--json"},
	} {
		var stdout, stderr bytes.Buffer
		Run(args, &stdout, &stderr)
		var envelope map[string]any
		if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
			t.Fatalf("walden %v printed no envelope: %v\n%s%s", args, err, stdout.String(), stderr.String())
		}
		result, _ := envelope["result"].(map[string]any)
		if result == nil {
			t.Fatalf("walden %v printed no result: %v", args, envelope)
		}
		if _, present := result["trace"]; present {
			t.Errorf("the result of walden %v holds a trace block", strings.Join(args, " "))
		}
	}
}
