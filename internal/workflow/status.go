package workflow

import (
	"fmt"
	"strings"

	"github.com/andrearaponi/walden/internal/spec"
)

// DocumentState is the workflow-oriented view of a loaded document.
type DocumentState struct {
	Path                         string
	Status                       string
	ApprovedAt                   string
	LastModified                 string
	ApprovedFingerprint          string
	SourceRequirementsApprovedAt string
	SourceDesignApprovedAt       string
	Exists                       bool
	Fresh                        bool
	Intact                       bool
	StaleCauses                  []string
}

// FeatureState describes the current workflow state of a feature.
type FeatureState struct {
	Name         string
	Root         string
	Requirements DocumentState
	Design       DocumentState
	Tasks        DocumentState
	CurrentPhase Phase
	NextAction   string
	IsStale      bool
	Blockers     []string
}

// LoadFeatureState loads a feature from disk and resolves its workflow state.
func LoadFeatureState(root, featureName string) (FeatureState, error) {
	feature, err := spec.LoadFeature(root, featureName)
	if err != nil {
		return FeatureState{}, err
	}

	return ResolveFeatureState(feature), nil
}

// ResolveFeatureState converts parsed documents into a workflow decision state.
func ResolveFeatureState(feature spec.Feature) FeatureState {
	state := FeatureState{
		Name:         feature.Name,
		Root:         feature.Root,
		Requirements: toDocumentState(feature.Requirements),
		Design:       toDocumentState(feature.Design),
		Tasks:        toDocumentState(feature.Tasks),
	}

	evaluateFreshness(&state, spec.EvaluateFreshness(feature))
	state.Blockers = collectBlockers(state)
	state.IsStale = !state.Requirements.Fresh && state.Requirements.Exists && state.Requirements.Status == "approved" ||
		!state.Design.Fresh && state.Design.Exists && state.Design.Status == "approved" ||
		!state.Tasks.Fresh && state.Tasks.Exists && state.Tasks.Status == "approved"
	state.CurrentPhase, state.NextAction = resolveCurrentPhaseAndAction(feature, state)

	return state
}

func toDocumentState(document spec.Document) DocumentState {
	return DocumentState{
		Path:                         document.Path,
		Status:                       document.Status,
		ApprovedAt:                   document.ApprovedAt,
		LastModified:                 document.LastModified,
		ApprovedFingerprint:          document.ApprovedFingerprint,
		SourceRequirementsApprovedAt: document.SourceRequirementsApprovedAt,
		SourceDesignApprovedAt:       document.SourceDesignApprovedAt,
		Exists:                       document.Exists,
		Fresh:                        document.Exists,
		Intact:                       true,
	}
}

func evaluateFreshness(state *FeatureState, report spec.FreshnessReport) {
	applyVerdict(&state.Requirements, report.Requirements)
	applyVerdict(&state.Design, report.Design)
	applyVerdict(&state.Tasks, report.Tasks)
}

func applyVerdict(document *DocumentState, verdict spec.DocumentFreshness) {
	document.Intact = verdict.Intact
	document.StaleCauses = verdict.Causes
	if document.Exists {
		document.Fresh = verdict.Fresh
	}
}

// staleDocumentMessage describes an approved document whose own content no
// longer matches its approval fingerprint.
func staleDocumentMessage(name string, document DocumentState) string {
	return fmt.Sprintf("%s is stale: %s", name, strings.Join(document.StaleCauses, "; "))
}

// staleChainMessage describes a downstream document whose approval no longer
// binds to the upstream's current approved content.
func staleChainMessage(name, upstream string, document DocumentState) string {
	if !document.Intact {
		return staleDocumentMessage(name, document)
	}
	return fmt.Sprintf("%s is stale relative to %s: %s", name, upstream, strings.Join(document.StaleCauses, "; "))
}

func collectBlockers(state FeatureState) []string {
	blockers := make([]string, 0, 6)

	if state.Design.Exists && !state.Requirements.Exists {
		blockers = append(blockers, "design.md exists without requirements.md")
	}
	if state.Tasks.Exists && !state.Design.Exists {
		blockers = append(blockers, "tasks.md exists without design.md")
	}
	if state.Tasks.Exists && !state.Requirements.Exists {
		blockers = append(blockers, "tasks.md exists without requirements.md")
	}

	if state.Design.Status == "approved" && state.Requirements.Status != "approved" {
		blockers = append(blockers, "design.md requires approved requirements.md")
	}
	if state.Tasks.Status == "approved" && state.Design.Status != "approved" {
		blockers = append(blockers, "tasks.md requires approved design.md")
	}

	if state.Requirements.Exists && state.Requirements.Status == "approved" && !state.Requirements.Fresh {
		blockers = append(blockers, staleDocumentMessage("requirements.md", state.Requirements))
	}

	if state.Design.Exists && state.Design.Status == "approved" && !state.Design.Fresh {
		blockers = append(blockers, staleChainMessage("design.md", "requirements.md", state.Design))
	}

	if state.Tasks.Exists {
		switch {
		case state.Design.Exists && state.Design.Status == "approved" && !state.Design.Fresh:
			blockers = append(blockers, "tasks.md is stale because design.md is stale")
		case state.Tasks.Status == "approved" && !state.Tasks.Fresh:
			blockers = append(blockers, staleChainMessage("tasks.md", "design.md", state.Tasks))
		}
	}

	return blockers
}

func resolveCurrentPhaseAndAction(feature spec.Feature, state FeatureState) (Phase, string) {
	switch {
	case !state.Requirements.Exists:
		return PhaseRequirements, "Create requirements.md"
	case state.Requirements.Status == "draft":
		return PhaseRequirements, "Edit requirements.md and move it to in-review"
	case state.Requirements.Status == "in-review":
		return PhaseRequirements, "Approve requirements.md"
	case state.Requirements.Status != "approved":
		return PhaseRequirements, "Resolve requirements.md status"
	case !state.Requirements.Fresh:
		return PhaseRequirements, "Run walden reconcile to repair the approval chain"
	case state.Design.Exists && state.Design.Status == "approved" && !state.Design.Fresh:
		if !state.Design.Intact {
			return PhaseDesign, "Run walden reconcile to repair the approval chain"
		}
		return PhaseDesign, "Update design.md to match requirements.md and return it to in-review"
	case !state.Design.Exists:
		return PhaseDesign, "Create design.md"
	case state.Design.Status == "draft":
		return PhaseDesign, "Edit design.md and move it to in-review"
	case state.Design.Status == "in-review":
		return PhaseDesign, "Approve design.md"
	case state.Design.Status != "approved":
		return PhaseDesign, "Resolve design.md status"
	case state.Tasks.Exists && state.Tasks.Status == "approved" && !state.Tasks.Fresh:
		if !state.Tasks.Intact {
			return PhaseTasks, "Run walden reconcile to repair the approval chain"
		}
		return PhaseTasks, "Update tasks.md to match the latest approved design and return it to in-review"
	case !state.Tasks.Exists:
		return PhaseTasks, "Create tasks.md"
	case state.Tasks.Status == "draft":
		return PhaseTasks, "Edit tasks.md and move it to in-review"
	case state.Tasks.Status == "in-review":
		return PhaseTasks, "Approve tasks.md"
	default:
		return PhaseTasks, resolveExecutionNextAction(feature.Tasks)
	}
}

func resolveExecutionNextAction(tasks spec.Document) string {
	if !tasks.Exists {
		return "Start execution from the next unchecked task"
	}

	tree, err := spec.ParseTaskTree(tasks)
	if err != nil {
		return "Start execution from the next unchecked task"
	}

	for _, task := range tree.LeafTasks() {
		if !task.Completed {
			return "Start execution from the next unchecked task"
		}
	}

	return "No runnable tasks remain; implementation plan is complete"
}
