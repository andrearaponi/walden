package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/output"
	"github.com/andrearaponi/walden/internal/workflow"
)

func runEvidence(args []string, stdout io.Writer, stderr io.Writer) int {
	if groupHelp("evidence", args, stdout) {
		return 0
	}
	if len(args) > 0 && args[0] == "status" {
		return runEvidenceStatus(args[1:], stdout, stderr)
	}

	_, _ = fmt.Fprintf(stderr, "unknown command: evidence %s\n\n", strings.Join(args, " "))
	printUsage(stderr)
	return 1
}

func runEvidenceStatus(args []string, stdout io.Writer, stderr io.Writer) int {
	parsed, handled, code := triageArgs("evidence status", "evidence-status", args, stdout, stderr)
	if handled {
		return code
	}
	jsonMode := parsed.Bool("--json")
	positional := parsed.Positionals

	if len(positional) != 1 {
		return emitResult("evidence-status", errorResult(errors.New("evidence status requires exactly one feature name")), jsonMode, stdout, stderr)
	}

	root, err := os.Getwd()
	if err != nil {
		return emitResult("evidence-status", errorResult(fmt.Errorf("resolve working directory: %w", err)), jsonMode, stdout, stderr)
	}

	featureName, entries, err := workflow.EvidenceReport(context.Background(), root, positional[0])
	if err != nil {
		return emitResult("evidence-status", errorResult(err), jsonMode, stdout, stderr)
	}

	counts := map[string]int{}
	rendered := make([]output.EvidenceStatus, 0, len(entries))
	for _, entry := range entries {
		counts[entry.State]++
		view := output.EvidenceStatus{
			TaskID:           entry.TaskID,
			State:            entry.State,
			RecordedIdentity: entry.RecordedIdentity,
			CurrentIdentity:  entry.CurrentIdentity,
			Profile:          entry.RecordedProfile,
			ProfileLegacy:    entry.ProfileLegacy,
		}
		// The stored result rides beside the derived state under the same
		// field verify uses; unrecorded tasks have no result to report.
		if entry.RecordedResult != "" {
			passed := entry.RecordedResult == evidence.ResultPassed
			view.Passed = &passed
		}
		for _, drift := range entry.ProfileDrift {
			view.ProfileDrift = append(view.ProfileDrift, output.ProfileDriftEntry{
				Key:      drift.Key,
				Recorded: drift.Recorded,
				Current:  drift.Current,
			})
		}
		rendered = append(rendered, view)
	}

	summaryParts := []string{}
	for _, state := range []string{"verified", "failed", "stale-spec", "stale-code", "unrecorded", "pending"} {
		if counts[state] > 0 {
			summaryParts = append(summaryParts, fmt.Sprintf("%d %s", counts[state], state))
		}
	}
	summary := fmt.Sprintf("evidence status for %s", featureName)
	if len(summaryParts) > 0 {
		summary = fmt.Sprintf("evidence status for %s: %s", featureName, strings.Join(summaryParts, ", "))
	}

	// Evidence status is a report, not a gate: derived states never change
	// the exit code.
	result := output.Result{Summary: summary, Evidence: rendered, ExitCode: 0}
	if counts["stale-spec"]+counts["stale-code"]+counts["failed"]+counts["unrecorded"] > 0 {
		result.NextAction = fmt.Sprintf("Run walden verify %s to re-prove the current code", featureName)
	}
	return emitResult("evidence-status", result, jsonMode, stdout, stderr)
}
