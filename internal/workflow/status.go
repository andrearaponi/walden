package workflow

import "github.com/andrearaponi/walden/internal/spec"

// DocumentState is the workflow-oriented view of a loaded document.
type DocumentState struct {
	Path                         string
	Status                       string
	ApprovedAt                   string
	LastModified                 string
	SourceRequirementsApprovedAt string
	SourceDesignApprovedAt       string
	Exists                       bool
	Fresh                        bool
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

	evaluateFreshness(&state)
	state.Blockers = collectBlockers(state)
	state.IsStale = !state.Design.Fresh && state.Design.Exists && state.Design.Status == "approved" ||
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
		SourceRequirementsApprovedAt: document.SourceRequirementsApprovedAt,
		SourceDesignApprovedAt:       document.SourceDesignApprovedAt,
		Exists:                       document.Exists,
		Fresh:                        document.Exists,
	}
}

func evaluateFreshness(state *FeatureState) {
	if state.Design.Exists && state.Design.Status == "approved" {
		state.Design.Fresh = state.Requirements.Status == "approved" &&
			state.Requirements.ApprovedAt != "" &&
			timestampsMatch(state.Design.SourceRequirementsApprovedAt, state.Requirements.ApprovedAt)
	}

	if state.Tasks.Exists && state.Tasks.Status == "approved" {
		state.Tasks.Fresh = state.Design.Status == "approved" &&
			state.Design.ApprovedAt != "" &&
			timestampsMatch(state.Tasks.SourceDesignApprovedAt, state.Design.ApprovedAt)
	}

	if state.Design.Exists && state.Design.Status == "approved" && !state.Design.Fresh {
		if state.Tasks.Exists {
			state.Tasks.Fresh = false
		}
	}
}

func timestampsMatch(a, b string) bool {
	if a == "" || b == "" {
		return a == b
	}
	equal, err := spec.TimestampsEqual(a, b)
	if err != nil {
		return a == b
	}
	return equal
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

	if state.Design.Exists && state.Design.Status == "approved" && !state.Design.Fresh {
		blockers = append(blockers, "design.md is stale relative to requirements.md")
	}

	if state.Tasks.Exists {
		switch {
		case state.Design.Exists && state.Design.Status == "approved" && !state.Design.Fresh:
			blockers = append(blockers, "tasks.md is stale because design.md is stale")
		case state.Tasks.Status == "approved" && !state.Tasks.Fresh:
			blockers = append(blockers, "tasks.md is stale relative to design.md")
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
	case state.Design.Exists && state.Design.Status == "approved" && !state.Design.Fresh:
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
