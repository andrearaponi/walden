package spec

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	taskLinePattern     = regexp.MustCompile(`^( *)(- \[([ x])\] )([0-9]+(?:\.[0-9]+)*)\.? (.+?)\s*$`)
	metadataLinePattern = regexp.MustCompile(`^ {4}- (Requirements|Design|Verification):(.*)$`)
	commandStepPattern  = regexp.MustCompile(`^ {6}- (?:command|argv): (\[.+\])\s*$`)
	expectExitPattern   = regexp.MustCompile(`^ {8}expect_exit: ([0-9]+)\s*$`)
	coversPattern       = regexp.MustCompile(`^ {8}covers: (\[.+\])\s*$`)
	backtickRefPattern  = regexp.MustCompile("`([^`]+)`")
)

// VerificationSpec is the normalized proof definition for an executable leaf task.
type VerificationSpec struct {
	LegacyCommand string
	Steps         []VerificationStep
}

// VerificationStep is one structured proof step executed without shell interpolation.
// Uses the Kubernetes command pattern: command: ["executable", "arg1", "arg2"].
type VerificationStep struct {
	Argv       []string
	ExpectExit *int
	Covers     []string
}

// Empty reports whether the verification spec contains any executable proof.
func (spec VerificationSpec) Empty() bool {
	return strings.TrimSpace(spec.LegacyCommand) == "" && len(spec.Steps) == 0
}

// Display renders a compact human-readable view of the proof definition.
func (spec VerificationSpec) Display() string {
	if strings.TrimSpace(spec.LegacyCommand) != "" {
		return spec.LegacyCommand
	}
	if len(spec.Steps) == 0 {
		return ""
	}

	rendered := make([]string, 0, len(spec.Steps))
	for _, step := range spec.Steps {
		payload, _ := json.Marshal(step.Argv)
		rendered = append(rendered, fmt.Sprintf("command %s", string(payload)))
	}
	return strings.Join(rendered, "; ")
}

// Task represents one parsed implementation task from tasks.md.
type Task struct {
	ID           string
	Title        string
	ParentID     string
	Completed    bool
	Level        int
	Requirements []string
	DesignRefs   []string
	Verification string
	Proof        VerificationSpec
	Children     []*Task

	checkboxLine     int
	requirementsLine int
	designLine       int
	verificationLine int
}

// TaskTree is the typed execution view of tasks.md.
type TaskTree struct {
	DocumentPath string
	Tasks        []*Task
	lines        []string
}

// LoadTaskTree loads the tasks document for a feature and parses its task tree.
func LoadTaskTree(root, featureName string) (TaskTree, error) {
	feature, err := LoadFeature(root, featureName)
	if err != nil {
		return TaskTree{}, err
	}

	return ParseTaskTree(feature.Tasks)
}

// ParseTaskTree parses the Markdown task hierarchy from tasks.md.
func ParseTaskTree(document Document) (TaskTree, error) {
	if !document.Exists {
		return TaskTree{}, fmt.Errorf("tasks.md does not exist")
	}

	lines := strings.Split(strings.ReplaceAll(document.Body, "\r\n", "\n"), "\n")
	tree := TaskTree{DocumentPath: document.Path, lines: lines}
	seenIDs := map[string]struct{}{}

	var currentTop *Task
	var currentTask *Task
	var pendingVerificationTask *Task
	foundTask := false

	for index, line := range lines {
		if pendingVerificationTask != nil {
			if match := commandStepPattern.FindStringSubmatch(line); match != nil {
				argv, err := parseVerificationArgv(match[1], index)
				if err != nil {
					return TaskTree{}, err
				}
				pendingVerificationTask.Proof.Steps = append(pendingVerificationTask.Proof.Steps, VerificationStep{Argv: argv})
				pendingVerificationTask.Verification = pendingVerificationTask.Proof.Display()
				continue
			}
			if match := expectExitPattern.FindStringSubmatch(line); match != nil {
				steps := pendingVerificationTask.Proof.Steps
				if len(steps) == 0 {
					return TaskTree{}, fmt.Errorf("line %d: expect_exit declared before any command step for task %q", index+1, pendingVerificationTask.ID)
				}
				exitCode, err := strconv.Atoi(match[1])
				if err != nil {
					return TaskTree{}, fmt.Errorf("line %d: invalid expect_exit value: %w", index+1, err)
				}
				steps[len(steps)-1].ExpectExit = &exitCode
				continue
			}
			if match := coversPattern.FindStringSubmatch(line); match != nil {
				steps := pendingVerificationTask.Proof.Steps
				if len(steps) == 0 {
					return TaskTree{}, fmt.Errorf("line %d: covers declared before any command step for task %q", index+1, pendingVerificationTask.ID)
				}
				covers, err := parseCoversField(match[1], index)
				if err != nil {
					return TaskTree{}, err
				}
				steps[len(steps)-1].Covers = covers
				continue
			}
			if strings.TrimSpace(line) == "" {
				continue
			}
			if len(pendingVerificationTask.Proof.Steps) == 0 {
				return TaskTree{}, fmt.Errorf(
					"line %d: structured Verification for task %q must include at least one command step",
					pendingVerificationTask.verificationLine+1,
					pendingVerificationTask.ID,
				)
			}
			pendingVerificationTask = nil
		}

		if match := taskLinePattern.FindStringSubmatch(line); match != nil {
			task, err := parseTaskLine(match, index)
			if err != nil {
				return TaskTree{}, err
			}
			if _, exists := seenIDs[task.ID]; exists {
				return TaskTree{}, fmt.Errorf("line %d: duplicate task ID %q", index+1, task.ID)
			}
			seenIDs[task.ID] = struct{}{}
			foundTask = true

			switch len(match[1]) {
			case 0:
				if task.Level != 1 {
					return TaskTree{}, fmt.Errorf("line %d: top-level task %q must not include child numbering", index+1, task.ID)
				}
				tree.Tasks = append(tree.Tasks, task)
				currentTop = task
				currentTask = task
			case 2:
				if task.Level != 2 {
					return TaskTree{}, fmt.Errorf("line %d: child task %q must use exactly one parent prefix", index+1, task.ID)
				}
				if currentTop == nil {
					return TaskTree{}, fmt.Errorf("line %d: task %q does not have a parent task", index+1, task.ID)
				}
				if task.ParentID != currentTop.ID {
					return TaskTree{}, fmt.Errorf("line %d: task %q does not belong to parent %q", index+1, task.ID, currentTop.ID)
				}
				currentTop.Children = append(currentTop.Children, task)
				currentTask = task
			default:
				return TaskTree{}, fmt.Errorf("line %d: invalid task indentation", index+1)
			}

			continue
		}

		if match := metadataLinePattern.FindStringSubmatch(line); match != nil {
			if currentTask == nil {
				return TaskTree{}, fmt.Errorf("line %d: task metadata declared before any task", index+1)
			}

			value := strings.TrimSpace(match[2])
			if match[1] != "Verification" && value == "" {
				return TaskTree{}, fmt.Errorf("line %d: empty %s metadata", index+1, match[1])
			}

			switch match[1] {
			case "Requirements":
				currentTask.Requirements = parseRequirementRefs(value)
				currentTask.requirementsLine = index
				if len(currentTask.Requirements) == 0 {
					return TaskTree{}, fmt.Errorf("line %d: invalid Requirements metadata for task %q", index+1, currentTask.ID)
				}
			case "Design":
				currentTask.DesignRefs = parseReferenceList(value)
				currentTask.designLine = index
				if len(currentTask.DesignRefs) == 0 {
					return TaskTree{}, fmt.Errorf("line %d: invalid Design metadata for task %q", index+1, currentTask.ID)
				}
			case "Verification":
				currentTask.Proof = VerificationSpec{}
				currentTask.verificationLine = index
				if value == "" {
					pendingVerificationTask = currentTask
					continue
				}
				currentTask.Proof.LegacyCommand = value
				currentTask.Verification = currentTask.Proof.Display()
			}

			continue
		}

		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "", strings.HasPrefix(trimmed, "#"):
			continue
		case strings.HasPrefix(trimmed, "- ["):
			return TaskTree{}, fmt.Errorf("line %d: invalid task indentation", index+1)
		case strings.HasPrefix(trimmed, "- Requirements:"), strings.HasPrefix(trimmed, "- Design:"), strings.HasPrefix(trimmed, "- Verification:"):
			return TaskTree{}, fmt.Errorf("line %d: invalid metadata indentation", index+1)
		default:
			continue
		}
	}

	if !foundTask {
		return TaskTree{}, fmt.Errorf("tasks.md does not contain any task entries")
	}
	if pendingVerificationTask != nil && len(pendingVerificationTask.Proof.Steps) == 0 {
		return TaskTree{}, fmt.Errorf(
			"line %d: structured Verification for task %q must include at least one command step",
			pendingVerificationTask.verificationLine+1,
			pendingVerificationTask.ID,
		)
	}

	for _, task := range tree.Tasks {
		if err := validateTaskMetadata(task); err != nil {
			return TaskTree{}, err
		}
	}

	return tree, nil
}

// LeafTasks returns all executable leaf tasks in document order.
func (tree TaskTree) LeafTasks() []*Task {
	leafTasks := []*Task{}
	for _, task := range tree.Tasks {
		collectLeafTasks(task, &leafTasks)
	}
	return leafTasks
}

// FindTask resolves a task by ID.
func (tree TaskTree) FindTask(id string) (*Task, bool) {
	for _, task := range tree.Tasks {
		if found, ok := findTask(task, id); ok {
			return found, true
		}
	}

	return nil, false
}

// MarkTaskComplete updates one executable leaf task checkbox and last_modified timestamp.
func MarkTaskComplete(document Document, taskID, lastModified string) (Document, error) {
	updated, _, err := markTaskComplete(document, taskID, lastModified)
	return updated, err
}

// MarkTaskCompleteWithCascade updates a leaf task and returns every task ID closed by the same deterministic mutation.
func MarkTaskCompleteWithCascade(document Document, taskID, lastModified string) (Document, []string, error) {
	return markTaskComplete(document, taskID, lastModified)
}

func markTaskComplete(document Document, taskID, lastModified string) (Document, []string, error) {
	tree, err := ParseTaskTree(document)
	if err != nil {
		return Document{}, nil, err
	}

	task, ok := tree.FindTask(taskID)
	if !ok {
		return Document{}, nil, fmt.Errorf("task %q does not exist", taskID)
	}
	if len(task.Children) > 0 {
		return Document{}, nil, fmt.Errorf("task %q is not an executable leaf task", taskID)
	}
	if task.Completed {
		return Document{}, nil, fmt.Errorf("task %q is already completed", taskID)
	}

	if task.checkboxLine < 0 || task.checkboxLine >= len(tree.lines) {
		return Document{}, nil, fmt.Errorf("task %q checkbox line is out of range", taskID)
	}
	tree.lines[task.checkboxLine] = strings.Replace(tree.lines[task.checkboxLine], "[ ]", "[x]", 1)
	task.Completed = true

	completedTasks := []string{task.ID}
	parentID := task.ParentID
	for parentID != "" {
		parent, ok := tree.FindTask(parentID)
		if !ok {
			break
		}
		if !allChildrenCompleted(parent) {
			break
		}
		if !parent.Completed {
			if parent.checkboxLine < 0 || parent.checkboxLine >= len(tree.lines) {
				return Document{}, nil, fmt.Errorf("task %q checkbox line is out of range", parent.ID)
			}
			tree.lines[parent.checkboxLine] = strings.Replace(tree.lines[parent.checkboxLine], "[ ]", "[x]", 1)
			parent.Completed = true
			completedTasks = append(completedTasks, parent.ID)
		}
		parentID = parent.ParentID
	}

	document.Body = strings.Join(tree.lines, "\n")
	document.LastModified = lastModified
	if document.Fields == nil {
		document.Fields = map[string]string{}
	}
	document.Fields["last_modified"] = lastModified

	return document, completedTasks, nil
}

func parseTaskLine(match []string, index int) (*Task, error) {
	id := match[4]
	parts := strings.Split(id, ".")
	if len(parts) > 2 {
		return nil, fmt.Errorf("line %d: task %q exceeds the supported two-level hierarchy", index+1, id)
	}

	task := &Task{
		ID:           id,
		Title:        strings.TrimSpace(match[5]),
		Completed:    strings.TrimSpace(match[3]) == "x",
		Level:        len(parts),
		checkboxLine: index,
	}
	if task.Title == "" {
		return nil, fmt.Errorf("line %d: task %q is missing a title", index+1, id)
	}
	if task.Level == 2 {
		task.ParentID = parts[0]
	}

	return task, nil
}

func validateTaskMetadata(task *Task) error {
	if len(task.Children) > 0 {
		if len(task.Requirements) > 0 || len(task.DesignRefs) > 0 || task.Verification != "" {
			return fmt.Errorf("non-leaf task %q cannot declare executable metadata", task.ID)
		}
		for _, child := range task.Children {
			if err := validateTaskMetadata(child); err != nil {
				return err
			}
		}
		return nil
	}

	if len(task.Requirements) == 0 {
		return fmt.Errorf("task %q is missing Requirements metadata", task.ID)
	}
	if len(task.DesignRefs) == 0 {
		return fmt.Errorf("task %q is missing Design metadata", task.ID)
	}
	if task.Proof.Empty() {
		return fmt.Errorf("task %q is missing Verification metadata", task.ID)
	}

	return nil
}

func collectLeafTasks(task *Task, leafTasks *[]*Task) {
	if len(task.Children) == 0 {
		*leafTasks = append(*leafTasks, task)
		return
	}

	for _, child := range task.Children {
		collectLeafTasks(child, leafTasks)
	}
}

func findTask(task *Task, id string) (*Task, bool) {
	if task.ID == id {
		return task, true
	}
	for _, child := range task.Children {
		if found, ok := findTask(child, id); ok {
			return found, true
		}
	}
	return nil, false
}

func allChildrenCompleted(task *Task) bool {
	if len(task.Children) == 0 {
		return task.Completed
	}
	for _, child := range task.Children {
		if !child.Completed {
			return false
		}
	}
	return true
}

func parseRequirementRefs(value string) []string {
	matches := backtickRefPattern.FindAllStringSubmatch(value, -1)
	if len(matches) > 0 {
		references := make([]string, 0, len(matches))
		for _, match := range matches {
			reference := strings.TrimSpace(match[1])
			if reference != "" {
				references = append(references, reference)
			}
		}
		return references
	}

	return parseReferenceList(value)
}

func parseReferenceList(value string) []string {
	parts := strings.Split(value, ",")
	references := make([]string, 0, len(parts))
	for _, part := range parts {
		reference := strings.Trim(strings.TrimSpace(part), "`")
		if reference != "" {
			references = append(references, reference)
		}
	}
	return references
}

func parseCoversField(raw string, index int) ([]string, error) {
	var covers []string
	if err := json.Unmarshal([]byte(raw), &covers); err != nil {
		return nil, fmt.Errorf("line %d: invalid covers field: %w", index+1, err)
	}
	if len(covers) == 0 {
		return nil, fmt.Errorf("line %d: covers field must include at least one AC ID", index+1)
	}
	return covers, nil
}

func parseVerificationArgv(raw string, index int) ([]string, error) {
	var argv []string
	if err := json.Unmarshal([]byte(raw), &argv); err != nil {
		return nil, fmt.Errorf("line %d: invalid argv verification step: %w", index+1, err)
	}
	if len(argv) == 0 {
		return nil, fmt.Errorf("line %d: argv verification step must include at least one argument", index+1)
	}
	for _, value := range argv {
		if strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("line %d: argv verification step cannot contain empty arguments", index+1)
		}
	}
	return argv, nil
}
