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

	// Purity chain: one manifest before the loop (its digest doubles as the
	// selection identity) and one after each task's proof. Completion may
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

		// The record binds the tree the proof actually left behind; when the
		// proof was pure that is the same tree it started from.
		var sideEffectPaths []string
		recordIdentity := ""
		if manifestOK {
			if post, postOK := evidence.CaptureManifest(ctx, identityRunner, root); postOK {
				sideEffectPaths = evidence.DiffPaths(previous, post)
				previous = post
				recordIdentity = post.Digest()
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
			Steps:                   stepResults,
			Result:                  evidence.ResultPassed,
			VerifiedAt:              verifiedAt,
		}

		outcome := VerifyOutcome{TaskID: task.ID, State: evidence.StateVerified, Passed: true}
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
			result.Warnings = append(result.Warnings, fmt.Sprintf("proof side effects modified the repository: %s", formatChangedPaths(drift)))
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
	identity, identityOK := evidence.Identity(ctx, identityRunner, root)
	current := evidence.ChainFingerprints{
		Requirements: feature.Requirements.Fields["approved_fingerprint"],
		Design:       feature.Design.Fields["approved_fingerprint"],
	}
	return feature.Name, evidence.Derive(ledger, current, identity, identityOK, leafs), nil
}
