package workflow

import (
	"context"
	"fmt"
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
}

// Verify re-executes the proofs of completed tasks against the current
// working tree: non-verified ones by default, every one with all. Fresh
// evidence replaces each task's entry unless check is set, which reports
// without persisting anything. A failing proof never aborts the run — every
// selected task is re-proven and failures are collected.
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

	identity, _ := evidence.Identity(ctx, identityRunner, root)
	current := evidence.ChainFingerprints{
		Requirements: feature.Requirements.Fields["approved_fingerprint"],
		Design:       feature.Design.Fields["approved_fingerprint"],
	}

	result := VerifyResult{Feature: feature.Name, Checked: check}
	verifiedAt := time.Now().UTC().Format("2006-01-02T15:04:05Z")

	for _, task := range tree.LeafTasks() {
		if !task.Completed {
			continue
		}

		executable := toExecutableTask(task)
		fingerprint := executableTaskFingerprint(executable)

		if !all {
			derived := evidence.Derive(ledger, current, identity, identity != "", []evidence.LeafTask{
				{ID: task.ID, Completed: true, Fingerprint: fingerprint},
			})
			if derived[0].State == evidence.StateVerified {
				result.Skipped = append(result.Skipped, task.ID)
				continue
			}
		}

		stepResults, _, proofErr := executeProof(ctx, runner, executable)
		record := evidence.Record{
			TaskFingerprint:         fingerprint,
			RequirementsFingerprint: current.Requirements,
			DesignFingerprint:       current.Design,
			TasksFingerprint:        feature.Tasks.Fields["approved_fingerprint"],
			CodeIdentity:            identity,
			Steps:                   stepResults,
			Result:                  evidence.ResultPassed,
			VerifiedAt:              verifiedAt,
		}

		outcome := VerifyOutcome{TaskID: task.ID, State: evidence.StateVerified, Passed: true}
		if proofErr != nil {
			record.Result = evidence.ResultFailed
			outcome.State = evidence.StateFailed
			outcome.Passed = false
			outcome.Failure = proofErr.Error()
			result.Failed = append(result.Failed, task.ID)
		}

		if !check {
			ledger.Tasks[task.ID] = record
		}
		result.Outcomes = append(result.Outcomes, outcome)
	}

	if !check && len(result.Outcomes) > 0 {
		if err := evidence.Save(root, ledger); err != nil {
			return VerifyResult{}, fmt.Errorf("persist refreshed evidence: %w", err)
		}
	}
	return result, nil
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
