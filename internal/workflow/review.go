package workflow

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/andrearaponi/walden/internal/spec"
)

// ReviewContext is the deterministic context returned when a review gate opens.
type ReviewContext struct {
	Feature    string
	Phase      Phase
	BranchName string
	Document   string
}

// ApprovalResult captures the deterministic outcome of a review approval.
type ApprovalResult struct {
	Feature      string
	Phase        Phase
	Document     string
	ApprovedAt   string
	CurrentPhase Phase
	NextAction   string
}

// OpenReview transitions a document into in-review after enforcing prerequisites.
func OpenReview(root, featureName string, phase Phase) (ReviewContext, error) {
	feature, err := spec.LoadFeature(root, featureName)
	if err != nil {
		return ReviewContext{}, err
	}

	state := ResolveFeatureState(feature)
	if err := validateOpenReview(state, phase); err != nil {
		return ReviewContext{}, err
	}

	document := documentForPhase(&feature, phase)
	if document == nil || !document.Exists {
		return ReviewContext{}, fmt.Errorf("%s.md does not exist", phase)
	}

	if document.Status != "in-review" {
		document.Status = "in-review"
		document.LastModified = time.Now().UTC().Format("2006-01-02T15:04:05Z")
		document.Fields["status"] = document.Status
		document.Fields["last_modified"] = document.LastModified

		if err := spec.SaveDocument(*document); err != nil {
			return ReviewContext{}, err
		}
	}

	relativePath, err := filepath.Rel(root, document.Path)
	if err != nil {
		return ReviewContext{}, fmt.Errorf("resolve review document path: %w", err)
	}

	return ReviewContext{
		Feature:    feature.Name,
		Phase:      phase,
		BranchName: fmt.Sprintf("%s/%s", phase, feature.Name),
		Document:   filepath.ToSlash(relativePath),
	}, nil
}

// ApproveReview transitions a document from in-review to approved.
func ApproveReview(root, featureName string, phase Phase) (ApprovalResult, error) {
	feature, err := spec.LoadFeature(root, featureName)
	if err != nil {
		return ApprovalResult{}, err
	}

	state := ResolveFeatureState(feature)
	if err := validateApproveReview(state, phase); err != nil {
		return ApprovalResult{}, err
	}

	document := documentForPhase(&feature, phase)
	if document == nil || !document.Exists {
		return ApprovalResult{}, fmt.Errorf("%s.md does not exist", phase)
	}

	approvedAt := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	document.Status = "approved"
	document.ApprovedAt = approvedAt
	document.LastModified = approvedAt
	document.Fields["status"] = document.Status
	document.Fields["approved_at"] = document.ApprovedAt
	document.Fields["last_modified"] = document.LastModified

	switch phase {
	case PhaseDesign:
		document.SourceRequirementsApprovedAt = feature.Requirements.ApprovedAt
		document.Fields["source_requirements_approved_at"] = document.SourceRequirementsApprovedAt
	case PhaseTasks:
		document.SourceDesignApprovedAt = feature.Design.ApprovedAt
		document.Fields["source_design_approved_at"] = document.SourceDesignApprovedAt
	}

	if err := spec.SaveDocument(*document); err != nil {
		return ApprovalResult{}, err
	}

	relativePath, err := filepath.Rel(root, document.Path)
	if err != nil {
		return ApprovalResult{}, fmt.Errorf("resolve approved document path: %w", err)
	}

	nextState := ResolveFeatureState(feature)
	return ApprovalResult{
		Feature:      feature.Name,
		Phase:        phase,
		Document:     filepath.ToSlash(relativePath),
		ApprovedAt:   approvedAt,
		CurrentPhase: nextState.CurrentPhase,
		NextAction:   nextState.NextAction,
	}, nil
}

func validateOpenReview(state FeatureState, phase Phase) error {
	switch phase {
	case PhaseRequirements:
		if !state.Requirements.Exists {
			return fmt.Errorf("requirements.md does not exist")
		}
		return nil
	case PhaseDesign:
		if state.Requirements.Status != "approved" {
			return fmt.Errorf("requirements.md must be approved before opening design review")
		}
		if state.Design.Status == "approved" && !state.Design.Fresh {
			return fmt.Errorf("design.md is stale relative to requirements.md")
		}
		if !state.Design.Exists {
			return fmt.Errorf("design.md does not exist")
		}
		return nil
	case PhaseTasks:
		if state.Requirements.Status != "approved" {
			return fmt.Errorf("requirements.md must be approved before opening tasks review")
		}
		if state.Design.Status != "approved" {
			return fmt.Errorf("design.md must be approved before opening tasks review")
		}
		if !state.Design.Fresh {
			return fmt.Errorf("design.md is stale relative to requirements.md")
		}
		if state.Tasks.Status == "approved" && !state.Tasks.Fresh {
			return fmt.Errorf("tasks.md is stale relative to design.md")
		}
		if !state.Tasks.Exists {
			return fmt.Errorf("tasks.md does not exist")
		}
		return nil
	default:
		return fmt.Errorf("invalid phase %q", phase)
	}
}

func validateApproveReview(state FeatureState, phase Phase) error {
	switch phase {
	case PhaseRequirements:
		if !state.Requirements.Exists {
			return fmt.Errorf("requirements.md does not exist")
		}
		if state.Requirements.Status != "in-review" {
			return fmt.Errorf("requirements.md must be in-review before approval")
		}
		return nil
	case PhaseDesign:
		if !state.Design.Exists {
			return fmt.Errorf("design.md does not exist")
		}
		if state.Requirements.Status != "approved" {
			return fmt.Errorf("requirements.md must be approved before approving design review")
		}
		if state.Design.Status != "in-review" {
			return fmt.Errorf("design.md must be in-review before approval")
		}
		return nil
	case PhaseTasks:
		if !state.Tasks.Exists {
			return fmt.Errorf("tasks.md does not exist")
		}
		if state.Requirements.Status != "approved" {
			return fmt.Errorf("requirements.md must be approved before approving tasks review")
		}
		if state.Design.Status != "approved" {
			return fmt.Errorf("design.md must be approved before approving tasks review")
		}
		if !state.Design.Fresh {
			return fmt.Errorf("design.md is stale relative to requirements.md")
		}
		if state.Tasks.Status != "in-review" {
			return fmt.Errorf("tasks.md must be in-review before approval")
		}
		return nil
	default:
		return fmt.Errorf("invalid phase %q", phase)
	}
}

func documentForPhase(feature *spec.Feature, phase Phase) *spec.Document {
	switch phase {
	case PhaseRequirements:
		return &feature.Requirements
	case PhaseDesign:
		return &feature.Design
	case PhaseTasks:
		return &feature.Tasks
	default:
		return nil
	}
}

// ParsePhase converts a user-facing phase string to the typed workflow phase.
func ParsePhase(raw string) (Phase, error) {
	switch raw {
	case string(PhaseRequirements):
		return PhaseRequirements, nil
	case string(PhaseDesign):
		return PhaseDesign, nil
	case string(PhaseTasks):
		return PhaseTasks, nil
	default:
		return "", fmt.Errorf("invalid phase %q", raw)
	}
}
