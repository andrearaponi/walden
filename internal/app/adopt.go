package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/andrearaponi/walden/internal/adopt"
	"github.com/andrearaponi/walden/internal/output"
	"github.com/andrearaponi/walden/internal/workflow"
)

func runAdopt(args []string, stdout io.Writer, stderr io.Writer) int {
	parsed, handled, code := triageArgs("adopt", "adopt", args, stdout, stderr)
	if handled {
		return code
	}
	jsonMode := parsed.Bool("--json")
	apply := parsed.Bool("--apply")
	positional := parsed.Positionals

	if len(positional) > 1 {
		return emitResult("adopt", errorResult(errors.New("adopt takes at most one feature name")), jsonMode, stdout, stderr)
	}
	featureName := ""
	if len(positional) == 1 {
		featureName = positional[0]
	}

	root, err := os.Getwd()
	if err != nil {
		return emitResult("adopt", errorResult(fmt.Errorf("resolve working directory: %w", err)), jsonMode, stdout, stderr)
	}

	if !apply {
		report, err := adopt.Plan(context.Background(), root, featureName)
		if err != nil {
			return emitResult("adopt", errorResult(err), jsonMode, stdout, stderr)
		}
		return emitResult("adopt", adoptPlanResult(report), jsonMode, stdout, stderr)
	}

	// Progress streams in text mode only: JSON stays one envelope, positions
	// live in the per-feature array order.
	options := adopt.ApplyOptions{}
	if !jsonMode {
		options.FeatureStarted = func(name string, index, total int) {
			_, _ = fmt.Fprintf(stdout, "adopting %s (%d/%d)\n", name, index, total)
		}
		options.TaskStarted = func(feature string, event workflow.VerifyTaskStart) {
			printAdoptTaskStart(stdout, feature, event)
		}
		options.TaskFinished = func(feature string, event workflow.VerifyTaskFinish) {
			printAdoptTaskFinish(stdout, feature, event)
		}
	}
	report, err := adopt.ApplyWithOptions(context.Background(), root, featureName, commandRunner, options)
	if err != nil {
		result := errorResult(err)
		result.Adoption = adoptApplyResult(report).Adoption
		return emitResult("adopt", result, jsonMode, stdout, stderr)
	}
	result := adoptApplyResult(report)
	if jsonMode {
		return emitResult("adopt", result, jsonMode, stdout, stderr)
	}
	if result.ExitCode != 0 {
		// A failed adoption is a work list, not an invocation error: render
		// the full partition, mirroring release check's convention.
		output.PrintText(stderr, result)
		return result.ExitCode
	}
	output.PrintText(stdout, result)
	return 0
}

func printAdoptTaskStart(w io.Writer, feature string, event workflow.VerifyTaskStart) {
	_, _ = fmt.Fprintf(w, "  %s task %s (%d/%d): starting proof\n", feature, event.TaskID, event.Index, event.Total)
}

func printAdoptTaskFinish(w io.Writer, feature string, event workflow.VerifyTaskFinish) {
	verdict := "rejected"
	if event.Passed {
		verdict = "accepted"
	}
	_, _ = fmt.Fprintf(w, "  %s task %s: execution %s, assertion=%s, integrity=%s, elapsed=%d ms\n",
		feature, event.TaskID, verdict, event.AssertionResult, event.Integrity, event.Elapsed.Milliseconds())
	if event.Failure != "" {
		_, _ = fmt.Fprintf(w, "    %s\n", event.Failure)
	}
}

func adoptPlanResult(report adopt.PlanReport) output.Result {
	status := &output.AdoptionStatus{Scope: &report.Scope, Workload: adoptionWorkloadSummary(report.Workload)}
	for _, feature := range report.Features {
		view := output.AdoptionFeature{
			Feature: feature.Name, Class: feature.Class, SealableDocs: append([]string(nil), feature.SealableDocs...),
			ReproveCount: feature.ReproveCount, Reason: feature.BlockReason,
			Workload: adoptionWorkload(feature.Workload),
		}
		for _, entry := range feature.Evidence {
			view.Evidence = append(view.Evidence, output.EvidenceView(entry))
		}
		status.Features = append(status.Features, view)
	}

	totals := report.Totals
	applyCommand := "walden adopt"
	if name := report.Scope.NamedFeature(); name != "" {
		applyCommand += " " + name
	}
	applyCommand += " --apply"
	nextAction := ""
	if totals.SealableDocs > 0 || totals.ReproveTasks > 0 {
		nextAction = "Review this scope and its trust assumptions before explicitly running " + applyCommand
	}
	if totals.Blocked > 0 && nextAction == "" {
		nextAction = "Resolve the named blockers in this scope; do not automatically reseal or discard historical evidence"
	}
	return output.Result{
		Summary: fmt.Sprintf("ADOPTION PLAN — %d backfill, %d re-prove, %d complete, %d blocked (%d doc(s) to seal, %d task(s) to re-prove)",
			totals.Backfill, totals.Reprove, totals.Complete, totals.Blocked, totals.SealableDocs, totals.ReproveTasks),
		Adoption:   status,
		NextAction: nextAction,
		ExitCode:   0,
	}
}

func adoptionWorkload(work adopt.Workload) *output.AdoptionWorkload {
	view := &output.AdoptionWorkload{Available: work.Available, Reason: work.Reason}
	if work.Available {
		view.Tasks, view.Steps = &work.Tasks, &work.Steps
	}
	return view
}

func adoptionWorkloadSummary(work adopt.WorkloadSummary) *output.AdoptionWorkloadSummary {
	return &output.AdoptionWorkloadSummary{
		AssessedTasks: work.AssessedTasks, AssessedSteps: work.AssessedSteps,
		UnassessedFeatures: append([]string{}, work.UnassessedFeatures...),
	}
}

func adoptApplyResult(report adopt.ApplyReport) output.Result {
	status := &output.AdoptionStatus{Apply: true, ElapsedMS: elapsedMilliseconds(report.Elapsed)}
	if report.Scope.Kind != "" {
		status.Scope = &report.Scope
	}
	if report.Workload != nil {
		status.Workload = adoptionWorkloadSummary(*report.Workload)
	}
	var warnings []string
	for _, feature := range report.Features {
		view := output.AdoptionFeature{
			Feature:           feature.Name,
			Class:             feature.Class,
			SealedDocs:        append([]string(nil), feature.SealedDocs...),
			Verified:          append([]string(nil), feature.Verified...),
			Failed:            append([]string(nil), feature.Failed...),
			Skipped:           feature.Skipped,
			Reason:            feature.Error,
			Workload:          adoptionWorkload(feature.Workload),
			EvidencePersisted: feature.EvidencePersisted,
			ElapsedMS:         elapsedMilliseconds(feature.Elapsed),
		}
		for _, outcome := range feature.Outcomes {
			view.Evidence = append(view.Evidence, verificationOutcomeView(outcome))
		}
		for _, warning := range feature.Warnings {
			warnings = append(warnings, feature.Name+": "+warning)
		}
		status.Features = append(status.Features, view)
	}

	totals := report.Totals
	result := output.Result{
		Summary: fmt.Sprintf("ADOPTION — sealed %d doc(s), verified %d, failed %d, skipped %d, blocked %d, errors %d",
			totals.SealedDocs, totals.Verified, totals.Failed, totals.Skipped, totals.Blocked, totals.Errors),
		Adoption: status,
		Warnings: warnings,
		ExitCode: 0,
	}
	if totals.Failed > 0 || totals.Errors > 0 || totals.Blocked > 0 {
		retry := "walden adopt"
		if name := report.Scope.NamedFeature(); name != "" {
			retry += " " + name
		}
		result.NextAction = "Inspect the failed/blocked partition and its assurance gaps before explicitly retrying " + retry + " --apply"
		result.ExitCode = 1
	}
	return result
}
