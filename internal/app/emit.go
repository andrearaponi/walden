package app

import (
	"fmt"
	"io"

	"github.com/andrearaponi/walden/internal/output"
)

// emitResult renders every command outcome under one contract: JSON mode
// prints the versioned envelope on stdout (ok follows the exit code); text
// mode prints errors as their summary on stderr — plus the next action when
// present — and successes through PrintText on stdout. Routing all handlers
// through this helper is what keeps the JSON contract a property of the
// dispatch layer instead of a per-command convention.
func emitResult(command string, result output.Result, jsonMode bool, stdout, stderr io.Writer) int {
	if jsonMode {
		if err := output.PrintJSON(stdout, command, result); err != nil {
			_, _ = fmt.Fprintf(stderr, "render json output: %v\n", err)
			return 1
		}
		return result.ExitCode
	}

	if result.ExitCode != 0 {
		_, _ = fmt.Fprintf(stderr, "%s\n", result.Summary)
		if result.NextAction != "" {
			_, _ = fmt.Fprintf(stderr, "Next action: %s\n", result.NextAction)
		}
		return result.ExitCode
	}

	output.PrintText(stdout, result)
	return 0
}

// errorResult wraps an execution error into the shared result shape.
func errorResult(err error) output.Result {
	return output.Result{Summary: err.Error(), ExitCode: 1}
}
