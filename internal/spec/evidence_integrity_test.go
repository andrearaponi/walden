package spec

import (
	"encoding/json"
	"strings"
	"testing"
)

const integrityIdentityPlan = `# Implementation Plan

- [ ] 1. Proof
  - Requirements: ` + "`R1.AC1`, `R1.AC2`" + `
  - Design: A, B
  - Verification:
    - command: ["printf", "PASS"]
      expect_exit: 0
      expect_output: "PASS"
      timeout: 60s
      covers: ["R1.AC1", "R1.AC2"]
    - command: ["true"]
- [ ] 2. Sibling
  - Requirements: ` + "`R2.AC1`" + `
  - Design: C
  - Verification:
    - command: ["true"]
`

func copyIntegrityTask(t *testing.T, task *Task) *Task {
	t.Helper()
	data, err := json.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	var copied Task
	if err := json.Unmarshal(data, &copied); err != nil {
		t.Fatal(err)
	}
	return &copied
}

func TestEvidenceIntegrityProofIdentity(t *testing.T) {
	base := parsePlanTask(t, integrityIdentityPlan, "1")
	fingerprint := TaskDefinitionFingerprint(base)
	changes := []struct {
		name string
		edit func(*Task)
	}{
		{"id", func(task *Task) { task.ID = "9" }},
		{"title", func(task *Task) { task.Title += " changed" }},
		{"requirements", func(task *Task) { task.Requirements[0] = "R9.AC1" }},
		{"requirement order", func(task *Task) {
			task.Requirements[0], task.Requirements[1] = task.Requirements[1], task.Requirements[0]
		}},
		{"design", func(task *Task) { task.DesignRefs[0] = "Other" }},
		{"design order", func(task *Task) { task.DesignRefs[0], task.DesignRefs[1] = task.DesignRefs[1], task.DesignRefs[0] }},
		{"argv", func(task *Task) { task.Proof.Steps[0].Argv[1] = "OTHER" }},
		{"expected exit", func(task *Task) { *task.Proof.Steps[0].ExpectExit = 1 }},
		{"declared zero versus default", func(task *Task) { task.Proof.Steps[0].ExpectExit = nil }},
		{"expected output", func(task *Task) { *task.Proof.Steps[0].ExpectOutput = "MISSING" }},
		{"absent output", func(task *Task) { task.Proof.Steps[0].ExpectOutput = nil }},
		{"timeout", func(task *Task) { *task.Proof.Steps[0].Timeout = "30s" }},
		{"timeout spelling", func(task *Task) { *task.Proof.Steps[0].Timeout = "1m" }},
		{"absent timeout", func(task *Task) { task.Proof.Steps[0].Timeout = nil }},
		{"coverage", func(task *Task) { task.Proof.Steps[0].Covers[0] = "R9.AC1" }},
		{"coverage order", func(task *Task) {
			task.Proof.Steps[0].Covers[0], task.Proof.Steps[0].Covers[1] = task.Proof.Steps[0].Covers[1], task.Proof.Steps[0].Covers[0]
		}},
		{"absent coverage", func(task *Task) { task.Proof.Steps[0].Covers = nil }},
		{"remove step", func(task *Task) { task.Proof.Steps = task.Proof.Steps[:1] }},
		{"add step", func(task *Task) {
			task.Proof.Steps = append(task.Proof.Steps, VerificationStep{Argv: []string{"false"}})
		}},
		{"step order", func(task *Task) { task.Proof.Steps[0], task.Proof.Steps[1] = task.Proof.Steps[1], task.Proof.Steps[0] }},
		{"legacy variant", func(task *Task) { task.Proof = VerificationSpec{LegacyCommand: "printf PASS"} }},
	}
	for _, change := range changes {
		t.Run(change.name, func(t *testing.T) {
			task := copyIntegrityTask(t, base)
			change.edit(task)
			task.Verification = task.Proof.Display()
			if got := TaskDefinitionFingerprint(task); got == fingerprint {
				t.Fatalf("%s retained identity %s for a different contract", change.name, got)
			}
		})
	}
	for name, body := range map[string]string{
		"progress":        strings.ReplaceAll(integrityIdentityPlan, "- [ ]", "- [x]"),
		"sibling":         strings.Replace(integrityIdentityPlan, "2. Sibling", "2. New sibling title", 1),
		"argv alias":      strings.ReplaceAll(integrityIdentityPlan, "command:", "argv:"),
		"JSON whitespace": strings.Replace(integrityIdentityPlan, `["printf", "PASS"]`, `[ "printf" , "PASS" ]`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if got := TaskDefinitionFingerprint(parsePlanTask(t, body, "1")); got != fingerprint {
				t.Fatalf("%s changed the unchanged task contract: %s != %s", name, got, fingerprint)
			}
		})
	}
	t.Run("display is not identity", func(t *testing.T) {
		task := copyIntegrityTask(t, base)
		task.Verification = "new human rendering"
		if TaskDefinitionFingerprint(task) != fingerprint {
			t.Fatal("presentation text changed the contract identity")
		}
	})
	t.Run("frozen legacy vectors", func(t *testing.T) {
		if got := LegacyTaskDefinitionFingerprint(base); got != "sha256:63cd82753563bb1f15287d8777af438b0315aeae8540c8ef7e927c38b7adef8b" {
			t.Fatalf("old structured identity changed: %s", got)
		}
		legacy := copyIntegrityTask(t, base)
		legacy.Proof = VerificationSpec{LegacyCommand: "go test ./..."}
		legacy.Verification = "a display change must not affect the frozen adapter"
		if got := LegacyTaskDefinitionFingerprint(legacy); got != "sha256:ecb919537c74dc483b1ea6e648ee3390acdc7e2cb6aeb8c18284fe3b97c3ae7b" {
			t.Fatalf("old legacy-command identity changed: %s", got)
		}
		if TaskDefinitionFingerprint(base) == LegacyTaskDefinitionFingerprint(base) {
			t.Fatal("new contract scheme was not separated from the legacy representation")
		}
	})
	t.Run("nil and empty collections", func(t *testing.T) {
		a := &Task{ID: "1", Title: "Empty", Proof: VerificationSpec{Steps: []VerificationStep{{Argv: []string{"true"}}}}}
		b := copyIntegrityTask(t, a)
		b.Requirements, b.DesignRefs, b.Proof.Steps[0].Covers = []string{}, []string{}, []string{}
		if TaskDefinitionFingerprint(a) != TaskDefinitionFingerprint(b) {
			t.Fatal("nil versus empty collection moved the fingerprint")
		}
	})
}
