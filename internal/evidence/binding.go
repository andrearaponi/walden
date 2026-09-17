package evidence

import (
	"reflect"
	"strings"

	"github.com/andrearaponi/walden/internal/spec"
)

// PlanWitness is a fingerprinted body, not a reconstructed approval or an
// execution attestation. Runtime I/O supplies witnesses; Derive only reads them.
type PlanWitness struct {
	Fingerprint string
	Path        string
	Commit      string
	Source      string
	Tasks       map[string]*spec.Task
}

type BindingAssessment struct {
	State                string `json:"state"`
	Source               string `json:"source,omitempty"`
	EffectiveFingerprint string `json:"effective_fingerprint,omitempty"`
	PlanFingerprint      string `json:"plan_fingerprint,omitempty"`
	Path                 string `json:"path,omitempty"`
	Commit               string `json:"commit,omitempty"`
	Detail               string `json:"detail,omitempty"`
}

// FeatureInputs is the shared projection used by all evidence consumers.
// The body is hashed rather than trusting its declared approval string.
func FeatureInputs(feature spec.Feature, tree spec.TaskTree) (ChainFingerprints, []LeafTask) {
	plan := PlanWitness{
		Fingerprint: spec.Fingerprint(feature.Tasks.Path, feature.Tasks.Body),
		Path:        feature.Tasks.Path, Source: "current-plan", Tasks: map[string]*spec.Task{},
	}
	leafs := make([]LeafTask, 0, len(tree.LeafTasks()))
	for _, task := range tree.LeafTasks() {
		plan.Tasks[task.ID] = task
		leafs = append(leafs, LeafTask{
			ID: task.ID, Completed: task.Completed, Fingerprint: spec.TaskDefinitionFingerprint(task), Definition: task,
		})
	}
	return ChainFingerprints{
		Requirements: feature.Requirements.ApprovedFingerprint,
		Design:       feature.Design.ApprovedFingerprint,
		Plans:        map[string]PlanWitness{plan.Fingerprint: plan},
	}, leafs
}

func assessBinding(record Record, current ChainFingerprints, leaf LeafTask) BindingAssessment {
	if record.TaskFingerprintScheme == spec.TaskFingerprintScheme {
		if record.TaskFingerprint == "" || leaf.Fingerprint == "" {
			return BindingAssessment{State: "unknown", Source: "current-scheme", Detail: "required recorded or current task fingerprint is absent"}
		}
		if record.TaskFingerprint != leaf.Fingerprint {
			return BindingAssessment{State: "changed", Source: "current-scheme", Detail: "recorded task contract differs from the current definition"}
		}
		if leaf.Definition != nil && !stepsConsistent(record, leaf.Definition.Proof) {
			return BindingAssessment{State: "contradictory", Source: "current-scheme", Detail: "recorded steps contradict the fingerprint-bound proof"}
		}
		return BindingAssessment{State: "current", Source: "current-scheme", EffectiveFingerprint: leaf.Fingerprint}
	}
	if record.TaskFingerprintScheme != "" {
		return BindingAssessment{State: "unknown", Detail: "unsupported task fingerprint scheme: " + record.TaskFingerprintScheme}
	}
	plan, found := current.Plans[record.TasksFingerprint]
	if !found || record.TasksFingerprint == "" {
		if leaf.Definition != nil && record.TaskFingerprint != "" && record.TaskFingerprint != spec.LegacyTaskDefinitionFingerprint(leaf.Definition) {
			return BindingAssessment{State: "changed", Source: "legacy-task", Detail: "even the legacy subset of the recorded task contract differs"}
		}
		detail := "fingerprint-bound historical plan is unavailable; argv alone cannot recover the complete proof"
		if reason := current.PlanGaps[record.TasksFingerprint]; reason != "" {
			detail = reason
		}
		return BindingAssessment{State: "unknown", PlanFingerprint: record.TasksFingerprint, Detail: detail}
	}
	binding := BindingAssessment{Source: plan.Source, PlanFingerprint: plan.Fingerprint, Path: plan.Path, Commit: plan.Commit}
	historical, found := plan.Tasks[leaf.ID]
	if !found || spec.LegacyTaskDefinitionFingerprint(historical) != record.TaskFingerprint || !stepsConsistent(record, historical.Proof) {
		binding.State, binding.Detail = "contradictory", "recorded task/steps contradict the fingerprint-bound historical plan"
		return binding
	}
	binding.EffectiveFingerprint = spec.TaskDefinitionFingerprint(historical)
	if binding.EffectiveFingerprint != leaf.Fingerprint {
		binding.State, binding.Detail = "changed", "recovered complete proof contract differs from the current definition"
	} else {
		binding.State = "reconstructed-equivalent"
	}
	return binding
}

func stepsConsistent(record Record, proof spec.VerificationSpec) bool {
	steps := proof.Steps
	if len(steps) == 0 && strings.TrimSpace(proof.LegacyCommand) != "" {
		// Frozen legacy execution tokenization; it adds no quoting/shell support.
		argv := strings.Fields(strings.Trim(strings.TrimSpace(proof.LegacyCommand), "`"))
		steps = []spec.VerificationStep{{Argv: argv}}
	}
	if len(record.Steps) > len(steps) || (record.Result == ResultPassed && len(record.Steps) != len(steps)) {
		return false
	}
	for i, actual := range record.Steps {
		exit := 0
		if steps[i].ExpectExit != nil {
			exit = *steps[i].ExpectExit
		}
		if !reflect.DeepEqual(actual.Command, steps[i].Argv) || actual.ExpectedExit != exit || (record.Result == ResultPassed && actual.ActualExit != exit) {
			return false
		}
	}
	return true
}
