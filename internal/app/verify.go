package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/output"
	"github.com/andrearaponi/walden/internal/workflow"
)

func runVerify(args []string, stdout io.Writer, stderr io.Writer) int {
	parsed, handled, code := triageArgs("verify", "verify", args, stdout, stderr)
	if handled {
		return code
	}
	jsonMode := parsed.Bool("--json")
	all := parsed.Bool("--all")
	check := parsed.Bool("--check")
	positional := parsed.Positionals

	if len(positional) != 1 {
		return emitResult("verify", errorResult(errors.New("verify requires exactly one feature name")), jsonMode, stdout, stderr)
	}

	taskID, err := selectedTask(parsed)
	if err != nil {
		return emitResult("verify", errorResult(err), jsonMode, stdout, stderr)
	}

	root, err := os.Getwd()
	if err != nil {
		return emitResult("verify", errorResult(fmt.Errorf("resolve working directory: %w", err)), jsonMode, stdout, stderr)
	}

	var verifyResult workflow.VerifyResult
	if taskID != "" {
		verifyResult, err = workflow.VerifyTask(context.Background(), root, positional[0], taskID, check, commandRunner)
	} else {
		verifyResult, err = workflow.Verify(context.Background(), root, positional[0], all, check, commandRunner)
	}
	if err != nil {
		return emitResult("verify", errorResult(err), jsonMode, stdout, stderr)
	}

	result := verifyOutputResult(verifyResult)
	return emitResult("verify", result, jsonMode, stdout, stderr)
}

// selectedTask returns the id passed with --task, or "" without the flag. It
// refuses what the arguments alone show to be wrong: the parser keeps the last
// of a repeated flag, and an empty value would widen into the whole feature.
func selectedTask(parsed commandArgs) (string, error) {
	taskID := parsed.Value("--task")
	switch count := parsed.Count("--task"); {
	case count > 1:
		return "", errors.New("--task accepts one task id: run 'walden verify --help'")
	case count == 1 && taskID == "":
		return "", errors.New("--task requires a task id: run 'walden verify --help'")
	case count == 1 && parsed.Bool("--all"):
		return "", errors.New("--task cannot be combined with --all: run 'walden verify --help'")
	}
	return taskID, nil
}

func elapsedMilliseconds(duration *time.Duration) *int64 {
	if duration == nil {
		return nil
	}
	milliseconds := duration.Milliseconds()
	return &milliseconds
}

func verificationOutcomeView(outcome workflow.VerifyOutcome) output.EvidenceStatus {
	passed := outcome.Passed
	view := output.EvidenceView(outcome.Assessment)
	view.TaskID, view.State = outcome.TaskID, outcome.State
	view.Passed, view.Failure, view.Profile = &passed, outcome.Failure, outcome.Profile
	view.RecordedIdentity, view.CurrentIdentity = outcome.RecordedIdentity, outcome.CurrentIdentity
	view.ElapsedMS = elapsedMilliseconds(outcome.Elapsed)
	return view
}

func verifyOutputResult(verifyResult workflow.VerifyResult) output.Result {
	entries := make([]output.EvidenceStatus, 0, len(verifyResult.Outcomes))
	for _, outcome := range verifyResult.Outcomes {
		entries = append(entries, verificationOutcomeView(outcome))
	}

	suffix := ""
	if verifyResult.Checked {
		suffix = " (check mode: nothing persisted)"
	}

	scope := evidence.NewScope(true, verifyResult.Feature)
	if verifyResult.Task != "" {
		scope = evidence.NewTaskScope(verifyResult.Feature, verifyResult.Task)
	}
	result := output.Result{Evidence: entries, Scope: &scope, ExitCode: 0}
	if len(verifyResult.Pruned) > 0 && !verifyResult.Checked {
		result.Warnings = append(result.Warnings, fmt.Sprintf("pruned %d orphaned evidence entr%s no longer in the plan: %s", len(verifyResult.Pruned), map[bool]string{true: "y", false: "ies"}[len(verifyResult.Pruned) == 1], strings.Join(verifyResult.Pruned, ", ")))
	}
	result.Warnings = append(result.Warnings, verifyResult.Warnings...)
	switch {
	case verifyResult.Task != "" && len(verifyResult.Failed) > 0:
		result.Summary = fmt.Sprintf("verification failed for task %s of %s%s", verifyResult.Task, verifyResult.Feature, suffix)
		result.NextAction = fmt.Sprintf("Fix the failing proof and rerun walden verify %s --task %s", verifyResult.Feature, verifyResult.Task)
		result.ExitCode = 1
	case verifyResult.Task != "":
		result.Summary = fmt.Sprintf("verification passed for task %s of %s%s", verifyResult.Task, verifyResult.Feature, suffix)
		if len(verifyResult.Open) > 0 {
			result.NextAction = fmt.Sprintf("Run walden verify %s to re-prove the other tasks", verifyResult.Feature)
		}
	case len(verifyResult.Failed) > 0:
		result.Summary = fmt.Sprintf("verification failed for %s: %s%s", verifyResult.Feature, strings.Join(verifyResult.Failed, ", "), suffix)
		result.NextAction = fmt.Sprintf("Fix the failing proofs and rerun walden verify %s", verifyResult.Feature)
		result.ExitCode = 1
	case len(verifyResult.Outcomes) == 0:
		result.Summary = fmt.Sprintf("nothing to verify for %s: %d completed task(s) already verified%s", verifyResult.Feature, len(verifyResult.Skipped), suffix)
	default:
		result.Summary = fmt.Sprintf("verification passed for %s: %d task(s) re-proven, %d skipped as verified%s", verifyResult.Feature, len(verifyResult.Outcomes), len(verifyResult.Skipped), suffix)
	}
	return result
}
