// Package evidence owns the execution-evidence ledger: durable records
// binding each completed task's proof to the approved chain fingerprints and
// to a code identity of the working tree, with states derived from those
// bindings at read time — never stored.
package evidence

import (
	"crypto/sha256"
	"encoding/hex"
)

// SchemaVersion identifies the evidence document format, versioned
// independently from the CLI output contract.
const SchemaVersion = "v1alpha1"

// StepResult captures one executed proof step.
type StepResult struct {
	Command      []string `json:"command"`
	ExpectedExit int      `json:"expected_exit"`
	ActualExit   int      `json:"actual_exit"`
	OutputDigest string   `json:"output_digest"`
}

// Record is the evidence entry for one task: the proof outcome bound to the
// chain fingerprints and code identity in force when it ran. VerifiedAt is
// human context only and never influences a derived state.
type Record struct {
	TaskFingerprint         string       `json:"task_fingerprint"`
	RequirementsFingerprint string       `json:"requirements_fingerprint"`
	DesignFingerprint       string       `json:"design_fingerprint"`
	TasksFingerprint        string       `json:"tasks_fingerprint"`
	CodeIdentity            string       `json:"code_identity,omitempty"`
	Steps                   []StepResult `json:"steps"`
	Result                  string       `json:"result"`
	VerifiedAt              string       `json:"verified_at"`
}

// Result values recorded on entries.
const (
	ResultPassed = "passed"
	ResultFailed = "failed"
)

// DigestOutput hashes a proof step's combined output for the record: small,
// deterministic, and free of secrets-in-logs concerns.
func DigestOutput(output string) string {
	sum := sha256.Sum256([]byte(output))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// Document is the per-feature evidence ledger: a current-state map keyed by
// task id. History lives in git, not in the file.
type Document struct {
	SchemaVersion string            `json:"schema_version"`
	Feature       string            `json:"feature"`
	Tasks         map[string]Record `json:"tasks"`
}
