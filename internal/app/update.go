package app

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/andrearaponi/walden/internal/output"
	"github.com/andrearaponi/walden/internal/selfupdate"
)

// updateOptions builds the production options for an update run. It is a
// seam so tests can point the flow at local doubles instead of GitHub.
var updateOptions = func(currentVersion string) (selfupdate.Options, error) {
	return selfupdate.DefaultOptions(currentVersion)
}

func runUpdate(args []string, stdout io.Writer, stderr io.Writer) int {
	jsonMode := false
	checkMode := false
	pinned := ""

	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--json":
			jsonMode = true
		case "--check":
			checkMode = true
		case "--version":
			if index+1 >= len(args) {
				_, _ = fmt.Fprintf(stderr, "update --version requires a tag (e.g. --version v0.7.0)\n")
				return 1
			}
			pinned = args[index+1]
			index++
		default:
			_, _ = fmt.Fprintf(stderr, "unknown command: update %s\n\n", strings.Join(args, " "))
			printUsage(stderr)
			return 1
		}
	}

	// The dev-build guard runs before options are assembled, so a source
	// build never reaches the network.
	current := effectiveVersion()
	if current == "dev" {
		result := output.Result{
			Summary:    "cannot update a source build: this binary carries no release version",
			NextAction: "Rebuild from your clone (./setup.sh) or install a release: curl -fsSL https://raw.githubusercontent.com/andrearaponi/walden/main/install.sh | sh",
			ExitCode:   1,
		}
		return emitUpdateResult(result, jsonMode, stdout, stderr)
	}

	opts, err := updateOptions(current)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	opts.TargetTag = pinned

	if checkMode {
		return runUpdateCheck(opts, jsonMode, stdout, stderr)
	}
	return runUpdateApply(opts, jsonMode, stdout, stderr)
}

func runUpdateCheck(opts selfupdate.Options, jsonMode bool, stdout, stderr io.Writer) int {
	status, err := selfupdate.Check(opts)
	if err != nil {
		return emitUpdateResult(output.Result{Summary: err.Error(), ExitCode: 1}, jsonMode, stdout, stderr)
	}

	// Check mode is a report, not a gate: exit 0 either way.
	result := output.Result{
		Summary: fmt.Sprintf("walden %s is up to date", status.CurrentVersion),
		Update: &output.UpdateStatus{
			CurrentVersion:  status.CurrentVersion,
			TargetVersion:   status.TargetVersion,
			UpdateAvailable: status.UpdateAvailable,
		},
		ExitCode: 0,
	}
	if status.UpdateAvailable {
		result.Summary = fmt.Sprintf("update available: %s -> %s", status.CurrentVersion, status.TargetVersion)
		result.NextAction = fmt.Sprintf("Run `walden update` to install %s", status.TargetVersion)
	}
	return emitUpdateResult(result, jsonMode, stdout, stderr)
}

func runUpdateApply(opts selfupdate.Options, jsonMode bool, stdout, stderr io.Writer) int {
	report, err := selfupdate.Apply(context.Background(), opts)
	if err != nil {
		return emitUpdateResult(output.Result{Summary: err.Error(), ExitCode: 1}, jsonMode, stdout, stderr)
	}

	if report.AlreadyUpToDate {
		result := output.Result{
			Summary: fmt.Sprintf("walden %s is already up to date", report.InstalledVersion),
			Update: &output.UpdateStatus{
				CurrentVersion: report.PreviousVersion,
				TargetVersion:  report.InstalledVersion,
			},
			ExitCode: 0,
		}
		return emitUpdateResult(result, jsonMode, stdout, stderr)
	}

	changed := []string{report.ExecutablePath}
	for _, skill := range report.SyncedSkills {
		changed = append(changed, skill.Path)
	}

	result := output.Result{
		Summary: fmt.Sprintf("walden updated: %s -> %s", report.PreviousVersion, report.InstalledVersion),
		Update: &output.UpdateStatus{
			CurrentVersion:  report.PreviousVersion,
			TargetVersion:   report.InstalledVersion,
			UpdateAvailable: true,
			Applied:         true,
		},
		ChangedFiles: changed,
		Warnings:     report.Warnings,
		NextAction:   fmt.Sprintf("Release notes: %s", report.ReleaseNotesURL),
		ExitCode:     0,
	}
	return emitUpdateResult(result, jsonMode, stdout, stderr)
}

// emitUpdateResult renders a result following the repo convention: JSON goes
// to stdout with ok reflecting the exit code, text errors go to stderr.
func emitUpdateResult(result output.Result, jsonMode bool, stdout, stderr io.Writer) int {
	if jsonMode {
		if err := output.PrintJSON(stdout, "update", result); err != nil {
			_, _ = fmt.Fprintf(stderr, "render json output: %v\n", err)
			return 1
		}
		return result.ExitCode
	}

	if result.ExitCode != 0 {
		output.PrintText(stderr, result)
		return result.ExitCode
	}
	output.PrintText(stdout, result)
	return 0
}
