package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/andrearaponi/walden/internal/output"
	"github.com/andrearaponi/walden/internal/release"
)

func runRelease(args []string, stdout io.Writer, stderr io.Writer) int {
	if groupHelp("release", args, stdout) {
		return 0
	}
	if len(args) > 0 && args[0] == "check" {
		return runReleaseCheck(args[1:], stdout, stderr)
	}

	_, _ = fmt.Fprintf(stderr, "unknown command: release %s\n\n", strings.Join(args, " "))
	printUsage(stderr)
	return 1
}

func runReleaseCheck(args []string, stdout io.Writer, stderr io.Writer) int {
	parsed, handled, code := triageArgs("release check", "release-check", args, stdout, stderr)
	if handled {
		return code
	}
	jsonMode := parsed.Bool("--json")
	strict := parsed.Bool("--strict")
	positional := parsed.Positionals

	if len(positional) > 1 {
		return emitResult("release-check", errorResult(errors.New("release check takes at most one feature name")), jsonMode, stdout, stderr)
	}
	featureName := ""
	if len(positional) == 1 {
		featureName = positional[0]
	}

	root, err := os.Getwd()
	if err != nil {
		return emitResult("release-check", errorResult(fmt.Errorf("resolve working directory: %w", err)), jsonMode, stdout, stderr)
	}

	report, err := release.ReleaseCheck(context.Background(), root, featureName, strict)
	if err != nil {
		return emitResult("release-check", errorResult(err), jsonMode, stdout, stderr)
	}

	result := releaseCheckResult(report)
	if jsonMode {
		return emitResult("release-check", result, jsonMode, stdout, stderr)
	}
	if result.ExitCode != 0 {
		// A failed certification is a work list, not an invocation error:
		// render the full report, mirroring validate's convention.
		output.PrintText(stderr, result)
		return result.ExitCode
	}
	output.PrintText(stdout, result)
	return 0
}

func releaseCheckResult(report release.ReleaseReport) output.Result {
	status := &output.ReleaseStatus{
		Releasable: report.Releasable(),
		Strict:     report.Strict,
		Worktree: output.ReleaseWorktree{
			Blockers:    append([]string(nil), report.WorktreeBlockers...),
			WaldenDirty: append([]string(nil), report.WaldenDirty...),
			GitSkipped:  report.GitSkipped,
		},
	}

	result := output.Result{ExitCode: 0}
	pendingTotal := 0
	for _, feature := range report.Features {
		view := output.ReleaseFeature{
			Feature: feature.Feature,
			Pending: append([]string(nil), feature.Pending...),
		}
		pendingTotal += len(feature.Pending)
		for _, criterion := range feature.Criteria {
			view.Criteria = append(view.Criteria, output.ReleaseCriterion{
				Name:     criterion.Name,
				Passed:   criterion.Passed,
				Blockers: append([]string(nil), criterion.Blockers...),
			})
			for _, blocker := range criterion.Blockers {
				result.Blockers = append(result.Blockers, fmt.Sprintf("%s: %s", feature.Feature, blocker))
			}
		}
		status.Features = append(status.Features, view)
	}
	result.Blockers = append(result.Blockers, report.WorktreeBlockers...)
	result.Release = status
	// Facts, not verdicts: both hold whether or not the check is releasable.
	result.Completion = report.Completion()
	result.CertifiedCommit = report.CertifiedCommit

	if !report.Strict {
		// Under strict these paths are already blockers; the not-blocking
		// warning would contradict the verdict.
		for _, path := range report.WaldenDirty {
			result.Warnings = append(result.Warnings, fmt.Sprintf("uncommitted under .walden/ (not blocking): %s", path))
		}
	}

	if report.Releasable() {
		result.Summary = fmt.Sprintf("RELEASABLE — %d feature(s) certified", len(report.Features))
		if pendingTotal > 0 {
			result.Summary += fmt.Sprintf(", %d task(s) still planned", pendingTotal)
		}
		if report.CertifiedCommit != "" {
			short := report.CertifiedCommit
			if len(short) > 12 {
				short = short[:12]
			}
			result.Summary += ", commit " + short
		}
		return result
	}

	result.Summary = fmt.Sprintf("NOT RELEASABLE — %d blocker(s)", report.BlockerCount())
	result.NextAction = "Resolve the blockers above (each names its remedy) and rerun walden release check"
	result.ExitCode = 1
	return result
}
