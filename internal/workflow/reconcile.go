package workflow

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/andrearaponi/walden/internal/spec"
)

// ReconcileResult captures the deterministic outcome of workflow normalization after upstream edits.
type ReconcileResult struct {
	Feature      string
	ChangedDocs  []string
	CurrentPhase Phase
	NextAction   string
}

// ReconcileFeature applies deterministic workflow normalization to a feature.
func ReconcileFeature(root, featureName string) (ReconcileResult, error) {
	return reconcileFeatureAt(root, featureName, time.Now().UTC().Format("2006-01-02T15:04:05Z"))
}

func reconcileFeatureAt(root, featureName, reconciledAt string) (ReconcileResult, error) {
	feature, err := spec.LoadFeature(root, featureName)
	if err != nil {
		return ReconcileResult{}, err
	}

	reconciled, changedDocs, err := reconcileFeature(feature, reconciledAt)
	if err != nil {
		return ReconcileResult{}, err
	}

	for _, name := range changedDocs {
		document := documentByName(&reconciled, name)
		if document == nil {
			return ReconcileResult{}, fmt.Errorf("unknown reconcile document %q", name)
		}
		if err := spec.SaveDocument(*document); err != nil {
			return ReconcileResult{}, err
		}
	}

	state := ResolveFeatureState(reconciled)
	return ReconcileResult{
		Feature:      reconciled.Name,
		ChangedDocs:  changedDocs,
		CurrentPhase: state.CurrentPhase,
		NextAction:   state.NextAction,
	}, nil
}

func reconcileFeature(feature spec.Feature, reconciledAt string) (spec.Feature, []string, error) {
	changedDocs := map[string]struct{}{}

	// Pass 1: approved documents whose own content no longer matches their
	// approval fingerprint — or that lack one — lose their approval.
	// Changed content is unreviewed content: draft is the honest state.
	report := spec.EvaluateFreshness(feature)
	if err := resetIfNotIntact(&feature.Requirements, report.Requirements, reconciledAt, changedDocs); err != nil {
		return spec.Feature{}, nil, err
	}
	if err := resetIfNotIntact(&feature.Design, report.Design, reconciledAt, changedDocs); err != nil {
		return spec.Feature{}, nil, err
	}
	if err := resetIfNotIntact(&feature.Tasks, report.Tasks, reconciledAt, changedDocs); err != nil {
		return spec.Feature{}, nil, err
	}

	// Pass 2: chain repair — a downstream document bound to an upstream
	// approval fingerprint that differs (or no longer exists) resets to
	// draft, cascading design -> tasks.
	effectiveRequirements := effectiveApprovedFingerprint(feature.Requirements)
	if feature.Design.Exists && feature.Design.SourceRequirementsFingerprint != effectiveRequirements {
		if err := resetToDraft(&feature.Design, reconciledAt, changedDocs); err != nil {
			return spec.Feature{}, nil, err
		}
		if feature.Tasks.Exists {
			if err := resetToDraft(&feature.Tasks, reconciledAt, changedDocs); err != nil {
				return spec.Feature{}, nil, err
			}
		}
	}

	effectiveDesign := effectiveApprovedFingerprint(feature.Design)
	if feature.Tasks.Exists && feature.Tasks.SourceDesignFingerprint != effectiveDesign {
		if err := resetToDraft(&feature.Tasks, reconciledAt, changedDocs); err != nil {
			return spec.Feature{}, nil, err
		}
	}

	return feature, orderedChangedDocs(changedDocs), nil
}

func resetIfNotIntact(document *spec.Document, verdict spec.DocumentFreshness, reconciledAt string, changedDocs map[string]struct{}) error {
	if !document.Exists || document.Status != "approved" || verdict.Intact {
		return nil
	}

	return resetToDraft(document, reconciledAt, changedDocs)
}

func resetToDraft(document *spec.Document, reconciledAt string, changedDocs map[string]struct{}) error {
	if !document.Exists {
		return nil
	}

	updated, err := spec.ResetDocumentToDraft(*document, reconciledAt)
	if err != nil {
		return err
	}
	if documentChanged(*document, updated) {
		*document = updated
		changedDocs[filepath.Base(document.Path)] = struct{}{}
	}

	return nil
}

func effectiveApprovedFingerprint(document spec.Document) string {
	if document.Exists && document.Status == "approved" {
		return document.ApprovedFingerprint
	}

	return ""
}

func documentChanged(before, after spec.Document) bool {
	return before.Status != after.Status ||
		before.ApprovedAt != after.ApprovedAt ||
		before.LastModified != after.LastModified ||
		before.ApprovedFingerprint != after.ApprovedFingerprint ||
		before.SourceRequirementsApprovedAt != after.SourceRequirementsApprovedAt ||
		before.SourceDesignApprovedAt != after.SourceDesignApprovedAt ||
		before.SourceRequirementsFingerprint != after.SourceRequirementsFingerprint ||
		before.SourceDesignFingerprint != after.SourceDesignFingerprint
}

func orderedChangedDocs(changed map[string]struct{}) []string {
	ordered := make([]string, 0, len(changed))
	for _, name := range []string{"requirements.md", "design.md", "tasks.md"} {
		if _, ok := changed[name]; ok {
			ordered = append(ordered, name)
		}
	}
	return ordered
}

func documentByName(feature *spec.Feature, name string) *spec.Document {
	switch name {
	case "requirements.md":
		return &feature.Requirements
	case "design.md":
		return &feature.Design
	case "tasks.md":
		return &feature.Tasks
	default:
		return nil
	}
}
