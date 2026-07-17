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

// VerifyOutcome describes one task touched by a verify run.
type VerifyOutcome struct {
	TaskID  string
	State   string
	Passed  bool
	Failure string
	// RecordedIdentity is the run-start digest the task's record binds;
	// CurrentIdentity is the digest of the tree its proof left — equal when
	// the proof was pure, visibly divergent on a contaminated run.
	RecordedIdentity string
	CurrentIdentity  string
	Profile          evidence.Profile
}

// VerifyResult is the outcome of re-proving a feature's completed tasks.
type VerifyResult struct {
	Feature  string
	Outcomes []VerifyOutcome
	Failed   []string
	Checked  bool
	Skipped  []string
	Pruned   []string
	// Warnings carry run-level purity observations: the tree drifted across
	// the run, or side-effect detection had to be skipped.
	Warnings []string
}

// Verify re-executes the proofs of completed tasks against the current
// working tree: non-verified ones by default, every one with all. Fresh
// evidence replaces each task's entry unless check is set, which reports
// without persisting anything. A failing proof never aborts the run — every
// selected task is re-proven and failures are collected. Re-verification is
// pure by contract: a proof that modifies the working tree fails its task,
// with `.walden/` excluded as the ledger's own legitimate write path.
func Verify(ctx context.Context, root, featureName string, all, check bool, runner shell.Runner) (VerifyResult, error) {
	if runner == nil {
		return VerifyResult{}, fmt.Errorf("proof runner is required")
	}

	readiness, err := LoadExecutionReadiness(root, featureName)
	if err != nil {
		return VerifyResult{}, err
	}
	if readiness.GateBlocked && len(readiness.Blockers) > 0 {
		return VerifyResult{}, fmt.Errorf("%s", readiness.Blockers[0])
	}

	feature, err := spec.LoadFeature(root, featureName)
	if err != nil {
		return VerifyResult{}, err
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

	// One profile for the whole run, like the identity: a malformed probe
	// declaration fails the command before any proof executes.
	profile, err := runProfile(ctx, root)
	if err != nil {
		return VerifyResult{}, err
	}

	// Purity chain: one manifest before the loop (its digest is both the
	// selection identity and the identity every record of this run binds)
	// and one after each task's proof. Completion may
	// legitimately mutate the tree; re-verification claims the current tree
	// satisfies the proofs, so a proof that moves the manifest fails its
	// task instead of silently dirtying the repository being certified.
	previous, manifestOK := evidence.CaptureManifest(ctx, identityRunner, root)
	initial := previous
	detectionDegraded := !manifestOK
	identity := ""
	if manifestOK {
		identity = previous.Digest()
	}
	current := evidence.ChainFingerprints{
		Requirements: feature.Requirements.Fields["approved_fingerprint"],
		Design:       feature.Design.Fields["approved_fingerprint"],
	}

	result := VerifyResult{Feature: feature.Name, Checked: check}
	refreshed := map[string]evidence.Record{}
	verifiedAt := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	// Tasks proven after the first side effect ran on a tree the run did not
	// promise; the drift warning names them so the victim list is visible.
	poisoned := []string{}
	contaminated := false

	planIDs := map[string]bool{}
	for _, task := range tree.LeafTasks() {
		planIDs[task.ID] = true
	}
	for taskID := range ledger.Tasks {
		if !planIDs[taskID] {
			result.Pruned = append(result.Pruned, taskID)
		}
	}
	sort.Strings(result.Pruned)

	for _, task := range tree.LeafTasks() {
		if !task.Completed {
			continue
		}

		executable := toExecutableTask(task)
		fingerprint := executableTaskFingerprint(executable)

		if !all {
			derived := evidence.Derive(ledger, current, identity, manifestOK, []evidence.LeafTask{
				{ID: task.ID, Completed: true, Fingerprint: fingerprint},
			})
			if derived[0].State == evidence.StateVerified {
				result.Skipped = append(result.Skipped, task.ID)
				continue
			}
		}

		stepResults, _, proofErr := executeProof(ctx, runner, executable)

		// The record binds the tree the run started from: purity forbids a
		// proof from moving it, so one mutant cannot poison the identity of
		// the tasks proven after it — it fails alone on its own side effects.
		var sideEffectPaths []string
		recordIdentity := ""
		currentIdentity := ""
		if manifestOK {
			if post, postOK := evidence.CaptureManifest(ctx, identityRunner, root); postOK {
				sideEffectPaths = evidence.DiffPaths(previous, post)
				previous = post
				recordIdentity = identity
				currentIdentity = post.Digest()
				if contaminated {
					poisoned = append(poisoned, task.ID)
				}
				if len(sideEffectPaths) > 0 {
					contaminated = true
				}
			} else {
				detectionDegraded = true
			}
		}

		record := evidence.Record{
			TaskFingerprint:         fingerprint,
			RequirementsFingerprint: current.Requirements,
			DesignFingerprint:       current.Design,
			TasksFingerprint:        feature.Tasks.Fields["approved_fingerprint"],
			CodeIdentity:            recordIdentity,
			Profile:                 profile,
			Steps:                   stepResults,
			Result:                  evidence.ResultPassed,
			VerifiedAt:              verifiedAt,
		}

		outcome := VerifyOutcome{
			TaskID:           task.ID,
			State:            evidence.StateVerified,
			Passed:           true,
			RecordedIdentity: recordIdentity,
			CurrentIdentity:  currentIdentity,
			Profile:          profile,
		}
		switch {
		case proofErr != nil:
			record.Result = evidence.ResultFailed
			outcome.State = evidence.StateFailed
			outcome.Passed = false
			outcome.Failure = proofErr.Error()
			result.Failed = append(result.Failed, task.ID)
		case len(sideEffectPaths) > 0:
			record.Result = evidence.ResultFailed
			outcome.State = evidence.StateFailed
			outcome.Passed = false
			outcome.Failure = fmt.Sprintf("working tree changed while task %s proof ran: modified paths: %s", task.ID, formatChangedPaths(sideEffectPaths))
			result.Failed = append(result.Failed, task.ID)
		}

		// A failure on a drifted environment gets the diagnosis appended:
		// the difference between "the code broke" and "the machine changed".
		if outcome.Failure != "" {
			if replaced, exists := ledger.Tasks[task.ID]; exists {
				if drift := evidence.DiffProfile(replaced.Profile, profile); len(drift) > 0 {
					outcome.Failure += "; environment drift: " + formatProfileDrift(drift)
				}
			}
		}

		if !check {
			refreshed[task.ID] = record
		}
		result.Outcomes = append(result.Outcomes, outcome)
	}

	if detectionDegraded {
		result.Warnings = append(result.Warnings, "side-effect detection skipped: code identity unavailable (git not usable here)")
	}
	if manifestOK {
		if drift := evidence.DiffPaths(initial, previous); len(drift) > 0 {
			warning := fmt.Sprintf("proof side effects modified the repository: %s", formatChangedPaths(drift))
			if len(poisoned) > 0 {
				warning += fmt.Sprintf("; tasks re-proven on the modified tree: %s", strings.Join(poisoned, ", "))
			}
			result.Warnings = append(result.Warnings, warning)
		}
	}

	if !check && (len(refreshed) > 0 || len(result.Pruned) > 0) {
		for taskID, record := range refreshed {
			ledger.Tasks[taskID] = record
		}
		// The state map is a claim about the current plan: entries for task
		// ids that left the plan are history, and history lives in git.
		for _, taskID := range result.Pruned {
			delete(ledger.Tasks, taskID)
		}
		if err := evidence.Save(root, ledger); err != nil {
			return VerifyResult{}, fmt.Errorf("persist refreshed evidence: %w", err)
		}
	}
	if check {
		// Nothing is persisted in check mode; report what a real run would prune.
		_ = check
	}
	return result, nil
}

// formatProfileDrift renders recorded-versus-current pairs — the diagnosis
// the profile exists to print.
func formatProfileDrift(drifts []evidence.ProfileDrift) string {
	parts := make([]string, 0, len(drifts))
	for _, drift := range drifts {
		parts = append(parts, fmt.Sprintf("%s: recorded %q → current %q", drift.Key, drift.Recorded, drift.Current))
	}
	return strings.Join(parts, ", ")
}

// formatChangedPaths keeps side-effect failures readable on wide mutations:
// the first ten paths verbatim, the rest as a count.
func formatChangedPaths(paths []string) string {
	const maxListed = 10
	if len(paths) <= maxListed {
		return strings.Join(paths, ", ")
	}
	return fmt.Sprintf("%s, +%d more", strings.Join(paths[:maxListed], ", "), len(paths)-maxListed)
}

// toExecutableTask bridges a parsed task into the execution view.
func toExecutableTask(task *spec.Task) ExecutableTask {
	return ExecutableTask{
		ID:           task.ID,
		Title:        task.Title,
		ParentID:     task.ParentID,
		Completed:    task.Completed,
		Level:        task.Level,
		Requirements: append([]string(nil), task.Requirements...),
		DesignRefs:   append([]string(nil), task.DesignRefs...),
		Verification: task.Verification,
		Proof:        task.Proof,
	}
}

// EvidenceReport derives every leaf task's execution-evidence state for a
// feature. It is a pure read: no gate, no writes, usable on any chain state.
func EvidenceReport(ctx context.Context, root, featureName string) (string, []evidence.TaskEvidence, error) {
	feature, err := spec.LoadFeature(root, featureName)
	if err != nil {
		return "", nil, err
	}
	tree, err := spec.ParseTaskTree(feature.Tasks)
	if err != nil {
		return "", nil, err
	}

	leafs := []evidence.LeafTask{}
	for _, task := range tree.LeafTasks() {
		leafs = append(leafs, evidence.LeafTask{
			ID:          task.ID,
			Completed:   task.Completed,
			Fingerprint: executableTaskFingerprint(toExecutableTask(task)),
		})
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
	current := evidence.ChainFingerprints{
		Requirements: feature.Requirements.Fields["approved_fingerprint"],
		Design:       feature.Design.Fields["approved_fingerprint"],
	}

	entries := evidence.Derive(ledger, current, identity, identityOK, leafs)
	// Profile context rides beside the derived states: recorded values,
	// drift against this machine, and the legacy marker for records written
	// before profiles existed.
	for index := range entries {
		record, exists := ledger.Tasks[entries[index].TaskID]
		if !exists {
			continue
		}
		entries[index].RecordedResult = record.Result
		entries[index].RecordedProfile = record.Profile
		if record.Profile == nil {
			entries[index].ProfileLegacy = true
			continue
		}
		entries[index].ProfileDrift = evidence.DiffProfile(record.Profile, currentProfile)
	}
	return feature.Name, entries, nil
}
