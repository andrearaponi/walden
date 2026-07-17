package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

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

	root, err := os.Getwd()
	if err != nil {
		return emitResult("verify", errorResult(fmt.Errorf("resolve working directory: %w", err)), jsonMode, stdout, stderr)
	}

	verifyResult, err := workflow.Verify(context.Background(), root, positional[0], all, check, commandRunner)
	if err != nil {
		return emitResult("verify", errorResult(err), jsonMode, stdout, stderr)
	}

	result := verifyOutputResult(verifyResult)
	return emitResult("verify", result, jsonMode, stdout, stderr)
}

func verifyOutputResult(verifyResult workflow.VerifyResult) output.Result {
	entries := make([]output.EvidenceStatus, 0, len(verifyResult.Outcomes))
	for _, outcome := range verifyResult.Outcomes {
		passed := outcome.Passed
		entries = append(entries, output.EvidenceStatus{
			TaskID:           outcome.TaskID,
			State:            outcome.State,
			Passed:           &passed,
			Failure:          outcome.Failure,
			RecordedIdentity: outcome.RecordedIdentity,
			CurrentIdentity:  outcome.CurrentIdentity,
			Profile:          outcome.Profile,
		})
	}

	suffix := ""
	if verifyResult.Checked {
		suffix = " (check mode: nothing persisted)"
	}

	result := output.Result{Evidence: entries, ExitCode: 0}
	if len(verifyResult.Pruned) > 0 && !verifyResult.Checked {
		result.Warnings = append(result.Warnings, fmt.Sprintf("pruned %d orphaned evidence entr%s no longer in the plan: %s", len(verifyResult.Pruned), map[bool]string{true: "y", false: "ies"}[len(verifyResult.Pruned) == 1], strings.Join(verifyResult.Pruned, ", ")))
	}
	result.Warnings = append(result.Warnings, verifyResult.Warnings...)
	switch {
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
