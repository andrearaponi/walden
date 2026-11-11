package evidence

import "github.com/andrearaponi/walden/internal/spec"

const (
	StateVerified   = "verified"
	StateFailed     = "failed"
	StateStaleSpec  = "stale-spec"
	StateStaleCode  = "stale-code"
	StateUnattested = "unattested"
	StateUnrecorded = "unrecorded"
	StatePending    = "pending"
	// StateDelivered is the state of a completed task whose delivery proof
	// was attested on a commit. It is never reported as verified: the proof
	// is not expected to pass on later code, and is not run again.
	StateDelivered = "delivered"
)

type ChainFingerprints struct {
	Requirements string
	Design       string
	Plans        map[string]PlanWitness
	PlanGaps     map[string]string
	// Statements holds the fingerprint of each statement the requirements
	// define now. An attestation is compared with it, statement by statement.
	Statements map[string]string
}

type LeafTask struct {
	ID          string
	Completed   bool
	Fingerprint string
	Definition  *spec.Task
	// Delivery reports that the plan declares the task's proof as a delivery
	// proof.
	Delivery bool
	// Undeclared is the fingerprint the definition has without that
	// declaration, empty for a task that declares none. A pass recorded
	// before the declaration is bound to it.
	Undeclared string
}

// AttestedCommit is what a report shows of an attestation: the commit and
// whether adoption sealed it.
type AttestedCommit struct {
	Commit string `json:"commit"`
	Sealed bool   `json:"sealed"`
}

type Gap struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

type ExecutionAssessment struct {
	State  string          `json:"state"`
	Facts  *ExecutionFacts `json:"facts,omitempty"`
	Detail string          `json:"detail,omitempty"`
}

// TaskEvidence reports independent assessments even when one wins state
// precedence. Profiles/timestamps do not judge any of the three dimensions.
type TaskEvidence struct {
	TaskID           string
	State            string
	RecordedIdentity string
	CurrentIdentity  string
	Binding          BindingAssessment
	CodeFreshness    string
	Execution        ExecutionAssessment
	Gaps             []Gap
	RecordedProfile  Profile
	ProfileDrift     []ProfileDrift
	ProfileLegacy    bool
	RecordedResult   string
	// Explanation is filled by the evidence report after Derive, never by
	// Derive: it explains an assessment and takes no part in it.
	Explanation *Explanation
	// RecordedDurationMS is how long the recorded execution took, filled
	// the same way; nil when the record keeps no measurement.
	RecordedDurationMS *int64
	// Delivery reports that the task declares a delivery proof.
	Delivery bool
	// Attestation is set when the state is delivered.
	Attestation *AttestedCommit
	// AwaitsAdoption reports a pass recorded before the plan declared the
	// delivery, for the same proof and with supported provenance: only
	// adoption can seal it, and only a run that names its task replaces it.
	AwaitsAdoption bool
}

// Derive is a pure assessment of explicit inputs. Missing observations never
// compare equal just because both strings happen to be empty.
func Derive(document Document, current ChainFingerprints, currentIdentity string, identityOK bool, tasks []LeafTask) []TaskEvidence {
	derived := make([]TaskEvidence, 0, len(tasks))
	if !identityOK {
		currentIdentity = ""
	}
	for _, task := range tasks {
		entry := TaskEvidence{TaskID: task.ID, CurrentIdentity: currentIdentity, Delivery: task.Delivery}
		record, recorded := document.Tasks[task.ID]
		if !recorded {
			entry.State = StateUnrecorded
			if !task.Completed {
				entry.State = StatePending
			}
			derived = append(derived, entry)
			continue
		}
		entry.RecordedIdentity, entry.RecordedResult = record.CodeIdentity, record.Result
		entry.RecordedProfile, entry.ProfileLegacy = record.Profile, record.Profile == nil
		entry.Binding = assessBinding(record, current, task)
		entry.Execution = assessExecution(record)
		entry.AwaitsAdoption = AwaitsAdoption(record, task)
		entry.CodeFreshness = "current"
		switch {
		case currentIdentity == "" || record.CodeIdentity == "":
			entry.CodeFreshness = "unavailable"
			entry.Gaps = append(entry.Gaps, Gap{"code-unavailable", "a recorded or current code identity is unavailable"})
		case currentIdentity != record.CodeIdentity:
			entry.CodeFreshness = "stale"
			entry.Gaps = append(entry.Gaps, Gap{"code-stale", "recorded and current code identities differ"})
		}
		chainChanged := (record.RequirementsFingerprint != "" && current.Requirements != "" && record.RequirementsFingerprint != current.Requirements) ||
			(record.DesignFingerprint != "" && current.Design != "" && record.DesignFingerprint != current.Design)
		chainUnknown := record.RequirementsFingerprint == "" || record.DesignFingerprint == "" || current.Requirements == "" || current.Design == ""
		if chainChanged {
			entry.Gaps = append(entry.Gaps, Gap{"chain-changed", "recorded requirements/design bindings differ from the current chain"})
		}
		if chainUnknown {
			entry.Gaps = append(entry.Gaps, Gap{"chain-unavailable", "required approval-chain fingerprints are absent"})
		}
		if entry.Binding.State != "current" && entry.Binding.State != "reconstructed-equivalent" {
			entry.Gaps = append(entry.Gaps, Gap{"binding-" + entry.Binding.State, entry.Binding.Detail})
		}
		if entry.Execution.State != "supported" {
			entry.Gaps = append(entry.Gaps, Gap{"execution-" + entry.Execution.State, entry.Execution.Detail})
		}
		if record.Result != ResultPassed {
			entry.Gaps = append(entry.Gaps, Gap{"proof-failed", "recorded proof outcome is not passed"})
		}
		switch {
		case !task.Completed:
			entry.State = StatePending
		case record.Result != ResultPassed || entry.Execution.State == "rejected":
			entry.State = StateFailed
		case attestationRead(record, entry.Execution):
			// The attestation reading. The task is judged on its definition
			// and on the statements it names, as they were attested: the code
			// identity and the document fingerprints take no part, so their
			// gaps are not reported either.
			entry.Binding, entry.Gaps = readAttestation(record, current, task)
			entry.State = StateStaleSpec
			if len(entry.Gaps) == 0 {
				entry.State = StateDelivered
				entry.Attestation = &AttestedCommit{Commit: record.Attestation.Commit, Sealed: record.Attestation.Sealed}
			}
		case chainChanged || entry.Binding.State == "changed" || entry.Binding.State == "contradictory":
			entry.State = StateStaleSpec
		case entry.CodeFreshness == "stale":
			entry.State = StateStaleCode
		case chainUnknown || entry.Binding.State == "unknown" || entry.Execution.State != "supported" || entry.CodeFreshness == "unavailable":
			entry.State = StateUnattested
		default:
			entry.State = StateVerified
		}
		derived = append(derived, entry)
	}
	return derived
}

func assessExecution(record Record) ExecutionAssessment {
	facts := record.Execution
	if facts == nil {
		return ExecutionAssessment{State: "legacy-unattested", Detail: "legacy record has no producer/purity attestation; binding recovery cannot supply it"}
	}
	assessment := ExecutionAssessment{State: "unknown", Facts: facts}
	supportedVerify := facts.Origin == "verify" && facts.Policy == VerifyPolicy
	supportedComplete := facts.Origin == "complete" && facts.Policy == CompletionPolicy
	if !supportedVerify && !supportedComplete {
		assessment.Detail = "execution origin/policy is missing or unsupported"
		return assessment
	}
	if facts.Integrity == "mutated" || facts.Integrity == "contaminated" || (supportedVerify && facts.Integrity == "identity-unavailable") {
		assessment.State, assessment.Detail = "rejected", "recorded verification policy violation: "+facts.Integrity
		return assessment
	}
	if facts.AssertionResult != ResultPassed && facts.AssertionResult != ResultFailed {
		assessment.Detail = "execution assertion result is missing or unsupported"
		return assessment
	}
	if record.Result == ResultPassed && facts.AssertionResult != ResultPassed {
		assessment.State, assessment.Detail = "rejected", "passing record contradicts its failed assertion result"
		return assessment
	}
	if facts.AfterCodeIdentity == "" || record.CodeIdentity == "" || (supportedVerify && facts.BeforeCodeIdentity == "") {
		assessment.Detail = "execution is missing a required code-boundary observation"
		return assessment
	}
	if facts.AfterCodeIdentity != record.CodeIdentity || (supportedVerify && (facts.BeforeCodeIdentity != record.CodeIdentity || facts.CauseTask != "" || len(facts.ChangedPaths) != 0)) {
		assessment.State, assessment.Detail = "rejected", "execution identities or contamination facts contradict its claimed policy"
		return assessment
	}
	if (supportedVerify && facts.Integrity != "pure") || (supportedComplete && facts.Integrity != "post-state") {
		assessment.Detail = "execution integrity result is missing or unsupported"
		return assessment
	}
	assessment.State = "supported"
	return assessment
}
