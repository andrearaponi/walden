package adopt

import (
	"context"

	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/workflow"
)

// FeatureAdoption is one feature's apply outcome.
type FeatureAdoption struct {
	Name       string
	Class      string
	SealedDocs []string
	Verified   []string
	Failed     []string
	Skipped    int
	Error      string
}

// ApplyTotals aggregates the run.
type ApplyTotals struct {
	SealedDocs int
	Verified   int
	Failed     int
	Skipped    int
	Blocked    int
}

// ApplyReport is the outcome of an adoption run.
type ApplyReport struct {
	Features []FeatureAdoption
	Totals   ApplyTotals
}

// Progress is invoked as each feature starts: its name and position in the
// run — the heartbeat of a 135-feature adoption.
type Progress func(name string, index, total int)

// Apply re-plans and acts, feature by feature in sorted order: seal absent
// fingerprints, then re-prove through the verify machinery in its default
// mode — which selects exactly the non-verified completed tasks, so an
// interrupted or partially failed run resumes for free. Blocked features are
// skipped untouched; proof failures land in the partition and the run
// continues.
func Apply(ctx context.Context, root, featureName string, runner shell.Runner, progress Progress) (ApplyReport, error) {
	plan, err := Plan(ctx, root, featureName)
	if err != nil {
		return ApplyReport{}, err
	}

	report := ApplyReport{}
	total := len(plan.Features)
	for index, featurePlan := range plan.Features {
		if progress != nil {
			progress(featurePlan.Name, index+1, total)
		}
		adoption := FeatureAdoption{Name: featurePlan.Name, Class: featurePlan.Class}

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
					adoption.Error = sealErr.Error()
					report.Features = append(report.Features, adoption)
					continue
				}
			}

			result, verifyErr := workflow.Verify(ctx, root, featurePlan.Name, false, false, runner)
			if verifyErr != nil {
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
		report.Features = append(report.Features, adoption)
	}
	return report, nil
}
