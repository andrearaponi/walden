package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/andrearaponi/walden/internal/output"
	"github.com/andrearaponi/walden/internal/repo"
	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/spec"
	"github.com/andrearaponi/walden/internal/validation"
	"github.com/andrearaponi/walden/internal/workflow"
)

const binaryName = "walden"

// Version is the build version of the CLI binary. It is set at build time via
// -ldflags and defaults to "dev" for untagged builds.
var Version = "dev"

var plannedCommands = []string{
	"version [--json]",
	"repo init [--json]",
	"feature init <name> [--json]",
	"status <feature> [--json]",
	"reconcile <feature> [--json]",
	"lesson log --feature <name> --phase requirements|design|tasks|execute|release --trigger <text> --lesson <text> --guardrail <text> [--json]",
	"task status <feature> [--json]",
	"task start <feature> [task-id] [--json]",
	"task complete <feature> <task-id> [--json]",
	"task complete-all <feature> [--json]",
	"validate <feature> [--all] [--json]",
	"review open <feature> --phase requirements|design|tasks [--json]",
	"review approve <feature> --phase requirements|design|tasks [--json]",
}

var commandRunner shell.Runner = shell.NewExecRunner()

// Run executes the root CLI flow for the current argument list.
func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stdout)
		return 0
	}

	switch args[0] {
	case "version":
		return runVersion(args[1:], stdout, stderr)
	case "repo":
		return runRepo(args[1:], stdout, stderr)
	case "feature":
		return runFeature(args[1:], stdout, stderr)
	case "status":
		return runStatus(args[1:], stdout, stderr)
	case "reconcile":
		return runReconcile(args[1:], stdout, stderr)
	case "lesson":
		return runLesson(args[1:], stdout, stderr)
	case "task":
		return runTask(args[1:], stdout, stderr)
	case "validate":
		return runValidate(args[1:], stdout, stderr)
	case "review":
		return runReview(args[1:], stdout, stderr)
	}

	_, _ = fmt.Fprintf(stderr, "unknown command: %s\n\n", strings.Join(args, " "))
	printUsage(stderr)
	return 1
}

func runVersion(args []string, stdout io.Writer, stderr io.Writer) int {
	jsonMode := false
	for _, arg := range args {
		if arg == "--json" {
			jsonMode = true
		}
	}

	result := output.Result{
		Summary:  fmt.Sprintf("walden %s (schema %s)", Version, "v0alpha1"),
		ExitCode: 0,
	}

	if jsonMode {
		if err := output.PrintJSON(stdout, "version", result); err != nil {
			_, _ = fmt.Fprintf(stderr, "render json output: %v\n", err)
			return 1
		}
		return 0
	}

	output.PrintText(stdout, result)
	return 0
}

func runRepo(args []string, stdout io.Writer, stderr io.Writer) int {
	jsonMode := false
	positional := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--json" {
			jsonMode = true
			continue
		}
		positional = append(positional, arg)
	}

	if len(positional) == 1 && positional[0] == "init" {
		root, err := os.Getwd()
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "resolve working directory: %v\n", err)
			return 1
		}

		report, err := repo.Init(root)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "%v\n", err)
			return 1
		}

		result := output.Result{
			Summary:               "repository initialized",
			CreatedFiles:          report.CreatedFiles,
			UpdatedFiles:          report.UpdatedFiles,
			SkippedFiles:          report.SkippedFiles,
			GitInitialized:        report.GitInitialized,
			GitAlreadyInitialized: report.GitAlreadyInitialized,
			NextAction:            "Run walden feature init <name>",
			ExitCode:              0,
		}

		if jsonMode {
			if err := output.PrintJSON(stdout, "repo-init", result); err != nil {
				_, _ = fmt.Fprintf(stderr, "render json output: %v\n", err)
				return 1
			}
			return 0
		}

		output.PrintText(stdout, result)
		return 0
	}

	_, _ = fmt.Fprintf(stderr, "unknown command: repo %s\n\n", strings.Join(positional, " "))
	printUsage(stderr)
	return 1
}

func runFeature(args []string, stdout io.Writer, stderr io.Writer) int {
	jsonMode := false
	positional := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--json" {
			jsonMode = true
			continue
		}
		positional = append(positional, arg)
	}

	if len(positional) >= 2 && positional[0] == "init" {
		root, err := os.Getwd()
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "resolve working directory: %v\n", err)
			return 1
		}

		report, err := repo.InitFeature(root, strings.Join(positional[1:], " "))
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "%v\n", err)
			return 1
		}

		summary := fmt.Sprintf("feature scaffold initialized for %s", report.FeatureName)
		warnings := []string{}
		if report.AlreadyExists {
			summary = fmt.Sprintf("feature scaffold already exists for %s", report.FeatureName)
			warnings = append(warnings, "feature already exists; existing files were preserved")
		}

		result := output.Result{
			Summary:      summary,
			CreatedFiles: report.CreatedFiles,
			SkippedFiles: report.SkippedFiles,
			CurrentPhase: report.CurrentPhase,
			NextAction:   fmt.Sprintf("Edit .walden/specs/%s/requirements.md and move it to in-review", report.FeatureName),
			Warnings:     warnings,
			ExitCode:     0,
		}

		if jsonMode {
			if err := output.PrintJSON(stdout, "feature-init", result); err != nil {
				_, _ = fmt.Fprintf(stderr, "render json output: %v\n", err)
				return 1
			}
			return 0
		}

		output.PrintText(stdout, result)
		return 0
	}

	_, _ = fmt.Fprintf(stderr, "unknown command: feature %s\n\n", strings.Join(positional, " "))
	printUsage(stderr)
	return 1
}

func runValidate(args []string, stdout io.Writer, stderr io.Writer) int {
	jsonMode := false
	fullSpecMode := false
	positional := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--json" {
			jsonMode = true
			continue
		}
		if arg == "--all" {
			fullSpecMode = true
			continue
		}
		positional = append(positional, arg)
	}

	if len(positional) != 1 {
		_, _ = fmt.Fprintf(stderr, "unknown command: validate %s\n\n", strings.Join(args, " "))
		printUsage(stderr)
		return 1
	}

	root, err := os.Getwd()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "resolve working directory: %v\n", err)
		return 1
	}

	scope := validation.ScopeCurrentPhase
	if fullSpecMode {
		scope = validation.ScopeFullSpec
	}

	result, err := validation.ValidateFeatureWithScope(root, positional[0], scope)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "validate feature: %v\n", err)
		return 1
	}

	exitCode := 0
	if !result.Valid {
		exitCode = 1
	}

	var earsResults []output.EARSCriterion
	for _, c := range result.EARSResults {
		earsResults = append(earsResults, output.EARSCriterion{
			ID:       c.ID,
			Form:     c.Form,
			Valid:    c.Valid,
			Errors:   c.Errors,
			Warnings: c.Warnings,
		})
	}

	var coverageReport *output.CoverageReport
	if result.Coverage != nil {
		coverageReport = &output.CoverageReport{
			TaskReferenceCoverage: output.CoverageStatus{
				Complete: result.Coverage.TaskReferenceCoverage.Complete,
				Missing:  result.Coverage.TaskReferenceCoverage.Missing,
			},
			ProofReferenceCoverage: output.CoverageStatus{
				Complete: result.Coverage.ProofReferenceCoverage.Complete,
				Missing:  result.Coverage.ProofReferenceCoverage.Missing,
			},
		}
	}

	var earsDist *output.EARSDistribution
	if result.EARSDistribution != nil {
		earsDist = &output.EARSDistribution{
			Ubiquitous:  result.EARSDistribution.Ubiquitous,
			EventDriven: result.EARSDistribution.EventDriven,
			StateDriven: result.EARSDistribution.StateDriven,
			Optional:    result.EARSDistribution.Optional,
			Unwanted:    result.EARSDistribution.Unwanted,
			Complex:     result.EARSDistribution.Complex,
			Total:       result.EARSDistribution.Total,
		}
	}

	outputResult := output.Result{
		Summary:          result.Message,
		ValidatedPhases:  result.ValidatedPhases,
		SkippedPhases:    result.SkippedPhases,
		Warnings:         result.Warnings,
		EARSValidation:   earsResults,
		Coverage:         coverageReport,
		EARSDistribution: earsDist,
		ExitCode:         exitCode,
	}

	if jsonMode {
		if err := output.PrintJSON(stdout, "validate", outputResult); err != nil {
			_, _ = fmt.Fprintf(stderr, "render json output: %v\n", err)
			return 1
		}
		return exitCode
	}

	if result.Valid {
		output.PrintText(stdout, outputResult)
		return 0
	}

	output.PrintText(stderr, outputResult)
	return 1
}

func runStatus(args []string, stdout io.Writer, stderr io.Writer) int {
	jsonMode := false
	positional := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--json" {
			jsonMode = true
			continue
		}
		positional = append(positional, arg)
	}

	if len(positional) != 1 {
		_, _ = fmt.Fprintf(stderr, "unknown command: status %s\n\n", strings.Join(args, " "))
		printUsage(stderr)
		return 1
	}

	featureName, err := spec.NormalizeFeatureName(positional[0])
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	root, err := os.Getwd()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "resolve working directory: %v\n", err)
		return 1
	}

	state, err := workflow.LoadFeatureState(root, featureName)
	if err != nil {
		result := statusErrorResult(featureName, err)
		if jsonMode {
			if err := output.PrintJSON(stdout, "status", result); err != nil {
				_, _ = fmt.Fprintf(stderr, "render json output: %v\n", err)
				return 1
			}
			return result.ExitCode
		}

		output.PrintText(stderr, result)
		return result.ExitCode
	}

	result := statusSuccessResult(state)
	if jsonMode {
		if err := output.PrintJSON(stdout, "status", result); err != nil {
			_, _ = fmt.Fprintf(stderr, "render json output: %v\n", err)
			return 1
		}
		return 0
	}

	output.PrintText(stdout, result)
	return 0
}

func runReconcile(args []string, stdout io.Writer, stderr io.Writer) int {
	jsonMode := false
	positional := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--json" {
			jsonMode = true
			continue
		}
		positional = append(positional, arg)
	}

	if len(positional) != 1 {
		_, _ = fmt.Fprintf(stderr, "unknown command: reconcile %s\n\n", strings.Join(args, " "))
		printUsage(stderr)
		return 1
	}

	featureName, err := spec.NormalizeFeatureName(positional[0])
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	root, err := os.Getwd()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "resolve working directory: %v\n", err)
		return 1
	}

	reconcileResult, err := workflow.ReconcileFeature(root, featureName)
	if err != nil {
		result := reconcileErrorResult(featureName, err)
		if jsonMode {
			if err := output.PrintJSON(stdout, "reconcile", result); err != nil {
				_, _ = fmt.Fprintf(stderr, "render json output: %v\n", err)
				return 1
			}
			return result.ExitCode
		}

		output.PrintText(stderr, result)
		return result.ExitCode
	}

	result := reconcileSuccessResult(reconcileResult)
	if jsonMode {
		if err := output.PrintJSON(stdout, "reconcile", result); err != nil {
			_, _ = fmt.Fprintf(stderr, "render json output: %v\n", err)
			return 1
		}
		return 0
	}

	output.PrintText(stdout, result)
	return 0
}

func runReview(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "open" {
		return runReviewOpen(args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "approve" {
		return runReviewApprove(args[1:], stdout, stderr)
	}

	_, _ = fmt.Fprintf(stderr, "unknown command: review %s\n\n", strings.Join(args, " "))
	printUsage(stderr)
	return 1
}

func runLesson(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "log" {
		return runLessonLog(args[1:], stdout, stderr)
	}

	_, _ = fmt.Fprintf(stderr, "unknown command: lesson %s\n\n", strings.Join(args, " "))
	printUsage(stderr)
	return 1
}

func runTask(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "status" {
		return runTaskStatus(args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "start" {
		return runTaskStart(args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "complete" {
		return runTaskComplete(args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "complete-all" {
		return runTaskCompleteAll(args[1:], stdout, stderr)
	}

	_, _ = fmt.Fprintf(stderr, "unknown command: task %s\n\n", strings.Join(args, " "))
	printUsage(stderr)
	return 1
}

func runTaskStatus(args []string, stdout io.Writer, stderr io.Writer) int {
	jsonMode := false
	positional := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--json" {
			jsonMode = true
			continue
		}
		positional = append(positional, arg)
	}

	if len(positional) != 1 {
		_, _ = fmt.Fprintf(stderr, "unknown command: task status %s\n\n", strings.Join(args, " "))
		printUsage(stderr)
		return 1
	}

	featureName, err := spec.NormalizeFeatureName(positional[0])
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	root, err := os.Getwd()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "resolve working directory: %v\n", err)
		return 1
	}

	readiness, err := workflow.LoadExecutionReadiness(root, featureName)
	if err != nil {
		result := taskStatusErrorResult(featureName, err)
		if jsonMode {
			if err := output.PrintJSON(stdout, "task-status", result); err != nil {
				_, _ = fmt.Fprintf(stderr, "render json output: %v\n", err)
				return 1
			}
			return result.ExitCode
		}

		output.PrintText(stderr, result)
		return result.ExitCode
	}

	result := taskStatusSuccessResult(readiness)
	if jsonMode {
		if err := output.PrintJSON(stdout, "task-status", result); err != nil {
			_, _ = fmt.Fprintf(stderr, "render json output: %v\n", err)
			return 1
		}
		return 0
	}

	output.PrintText(stdout, result)
	return 0
}

func runLessonLog(args []string, stdout io.Writer, stderr io.Writer) int {
	jsonMode := false
	values := map[string]string{}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--json" {
			jsonMode = true
			continue
		}
		if !strings.HasPrefix(arg, "--") || index+1 >= len(args) {
			_, _ = fmt.Fprintf(stderr, "unknown command: lesson log %s\n\n", strings.Join(args, " "))
			printUsage(stderr)
			return 1
		}

		values[strings.TrimPrefix(arg, "--")] = args[index+1]
		index++
	}

	for _, required := range []string{"feature", "phase", "trigger", "lesson", "guardrail"} {
		if strings.TrimSpace(values[required]) == "" {
			_, _ = fmt.Fprintf(stderr, "lesson log requires --feature, --phase, --trigger, --lesson, and --guardrail\n")
			return 1
		}
	}

	root, err := os.Getwd()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "resolve working directory: %v\n", err)
		return 1
	}

	entry, lessonsPath, err := spec.AppendLesson(root, spec.LessonEntry{
		Feature:   values["feature"],
		Phase:     values["phase"],
		Trigger:   values["trigger"],
		Lesson:    values["lesson"],
		Guardrail: values["guardrail"],
	})
	if err != nil {
		result := lessonLogErrorResult(err)
		if jsonMode {
			if err := output.PrintJSON(stdout, "lesson-log", result); err != nil {
				_, _ = fmt.Fprintf(stderr, "render json output: %v\n", err)
				return 1
			}
			return result.ExitCode
		}

		output.PrintText(stderr, result)
		return result.ExitCode
	}

	result := lessonLogSuccessResult(entry.Feature, lessonsPath)
	if jsonMode {
		if err := output.PrintJSON(stdout, "lesson-log", result); err != nil {
			_, _ = fmt.Fprintf(stderr, "render json output: %v\n", err)
			return 1
		}
		return 0
	}

	output.PrintText(stdout, result)
	return 0
}

func runTaskStart(args []string, stdout io.Writer, stderr io.Writer) int {
	jsonMode := false
	positional := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--json" {
			jsonMode = true
			continue
		}
		positional = append(positional, arg)
	}

	if len(positional) < 1 || len(positional) > 2 {
		_, _ = fmt.Fprintf(stderr, "unknown command: task start %s\n\n", strings.Join(args, " "))
		printUsage(stderr)
		return 1
	}

	featureName, err := spec.NormalizeFeatureName(positional[0])
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	taskID := ""
	if len(positional) == 2 {
		taskID = positional[1]
	}

	root, err := os.Getwd()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "resolve working directory: %v\n", err)
		return 1
	}

	context, err := workflow.StartTask(root, featureName, taskID)
	if err != nil {
		result := taskStartErrorResult(featureName, err)
		if jsonMode {
			if err := output.PrintJSON(stdout, "task-start", result); err != nil {
				_, _ = fmt.Fprintf(stderr, "render json output: %v\n", err)
				return 1
			}
			return result.ExitCode
		}
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	result := taskStartSuccessResult(context)
	if jsonMode {
		if err := output.PrintJSON(stdout, "task-start", result); err != nil {
			_, _ = fmt.Fprintf(stderr, "render json output: %v\n", err)
			return 1
		}
		return 0
	}

	output.PrintText(stdout, result)
	return 0
}

func runTaskComplete(args []string, stdout io.Writer, stderr io.Writer) int {
	jsonMode := false
	positional := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--json" {
			jsonMode = true
			continue
		}
		positional = append(positional, arg)
	}

	if len(positional) != 2 {
		_, _ = fmt.Fprintf(stderr, "unknown command: task complete %s\n\n", strings.Join(args, " "))
		printUsage(stderr)
		return 1
	}

	featureName, err := spec.NormalizeFeatureName(positional[0])
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	root, err := os.Getwd()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "resolve working directory: %v\n", err)
		return 1
	}

	resultData, err := workflow.CompleteTask(context.Background(), root, featureName, positional[1], commandRunner)
	if err != nil {
		result := taskCompleteErrorResult(featureName, err)
		if jsonMode {
			if err := output.PrintJSON(stdout, "task-complete", result); err != nil {
				_, _ = fmt.Fprintf(stderr, "render json output: %v\n", err)
				return 1
			}
			return result.ExitCode
		}
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	result := taskCompleteSuccessResult(resultData)
	if jsonMode {
		if err := output.PrintJSON(stdout, "task-complete", result); err != nil {
			_, _ = fmt.Fprintf(stderr, "render json output: %v\n", err)
			return 1
		}
		return 0
	}

	output.PrintText(stdout, result)
	return 0
}

func runTaskCompleteAll(args []string, stdout io.Writer, stderr io.Writer) int {
	jsonMode := false
	positional := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--json" {
			jsonMode = true
			continue
		}
		positional = append(positional, arg)
	}

	if len(positional) != 1 {
		_, _ = fmt.Fprintf(stderr, "unknown command: task complete-all %s\n\n", strings.Join(args, " "))
		printUsage(stderr)
		return 1
	}

	featureName, err := spec.NormalizeFeatureName(positional[0])
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	root, err := os.Getwd()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "resolve working directory: %v\n", err)
		return 1
	}

	resultData, err := workflow.CompleteAllTasks(context.Background(), root, featureName, commandRunner)
	if err != nil {
		result := taskCompleteAllErrorResult(featureName, resultData, err)
		if jsonMode {
			if err := output.PrintJSON(stdout, "task-complete-all", result); err != nil {
				_, _ = fmt.Fprintf(stderr, "render json output: %v\n", err)
				return 1
			}
			return result.ExitCode
		}

		output.PrintText(stderr, result)
		return result.ExitCode
	}

	result := taskCompleteAllSuccessResult(resultData)
	if jsonMode {
		if err := output.PrintJSON(stdout, "task-complete-all", result); err != nil {
			_, _ = fmt.Fprintf(stderr, "render json output: %v\n", err)
			return 1
		}
		return 0
	}

	output.PrintText(stdout, result)
	return 0
}

func runReviewOpen(args []string, stdout io.Writer, stderr io.Writer) int {
	jsonMode := false
	positional := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--json" {
			jsonMode = true
			continue
		}
		positional = append(positional, arg)
	}

	if len(positional) < 3 {
		_, _ = fmt.Fprintf(stderr, "unknown command: review open %s\n\n", strings.Join(positional, " "))
		printUsage(stderr)
		return 1
	}

	featureName := positional[0]
	phaseName := ""
	for index := 1; index < len(positional); index++ {
		if positional[index] == "--phase" && index+1 < len(positional) {
			phaseName = positional[index+1]
			index++
		}
	}

	if phaseName == "" {
		_, _ = fmt.Fprintf(stderr, "review open requires --phase requirements|design|tasks\n")
		return 1
	}

	phase, err := workflow.ParsePhase(phaseName)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	root, err := os.Getwd()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "resolve working directory: %v\n", err)
		return 1
	}

	context, err := workflow.OpenReview(root, featureName, phase)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	result := output.Result{
		Summary:    fmt.Sprintf("review gate opened for %s.md", phase),
		BranchName: context.BranchName,
		Document:   context.Document,
		NextAction: fmt.Sprintf("Use branch %s to review %s", context.BranchName, context.Document),
		ExitCode:   0,
	}

	if jsonMode {
		if err := output.PrintJSON(stdout, "review-open", result); err != nil {
			_, _ = fmt.Fprintf(stderr, "render json output: %v\n", err)
			return 1
		}
		return 0
	}

	output.PrintText(stdout, result)
	return 0
}

func runReviewApprove(args []string, stdout io.Writer, stderr io.Writer) int {
	jsonMode := false
	positional := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--json" {
			jsonMode = true
			continue
		}
		positional = append(positional, arg)
	}

	if len(positional) < 3 {
		_, _ = fmt.Fprintf(stderr, "unknown command: review approve %s\n\n", strings.Join(positional, " "))
		printUsage(stderr)
		return 1
	}

	featureName := positional[0]
	phaseName := ""
	for index := 1; index < len(positional); index++ {
		if positional[index] == "--phase" && index+1 < len(positional) {
			phaseName = positional[index+1]
			index++
		}
	}

	if phaseName == "" {
		_, _ = fmt.Fprintf(stderr, "review approve requires --phase requirements|design|tasks\n")
		return 1
	}

	phase, err := workflow.ParsePhase(phaseName)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	root, err := os.Getwd()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "resolve working directory: %v\n", err)
		return 1
	}

	approveResult, err := workflow.ApproveReview(root, featureName, phase)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	result := output.Result{
		Summary:      fmt.Sprintf("review gate approved for %s.md", phase),
		Document:     approveResult.Document,
		CurrentPhase: string(approveResult.CurrentPhase),
		NextAction:   approveResult.NextAction,
		ExitCode:     0,
	}

	if jsonMode {
		if err := output.PrintJSON(stdout, "review-approve", result); err != nil {
			_, _ = fmt.Fprintf(stderr, "render json output: %v\n", err)
			return 1
		}
		return 0
	}

	output.PrintText(stdout, result)
	return 0
}

func printUsage(w io.Writer) {
	_, _ = fmt.Fprintf(w, "%s\n\n", binaryName)
	_, _ = fmt.Fprintln(w, "Planned commands:")
	for _, command := range plannedCommands {
		_, _ = fmt.Fprintf(w, "- %s\n", command)
	}
}

func statusSuccessResult(state workflow.FeatureState) output.Result {
	warnings := []string{}
	if state.IsStale {
		warnings = append(warnings, "feature is stale relative to upstream approvals")
	}

	return output.Result{
		Summary:      fmt.Sprintf("workflow status for %s", state.Name),
		CurrentPhase: string(state.CurrentPhase),
		Documents: []output.DocumentStatus{
			toOutputDocumentStatus("requirements.md", state.Requirements),
			toOutputDocumentStatus("design.md", state.Design),
			toOutputDocumentStatus("tasks.md", state.Tasks),
		},
		Blockers:   state.Blockers,
		NextAction: state.NextAction,
		Warnings:   warnings,
		ExitCode:   0,
	}
}

func statusErrorResult(featureName string, err error) output.Result {
	result := output.Result{
		Summary:  err.Error(),
		ExitCode: 1,
	}

	if strings.Contains(err.Error(), "does not exist") {
		result.Summary = fmt.Sprintf("%s; run `walden feature init %s` to initialize it", err.Error(), featureName)
		result.NextAction = fmt.Sprintf("Run walden feature init %s", featureName)
	}

	return result
}

func toOutputDocumentStatus(name string, document workflow.DocumentState) output.DocumentStatus {
	status := document.Status
	if !document.Exists {
		status = "missing"
	}

	return output.DocumentStatus{
		Name:       name,
		Status:     status,
		Fresh:      document.Exists && document.Fresh,
		ApprovedAt: document.ApprovedAt,
	}
}

func taskStatusSuccessResult(readiness workflow.ExecutionReadiness) output.Result {
	result := output.Result{
		Summary:      fmt.Sprintf("execution readiness for %s", readiness.Feature),
		CurrentPhase: string(readiness.CurrentPhase),
		Blockers:     readiness.Blockers,
		NextAction:   readiness.NextAction,
		ExitCode:     0,
	}

	if readiness.NextTask != nil {
		result.Task = toOutputTaskStatus(*readiness.NextTask)
	}

	return result
}

func taskStatusErrorResult(featureName string, err error) output.Result {
	result := output.Result{
		Summary:  err.Error(),
		ExitCode: 1,
	}

	if strings.Contains(err.Error(), "does not exist") {
		result.Summary = fmt.Sprintf("%s; run `walden feature init %s` to initialize it", err.Error(), featureName)
		result.NextAction = fmt.Sprintf("Run walden feature init %s", featureName)
	}

	return result
}

func reconcileSuccessResult(result workflow.ReconcileResult) output.Result {
	summary := fmt.Sprintf("reconciliation completed for %s", result.Feature)
	changedFiles := make([]string, 0, len(result.ChangedDocs))
	for _, name := range result.ChangedDocs {
		changedFiles = append(changedFiles, ".walden/specs/"+result.Feature+"/"+name)
	}
	if len(changedFiles) == 0 {
		summary = fmt.Sprintf("workflow state already normalized for %s", result.Feature)
	}

	return output.Result{
		Summary:      summary,
		ChangedFiles: changedFiles,
		CurrentPhase: string(result.CurrentPhase),
		NextAction:   result.NextAction,
		ExitCode:     0,
	}
}

func reconcileErrorResult(featureName string, err error) output.Result {
	result := output.Result{
		Summary:  err.Error(),
		ExitCode: 1,
	}

	if strings.Contains(err.Error(), "does not exist") && strings.Contains(err.Error(), "feature") {
		result.Summary = fmt.Sprintf("%s; run `walden feature init %s` to initialize it", err.Error(), featureName)
		result.NextAction = fmt.Sprintf("Run walden feature init %s", featureName)
	}

	return result
}

func lessonLogSuccessResult(featureName, lessonsPath string) output.Result {
	relativePath := filepathToSlashIfPossible(lessonsPath)
	return output.Result{
		Summary:      fmt.Sprintf("lesson logged for %s", featureName),
		ChangedFiles: []string{relativePath},
		NextAction:   "Review .walden/lessons.md before similar future work",
		ExitCode:     0,
	}
}

func lessonLogErrorResult(err error) output.Result {
	return output.Result{
		Summary:  err.Error(),
		ExitCode: 1,
	}
}

func taskStartSuccessResult(context workflow.TaskStartContext) output.Result {
	return output.Result{
		Summary:      fmt.Sprintf("task start context for %s", context.Feature),
		CurrentPhase: string(context.CurrentPhase),
		Task:         toOutputTaskStatus(context.Task),
		NextAction:   context.NextAction,
		ExitCode:     0,
	}
}

func taskStartErrorResult(featureName string, err error) output.Result {
	result := output.Result{
		Summary:  err.Error(),
		ExitCode: 1,
	}

	if strings.Contains(err.Error(), "does not exist") && strings.Contains(err.Error(), "feature") {
		result.Summary = fmt.Sprintf("%s; run `walden feature init %s` to initialize it", err.Error(), featureName)
		result.NextAction = fmt.Sprintf("Run walden feature init %s", featureName)
	}

	return result
}

func toOutputTaskStatus(task workflow.ExecutableTask) *output.TaskStatus {
	return &output.TaskStatus{
		ID:           task.ID,
		Title:        task.Title,
		ParentID:     task.ParentID,
		Requirements: append([]string(nil), task.Requirements...),
		DesignRefs:   append([]string(nil), task.DesignRefs...),
		Verification: task.Verification,
	}
}

func taskCompleteSuccessResult(result workflow.TaskCompletionResult) output.Result {
	return output.Result{
		Summary:      fmt.Sprintf("task completed for %s", result.Feature),
		CurrentPhase: string(result.CurrentPhase),
		Task:         toOutputTaskStatus(result.Task),
		ChangedFiles: []string{".walden/specs/" + result.Feature + "/tasks.md"},
		NextAction:   result.NextAction,
		ExitCode:     0,
	}
}

func taskCompleteAllSuccessResult(result workflow.BatchCompletionResult) output.Result {
	outputResult := output.Result{
		Summary:        fmt.Sprintf("batch task completion finished for %s", result.Feature),
		CurrentPhase:   string(result.CurrentPhase),
		CompletedTasks: append([]string(nil), result.CompletedLeafTasks...),
		AutoCompleted:  append([]string(nil), result.AutoCompletedParentIDs...),
		NextAction:     result.NextAction,
		ExitCode:       0,
	}
	if len(result.CompletedTasks) > 0 {
		outputResult.ChangedFiles = []string{".walden/specs/" + result.Feature + "/tasks.md"}
	}
	if len(result.CompletedLeafTasks) == 0 {
		outputResult.Summary = fmt.Sprintf("no runnable tasks remained for %s", result.Feature)
	}
	return outputResult
}

func taskCompleteErrorResult(featureName string, err error) output.Result {
	result := output.Result{
		Summary:  err.Error(),
		ExitCode: 1,
	}

	if strings.Contains(err.Error(), "does not exist") && strings.Contains(err.Error(), "feature") {
		result.Summary = fmt.Sprintf("%s; run `walden feature init %s` to initialize it", err.Error(), featureName)
		result.NextAction = fmt.Sprintf("Run walden feature init %s", featureName)
	}

	return result
}

func taskCompleteAllErrorResult(featureName string, batch workflow.BatchCompletionResult, err error) output.Result {
	result := output.Result{
		Summary:        err.Error(),
		CurrentPhase:   string(batch.CurrentPhase),
		CompletedTasks: append([]string(nil), batch.CompletedLeafTasks...),
		AutoCompleted:  append([]string(nil), batch.AutoCompletedParentIDs...),
		NextAction:     batch.NextAction,
		ExitCode:       1,
	}
	if len(batch.CompletedTasks) > 0 {
		result.ChangedFiles = []string{".walden/specs/" + batch.Feature + "/tasks.md"}
	}
	if strings.Contains(err.Error(), "does not exist") && strings.Contains(err.Error(), "feature") {
		result.Summary = fmt.Sprintf("%s; run `walden feature init %s` to initialize it", err.Error(), featureName)
		result.NextAction = fmt.Sprintf("Run walden feature init %s", featureName)
	}
	return result
}

func filepathToSlashIfPossible(path string) string {
	wd, err := os.Getwd()
	if err != nil {
		return filepath.ToSlash(path)
	}

	relative, err := filepath.Rel(wd, path)
	if err != nil {
		return filepath.ToSlash(path)
	}

	return filepath.ToSlash(relative)
}
