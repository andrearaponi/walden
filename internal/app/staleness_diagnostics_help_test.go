package app

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// evidenceStatusSummary is the one line the design fixes for the command.
const evidenceStatusSummary = "Report each completed task's derived evidence state and what changed since its proof ran"

// evidenceStatusHelp is the whole help of the command. The summary and the
// first six detail lines are the text the design of the staleness
// diagnostics fixes, and the seventh is the one the design of proof kinds
// adds; the frame around them is
// what the help renderer prints for a command with one flag. The test states
// the text here instead of reading it from the command catalog.
const evidenceStatusHelp = `Usage:
  walden evidence status <feature> [--json]
      Report each completed task's derived evidence state and what changed since its proof ran

  Before the tasks: what the current code identity is made of, the commit and each uncommitted path, untracked ones marked.
  Under a task whose code is stale: the paths that changed, were added or were removed since its proof ran.
  Under a task whose specification is stale: the approvals that changed and the statements it names that changed or are no longer defined.
  Under a task whose record keeps step durations: how long that recorded execution took.
  An explanation is exact or withheld: when a record lacks a fact, names a commit that is not in this repository, or keeps facts that do not rebuild its code identity, the report says so and names no path or statement. Re-proving the task writes a record with the facts available then.
  Text lists show the first 10 items and the number omitted; --json carries them whole.
  A delivered task is reported with the commit it was attested on, marked when adoption sealed the attestation, and with no explanation of later changes; a completed delivery task without an attestation says why its recorded pass is not one and what attests it.

Flags:
  --json             Emit machine-readable JSON instead of text
`

// usageCommandsV0144 is the commands block that walden v0.14.4 prints for
// `walden --help`, taken from that release and not from the command catalog.
// It is the oracle for the lines this feature must not change. A change that
// alters the usage text on purpose updates it.
const usageCommandsV0144 = `Commands:
  version [--json]
      Print the CLI and schema version
  update [--check] [--version <tag>] [--json]
      Update the walden binary from GitHub releases
  repo init [--json]
      Initialize Walden in the current repository
  feature init <name> [--json]
      Scaffold a new feature spec
  status <feature> [--json]
      Show a feature's phase, blockers, and next action
  reconcile <feature> [--json]
      Re-sync the approval chain after upstream edits
  lesson log --feature <name> --phase <phase> --trigger <text> --lesson <text> --guardrail <text> [--json]
      Append a structured lesson to .walden/lessons.md
  task status <feature> [--json]
      Show execution readiness and the next runnable task
  task start <feature> [task-id] [--json]
      Resolve normalized execution context for a task
  task complete <feature> <task-id> [--json]
      Run a task's proofs and mark it complete
  task complete-all <feature> [--json]
      Complete all runnable tasks in order
  verify <feature> [--all] [--check] [--task <id>] [--json]
      Re-prove completed tasks against the current code and refresh evidence
  adopt [<feature>] [--apply] [--json]
      Plan or apply brownfield adoption: seal recorded approvals, re-prove unrecorded work
  evidence status <feature> [--json]
      Report each completed task's derived execution-evidence state
  release check [<feature>] [--strict] [--allow-pending --reason <text>] [--json]
      Certify the repository (or one feature) as releasable: chain, validation, decisions, evidence, worktree
  validate [<feature>] [--all] [--json]
      Check EARS, traceability, and freshness (all features when no name is given)
  review open <feature> --phase <phase> [--json]
      Open the review gate (move to in-review)
  review approve <feature> --phase <phase> [--json]
      Approve the review gate (move to approved)
  consolidate report [--json]
      Report pending contract changes, the review scope, findings and comparisons (read-only; also plain walden consolidate)
  consolidate start [--json]
      Start consolidation tracking; approved features become the unconsolidated backlog
  consolidate open [--json]
      Put .walden/contracts.md in review; refused while it disagrees with the specifications or its coherence review is incomplete
  consolidate approve [--json]
      After the user's explicit approval, seal the reviewed view and record the consolidation checkpoint
`

// firstDifference names the first line at which two texts differ, so a
// failure points at the line instead of printing two long blocks.
func firstDifference(got, want string) string {
	gotLines, wantLines := strings.Split(got, "\n"), strings.Split(want, "\n")
	for index := 0; index < len(gotLines) || index < len(wantLines); index++ {
		var gotLine, wantLine string
		if index < len(gotLines) {
			gotLine = gotLines[index]
		}
		if index < len(wantLines) {
			wantLine = wantLines[index]
		}
		if gotLine != wantLine {
			return fmt.Sprintf("line %d:\n  got:  %s\n  want: %s", index+1, gotLine, wantLine)
		}
	}
	return "no difference"
}

func TestEvidenceStatusHelp(t *testing.T) {
	// An empty directory: the help needs no repository and no feature.
	t.Chdir(t.TempDir())

	for _, flag := range []string{"--help", "-h"} {
		t.Run(flag, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := Run([]string{"evidence", "status", flag}, &stdout, &stderr); code != 0 {
				t.Fatalf("evidence status %s exited %d: %s", flag, code, stderr.String())
			}
			if stderr.Len() != 0 {
				t.Errorf("evidence status %s wrote to stderr: %q", flag, stderr.String())
			}
			if got := stdout.String(); got != evidenceStatusHelp {
				t.Errorf("the help is not the text the design fixes, %s", firstDifference(got, evidenceStatusHelp))
			}
		})
	}
}

func TestUsageCommandsBlock(t *testing.T) {
	const syntax = "  evidence status <feature> [--json]\n"
	const before = syntax + "      Report each completed task's derived execution-evidence state\n"
	const after = syntax + "      " + evidenceStatusSummary + "\n"
	if strings.Count(usageCommandsV0144, before) != 1 {
		t.Fatal("the recorded block does not hold the line of evidence status exactly once")
	}
	want := strings.Replace(usageCommandsV0144, before, after, 1)

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

	// One line for evidence status, saying that the report explains what
	// changed; the line of every other command as v0.14.4 printed it.
	if got := usage[start:end]; got != want {
		t.Errorf("the commands block differs from the block of v0.14.4 with the line of evidence status replaced, %s", firstDifference(got, want))
	}
}
