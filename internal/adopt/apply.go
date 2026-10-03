package adopt

import (
	"context"
	"time"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/workflow"
)

// FeatureAdoption is one feature's apply outcome.
type FeatureAdoption struct {
	Name              string
	Class             string
	SealedDocs        []string
	Verified          []string
	Failed            []string
	Skipped           int
	Error             string
	Workload          Workload
	Outcomes          []workflow.VerifyOutcome
	Warnings          []string
	EvidencePersisted *bool
	Elapsed           *time.Duration
}

// ApplyTotals aggregates the run.
type ApplyTotals struct {
	SealedDocs int
	Verified   int
	Failed     int
	Skipped    int
	Blocked    int
	Errors     int
}

// ApplyReport is the outcome of an adoption run.
type ApplyReport struct {
	Scope    evidence.Scope
	Features []FeatureAdoption
	Totals   ApplyTotals
	Workload *WorkloadSummary
	Elapsed  *time.Duration
}

// Progress is invoked as each feature starts: its name and position in the
// run — the heartbeat of a 135-feature adoption.
type Progress func(name string, index, total int)

// ApplyOptions contains observation hooks, not execution-policy switches.
type ApplyOptions struct {
	FeatureStarted Progress
	TaskStarted    func(string, workflow.VerifyTaskStart)
	TaskFinished   func(string, workflow.VerifyTaskFinish)
	Now            func() time.Time
}

// Apply re-plans and acts, feature by feature in sorted order: seal absent
// fingerprints, then re-prove through the verify machinery in its default
// mode — which selects exactly the non-verified completed tasks, so an
// interrupted or partially failed run resumes for free. Blocked features are
// skipped untouched; proof failures land in the partition and the run
// continues.
func Apply(ctx context.Context, root, featureName string, runner shell.Runner, progress Progress) (ApplyReport, error) {
	return ApplyWithOptions(ctx, root, featureName, runner, ApplyOptions{FeatureStarted: progress})
}

// ApplyWithOptions observes the same plan/seal/verify sequence as Apply.
func ApplyWithOptions(ctx context.Context, root, featureName string, runner shell.Runner, options ApplyOptions) (report ApplyReport, err error) {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	started := now()
	defer func() {
		elapsed := now().Sub(started)
		report.Elapsed = &elapsed
	}()
	plan, err := Plan(ctx, root, featureName)
	if err != nil {
		return ApplyReport{}, err
	}

	report = ApplyReport{Scope: plan.Scope, Workload: &plan.Workload}
	total := len(plan.Features)
	for index, featurePlan := range plan.Features {
		featureStarted := now()
		if options.FeatureStarted != nil {
			options.FeatureStarted(featurePlan.Name, index+1, total)
		}
		adoption := FeatureAdoption{Name: featurePlan.Name, Class: featurePlan.Class, Workload: featurePlan.Workload}
		finishFeature := func() {
			elapsed := now().Sub(featureStarted)
			adoption.Elapsed = &elapsed
			report.Features = append(report.Features, adoption)
		}

		switch featurePlan.Class {
		case ClassBlocked:
			adoption.Error = featurePlan.BlockReason
			report.Totals.Blocked++
		case ClassComplete:
			// Nothing to adopt.
		default:
			if featurePlan.Class == ClassBackfill {
				sealed, sealErr := sealFeature(root, featurePlan.Name)
				adoption.SealedDocs = sealed
				report.Totals.SealedDocs += len(sealed)
				if sealErr != nil {
					report.Totals.Errors++
					adoption.Error = sealErr.Error()
					finishFeature()
					continue
				}
			}

			observation := workflow.VerifyOptions{Now: now}
			if options.TaskStarted != nil {
				observation.TaskStarted = func(event workflow.VerifyTaskStart) {
					options.TaskStarted(featurePlan.Name, event)
				}
			}
			if options.TaskFinished != nil {
				observation.TaskFinished = func(event workflow.VerifyTaskFinish) {
					options.TaskFinished(featurePlan.Name, event)
				}
			}
			result, verifyErr := workflow.VerifyWithOptions(ctx, root, featurePlan.Name, false, false, runner, observation)
			adoption.Outcomes = result.Outcomes
			adoption.Warnings = result.Warnings
			adoption.EvidencePersisted = result.EvidencePersisted
			if verifyErr != nil {
				report.Totals.Errors++
				adoption.Error = verifyErr.Error()
			} else {
				for _, outcome := range result.Outcomes {
					if outcome.Passed {
						adoption.Verified = append(adoption.Verified, outcome.TaskID)
					} else {
						adoption.Failed = append(adoption.Failed, outcome.TaskID)
					}
				}
				adoption.Skipped = len(result.Skipped)
			}
			report.Totals.Verified += len(adoption.Verified)
			report.Totals.Failed += len(adoption.Failed)
			report.Totals.Skipped += adoption.Skipped
		}
		finishFeature()
	}
	return report, nil
}
