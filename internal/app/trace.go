package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/andrearaponi/walden/internal/consolidate"
	"github.com/andrearaponi/walden/internal/output"
	"github.com/andrearaponi/walden/internal/release"
	"github.com/andrearaponi/walden/internal/trace"
	"github.com/andrearaponi/walden/internal/validation"
)

// runTrace reports each statement of a feature with the proof steps that
// cover it and the evidence state of their leaves, and lists the holes. It
// joins what three commands already read: the identifiers the requirements
// define, the evidence the release gate derives and the coverage the
// validator judges.
//
// It is a report and not a gate: a completed report exits 0 whatever its
// holes. It writes nothing in the repository and starts no proof and no
// environment probe.
func runTrace(args []string, stdout io.Writer, stderr io.Writer) int {
	parsed, handled, code := triageArgs("trace", "trace", args, stdout, stderr)
	if handled {
		return code
	}
	jsonMode, markdown := parsed.Bool("--json"), parsed.Bool("--markdown")
	if jsonMode && markdown {
		return emitResult("trace", errorResult(errors.New("--markdown and --json cannot be combined: choose one format")), jsonMode, stdout, stderr)
	}
	if len(parsed.Positionals) != 1 {
		return emitResult("trace", errorResult(errors.New("trace requires exactly one feature name")), jsonMode, stdout, stderr)
	}

	root, err := os.Getwd()
	if err != nil {
		return emitResult("trace", errorResult(fmt.Errorf("resolve working directory: %w", err)), jsonMode, stdout, stderr)
	}
	reading, err := release.ReadFeatureEvidence(context.Background(), root, parsed.Positionals[0])
	if err != nil {
		return emitResult("trace", errorResult(err), jsonMode, stdout, stderr)
	}

	defined := consolidate.DefinedIdentifiers(reading.Feature.Requirements.Body)
	coverage := validation.Coverage(reading.Feature)
	states := make(map[string]string, len(reading.Evidence))
	for _, entry := range reading.Evidence {
		states[entry.TaskID] = entry.State
	}
	report := trace.Build(trace.Inputs{
		Feature:     reading.Feature.Name,
		Criteria:    defined.Criteria,
		NFRs:        defined.NFRs,
		Constraints: defined.Constraints,
		Leaves:      reading.Tree.LeafTasks(),
		States:      states,
		Unnamed:     coverage.TaskReferenceCoverage.Missing,
		Uncovered:   coverage.ProofReferenceCoverage.Missing,
	})

	if markdown {
		output.PrintTraceMarkdown(stdout, report)
		return 0
	}
	return emitResult("trace", output.Result{Summary: report.Summary(), Trace: &report, ExitCode: 0}, jsonMode, stdout, stderr)
}
