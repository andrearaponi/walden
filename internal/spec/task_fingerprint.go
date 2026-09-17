package spec

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

// TaskFingerprintScheme identifies the canonical contract, independently of
// the document approval and ledger formats. Changing its encoding requires
// a new scheme, not a change to a human-readable rendering.
const TaskFingerprintScheme = "walden/task-definition/v2"

type taskContract struct {
	ID           string        `json:"id"`
	Title        string        `json:"title"`
	Requirements []string      `json:"requirements"`
	DesignRefs   []string      `json:"design_refs"`
	Proof        proofContract `json:"proof"`
}

type proofContract struct {
	Kind          string         `json:"kind"`
	LegacyCommand string         `json:"legacy_command"`
	Steps         []stepContract `json:"steps"`
}

type stepContract struct {
	Argv         []string `json:"argv"`
	ExpectExit   *int     `json:"expect_exit"`
	ExpectOutput *string  `json:"expect_output"`
	Timeout      *string  `json:"timeout"`
	Covers       []string `json:"covers"`
}

// TaskDefinitionFingerprint binds the entire parsed contract. Optional
// declarations retain presence and raw values; nil collections normalize to
// empty arrays. Progress, siblings and presentation never participate.
func TaskDefinitionFingerprint(task *Task) string {
	proof := proofContract{Kind: "steps", LegacyCommand: task.Proof.LegacyCommand, Steps: []stepContract{}}
	if task.Proof.LegacyCommand != "" {
		proof.Kind = "legacy"
	}
	for _, step := range task.Proof.Steps {
		proof.Steps = append(proof.Steps, stepContract{
			Argv: nonNilStrings(step.Argv), ExpectExit: step.ExpectExit,
			ExpectOutput: step.ExpectOutput, Timeout: step.Timeout,
			Covers: nonNilStrings(step.Covers),
		})
	}
	contract := taskContract{
		ID: task.ID, Title: task.Title, Requirements: nonNilStrings(task.Requirements),
		DesignRefs: nonNilStrings(task.DesignRefs), Proof: proof,
	}
	// These fixed structs contain only JSON-supported values and no cycles.
	payload, _ := json.Marshal(contract)
	return taskDigest(TaskFingerprintScheme + "\n" + string(payload))
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// LegacyTaskDefinitionFingerprint reproduces the pre-v2 identity solely for
// recovering a binding from a fingerprinted plan snapshot. It deliberately
// retains the old omissions; it must never authorize a current guarantee on
// its own or call the evolving Display method.
func LegacyTaskDefinitionFingerprint(task *Task) string {
	verification := task.Proof.LegacyCommand
	if strings.TrimSpace(verification) == "" {
		steps := make([]string, 0, len(task.Proof.Steps))
		for _, step := range task.Proof.Steps {
			argv, _ := json.Marshal(step.Argv)
			entry := "command " + string(argv)
			if step.Timeout != nil {
				entry += " timeout=" + *step.Timeout
			}
			steps = append(steps, entry)
		}
		verification = strings.Join(steps, "; ")
	}
	return taskDigest(strings.Join([]string{
		task.ID, task.Title, strings.Join(task.Requirements, ","),
		strings.Join(task.DesignRefs, ","), verification,
	}, "\x00"))
}

func taskDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])
}
