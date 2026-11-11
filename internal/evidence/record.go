// Package evidence owns the execution-evidence ledger: durable records
// binding each completed task's proof to the approved chain fingerprints and
// to a code identity of the working tree, with states derived from those
// bindings at read time — never stored.
package evidence

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
)

// SchemaVersion identifies the evidence document format, versioned
// independently from the CLI output contract. It is the format of a ledger
// in which no record holds an attestation.
const SchemaVersion = "v1alpha2"

// AttestedSchemaVersion is the format of a ledger in which at least one
// record holds an attestation. Releases before proof kinds refuse it, naming
// the version: they would otherwise prove a delivery again, as a regression,
// and replace its attestation with a failure.
const AttestedSchemaVersion = "v1alpha3"

const (
	Guarantee        = "walden/evidence-integrity/v2"
	CompletionPolicy = "completion-post-state/v1"
	VerifyPolicy     = "verify-purity/v1"
)

// ExecutionFacts describe the producer lane and observed boundaries, not a
// derived status. A passing assertion is distinct from policy acceptance.
type ExecutionFacts struct {
	Origin             string   `json:"origin"`
	Policy             string   `json:"policy"`
	AssertionResult    string   `json:"assertion_result"`
	Integrity          string   `json:"integrity"`
	BeforeCodeIdentity string   `json:"before_code_identity,omitempty"`
	AfterCodeIdentity  string   `json:"after_code_identity,omitempty"`
	CauseTask          string   `json:"cause_task,omitempty"`
	ChangedPaths       []string `json:"changed_paths,omitempty"`
}

// TreeFacts are what a record's code identity was computed from: the commit
// HEAD named and the paths that differed from it. They are diagnostic, like
// the profile: Derive never reads them. Paths maps each differing path to its
// "mode:blob" entry, or to "" when the commit holds the path and the working
// tree did not. PathsOmitted is set instead when more paths differed than the
// recording limit, and says how many.
type TreeFacts struct {
	Commit       string            `json:"commit"`
	Paths        map[string]string `json:"paths,omitempty"`
	PathsOmitted int               `json:"paths_omitted,omitempty"`
}

// StatementFacts are the fingerprints of the requirement statements a task
// named when its record was written, with the scheme that built them. They
// let a later reader tell a change to those statements from an edit elsewhere
// in the requirements. Diagnostic, like TreeFacts: Derive never reads them.
type StatementFacts struct {
	Scheme       string            `json:"scheme"`
	Fingerprints map[string]string `json:"fingerprints"`
}

// StepResult captures one executed proof step. DurationMS is how long the
// step took, in milliseconds: a measurement of this one execution, never an
// estimate. It is a pointer so that zero is a measurement and absence, on a
// record written before durations were kept, is not.
type StepResult struct {
	Command      []string `json:"command"`
	ExpectedExit int      `json:"expected_exit"`
	ActualExit   int      `json:"actual_exit"`
	OutputDigest string   `json:"output_digest"`
	DurationMS   *int64   `json:"duration_ms,omitempty"`
}

// Profile is the execution environment captured when a record was produced:
// the platform and recording CLI version always, plus one entry per declared
// probe. Diagnostic data — Derive never reads it, and comparison happens
// only where evidence is presented.
type Profile map[string]string

// ProfileDrift is one differing profile entry: the recorded value against
// the current one.
type ProfileDrift struct {
	Key      string
	Recorded string
	Current  string
}

// Record is the evidence entry for one task: the proof outcome bound to the
// chain fingerprints and code identity in force when it ran. VerifiedAt is
// human context only and never influences a derived state; Profile is
// diagnostic context with the same standing.
type Record struct {
	TaskFingerprint         string          `json:"task_fingerprint"`
	TaskFingerprintScheme   string          `json:"task_fingerprint_scheme,omitempty"`
	Execution               *ExecutionFacts `json:"execution,omitempty"`
	RequirementsFingerprint string          `json:"requirements_fingerprint"`
	DesignFingerprint       string          `json:"design_fingerprint"`
	TasksFingerprint        string          `json:"tasks_fingerprint"`
	CodeIdentity            string          `json:"code_identity,omitempty"`
	Tree                    *TreeFacts      `json:"tree,omitempty"`
	Statements              *StatementFacts `json:"statements,omitempty"`
	Profile                 Profile         `json:"profile,omitempty"`
	Steps                   []StepResult    `json:"steps"`
	Result                  string          `json:"result"`
	VerifiedAt              string          `json:"verified_at"`
	// Attestation is present when the recorded pass of a delivery proof ran
	// on the tree of a commit. It is a fact of its own: it reads none of the
	// diagnostic facts above, and nothing else in the record reads it.
	Attestation *Attestation `json:"attestation,omitempty"`
}

// Attestation says that the recorded pass ran on the tree of Commit: the code
// identity of the record is the identity of that tree. An execution writes it
// from its own capture; adoption writes it, sealed, after finding the commit.
// It carries the definition and the statements it attests, because sealing
// adds it to a record and changes nothing else there: a sealed record stays
// bound to the definition without the declaration and may keep no statement
// fingerprints of its own.
type Attestation struct {
	Commit string `json:"commit"`
	// TaskFingerprint is the fingerprint of the definition that was attested.
	TaskFingerprint string `json:"task_fingerprint"`
	// Statements are the statements the task named, as they read then.
	Statements StatementFacts `json:"statements"`
	// Sealed marks an attestation written by adoption, not by an execution.
	Sealed bool `json:"sealed,omitempty"`
}

// DiffProfile compares a recorded profile against the current one, sorted by
// key. Either side nil — a legacy record, or capture unavailable — yields no
// drift: a comparison needs both facts.
func DiffProfile(recorded, current Profile) []ProfileDrift {
	if recorded == nil || current == nil {
		return nil
	}
	keys := map[string]bool{}
	for key := range recorded {
		keys[key] = true
	}
	for key := range current {
		keys[key] = true
	}

	drifts := []ProfileDrift{}
	for key := range keys {
		if recorded[key] != current[key] {
			drifts = append(drifts, ProfileDrift{Key: key, Recorded: recorded[key], Current: current[key]})
		}
	}
	sort.Slice(drifts, func(i, j int) bool { return drifts[i].Key < drifts[j].Key })
	return drifts
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
