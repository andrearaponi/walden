package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/spec"
)

// ExecutableTask is the workflow-facing execution view of one runnable leaf task.
type ExecutableTask struct {
	ID           string
	Title        string
	ParentID     string
	Completed    bool
	Level        int
	Requirements []string
	DesignRefs   []string
	Verification string
	Proof        spec.VerificationSpec
}

// ExecutionReadiness captures whether a feature can enter deterministic execution.
type ExecutionReadiness struct {
	Feature          string
	Runnable         bool
	CurrentPhase     Phase
	BlockingDocument string
	NextTask         *ExecutableTask
	Blockers         []string
	NextAction       string
}

// TaskStartContext is the deterministic execution context returned when a task starts.
type TaskStartContext struct {
	Feature      string
	CurrentPhase Phase
	Task         ExecutableTask
	NextAction   string
}

// TaskCompletionResult is the deterministic outcome of a proof-gated task completion.
type TaskCompletionResult struct {
	Feature        string
	CurrentPhase   Phase
	Task           ExecutableTask
	CompletedTasks []string
	ProofCommand   string
	NextAction     string
}

// BatchCompletionResult is the deterministic outcome of completing multiple runnable leaf tasks in order.
type BatchCompletionResult struct {
	Feature                string
	CurrentPhase           Phase
	CompletedTasks         []string
	CompletedLeafTasks     []string
	AutoCompletedParentIDs []string
	FailedTask             string
	Failure                string
	NextAction             string
}

// LoadExecutionReadiness loads and resolves execution readiness for a feature.
func LoadExecutionReadiness(root, featureName string) (ExecutionReadiness, error) {
	feature, err := spec.LoadFeature(root, featureName)
	if err != nil {
		return ExecutionReadiness{}, err
	}

	return ResolveExecutionReadiness(feature)
}

// ResolveExecutionReadiness determines whether execution can start and which task is next.
func ResolveExecutionReadiness(feature spec.Feature) (ExecutionReadiness, error) {
	state := ResolveFeatureState(feature)
	readiness := ExecutionReadiness{
		Feature:      feature.Name,
		CurrentPhase: state.CurrentPhase,
		NextAction:   state.NextAction,
	}

	if blockingDocument, blockers := executionBlockers(state); len(blockers) > 0 {
		readiness.BlockingDocument = blockingDocument
		readiness.Blockers = blockers
		return readiness, nil
	}

	tree, err := spec.ParseTaskTree(feature.Tasks)
	if err != nil {
		return ExecutionReadiness{}, err
	}

	for _, task := range tree.LeafTasks() {
		if task.Completed {
			continue
		}

		readiness.Runnable = true
		readiness.NextTask = &ExecutableTask{
			ID:           task.ID,
			Title:        task.Title,
			ParentID:     task.ParentID,
			Completed:    task.Completed,
			Level:        task.Level,
			Requirements: append([]string(nil), task.Requirements...),
			DesignRefs:   append([]string(nil), task.DesignRefs...),
			Verification: task.Verification,
			Proof:        task.Proof,
		}
		readiness.NextAction = fmt.Sprintf("Start task %s", task.ID)
		return readiness, nil
	}

	readiness.BlockingDocument = "tasks.md"
	readiness.Blockers = []string{"implementation plan has no remaining runnable leaf tasks"}
	readiness.NextAction = "No runnable tasks remain; implementation plan is complete"
	return readiness, nil
}

func executionBlockers(state FeatureState) (string, []string) {
	switch {
	case !state.Requirements.Exists:
		return "requirements.md", []string{"requirements.md must be approved and fresh before execution"}
	case state.Requirements.Status != "approved":
		return "requirements.md", []string{"requirements.md must be approved and fresh before execution"}
	case !state.Design.Exists:
		return "design.md", []string{"design.md must be approved and fresh before execution"}
	case state.Design.Status != "approved":
		return "design.md", []string{"design.md must be approved and fresh before execution"}
	case !state.Design.Fresh:
		return "design.md", []string{"design.md is stale relative to requirements.md"}
	case !state.Tasks.Exists:
		return "tasks.md", []string{"tasks.md must be approved and fresh before execution"}
	case state.Tasks.Status != "approved":
		return "tasks.md", []string{"tasks.md must be approved and fresh before execution"}
	case !state.Tasks.Fresh:
		return "tasks.md", []string{"tasks.md is stale relative to design.md"}
	default:
		return "", nil
	}
}

// StartTask resolves and validates the next executable task context without mutating task state.
func StartTask(root, featureName, taskID string) (TaskStartContext, error) {
	feature, err := spec.LoadFeature(root, featureName)
	if err != nil {
		return TaskStartContext{}, err
	}

	state := ResolveFeatureState(feature)
	if _, blockers := executionBlockers(state); len(blockers) > 0 {
		return TaskStartContext{}, fmt.Errorf("%s", blockers[0])
	}

	tree, err := spec.ParseTaskTree(feature.Tasks)
	if err != nil {
		return TaskStartContext{}, err
	}

	nextRunnable := nextRunnableTask(tree)
	selected := nextRunnable
	if taskID != "" {
		task, ok := tree.FindTask(taskID)
		if !ok {
			return TaskStartContext{}, fmt.Errorf("task %q does not exist", taskID)
		}
		if len(task.Children) > 0 {
			return TaskStartContext{}, fmt.Errorf("task %q is not an executable leaf task", taskID)
		}
		if task.Completed {
			return TaskStartContext{}, fmt.Errorf("task %q is already completed", taskID)
		}
		if nextRunnable == nil || task.ID != nextRunnable.ID {
			blockingID := ""
			if nextRunnable != nil {
				blockingID = nextRunnable.ID
			}
			if blockingID != "" {
				return TaskStartContext{}, fmt.Errorf("task %q is blocked by incomplete prerequisite task %q", taskID, blockingID)
			}
			return TaskStartContext{}, fmt.Errorf("task %q cannot start", taskID)
		}

		selected = &ExecutableTask{
			ID:           task.ID,
			Title:        task.Title,
			ParentID:     task.ParentID,
			Completed:    task.Completed,
			Level:        task.Level,
			Requirements: append([]string(nil), task.Requirements...),
			DesignRefs:   append([]string(nil), task.DesignRefs...),
			Verification: task.Verification,
			Proof:        task.Proof,
		}
	}

	if selected == nil {
		return TaskStartContext{}, fmt.Errorf("implementation plan has no remaining runnable leaf tasks")
	}

	return TaskStartContext{
		Feature:      feature.Name,
		CurrentPhase: state.CurrentPhase,
		Task:         *selected,
		NextAction:   fmt.Sprintf("Implement the task, run %s, then complete task %s", selected.Verification, selected.ID),
	}, nil
}

func nextRunnableTask(tree spec.TaskTree) *ExecutableTask {
	for _, task := range tree.LeafTasks() {
		if task.Completed {
			continue
		}

		return &ExecutableTask{
			ID:           task.ID,
			Title:        task.Title,
			ParentID:     task.ParentID,
			Completed:    task.Completed,
			Level:        task.Level,
			Requirements: append([]string(nil), task.Requirements...),
			DesignRefs:   append([]string(nil), task.DesignRefs...),
			Verification: task.Verification,
			Proof:        task.Proof,
		}
	}

	return nil
}

// CompleteTask runs the declared proof for a leaf task and marks it complete only if the proof succeeds.
func CompleteTask(ctx context.Context, root, featureName, taskID string, runner shell.Runner) (TaskCompletionResult, error) {
	if runner == nil {
		return TaskCompletionResult{}, fmt.Errorf("proof runner is required")
	}

	startContext, err := StartTask(root, featureName, taskID)
	if err != nil {
		return TaskCompletionResult{}, err
	}

	proofCommands, proofCommand, err := resolveProofCommands(startContext.Task)
	if err != nil {
		return TaskCompletionResult{}, err
	}

	for _, proofStep := range proofCommands {
		response, err := runner.Run(ctx, proofStep.Name, proofStep.Args...)
		if err != nil {
			hint := ""
			if strings.Contains(err.Error(), "executable file not found") {
				hint = " (hint: shell operators require command: [\"sh\", \"-c\", \"...\"])"
			}
			return TaskCompletionResult{}, fmt.Errorf("verification failed for task %q: %w%s", startContext.Task.ID, err, hint)
		}
		if response.ExitCode != proofStep.ExpectExit {
			detail := strings.TrimSpace(response.Stderr)
			if detail == "" {
				detail = strings.TrimSpace(response.Stdout)
			}
			if detail != "" {
				return TaskCompletionResult{}, fmt.Errorf("verification failed for task %q: command %q exited with code %d (expected %d): %s", startContext.Task.ID, proofStep.Display, response.ExitCode, proofStep.ExpectExit, detail)
			}
			return TaskCompletionResult{}, fmt.Errorf("verification failed for task %q: command %q exited with code %d (expected %d)", startContext.Task.ID, proofStep.Display, response.ExitCode, proofStep.ExpectExit)
		}
	}

	feature, err := spec.LoadFeature(root, featureName)
	if err != nil {
		return TaskCompletionResult{}, err
	}

	completedAt := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	updatedTasks, completedTasks, err := spec.MarkTaskCompleteWithCascade(feature.Tasks, startContext.Task.ID, completedAt)
	if err != nil {
		return TaskCompletionResult{}, err
	}
	if err := spec.SaveDocument(updatedTasks); err != nil {
		return TaskCompletionResult{}, err
	}

	feature.Tasks = updatedTasks
	readiness, err := ResolveExecutionReadiness(feature)
	if err != nil {
		return TaskCompletionResult{}, err
	}

	return TaskCompletionResult{
		Feature:        feature.Name,
		CurrentPhase:   startContext.CurrentPhase,
		Task:           startContext.Task,
		CompletedTasks: completedTasks,
		ProofCommand:   proofCommand,
		NextAction:     readiness.NextAction,
	}, nil
}

// CompleteAllTasks completes runnable leaf tasks in plan order until the plan is exhausted or one proof fails.
func CompleteAllTasks(ctx context.Context, root, featureName string, runner shell.Runner) (BatchCompletionResult, error) {
	if runner == nil {
		return BatchCompletionResult{}, fmt.Errorf("proof runner is required")
	}

	completedTasks := []string{}
	completedLeafTasks := []string{}
	autoCompletedParentIDs := []string{}

	for {
		readiness, err := LoadExecutionReadiness(root, featureName)
		if err != nil {
			return BatchCompletionResult{}, err
		}
		if !readiness.Runnable || readiness.NextTask == nil {
			return BatchCompletionResult{
				Feature:                readiness.Feature,
				CurrentPhase:           readiness.CurrentPhase,
				CompletedTasks:         completedTasks,
				CompletedLeafTasks:     completedLeafTasks,
				AutoCompletedParentIDs: autoCompletedParentIDs,
				NextAction:             readiness.NextAction,
			}, nil
		}

		result, err := CompleteTask(ctx, root, featureName, readiness.NextTask.ID, runner)
		if err != nil {
			return BatchCompletionResult{
				Feature:                readiness.Feature,
				CurrentPhase:           readiness.CurrentPhase,
				CompletedTasks:         completedTasks,
				CompletedLeafTasks:     completedLeafTasks,
				AutoCompletedParentIDs: autoCompletedParentIDs,
				FailedTask:             readiness.NextTask.ID,
				Failure:                err.Error(),
				NextAction:             fmt.Sprintf("Fix failing proof for task %s and rerun batch completion", readiness.NextTask.ID),
			}, err
		}

		completedTasks = append(completedTasks, result.CompletedTasks...)
		completedLeafTasks = append(completedLeafTasks, result.Task.ID)
		for _, completedTaskID := range result.CompletedTasks {
			if completedTaskID != result.Task.ID {
				autoCompletedParentIDs = append(autoCompletedParentIDs, completedTaskID)
			}
		}
	}
}

type proofCommand struct {
	Name       string
	Args       []string
	Display    string
	ExpectExit int
}

func resolveProofCommands(task ExecutableTask) ([]proofCommand, string, error) {
	if len(task.Proof.Steps) > 0 {
		commands := make([]proofCommand, 0, len(task.Proof.Steps))
		for _, step := range task.Proof.Steps {
			if len(step.Argv) == 0 {
				return nil, "", fmt.Errorf("verification command is required")
			}
			expectExit := 0
			if step.ExpectExit != nil {
				expectExit = *step.ExpectExit
			}
			commands = append(commands, proofCommand{
				Name:       step.Argv[0],
				Args:       append([]string(nil), step.Argv[1:]...),
				Display:    formatCommandProofStep(step.Argv),
				ExpectExit: expectExit,
			})
		}
		return commands, task.Verification, nil
	}

	name, args, display, err := parseVerificationCommand(task.Verification)
	if err != nil {
		return nil, "", err
	}
	return []proofCommand{{Name: name, Args: args, Display: display}}, display, nil
}

func parseVerificationCommand(verification string) (string, []string, string, error) {
	trimmed := strings.TrimSpace(verification)
	if trimmed == "" {
		return "", nil, "", fmt.Errorf("verification command is required")
	}

	display := trimmed
	trimmed = strings.Trim(trimmed, "`")

	fmt.Fprintf(os.Stderr, "warning: legacy verification format is deprecated; migrate to structured command: [\"...\"] format\n")

	if strings.ContainsAny(trimmed, "'\"|;$") {
		fmt.Fprintf(os.Stderr, "warning: verification %q contains shell operators; consider using structured format: command: [\"sh\", \"-c\", \"...\"]\n", display)
	}

	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return "", nil, "", fmt.Errorf("verification command is required")
	}

	return fields[0], fields[1:], display, nil
}

func formatCommandProofStep(argv []string) string {
	payload, _ := json.Marshal(argv)
	return fmt.Sprintf("command %s", string(payload))
}
