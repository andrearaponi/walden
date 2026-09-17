package workflow

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/spec"
)

// VerifyOutcome separates an accepted execution from final-tree freshness.
// A pure prefix can pass while becoming stale-code after a later mutation.
type VerifyOutcome struct {
	TaskID           string
	State            string
	Passed           bool
	Failure          string
	RecordedIdentity string
	CurrentIdentity  string
	Profile          evidence.Profile
	Assessment       evidence.TaskEvidence
}

type VerifyResult struct {
	Feature  string
	Outcomes []VerifyOutcome
	Failed   []string
	Checked  bool
	Skipped  []string
	Pruned   []string
	Warnings []string
}

// Verify re-proves one selected feature. Selection is fixed at the initial
// assessment; contamination is sticky and cannot be repaired by restoring
// source bytes within the invocation. Check mode has identical judgments but
// no Walden writes. Commands themselves are not sandboxed or rolled back.
func Verify(ctx context.Context, root, featureName string, all, check bool, runner shell.Runner) (VerifyResult, error) {
	if runner == nil {
		return VerifyResult{}, fmt.Errorf("proof runner is required")
	}
	feature, err := spec.LoadFeature(root, featureName)
	if err != nil {
		return VerifyResult{}, err
	}
	readiness, err := ResolveExecutionReadiness(feature)
	if err != nil {
		return VerifyResult{}, err
	}
	if readiness.GateBlocked && len(readiness.Blockers) > 0 {
		return VerifyResult{}, fmt.Errorf("%s", readiness.Blockers[0])
	}
	tree, err := spec.ParseTaskTree(feature.Tasks)
	if err != nil {
		return VerifyResult{}, err
	}
	ledger, err := evidence.Load(root, feature.Name)
	if err != nil {
		return VerifyResult{}, err
	}
	ledger.Feature = feature.Name
	profile, err := runProfile(ctx, root)
	if err != nil {
		return VerifyResult{}, err
	}
	current, leafs := evidence.FeatureInputs(feature, tree)
	evidence.ResolvePlans(ctx, identityRunner, root, feature, ledger, &current, "")
	initial, initialOK := evidence.CaptureManifest(ctx, identityRunner, root)
	identity := ""
	if initialOK {
		identity = initial.Digest()
	}
	selected := map[string]bool{}
	result := VerifyResult{Feature: feature.Name, Checked: check}
	for _, entry := range evidence.Derive(ledger, current, identity, initialOK, leafs) {
		if entry.State == evidence.StatePending {
			continue
		}
		if !all && entry.State == evidence.StateVerified {
			result.Skipped = append(result.Skipped, entry.TaskID)
		} else {
			selected[entry.TaskID] = true
		}
	}
	if len(selected) > 0 && !initialOK {
		return VerifyResult{}, fmt.Errorf("verification requires a usable initial code identity; no proofs executed — restore usable Git and retry walden verify %s", feature.Name)
	}
	planIDs := map[string]bool{}
	for _, leaf := range leafs {
		planIDs[leaf.ID] = true
	}
	for id := range ledger.Tasks {
		if !planIDs[id] {
			result.Pruned = append(result.Pruned, id)
		}
	}
	sort.Strings(result.Pruned)

	previous, previousOK := initial, initialOK
	causeTask := ""
	var causePaths, poisoned []string
	verifiedAt := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	for _, task := range tree.LeafTasks() {
		if !selected[task.ID] {
			continue
		}
		pre, preOK := initial, initialOK
		if len(result.Outcomes) > 0 {
			pre, preOK = evidence.CaptureManifest(ctx, identityRunner, root)
		}
		priorContamination := causeTask != ""
		var policyProblems []string
		if priorContamination {
			poisoned = append(poisoned, task.ID)
			policyProblems = append(policyProblems, fmt.Sprintf("verification contaminated by task %s; a new clean invocation is required", causeTask))
		}
		var boundaryPaths []string
		if preOK && previousOK {
			boundaryPaths = evidence.DiffPaths(previous, pre)
		}
		if len(boundaryPaths) > 0 {
			policyProblems = append(policyProblems, "code changed before this proof: "+formatChangedPaths(boundaryPaths))
			if causeTask == "" {
				causeTask, causePaths = task.ID, boundaryPaths
			}
		}
		if !preOK {
			policyProblems = append(policyProblems, "required pre-proof code identity unavailable")
			if causeTask == "" {
				causeTask = task.ID
			}
		}
		stepResults, _, proofErr := executeProof(ctx, runner, toExecutableTask(task))
		post, postOK := evidence.CaptureManifest(ctx, identityRunner, root)
		var changed []string
		if preOK && postOK {
			changed = evidence.DiffPaths(pre, post)
		}
		if !postOK {
			policyProblems = append(policyProblems, "required post-proof code identity unavailable")
			if causeTask == "" {
				causeTask = task.ID
			}
		}
		if len(changed) > 0 {
			policyProblems = append(policyProblems, fmt.Sprintf("working tree changed while task %s proof ran: modified paths: %s", task.ID, formatChangedPaths(changed)))
			if causeTask == "" {
				causeTask, causePaths = task.ID, changed
			}
		}
		facts := &evidence.ExecutionFacts{Origin: "verify", Policy: evidence.VerifyPolicy, Integrity: "pure", AssertionResult: evidence.ResultPassed}
		if preOK {
			facts.BeforeCodeIdentity = pre.Digest()
		}
		if postOK {
			facts.AfterCodeIdentity = post.Digest()
		}
		if proofErr != nil {
			facts.AssertionResult = evidence.ResultFailed
		}
		switch {
		case !preOK || !postOK:
			facts.Integrity = "identity-unavailable"
		case priorContamination || len(boundaryPaths) > 0:
			facts.Integrity = "contaminated"
		case len(changed) > 0:
			facts.Integrity = "mutated"
		}
		if len(policyProblems) > 0 {
			facts.CauseTask = causeTask
		}
		facts.ChangedPaths = changed
		if len(boundaryPaths) > 0 {
			facts.ChangedPaths = append(append([]string(nil), boundaryPaths...), changed...)
		}
		record := evidence.Record{
			TaskFingerprint: spec.TaskDefinitionFingerprint(task), TaskFingerprintScheme: spec.TaskFingerprintScheme,
			RequirementsFingerprint: current.Requirements, DesignFingerprint: current.Design,
			TasksFingerprint: feature.Tasks.ApprovedFingerprint, CodeIdentity: facts.BeforeCodeIdentity,
			Profile: profile, Steps: stepResults, Result: evidence.ResultPassed, VerifiedAt: verifiedAt, Execution: facts,
		}
		outcome := VerifyOutcome{TaskID: task.ID, Passed: true, Profile: profile}
		if proofErr != nil {
			outcome.Failure = proofErr.Error()
		}
		if len(policyProblems) > 0 {
			if outcome.Failure != "" {
				outcome.Failure += "; "
			}
			outcome.Failure += "verification policy: " + strings.Join(policyProblems, "; ")
		}
		if outcome.Failure != "" {
			record.Result, outcome.Passed = evidence.ResultFailed, false
			result.Failed = append(result.Failed, task.ID)
			if old, exists := ledger.Tasks[task.ID]; exists {
				if drift := evidence.DiffProfile(old.Profile, profile); len(drift) > 0 {
					outcome.Failure += "; environment drift: " + formatProfileDrift(drift)
				}
			}
		}
		// Check mode uses the same in-memory ledger; only Save is suppressed.
		ledger.Tasks[task.ID] = record
		result.Outcomes = append(result.Outcomes, outcome)
		previous, previousOK = post, postOK
	}
	finalIdentity := ""
	if previousOK {
		finalIdentity = previous.Digest()
	}
	assessments := map[string]evidence.TaskEvidence{}
	for _, entry := range evidence.Derive(ledger, current, finalIdentity, previousOK, leafs) {
		assessments[entry.TaskID] = entry
	}
	for i := range result.Outcomes {
		entry := assessments[result.Outcomes[i].TaskID]
		result.Outcomes[i].State = entry.State
		result.Outcomes[i].RecordedIdentity = entry.RecordedIdentity
		result.Outcomes[i].CurrentIdentity = entry.CurrentIdentity
		result.Outcomes[i].Assessment = entry
	}
	if causeTask != "" {
		warning := "verification integrity unavailable after task " + causeTask
		if len(causePaths) > 0 {
			warning = "proof side effects modified the repository: " + formatChangedPaths(causePaths)
		}
		warning += "; run contamination remains until a new valid invocation"
		if len(poisoned) > 0 {
			warning += "; tasks re-proven on the modified tree: " + strings.Join(poisoned, ", ")
		}
		result.Warnings = append(result.Warnings, warning)
	}
	if !check && (len(result.Outcomes) > 0 || len(result.Pruned) > 0) {
		for _, id := range result.Pruned {
			delete(ledger.Tasks, id)
		}
		if err := evidence.Save(root, ledger); err != nil {
			return VerifyResult{}, fmt.Errorf("persist refreshed evidence: %w", err)
		}
	}
	return result, nil
}

func formatProfileDrift(drifts []evidence.ProfileDrift) string {
	parts := make([]string, 0, len(drifts))
	for _, drift := range drifts {
		parts = append(parts, fmt.Sprintf("%s: recorded %q → current %q", drift.Key, drift.Recorded, drift.Current))
	}
	return strings.Join(parts, ", ")
}

func formatChangedPaths(paths []string) string {
	const maxListed = 10
	if len(paths) <= maxListed {
		return strings.Join(paths, ", ")
	}
	return fmt.Sprintf("%s, +%d more", strings.Join(paths[:maxListed], ", "), len(paths)-maxListed)
}

func toExecutableTask(task *spec.Task) ExecutableTask {
	return ExecutableTask{ID: task.ID, Title: task.Title, ParentID: task.ParentID,
		Completed: task.Completed, Level: task.Level, Requirements: append([]string(nil), task.Requirements...),
		DesignRefs: append([]string(nil), task.DesignRefs...), Verification: task.Verification, Proof: task.Proof}
}

// EvidenceReport is a diagnostic view, not the probe-free migration surface:
// it retains profile probes. Adoption planning and release do not call it.
func EvidenceReport(ctx context.Context, root, featureName string) (string, []evidence.TaskEvidence, error) {
	feature, err := spec.LoadFeature(root, featureName)
	if err != nil {
		return "", nil, err
	}
	tree, err := spec.ParseTaskTree(feature.Tasks)
	if err != nil {
		return "", nil, err
	}
	ledger, err := evidence.Load(root, feature.Name)
	if err != nil {
		return "", nil, err
	}
	currentProfile, err := runProfile(ctx, root)
	if err != nil {
		return "", nil, err
	}
	identity, identityOK := evidence.Identity(ctx, identityRunner, root)
	current, leafs := evidence.FeatureInputs(feature, tree)
	evidence.ResolvePlans(ctx, identityRunner, root, feature, ledger, &current, "")
	entries := evidence.Derive(ledger, current, identity, identityOK, leafs)
	for i := range entries {
		entries[i].ProfileDrift = evidence.DiffProfile(entries[i].RecordedProfile, currentProfile)
	}
	return feature.Name, entries, nil
}
