// Package adopt owns the brownfield adoption lane: classify a repository's
// features against the current contract, seal recorded approvals whose
// fingerprints predate the fingerprint era, and re-prove unrecorded work
// through the verify machinery. It composes the kernel's existing guarantees
// and invents no evidence semantics of its own.
package adopt

import (
	"context"
	"fmt"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/spec"
)

// gitRunner executes the lane's read-only git plumbing (code identity for
// evidence derivation). A seam, like the gate's, so tests script outcomes.
var gitRunner shell.Runner = shell.NewExecRunner()

// Adoption classes: exactly one per feature.
const (
	// ClassBackfill: approved documents miss their fingerprints — sealable.
	ClassBackfill = "backfill"
	// ClassReprove: the chain is sealed and fresh, completed work lacks
	// verified evidence.
	ClassReprove = "re-prove"
	// ClassComplete: nothing to adopt.
	ClassComplete = "complete"
	// ClassBlocked: a present fingerprint contradicts reality — human
	// territory, never written by the lane.
	ClassBlocked = "blocked"
)

// FeaturePlan is one feature's classification and work counts.
type FeaturePlan struct {
	Name         string
	Class        string
	SealableDocs []string
	ReproveCount int
	BlockReason  string
	Evidence     []evidence.TaskEvidence
}

// Totals aggregates the portfolio.
type Totals struct {
	Backfill     int
	Reprove      int
	Complete     int
	Blocked      int
	SealableDocs int
	ReproveTasks int
}

// PlanReport is the read-only adoption plan.
type PlanReport struct {
	Scope    evidence.Scope
	Features []FeaturePlan
	Totals   Totals
}

// Plan classifies every feature (or one, when named) and counts the work.
// Strictly read-only: the plan is the reviewable decision.
func Plan(ctx context.Context, root, featureName string) (PlanReport, error) {
	names, err := adoptionTargets(root, featureName)
	if err != nil {
		return PlanReport{}, err
	}

	// One identity for the whole run, like the gate: evidence counting is
	// judged against a single tree.
	identity, identityOK := evidence.Identity(ctx, gitRunner, root)

	report := PlanReport{Scope: evidence.NewScope(featureName != "", names...)}
	for _, name := range names {
		plan := classify(ctx, root, name, identity, identityOK)
		report.Features = append(report.Features, plan)
		switch plan.Class {
		case ClassBackfill:
			report.Totals.Backfill++
		case ClassReprove:
			report.Totals.Reprove++
		case ClassComplete:
			report.Totals.Complete++
		case ClassBlocked:
			report.Totals.Blocked++
		}
		report.Totals.SealableDocs += len(plan.SealableDocs)
		report.Totals.ReproveTasks += plan.ReproveCount
	}
	return report, nil
}

func adoptionTargets(root, featureName string) ([]string, error) {
	if featureName != "" {
		normalized, err := spec.NormalizeFeatureName(featureName)
		if err != nil {
			return nil, err
		}
		return []string{normalized}, nil
	}
	features, err := spec.ListFeatures(root)
	if err != nil {
		return nil, err
	}
	if len(features) == 0 {
		return nil, fmt.Errorf("no features under .walden/specs to adopt")
	}
	return features, nil
}

// classify applies the lane's honesty rule: an absent fingerprint is
// sealable; a present fingerprint that contradicts reality is blocked.
func classify(ctx context.Context, root, name string, identity string, identityOK bool) FeaturePlan {
	plan := FeaturePlan{Name: name, Class: ClassComplete}

	feature, err := spec.LoadFeature(root, name)
	if err != nil {
		plan.Class = ClassBlocked
		plan.BlockReason = err.Error()
		return plan
	}

	plan.Evidence, err = assessEvidence(ctx, root, feature, identity, identityOK)
	if err != nil {
		plan.Class, plan.BlockReason = ClassBlocked, err.Error()
		return plan
	}
	documents := []struct {
		label    string
		document spec.Document
	}{
		{"requirements.md", feature.Requirements},
		{"design.md", feature.Design},
		{"tasks.md", feature.Tasks},
	}
	for _, entry := range documents {
		doc := entry.document
		if !doc.Exists || doc.Status != "approved" {
			continue
		}
		if doc.ApprovedFingerprint == "" {
			plan.SealableDocs = append(plan.SealableDocs, entry.label)
			continue
		}
		if !spec.BodyMatchesFingerprint(doc.Path, doc.Body, doc.ApprovedFingerprint) {
			plan.Class = ClassBlocked
			plan.BlockReason = fmt.Sprintf("%s is stale: its approved fingerprint no longer matches the content — run walden reconcile %s and re-approve", entry.label, name)
			return plan
		}
	}

	// Chain contradictions: both sides sealed, link disagrees. Empty links
	// beside sealable upstreams are repairable, never blocking.
	if reason := chainContradiction(feature); reason != "" {
		plan.Class = ClassBlocked
		plan.BlockReason = reason
		return plan
	}

	for _, entry := range plan.Evidence {
		if entry.State != evidence.StateVerified && entry.State != evidence.StatePending {
			plan.ReproveCount++
		}
	}
	switch {
	case len(plan.SealableDocs) > 0:
		plan.Class = ClassBackfill
	case plan.ReproveCount > 0:
		plan.Class = ClassReprove
	}
	return plan
}

func chainContradiction(feature spec.Feature) string {
	design, requirements := feature.Design, feature.Requirements
	if design.Status == "approved" && requirements.Status == "approved" &&
		design.SourceRequirementsFingerprint != "" && requirements.ApprovedFingerprint != "" &&
		design.SourceRequirementsFingerprint != requirements.ApprovedFingerprint {
		return fmt.Sprintf("design.md records a source requirements fingerprint that contradicts the sealed requirements.md — run walden reconcile %s", feature.Name)
	}
	tasks := feature.Tasks
	if tasks.Status == "approved" && design.Status == "approved" &&
		tasks.SourceDesignFingerprint != "" && design.ApprovedFingerprint != "" &&
		tasks.SourceDesignFingerprint != design.ApprovedFingerprint {
		return fmt.Sprintf("tasks.md records a source design fingerprint that contradicts the sealed design.md — run walden reconcile %s", feature.Name)
	}
	return ""
}

// assessEvidence never executes a profile probe or proof. Invalid approved
// plans and unreadable ledgers are assessment blockers, not zero-work success.
func assessEvidence(ctx context.Context, root string, feature spec.Feature, identity string, identityOK bool) ([]evidence.TaskEvidence, error) {
	if !feature.Tasks.Exists || feature.Tasks.Status != "approved" {
		return nil, nil
	}
	tree, err := spec.ParseTaskTree(feature.Tasks)
	if err != nil {
		return nil, fmt.Errorf("tasks assessment unavailable: %w", err)
	}
	ledger, err := evidence.Load(root, feature.Name)
	if err != nil {
		return nil, err
	}
	current, leafs := evidence.FeatureInputs(feature, tree)
	evidence.ResolvePlans(ctx, gitRunner, root, feature, ledger, &current, "")
	return evidence.Derive(ledger, current, identity, identityOK, leafs), nil
}
