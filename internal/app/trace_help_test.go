package app

import (
	"bytes"
	"strings"
	"testing"
)

// traceSummary is the one line that describes the command, in its help and
// in the usage text.
const traceSummary = "Report each statement of a feature with the proof steps that cover it and their evidence state"

// traceUsageLines are the two lines of the command in the commands block of
// `walden --help`.
const traceUsageLines = "  trace <feature> [--markdown] [--json]\n      " + traceSummary + "\n"

// traceHelp is the whole help of the command, as the design fixes it. The
// test states the text here instead of reading it from the command catalog.
const traceHelp = `Usage:
  walden trace <feature> [--markdown] [--json]
      Report each statement of a feature with the proof steps that cover it and their evidence state

  A row is one statement, a criterion or a non-functional requirement or constraint the plan names, with one proof step whose covers lists it and the evidence state of the leaf task that step belongs to.
  A hole is a criterion that no leaf task names (unnamed), a criterion that no proof step covers (uncovered), or a covered statement whose leaf task is neither verified nor delivered (not-standing).
  The summary counts the holes of each kind, and they are listed in that order: the criteria that lack a proof come first.
  Text by default; --markdown prints a table and the holes, to paste into a pull request; --json carries every field.
  Text and --markdown show a command of more than 120 characters cut to 120, the last one being …; --json carries it whole, with the design references of each leaf task as the plan writes them.
  A report and not a gate: it exits 0 with holes, writes nothing in the repository and runs no proof. walden release check judges, and walden evidence status explains a state.

Flags:
  --markdown         Print a Markdown table and the list of holes, and nothing else
  --json             Emit machine-readable JSON instead of text
`

// TestTraceHelp: the help of the command is the text the design fixes: the
// syntax with its two flags, what a row and a hole are, the limit of a shown
// command, and the command that judges in its place.
func TestTraceHelp(t *testing.T) {
	// An empty directory: the help needs no repository and no feature.
	t.Chdir(t.TempDir())

	for _, flag := range []string{"--help", "-h"} {
		t.Run(flag, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := Run([]string{"trace", flag}, &stdout, &stderr); code != 0 {
				t.Fatalf("trace %s exited %d: %s", flag, code, stderr.String())
			}
			if stderr.Len() != 0 {
				t.Errorf("trace %s wrote to stderr: %q", flag, stderr.String())
			}
			if got := stdout.String(); got != traceHelp {
				t.Errorf("the help is not the text the design fixes, %s", firstDifference(got, traceHelp))
			}
		})
	}
}

// TestUsageListsTrace: the commands block of `walden --help` holds the
// syntax of the command and its one-line description, once, after the lines
// of evidence status and before those of release check.
func TestUsageListsTrace(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("--help exited %d: %s", code, stderr.String())
	}
	usage := stdout.String()
	start := strings.Index(usage, "Commands:\n")
	end := strings.Index(usage, "\nFlags:\n")
	if start < 0 || end < start {
		t.Fatalf("the usage text has no commands block:\n%s", usage)
	}
	block := usage[start:end]

	if count := strings.Count(block, traceUsageLines); count != 1 {
		t.Fatalf("the commands block holds the two lines of trace %d time(s), want once:\n%s", count, block)
	}
	const before = "  evidence status <feature> [--json]\n      " + evidenceStatusSummary + "\n"
	const after = "  release check [<feature>]"
	if !strings.Contains(block, before+traceUsageLines+after) {
		t.Errorf("the lines of trace do not sit between those of evidence status and those of release check:\n%s", block)
	}
}
