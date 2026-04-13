package workflow

// Phase is the current planning phase for a feature.
type Phase string

const (
	PhaseRequirements Phase = "requirements"
	PhaseDesign       Phase = "design"
	PhaseTasks        Phase = "tasks"
)
