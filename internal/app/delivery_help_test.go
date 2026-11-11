package app

import (
	"bytes"
	"testing"
)

// The whole help of the four commands that say how delivery proofs are
// treated: the text walden v0.14.5 prints for each, with the sentences and
// the flag descriptions the design of proof kinds fixes. The text is stated
// here and not read from the command catalog, so a stray line fails too.

const deliveryVerifyHelp = `Usage:
  walden verify <feature> [--all] [--check] [--task <id>] [--json]
      Re-prove completed tasks against the current code and refresh evidence

  --task re-proves one completed leaf task for a repair and leaves every other record as it is.
  Refused, running nothing, with --all, when repeated or empty, or for an id that is not a completed leaf task.
  It proves nothing about the feature: release check still judges every task, so finish with walden verify <feature>.
  A proof that passes while go test ran no test warns and keeps its exit code: assert the test by name with -v and expect_output.
  A delivered task is not proven again, with or without --all, and --task refuses it.
  On a tree without uncommitted changes outside .walden/, a run without --task also proves each completed delivery task that holds neither an attestation nor a pass that awaits adoption, and a pass attests it.
  A recorded pass that awaits adoption, recorded before the plan declared the delivery, is left for walden adopt <feature>; only --task proves its task again.

Flags:
  --all              Re-prove every completed task, not just stale ones, except delivered tasks and passes that await adoption
  --check            Report without persisting evidence
  --json             Emit machine-readable JSON instead of text
  --task <id>        Re-prove one completed task, whatever its state, unless it is delivered
`

const deliveryAdoptHelp = `Usage:
  walden adopt [<feature>] [--apply] [--json]
      Plan or apply brownfield adoption: seal recorded approvals, re-prove unrecorded work

  --apply seals as an attestation the recorded pass of a completed delivery task, without executing its proof, when the pass was recorded before the plan declared the delivery; the plan names each pass it can seal with the commit that holds its tree.
  A pass is left unsealed when its execution provenance is not supported or its proof differs from the approved one in more than the declaration: --apply proves its task again.
  A pass is also left unsealed when no commit among the last 200 reachable from HEAD holds its tree, or when its statements cannot be shown to read as they did: the pass stays, and walden verify <feature> --task <id> proves its task again.

Flags:
  --apply            Execute the plan: seal absent fingerprints and recorded delivery passes, re-prove evidence
  --json             Emit machine-readable JSON instead of text
`

const deliveryReleaseCheckHelp = `Usage:
  walden release check [<feature>] [--strict] [--allow-pending --reason <text>] [--json]
      Certify the repository (or one feature) as releasable: chain, validation, decisions, evidence, worktree

  A delivered task satisfies the evidence criterion without being proven again: the verdict lists it with the commit it was attested on, and a releasable verdict states the number of delivered tasks in its summary.
  For each feature the verdict lists the statements that only delivery proofs cover.

Flags:
  --strict           Also require committed .walden/ state (final certification)
  --allow-pending    Waive pending tasks for this verdict; requires --reason
  --json             Emit machine-readable JSON instead of text
  --reason <text>    The recorded justification for waiving pending tasks
`

func TestDeliveryHelp(t *testing.T) {
	// An empty directory: the help needs no repository and no feature.
	t.Chdir(t.TempDir())

	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"verify", []string{"verify"}, deliveryVerifyHelp},
		{"adopt", []string{"adopt"}, deliveryAdoptHelp},
		{"release-check", []string{"release", "check"}, deliveryReleaseCheckHelp},
		{"evidence-status", []string{"evidence", "status"}, evidenceStatusHelp},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, flag := range []string{"--help", "-h"} {
				var stdout, stderr bytes.Buffer
				if code := Run(append(append([]string{}, tc.args...), flag), &stdout, &stderr); code != 0 {
					t.Fatalf("%v %s exited %d: %s", tc.args, flag, code, stderr.String())
				}
				if stderr.Len() != 0 {
					t.Errorf("%v %s wrote to stderr: %q", tc.args, flag, stderr.String())
				}
				if got := stdout.String(); got != tc.want {
					t.Errorf("%v %s: the help is not the text the design fixes, %s", tc.args, flag, firstDifference(got, tc.want))
				}
			}
		})
	}
}
