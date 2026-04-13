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

	if err := downgradeIfModified(&feature.Requirements, reconciledAt, changedDocs); err != nil {
		return spec.Feature{}, nil, err
	}
	if err := downgradeIfModified(&feature.Design, reconciledAt, changedDocs); err != nil {
		return spec.Feature{}, nil, err
	}
	if err := downgradeIfModified(&feature.Tasks, reconciledAt, changedDocs); err != nil {
		return spec.Feature{}, nil, err
	}

	effectiveRequirementsApprovedAt := effectiveApprovedAt(feature.Requirements)
	if feature.Design.Exists && feature.Design.SourceRequirementsApprovedAt != effectiveRequirementsApprovedAt {
		updatedDesign, err := spec.ResetDocumentToDraft(feature.Design, reconciledAt)
		if err != nil {
			return spec.Feature{}, nil, err
		}
		if documentChanged(feature.Design, updatedDesign) {
			feature.Design = updatedDesign
			changedDocs["design.md"] = struct{}{}
		}
		if feature.Tasks.Exists {
			updatedTasks, err := spec.ResetDocumentToDraft(feature.Tasks, reconciledAt)
			if err != nil {
				return spec.Feature{}, nil, err
			}
			if documentChanged(feature.Tasks, updatedTasks) {
				feature.Tasks = updatedTasks
				changedDocs["tasks.md"] = struct{}{}
			}
		}
	}

	effectiveDesignApprovedAt := effectiveApprovedAt(feature.Design)
	if feature.Tasks.Exists && feature.Tasks.SourceDesignApprovedAt != effectiveDesignApprovedAt {
		updatedTasks, err := spec.ResetDocumentToDraft(feature.Tasks, reconciledAt)
		if err != nil {
			return spec.Feature{}, nil, err
		}
		if documentChanged(feature.Tasks, updatedTasks) {
			feature.Tasks = updatedTasks
			changedDocs["tasks.md"] = struct{}{}
		}
	}

	return feature, orderedChangedDocs(changedDocs), nil
}

func downgradeIfModified(document *spec.Document, reconciledAt string, changedDocs map[string]struct{}) error {
	if !document.Exists || document.Status != "approved" {
		return nil
	}

	modified, err := modifiedAfterApproval(*document)
	if err != nil {
		return err
	}
	if !modified {
		return nil
	}

	updated, err := spec.DowngradeDocumentToInReview(*document, reconciledAt)
	if err != nil {
		return err
	}
	if documentChanged(*document, updated) {
		*document = updated
		changedDocs[filepath.Base(document.Path)] = struct{}{}
	}

	return nil
}

func modifiedAfterApproval(document spec.Document) (bool, error) {
	if document.Status != "approved" {
		return false, nil
	}

	approvedAt, err := parseTimestamp(document.Path, "approved_at", document.ApprovedAt)
	if err != nil {
		return false, err
	}
	lastModified, err := parseTimestamp(document.Path, "last_modified", document.LastModified)
	if err != nil {
		return false, err
	}

	return lastModified.After(approvedAt), nil
}

func parseTimestamp(path, field, raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, fmt.Errorf("%s: %s is required", filepath.Base(path), field)
	}

	parsed, err := time.Parse("2006-01-02T15:04:05Z", raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s: invalid %s %q", filepath.Base(path), field, raw)
	}

	return parsed, nil
}

func effectiveApprovedAt(document spec.Document) string {
	if document.Exists && document.Status == "approved" {
		return document.ApprovedAt
	}

	return ""
}

func documentChanged(before, after spec.Document) bool {
	return before.Status != after.Status ||
		before.ApprovedAt != after.ApprovedAt ||
		before.LastModified != after.LastModified ||
		before.SourceRequirementsApprovedAt != after.SourceRequirementsApprovedAt ||
		before.SourceDesignApprovedAt != after.SourceDesignApprovedAt
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
