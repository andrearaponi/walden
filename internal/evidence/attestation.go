package evidence

import (
	"fmt"
	"sort"
	"strings"

	"github.com/andrearaponi/walden/internal/spec"
)

// Attest returns the attestation of a recorded pass, or nil when the capture
// the record is bound to is not committed. Callers ask only for a delivery
// task: a regression proof is never attested.
func Attest(record Record, capture Capture) *Attestation {
	if record.Result != ResultPassed || record.Statements == nil || !capture.Committed() {
		return nil
	}
	statements := StatementFacts{Scheme: record.Statements.Scheme, Fingerprints: map[string]string{}}
	for id, fingerprint := range record.Statements.Fingerprints {
		statements.Fingerprints[id] = fingerprint
	}
	return &Attestation{Commit: capture.Commit, TaskFingerprint: record.TaskFingerprint, Statements: statements}
}

// NamedStatements picks, from the fingerprints of the statements a
// requirements body defines, those a task names: the identifiers of its
// Requirements line and those in the covers of its proof steps. An identifier
// that defines no statement, such as a requirement heading, gets no entry. A
// task that names none still gets the scheme with an empty set, so a later
// reader can tell "names no statement" from "no facts kept".
func NamedStatements(fingerprints map[string]string, requirements []string, proof spec.VerificationSpec) *StatementFacts {
	facts := &StatementFacts{Scheme: spec.StatementScheme, Fingerprints: map[string]string{}}
	add := func(id string) {
		if fingerprint, defined := fingerprints[id]; defined {
			facts.Fingerprints[id] = fingerprint
		}
	}
	for _, id := range requirements {
		add(id)
	}
	for _, step := range proof.Steps {
		for _, id := range step.Covers {
			add(id)
		}
	}
	return facts
}

// AwaitsAdoption reports that the recorded pass of a completed delivery task
// was written before the plan declared the delivery, for the proof the plan
// approves apart from that declaration, with supported execution provenance.
// It reads no tree fact and no statement fingerprint of the record: whether a
// commit holds the tree of the pass and whether its statements read as they
// did is for adoption to establish.
func AwaitsAdoption(record Record, task LeafTask) bool {
	return task.Delivery && task.Completed &&
		record.Attestation == nil &&
		record.Result == ResultPassed &&
		assessExecution(record).State == "supported" &&
		record.TaskFingerprintScheme == spec.TaskFingerprintScheme &&
		task.Undeclared != "" && record.TaskFingerprint == task.Undeclared &&
		(task.Definition == nil || stepsConsistent(record, task.Definition.Proof))
}

// attestationRead reports that a record is judged through its attestation:
// it holds a pass with supported execution provenance, a task fingerprint of
// the current scheme and an attestation this binary can read. A failed or
// rejected record, legacy or unknown provenance and an attestation in another
// statement scheme leave the task to the regression reading: no state is
// granted on a fact this binary cannot check.
func attestationRead(record Record, execution ExecutionAssessment) bool {
	attestation := record.Attestation
	return attestation != nil &&
		attestation.Commit != "" && attestation.TaskFingerprint != "" &&
		attestation.Statements.Scheme == spec.StatementScheme &&
		record.Result == ResultPassed && execution.State == "supported" &&
		record.TaskFingerprintScheme == spec.TaskFingerprintScheme
}

// readAttestation asks the two questions an attestation answers. Is the
// task's definition the attested one, and is the record a pass of it? Do the
// attested statements read as they did? It returns the binding and one gap
// for each answer that is no: none means the task is delivered.
func readAttestation(record Record, current ChainFingerprints, task LeafTask) (BindingAssessment, []Gap) {
	attestation := record.Attestation
	binding := BindingAssessment{Source: "attestation"}
	var gaps []Gap

	// A sealed record stays bound to the definition without the declaration:
	// sealing adds the attestation and changes nothing else.
	boundToAttested := record.TaskFingerprint == task.Fingerprint ||
		(attestation.Sealed && task.Undeclared != "" && record.TaskFingerprint == task.Undeclared)
	switch {
	case attestation.TaskFingerprint != task.Fingerprint:
		binding.State = "changed"
		binding.Detail = "the task's definition differs from the one attested at commit " + attestation.Commit
	case !boundToAttested || (task.Definition != nil && !stepsConsistent(record, task.Definition.Proof)):
		binding.State = "contradictory"
		binding.Detail = "the record is not a pass of the definition attested at commit " + attestation.Commit
	default:
		binding.State, binding.EffectiveFingerprint = "current", task.Fingerprint
	}
	if binding.State != "current" {
		gaps = append(gaps, Gap{"binding-" + binding.State, binding.Detail})
	}

	ids := make([]string, 0, len(attestation.Statements.Fingerprints))
	for id := range attestation.Statements.Fingerprints {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return spec.StatementLess(ids[i], ids[j]) })
	var changed []string
	for _, id := range ids {
		now, defined := current.Statements[id]
		switch {
		case !defined:
			changed = append(changed, id+" no longer defined")
		case now != attestation.Statements.Fingerprints[id]:
			changed = append(changed, id+" changed")
		}
	}
	if len(changed) > 0 {
		gaps = append(gaps, Gap{"attested-statements-changed", "statements attested at commit " + attestation.Commit + " read differently now: " + strings.Join(changed, ", ")})
	}
	return binding, gaps
}

// ExplainAttestation says why the recorded pass of a completed delivery task
// is no attestation, and what attests the task. It returns nil when there is
// nothing to say: the task is delivered, is no delivery task, or holds no
// recorded pass.
func ExplainAttestation(record Record, entry TaskEvidence, feature string) *AttestationExplanation {
	if !entry.Delivery || entry.State == StateDelivered || entry.State == StatePending ||
		record.Result != ResultPassed || record.Attestation != nil {
		return nil
	}

	explanation := &AttestationExplanation{}
	tree := record.Tree
	switch {
	case (tree == nil || tree.Commit == "") && record.Statements == nil:
		// Records keep statement facts since they keep tree facts: without
		// either, the record predates both.
		explanation.Unavailable = ReasonNoTree
		explanation.Detail = "the record keeps no commit and no paths, so it does not show whether its tree was committed"
	case tree == nil || tree.Commit == "":
		explanation.Unavailable = ReasonNoCommit
		explanation.Detail = "no commit could be read when the proof ran"
	case len(tree.Paths) > 0 || tree.PathsOmitted > 0:
		paths := len(tree.Paths)
		if tree.PathsOmitted > 0 {
			paths = tree.PathsOmitted
		}
		explanation.Unavailable = ReasonUncommittedPaths
		explanation.Detail = fmt.Sprintf("the proof ran on a tree with %d uncommitted path(s)", paths)
	default:
		explanation.Unavailable = ReasonUndeclaredRun
		explanation.Detail = fmt.Sprintf("the pass was recorded on commit %s by a run that did not read the task's proof as a delivery proof", tree.Commit)
	}

	// What attests the task. A pass that awaits adoption is not proven again
	// by a run that does not name its task: adoption comes first.
	if entry.AwaitsAdoption {
		explanation.Detail += fmt.Sprintf("; walden adopt %s shows whether the pass can be sealed and --apply seals it; a pass left unsealed is proven again with walden verify %s --task %s", feature, feature, entry.TaskID)
	} else {
		explanation.Detail += fmt.Sprintf("; a pass of walden verify %s on a tree without uncommitted changes attests it", feature)
	}
	return explanation
}

// SupportedExecution reports that a record holds supported execution
// provenance: the facts of a producer lane this binary knows, consistent
// with the record. A pass without them is never sealed.
func SupportedExecution(record Record) bool {
	return assessExecution(record).State == "supported"
}
