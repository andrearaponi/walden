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
)

type ChainFingerprints struct {
	Requirements string
	Design       string
	Plans        map[string]PlanWitness
	PlanGaps     map[string]string
}

type LeafTask struct {
	ID          string
	Completed   bool
	Fingerprint string
	Definition  *spec.Task
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
}

// Derive is a pure assessment of explicit inputs. Missing observations never
// compare equal just because both strings happen to be empty.
func Derive(document Document, current ChainFingerprints, currentIdentity string, identityOK bool, tasks []LeafTask) []TaskEvidence {
	derived := make([]TaskEvidence, 0, len(tasks))
	if !identityOK {
		currentIdentity = ""
	}
	for _, task := range tasks {
		entry := TaskEvidence{TaskID: task.ID, CurrentIdentity: currentIdentity}
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
