package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
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

// commandUsage pairs a command's invocation syntax with a one-line summary
// for the help output.
type commandUsage struct {
	Syntax  string
	Summary string
}

// commandUsages is the flat usage table for the global help, derived from
// commandRegistry so global and per-command help share one source of truth.
var commandUsages = buildCommandUsages()

var commandRunner shell.Runner = shell.NewExecRunner()

// Run executes the root CLI flow for the current argument list.
func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stdout)
		return 0
	}

	switch args[0] {
	case "--help", "-h":
		printUsage(stdout)
		return 0
	case "version":
		return runVersion(args[1:], stdout, stderr)
	case "update":
		return runUpdate(args[1:], stdout, stderr)
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
	case "verify":
		return runVerify(args[1:], stdout, stderr)
	case "evidence":
		return runEvidence(args[1:], stdout, stderr)
	case "release":
		return runRelease(args[1:], stdout, stderr)
	case "review":
		return runReview(args[1:], stdout, stderr)
	case "skill":
		return runSkill(args[1:], stdout, stderr)
	}

	_, _ = fmt.Fprintf(stderr, "unknown command: %s\n\n", strings.Join(args, " "))
	printUsage(stderr)
	return 1
}

func runVersion(args []string, stdout io.Writer, stderr io.Writer) int {
	parsed, handled, code := triageArgs("version", "version", args, stdout, stderr)
	if handled {
		return code
	}
	jsonMode := parsed.Bool("--json")

	result := output.Result{
		Summary:               fmt.Sprintf("walden %s (schema %s, documents %s)", effectiveVersion(), "v0beta1", spec.DocumentSchemaVersion),
		DocumentSchemaVersion: spec.DocumentSchemaVersion,
		ExitCode:              0,
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
	if groupHelp("repo", args, stdout) {
		return 0
	}
	if len(args) > 0 && args[0] == "init" {
		return runRepoInit(args[1:], stdout, stderr)
	}

	_, _ = fmt.Fprintf(stderr, "unknown command: repo %s\n\n", strings.Join(args, " "))
	printUsage(stderr)
	return 1
}

func runRepoInit(args []string, stdout io.Writer, stderr io.Writer) int {
	parsed, handled, code := triageArgs("repo init", "repo-init", args, stdout, stderr)
	if handled {
		return code
	}
	jsonMode := parsed.Bool("--json")

	if len(parsed.Positionals) == 0 {
		root, err := os.Getwd()
		if err != nil {
			return emitResult("repo-init", errorResult(fmt.Errorf("resolve working directory: %w", err)), jsonMode, stdout, stderr)
		}

		report, err := repo.Init(root, effectiveVersion())
		if err != nil {
			return emitResult("repo-init", errorResult(err), jsonMode, stdout, stderr)
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

	return emitResult("repo-init", errorResult(fmt.Errorf("repo init takes no arguments, got %q", strings.Join(parsed.Positionals, " "))), jsonMode, stdout, stderr)
}

func runFeature(args []string, stdout io.Writer, stderr io.Writer) int {
	if groupHelp("feature", args, stdout) {
		return 0
	}
	if len(args) > 0 && args[0] == "init" {
		return runFeatureInit(args[1:], stdout, stderr)
	}

	_, _ = fmt.Fprintf(stderr, "unknown command: feature %s\n\n", strings.Join(args, " "))
	printUsage(stderr)
	return 1
}

func runFeatureInit(args []string, stdout io.Writer, stderr io.Writer) int {
	parsed, handled, code := triageArgs("feature init", "feature-init", args, stdout, stderr)
	if handled {
		return code
	}
	jsonMode := parsed.Bool("--json")
	positional := parsed.Positionals

	if len(positional) == 0 {
		return emitResult("feature-init", errorResult(errors.New("feature init requires a feature name")), jsonMode, stdout, stderr)
	}

	{
		root, err := os.Getwd()
		if err != nil {
			return emitResult("feature-init", errorResult(fmt.Errorf("resolve working directory: %w", err)), jsonMode, stdout, stderr)
		}

		report, err := repo.InitFeature(root, strings.Join(positional, " "))
		if err != nil {
			return emitResult("feature-init", errorResult(err), jsonMode, stdout, stderr)
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
}

func runValidate(args []string, stdout io.Writer, stderr io.Writer) int {
	parsed, handled, code := triageArgs("validate", "validate", args, stdout, stderr)
	if handled {
		return code
	}
	jsonMode := parsed.Bool("--json")
	fullSpecMode := parsed.Bool("--all")
	positional := parsed.Positionals

	if len(positional) > 1 {
		return emitResult("validate", errorResult(errors.New("validate requires at most one feature name")), jsonMode, stdout, stderr)
	}

	root, err := os.Getwd()
	if err != nil {
		return emitResult("validate", errorResult(fmt.Errorf("resolve working directory: %w", err)), jsonMode, stdout, stderr)
	}

	scope := validation.ScopeCurrentPhase
	if fullSpecMode {
		scope = validation.ScopeFullSpec
	}

	// Portfolio mode: no feature name validates every feature (R3).
	if len(positional) == 0 {
		return runValidatePortfolio(root, scope, jsonMode, stdout, stderr)
	}

	result, err := validation.ValidateFeatureWithScope(root, positional[0], scope)
	if err != nil {
		return emitResult("validate", errorResult(fmt.Errorf("validate feature: %w", err)), jsonMode, stdout, stderr)
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

// runValidatePortfolio validates every feature under .walden/specs/ with the
// given scope, reporting one verdict per feature (R3).
func runValidatePortfolio(root string, scope validation.Scope, jsonMode bool, stdout, stderr io.Writer) int {
	specsDir := filepath.Join(root, ".walden", "specs")
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		return emitResult("validate", errorResult(errors.New("no feature specs found: run 'walden feature init <feature-name>' to create one")), jsonMode, stdout, stderr)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)

	if len(names) == 0 {
		return emitResult("validate", errorResult(errors.New("no feature specs found: run 'walden feature init <feature-name>' to create one")), jsonMode, stdout, stderr)
	}

	features := make([]output.FeatureValidation, 0, len(names))
	invalid := 0
	for _, name := range names {
		result, err := validation.ValidateFeatureWithScope(root, name, scope)
		if err != nil {
			return emitResult("validate", errorResult(fmt.Errorf("validate feature %s: %w", name, err)), jsonMode, stdout, stderr)
		}
		if !result.Valid {
			invalid++
		}
		features = append(features, output.FeatureValidation{
			Feature: name,
			Valid:   result.Valid,
			Summary: result.Message,
		})
	}

	exitCode := 0
	summary := fmt.Sprintf("VALID: %d feature(s) validated", len(names))
	if invalid > 0 {
		exitCode = 1
		summary = fmt.Sprintf("INVALID: %d of %d feature(s) failed validation", invalid, len(names))
	}

	outputResult := output.Result{
		Summary:  summary,
		Features: features,
		ExitCode: exitCode,
	}

	if jsonMode {
		if err := output.PrintJSON(stdout, "validate", outputResult); err != nil {
			_, _ = fmt.Fprintf(stderr, "render json output: %v\n", err)
			return 1
		}
		return exitCode
	}

	if exitCode == 0 {
		output.PrintText(stdout, outputResult)
		return 0
	}
	output.PrintText(stderr, outputResult)
	return 1
}

func runStatus(args []string, stdout io.Writer, stderr io.Writer) int {
	parsed, handled, code := triageArgs("status", "status", args, stdout, stderr)
	if handled {
		return code
	}
	jsonMode := parsed.Bool("--json")
	positional := parsed.Positionals

	if len(positional) != 1 {
		return emitResult("status", errorResult(errors.New("status requires exactly one feature name")), jsonMode, stdout, stderr)
	}

	featureName, err := spec.NormalizeFeatureName(positional[0])
	if err != nil {
		return emitResult("status", errorResult(err), jsonMode, stdout, stderr)
	}

	root, err := os.Getwd()
	if err != nil {
		return emitResult("status", errorResult(fmt.Errorf("resolve working directory: %w", err)), jsonMode, stdout, stderr)
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
	parsed, handled, code := triageArgs("reconcile", "reconcile", args, stdout, stderr)
	if handled {
		return code
	}
	jsonMode := parsed.Bool("--json")
	positional := parsed.Positionals

	if len(positional) != 1 {
		return emitResult("reconcile", errorResult(errors.New("reconcile requires exactly one feature name")), jsonMode, stdout, stderr)
	}

	featureName, err := spec.NormalizeFeatureName(positional[0])
	if err != nil {
		return emitResult("reconcile", errorResult(err), jsonMode, stdout, stderr)
	}

	root, err := os.Getwd()
	if err != nil {
		return emitResult("reconcile", errorResult(fmt.Errorf("resolve working directory: %w", err)), jsonMode, stdout, stderr)
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
	if groupHelp("review", args, stdout) {
		return 0
	}
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
	if groupHelp("lesson", args, stdout) {
		return 0
	}
	if len(args) > 0 && args[0] == "log" {
		return runLessonLog(args[1:], stdout, stderr)
	}

	_, _ = fmt.Fprintf(stderr, "unknown command: lesson %s\n\n", strings.Join(args, " "))
	printUsage(stderr)
	return 1
}

func runTask(args []string, stdout io.Writer, stderr io.Writer) int {
	if groupHelp("task", args, stdout) {
		return 0
	}
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
	parsed, handled, code := triageArgs("task status", "task-status", args, stdout, stderr)
	if handled {
		return code
	}
	jsonMode := parsed.Bool("--json")
	positional := parsed.Positionals

	if len(positional) != 1 {
		return emitResult("task-status", errorResult(errors.New("task status requires exactly one feature name")), jsonMode, stdout, stderr)
	}

	featureName, err := spec.NormalizeFeatureName(positional[0])
	if err != nil {
		return emitResult("task-status", errorResult(err), jsonMode, stdout, stderr)
	}

	root, err := os.Getwd()
	if err != nil {
		return emitResult("task-status", errorResult(fmt.Errorf("resolve working directory: %w", err)), jsonMode, stdout, stderr)
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
	parsed, handled, code := triageArgs("lesson log", "lesson-log", args, stdout, stderr)
	if handled {
		return code
	}
	jsonMode := parsed.Bool("--json")
	if len(parsed.Positionals) > 0 {
		return emitResult("lesson-log", errorResult(fmt.Errorf("lesson log cannot parse argument %q", parsed.Positionals[0])), jsonMode, stdout, stderr)
	}

	values := map[string]string{}
	for _, name := range []string{"feature", "phase", "trigger", "lesson", "guardrail"} {
		values[name] = parsed.Value("--" + name)
	}

	for _, required := range []string{"feature", "phase", "trigger", "lesson", "guardrail"} {
		if strings.TrimSpace(values[required]) == "" {
			return emitResult("lesson-log", errorResult(errors.New("lesson log requires --feature, --phase, --trigger, --lesson, and --guardrail")), jsonMode, stdout, stderr)
		}
	}

	root, err := os.Getwd()
	if err != nil {
		return emitResult("lesson-log", errorResult(fmt.Errorf("resolve working directory: %w", err)), jsonMode, stdout, stderr)
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
	parsed, handled, code := triageArgs("task start", "task-start", args, stdout, stderr)
	if handled {
		return code
	}
	jsonMode := parsed.Bool("--json")
	positional := parsed.Positionals

	if len(positional) < 1 || len(positional) > 2 {
		return emitResult("task-start", errorResult(errors.New("task start requires a feature name and an optional task id")), jsonMode, stdout, stderr)
	}

	featureName, err := spec.NormalizeFeatureName(positional[0])
	if err != nil {
		return emitResult("task-start", errorResult(err), jsonMode, stdout, stderr)
	}

	taskID := ""
	if len(positional) == 2 {
		taskID = positional[1]
	}

	root, err := os.Getwd()
	if err != nil {
		return emitResult("task-start", errorResult(fmt.Errorf("resolve working directory: %w", err)), jsonMode, stdout, stderr)
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
	parsed, handled, code := triageArgs("task complete", "task-complete", args, stdout, stderr)
	if handled {
		return code
	}
	jsonMode := parsed.Bool("--json")
	positional := parsed.Positionals

	if len(positional) != 2 {
		return emitResult("task-complete", errorResult(errors.New("task complete requires a feature name and a task id")), jsonMode, stdout, stderr)
	}

	featureName, err := spec.NormalizeFeatureName(positional[0])
	if err != nil {
		return emitResult("task-complete", errorResult(err), jsonMode, stdout, stderr)
	}

	root, err := os.Getwd()
	if err != nil {
		return emitResult("task-complete", errorResult(fmt.Errorf("resolve working directory: %w", err)), jsonMode, stdout, stderr)
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
	parsed, handled, code := triageArgs("task complete-all", "task-complete-all", args, stdout, stderr)
	if handled {
		return code
	}
	jsonMode := parsed.Bool("--json")
	positional := parsed.Positionals

	if len(positional) != 1 {
		return emitResult("task-complete-all", errorResult(errors.New("task complete-all requires exactly one feature name")), jsonMode, stdout, stderr)
	}

	featureName, err := spec.NormalizeFeatureName(positional[0])
	if err != nil {
		return emitResult("task-complete-all", errorResult(err), jsonMode, stdout, stderr)
	}

	root, err := os.Getwd()
	if err != nil {
		return emitResult("task-complete-all", errorResult(fmt.Errorf("resolve working directory: %w", err)), jsonMode, stdout, stderr)
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
	parsed, handled, code := triageArgs("review open", "review-open", args, stdout, stderr)
	if handled {
		return code
	}
	jsonMode := parsed.Bool("--json")

	if len(parsed.Positionals) != 1 {
		return emitResult("review-open", errorResult(errors.New("review open requires a feature and --phase requirements|design|tasks")), jsonMode, stdout, stderr)
	}

	featureName := parsed.Positionals[0]
	phaseName := parsed.Value("--phase")

	if phaseName == "" {
		return emitResult("review-open", errorResult(errors.New("review open requires --phase requirements|design|tasks")), jsonMode, stdout, stderr)
	}

	phase, err := workflow.ParsePhase(phaseName)
	if err != nil {
		return emitResult("review-open", errorResult(err), jsonMode, stdout, stderr)
	}

	root, err := os.Getwd()
	if err != nil {
		return emitResult("review-open", errorResult(fmt.Errorf("resolve working directory: %w", err)), jsonMode, stdout, stderr)
	}

	context, err := workflow.OpenReview(root, featureName, phase)
	if err != nil {
		return emitResult("review-open", errorResult(err), jsonMode, stdout, stderr)
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
	parsed, handled, code := triageArgs("review approve", "review-approve", args, stdout, stderr)
	if handled {
		return code
	}
	jsonMode := parsed.Bool("--json")

	if len(parsed.Positionals) != 1 {
		return emitResult("review-approve", errorResult(errors.New("review approve requires a feature and --phase requirements|design|tasks")), jsonMode, stdout, stderr)
	}

	featureName := parsed.Positionals[0]
	phaseName := parsed.Value("--phase")

	if phaseName == "" {
		return emitResult("review-approve", errorResult(errors.New("review approve requires --phase requirements|design|tasks")), jsonMode, stdout, stderr)
	}

	phase, err := workflow.ParsePhase(phaseName)
	if err != nil {
		return emitResult("review-approve", errorResult(err), jsonMode, stdout, stderr)
	}

	root, err := os.Getwd()
	if err != nil {
		return emitResult("review-approve", errorResult(fmt.Errorf("resolve working directory: %w", err)), jsonMode, stdout, stderr)
	}

	// Fail-closed gate: a document the toolchain cannot validate (and, for
	// tasks, cannot execute) must never become approved. Validation runs
	// before any approval state is written.
	validationResult, err := validation.ValidateFeatureWithScope(root, featureName, validation.ScopeCurrentPhase)
	if err != nil {
		return emitResult("review-approve", errorResult(fmt.Errorf("pre-approval validation: %w", err)), jsonMode, stdout, stderr)
	}
	if !validationResult.Valid {
		return emitResult("review-approve", errorResult(fmt.Errorf("approval refused: %s", validationResult.Message)), jsonMode, stdout, stderr)
	}

	approveResult, err := workflow.ApproveReview(root, featureName, phase)
	if err != nil {
		return emitResult("review-approve", errorResult(err), jsonMode, stdout, stderr)
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
	_, _ = fmt.Fprintf(w, "Usage:\n  %s <command> [arguments]\n\n", binaryName)
	_, _ = fmt.Fprintln(w, "Commands:")
	for _, command := range commandUsages {
		_, _ = fmt.Fprintf(w, "  %s\n      %s\n", command.Syntax, command.Summary)
	}
	_, _ = fmt.Fprintln(w, "\nFlags:")
	_, _ = fmt.Fprintln(w, "  --json           Emit machine-readable JSON instead of text")
	_, _ = fmt.Fprintln(w, "  --all            Validate the full spec, not just the current phase (validate)")
	_, _ = fmt.Fprintln(w, "  --phase <phase>  Target requirements|design|tasks (review open/approve)")
	_, _ = fmt.Fprintf(w, "\nRun '%s --help' to show this help.\n", binaryName)
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
		Name:                name,
		Status:              status,
		Fresh:               document.Exists && document.Fresh,
		ApprovedAt:          document.ApprovedAt,
		ApprovedFingerprint: document.ApprovedFingerprint,
		StaleCauses:         document.StaleCauses,
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
	result.Warnings = append(result.Warnings, readiness.EvidenceWarnings...)

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
		ChangedFiles: []string{".walden/specs/" + result.Feature + "/tasks.md", ".walden/evidence/" + result.Feature + ".json"},
		NextAction:   result.NextAction,
		Warnings:     result.Warnings,
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
