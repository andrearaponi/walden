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
	Elapsed          *time.Duration
}

type VerifyResult struct {
	Feature  string
	Task     string   // the selected id; empty without a selection
	Open     []string // completed tasks outside the selection not verified, in plan order
	Outcomes []VerifyOutcome
	Failed   []string
	Checked  bool
	Skipped  []string
	// Kept holds the delivered tasks the run did not prove again, with the
	// commit each was attested on.
	Kept []KeptTask
	// Left holds the tasks whose recorded pass awaits adoption: the run left
	// their proofs alone.
	Left              []string
	Pruned            []string
	Warnings          []string
	EvidencePersisted *bool
}

// KeptTask is a delivered task a run kept as it was.
type KeptTask struct {
	TaskID string
	Commit string
	Sealed bool
}

// VerifyTaskStart identifies an actual selected proof attempt.
type VerifyTaskStart struct {
	TaskID string
	Index  int
	Total  int
}

// VerifyTaskFinish is an execution observation, not final freshness or a
// persistence claim. Its collections never alias the verifier's records.
type VerifyTaskFinish struct {
	TaskID          string
	Index           int
	Total           int
	Passed          bool
	Failure         string
	AssertionResult string
	Integrity       string
	CauseTask       string
	ChangedPaths    []string
	Elapsed         time.Duration
}

// verifySelection decides which completed tasks one invocation re-proves:
// those not verified by default, every one with all, exactly one with task.
// attest is set by walden verify and not by the verify run of adoption: on a
// committed tree it also proves the delivery tasks that hold no attestation.
type verifySelection struct {
	all    bool
	task   string
	attest bool
}

// VerifyOptions configures observation, never proof selection or policy.
// Now is invocation-local; production time.Time values retain monotonic time.
type VerifyOptions struct {
	TaskStarted  func(VerifyTaskStart)
	TaskFinished func(VerifyTaskFinish)
	Now          func() time.Time
}

// Verify re-proves one selected feature. Selection is fixed at the initial
// assessment; contamination is sticky and cannot be repaired by restoring
// source bytes within the invocation. Check mode has identical judgments but
// no Walden writes. Commands themselves are not sandboxed or rolled back.
func Verify(ctx context.Context, root, featureName string, all, check bool, runner shell.Runner) (VerifyResult, error) {
	result, err := verify(ctx, root, featureName, verifySelection{all: all, attest: true}, check, runner, nil)
	if err != nil {
		return VerifyResult{}, err
	}
	return result, nil
}

// VerifyTask re-proves one completed leaf task. Records outside the selection,
// orphans included, stay as they are, and the result claims nothing about the
// rest of the feature.
func VerifyTask(ctx context.Context, root, featureName, taskID string, check bool, runner shell.Runner) (VerifyResult, error) {
	result, err := verify(ctx, root, featureName, verifySelection{task: taskID}, check, runner, nil)
	if err != nil {
		return VerifyResult{}, err
	}
	return result, nil
}

// VerifyWithOptions preserves observed results even if their final save fails.
// Verify keeps its compatibility error-return convention; both use this loop.
func VerifyWithOptions(ctx context.Context, root, featureName string, all, check bool, runner shell.Runner, options VerifyOptions) (VerifyResult, error) {
	if options.Now == nil {
		options.Now = time.Now
	}
	return verify(ctx, root, featureName, verifySelection{all: all}, check, runner, &options)
}

func verify(ctx context.Context, root, featureName string, selection verifySelection, check bool, runner shell.Runner, options *VerifyOptions) (VerifyResult, error) {
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
	if selection.task != "" {
		if err := checkTaskSelection(tree, feature.Name, selection.task); err != nil {
			return VerifyResult{}, err
		}
	}
	ledger, err := evidence.Load(root, feature.Name)
	if err != nil {
		return VerifyResult{}, err
	}
	ledger.Feature = feature.Name
	current, leafs := evidence.FeatureInputs(feature, tree)
	if selection.task != "" {
		// A delivered task is not proven again. The refusal comes before the
		// first effect of the run: no probe, no capture, no proof, no write.
		if err := refuseDelivered(ledger, current, leafs, selection.task); err != nil {
			return VerifyResult{}, err
		}
	}
	profile, err := runProfile(ctx, root)
	if err != nil {
		return VerifyResult{}, err
	}
	evidence.ResolvePlans(ctx, identityRunner, root, feature, ledger, &current, "")
	// The pre-proof capture of each task is the one a record binds, so it is
	// the one that also keeps the commit and the differing paths. The
	// post-proof captures only judge purity and stay identity-only.
	initialCapture, initialOK := evidence.CaptureTree(ctx, identityRunner, root)
	initial := initialCapture.Manifest
	identity := ""
	if initialOK {
		identity = initial.Digest()
	}
	selected := map[string]bool{}
	result := VerifyResult{Feature: feature.Name, Task: selection.task, Checked: check}
	entries := evidence.Derive(ledger, current, identity, initialOK, leafs)
	choice := SelectProofs(entries, selection.all, selection.task, selection.attest && initialOK && initialCapture.Committed())
	for _, id := range choice.Run {
		selected[id] = true
	}
	result.Skipped, result.Left = choice.Skipped, choice.Left
	kept := map[string]bool{}
	for _, id := range choice.Kept {
		kept[id] = true
	}
	for _, entry := range entries {
		if kept[entry.TaskID] && entry.Attestation != nil {
			result.Kept = append(result.Kept, KeptTask{TaskID: entry.TaskID, Commit: entry.Attestation.Commit, Sealed: entry.Attestation.Sealed})
		}
	}
	if len(selected) > 0 && !initialOK {
		return VerifyResult{}, fmt.Errorf("verification requires a usable initial code identity; no proofs executed — restore usable Git and retry walden verify %s", feature.Name)
	}
	if selection.task == "" {
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
	}

	previous, previousOK := initial, initialOK
	causeTask := ""
	var causePaths, poisoned []string
	verifiedAt := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	position := 0
	for _, task := range tree.LeafTasks() {
		if !selected[task.ID] {
			continue
		}
		preCapture, preOK := initialCapture, initialOK
		if len(result.Outcomes) > 0 {
			preCapture, preOK = evidence.CaptureTree(ctx, identityRunner, root)
		}
		pre := preCapture.Manifest
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
			policyProblems = append(policyProblems, "code changed before this proof: "+formatBounded(boundaryPaths))
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
		position++
		var started time.Time
		var elapsed *time.Duration
		if options != nil {
			if options.TaskStarted != nil {
				options.TaskStarted(VerifyTaskStart{TaskID: task.ID, Index: position, Total: len(selected)})
			}
			started = options.Now()
		}
		run, proofErr := executeProof(ctx, runner, toExecutableTask(task))
		result.Warnings = append(result.Warnings, run.warnings...)
		if options != nil {
			duration := options.Now().Sub(started)
			elapsed = &duration
		}
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
			policyProblems = append(policyProblems, fmt.Sprintf("working tree changed while task %s proof ran: modified paths: %s", task.ID, formatBounded(changed)))
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
			Profile: profile, Steps: run.steps, Result: evidence.ResultPassed, VerifiedAt: verifiedAt, Execution: facts,
		}
		if preOK {
			record.Tree = preCapture.Facts()
		}
		record.Statements = statementFacts(feature.Requirements.Body, task.Requirements, task.Proof)
		outcome := VerifyOutcome{TaskID: task.ID, Passed: true, Profile: profile, Elapsed: elapsed}
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
		// A delivery proof that passed is attested when the tree it ran on
		// was the tree of its commit. The pre-proof capture is the one the
		// record binds, and a rejected run holds no pass to attest.
		if task.Proof.Delivery && preOK {
			record.Attestation = evidence.Attest(record, preCapture)
		}
		// Check mode uses the same in-memory ledger; only Save is suppressed.
		ledger.Tasks[task.ID] = record
		result.Outcomes = append(result.Outcomes, outcome)
		previous, previousOK = post, postOK
		if options != nil && options.TaskFinished != nil {
			options.TaskFinished(VerifyTaskFinish{
				TaskID: task.ID, Index: position, Total: len(selected),
				Passed: outcome.Passed, Failure: outcome.Failure,
				AssertionResult: facts.AssertionResult, Integrity: facts.Integrity,
				CauseTask: facts.CauseTask, ChangedPaths: append([]string(nil), facts.ChangedPaths...),
				Elapsed: *elapsed,
			})
		}
	}
	finalIdentity := ""
	if previousOK {
		finalIdentity = previous.Digest()
	}
	assessments := map[string]evidence.TaskEvidence{}
	for _, entry := range evidence.Derive(ledger, current, finalIdentity, previousOK, leafs) {
		assessments[entry.TaskID] = entry
		// A delivered task is not among the completed tasks that are not
		// verified: nothing is left to prove for it.
		if selection.task != "" && entry.TaskID != selection.task && entry.State != evidence.StatePending && entry.State != evidence.StateVerified && entry.State != evidence.StateDelivered {
			result.Open = append(result.Open, entry.TaskID)
		}
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
			warning = "proof side effects modified the repository: " + formatBounded(causePaths)
		}
		warning += "; run contamination remains until a new valid invocation"
		if len(poisoned) > 0 {
			warning += "; tasks re-proven on the modified tree: " + strings.Join(poisoned, ", ")
		}
		result.Warnings = append(result.Warnings, warning)
	}
	if len(result.Open) > 0 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("%d other completed task(s) not verified: %s", len(result.Open), formatBounded(result.Open)))
	}
	if !check && (len(result.Outcomes) > 0 || len(result.Pruned) > 0) {
		for _, id := range result.Pruned {
			delete(ledger.Tasks, id)
		}
		persisted := false
		result.EvidencePersisted = &persisted
		if err := evidence.Save(root, ledger); err != nil {
			return result, fmt.Errorf("persist refreshed evidence: %w", err)
		}
		persisted = true
	}
	return result, nil
}

// checkTaskSelection accepts only a completed leaf of the plan. It runs ahead
// of the first effect of the path, so a wrong id executes nothing.
func checkTaskSelection(tree spec.TaskTree, feature, id string) error {
	for _, task := range tree.LeafTasks() {
		if task.ID != id {
			continue
		}
		if !task.Completed {
			return fmt.Errorf("task %q is not completed: run walden task complete %s %s", id, feature, id)
		}
		return nil
	}
	return fmt.Errorf("task %q is not a leaf task of %s", id, feature)
}

// refuseDelivered rejects a run that names a delivered task. The state is
// derived without a code identity, which the reading of an attestation does
// not need, so nothing is asked of git either.
func refuseDelivered(ledger evidence.Document, current evidence.ChainFingerprints, leafs []evidence.LeafTask, id string) error {
	for _, entry := range evidence.Derive(ledger, current, "", false, leafs) {
		if entry.TaskID == id && entry.State == evidence.StateDelivered && entry.Attestation != nil {
			return fmt.Errorf(
				"task %q is delivered: its proof was attested at commit %s and is not run again — a delivered task is proven again only after its declaration or its proof changes in the approved plan, or after a statement it names changes",
				id, entry.Attestation.Commit,
			)
		}
	}
	return nil
}

func formatProfileDrift(drifts []evidence.ProfileDrift) string {
	parts := make([]string, 0, len(drifts))
	for _, drift := range drifts {
		parts = append(parts, fmt.Sprintf("%s: recorded %q → current %q", drift.Key, drift.Recorded, drift.Current))
	}
	return strings.Join(parts, ", ")
}

func formatBounded(items []string) string {
	const maxListed = 10
	if len(items) <= maxListed {
		return strings.Join(items, ", ")
	}
	return fmt.Sprintf("%s, +%d more", strings.Join(items[:maxListed], ", "), len(items)-maxListed)
}

func toExecutableTask(task *spec.Task) ExecutableTask {
	return ExecutableTask{ID: task.ID, Title: task.Title, ParentID: task.ParentID,
		Completed: task.Completed, Level: task.Level, Requirements: append([]string(nil), task.Requirements...),
		DesignRefs: append([]string(nil), task.DesignRefs...), Verification: task.Verification, Proof: task.Proof}
}

// EvidenceReading is what `evidence status` shows: each task's assessment
// with the explanation of what changed since its record was written.
type EvidenceReading struct {
	Feature string
	Entries []evidence.TaskEvidence
	// Current is what the current identity is made of, nil when no
	// identity could be captured.
	Current *evidence.CurrentTree
}

// EvidenceReport is a diagnostic view, not the probe-free migration surface:
// it retains profile probes. Adoption planning and release do not call it.
func EvidenceReport(ctx context.Context, root, featureName string) (string, []evidence.TaskEvidence, error) {
	reading, err := ReadEvidence(ctx, root, featureName)
	if err != nil {
		return "", nil, err
	}
	return reading.Feature, reading.Entries, nil
}

// ReadEvidence derives the states as every evidence consumer does and then
// explains them. The explanations are a second reading of the same records:
// they are computed after Derive and never passed back to it. Git is asked
// once for the current tree and once for each distinct recorded commit that
// a stale record names, however many tasks were recorded at it.
func ReadEvidence(ctx context.Context, root, featureName string) (EvidenceReading, error) {
	feature, err := spec.LoadFeature(root, featureName)
	if err != nil {
		return EvidenceReading{}, err
	}
	tree, err := spec.ParseTaskTree(feature.Tasks)
	if err != nil {
		return EvidenceReading{}, err
	}
	ledger, err := evidence.Load(root, feature.Name)
	if err != nil {
		return EvidenceReading{}, err
	}
	currentProfile, err := runProfile(ctx, root)
	if err != nil {
		return EvidenceReading{}, err
	}
	capture, identityOK := evidence.CaptureTree(ctx, identityRunner, root)
	identity := ""
	if identityOK {
		identity = capture.Manifest.Digest()
	}
	current, leafs := evidence.FeatureInputs(feature, tree)
	evidence.ResolvePlans(ctx, identityRunner, root, feature, ledger, &current, "")
	entries := evidence.Derive(ledger, current, identity, identityOK, leafs)

	listings := map[string]evidence.CommitListing{}
	statements := spec.StatementFingerprints(feature.Requirements.Body)
	for i := range entries {
		entries[i].ProfileDrift = evidence.DiffProfile(entries[i].RecordedProfile, currentProfile)

		record, recorded := ledger.Tasks[entries[i].TaskID]
		if !recorded {
			continue
		}
		if commit, needed := commitToList(record, entries[i]); needed {
			if _, listed := listings[commit]; !listed {
				listings[commit] = evidence.ListCommit(ctx, identityRunner, root, commit)
			}
		}
		entries[i].RecordedDurationMS = evidence.RecordedDuration(record)
		entries[i].Explanation = evidence.Explain(record, entries[i], evidence.ExplainInput{
			Feature:    feature.Name,
			Current:    capture.Manifest,
			Listings:   listings,
			Chain:      current,
			Statements: statements,
		})
	}
	reading := EvidenceReading{Feature: feature.Name, Entries: entries}
	if identityOK {
		reading.Current = capture.Current()
	}
	return reading, nil
}

// commitToList reports the recorded commit whose tree an explanation needs:
// only a stale record that kept its commit and its paths has one.
func commitToList(record evidence.Record, entry evidence.TaskEvidence) (string, bool) {
	// A delivered task is not explained, so git is asked nothing for it.
	if entry.State == evidence.StateDelivered {
		return "", false
	}
	if entry.CodeFreshness != "stale" || record.Tree == nil || record.Tree.Commit == "" || record.Tree.PathsOmitted > 0 {
		return "", false
	}
	return record.Tree.Commit, true
}
