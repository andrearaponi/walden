package app

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/output"
	"github.com/andrearaponi/walden/internal/testutil"
)

// deliveryStatusTasks holds a delivery leaf that names two criteria and a
// regression leaf, on the requirements of the specification diagnostics.
const deliveryStatusTasks = `---
status: approved
approved_at: 2026-03-22T07:20:00Z
last_modified: 2026-03-22T07:20:00Z
source_design_approved_at: 2026-03-22T07:10:00Z
---

# Implementation Plan

- [ ] 1. Deliver and keep
  - [ ] 1.1 Publish the notes
    - Requirements: ` + "`R1.AC1`, `R2.AC1`" + `
    - Design: Task Store
    - Proof: delivery
    - Verification:
      - command: ["grep", "-q", "released", "notes.txt"]
        covers: ["R1.AC1"]
  - [ ] 1.2 Keep it deterministic
    - Requirements: ` + "`NFR1`" + `
    - Design: Task Store
    - Verification:
      - command: ["go", "test", "./internal/workflow"]
`

const deliveryDeclarationLine = "    - Proof: delivery\n"

// deliveryStatusFixture enters an isolated directory holding the delivery
// feature and two files of code, with an ignore rule for *.log. Nothing is
// committed and no task is completed: each case decides both.
func deliveryStatusFixture(t *testing.T, tasks string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in PATH")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	root := chdirContract(t)
	writeStatusFeatureFile(t, root, verifyTaskFeature, "requirements.md", specDiagnosticsRequirements)
	writeStatusFeatureFile(t, root, verifyTaskFeature, "design.md", specDiagnosticsDesign)
	writeStatusFeatureFile(t, root, verifyTaskFeature, "tasks.md", tasks)
	diagnosticsFile(t, root, "main.go", "package main\n")
	diagnosticsFile(t, root, "notes.txt", "released\n")
	diagnosticsFile(t, root, ".gitignore", "*.log\n")
	return root
}

func completeStatusTask(t *testing.T, id string) {
	t.Helper()
	overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: "ok"}))
	integrityMustCLI(t, "task", "complete", verifyTaskFeature, id)
}

// deliveredStatusRepo is a committed repository in which the delivery leaf
// 1.1 was completed on the committed tree, so it is attested at HEAD, and
// the regression leaf 1.2 is verified.
func deliveredStatusRepo(t *testing.T) (root, head string) {
	t.Helper()
	root = deliveryStatusFixture(t, deliveryStatusTasks)
	gitify(t, root)
	completeStatusTask(t, "1.1")
	completeStatusTask(t, "1.2")
	return root, integrityGit(t, root, "rev-parse", "HEAD")
}

// statusLine returns the entry line of a task in the text of `evidence status`.
func statusLine(t *testing.T, text, id string) string {
	t.Helper()
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "- "+id+": ") {
			return line
		}
	}
	t.Fatalf("the text holds no entry for task %s:\n%s", id, text)
	return ""
}

func attestationExplanation(t *testing.T, entry output.EvidenceStatus) *evidence.AttestationExplanation {
	t.Helper()
	if entry.Explanation == nil || entry.Explanation.Attestation == nil {
		t.Fatalf("task %s (%s) is not told why its pass is no attestation: %+v", entry.TaskID, entry.State, entry.Explanation)
	}
	return entry.Explanation.Attestation
}

func TestEvidenceStatusDelivery(t *testing.T) {
	t.Run("delivered", func(t *testing.T) {
		root, head := deliveredStatusRepo(t)
		entry := statusEntries(t)["1.1"]
		if entry.State != evidence.StateDelivered || !entry.Delivery {
			t.Fatalf("task 1.1: state %s, delivery=%t, want delivered", entry.State, entry.Delivery)
		}
		if entry.Attestation == nil || entry.Attestation.Commit != head || entry.Attestation.Sealed {
			t.Fatalf("task 1.1: attestation %+v, want the commit %s, not sealed", entry.Attestation, head)
		}
		text := statusText(t)
		if want := "- 1.1: delivered at " + head + " ["; !strings.HasPrefix(statusLine(t, text, "1.1"), want) {
			t.Errorf("entry line %q does not start with %q", statusLine(t, text, "1.1"), want)
		}
		if regression := statusEntries(t)["1.2"]; regression.State != evidence.StateVerified || regression.Delivery || regression.Attestation != nil {
			t.Errorf("the regression task reads %s, delivery=%t", regression.State, regression.Delivery)
		}
		if !strings.Contains(text, "evidence status for "+verifyTaskFeature+": 1 verified, 1 delivered\n") {
			t.Errorf("the summary does not count the delivered task:\n%s", text)
		}
		if strings.Contains(text, "Next action:") {
			t.Errorf("a feature with verified and delivered tasks proposes an action:\n%s", text)
		}

		// After the delivered content changes, in a new commit, the proof
		// would fail. The task stays delivered and is not explained; the
		// regression task is stale and explained as today.
		diagnosticsFile(t, root, "notes.txt", "superseded\n")
		integrityGit(t, root, "commit", "-qam", "the notes move on")
		entries := statusEntries(t)
		delivered := entries["1.1"]
		if delivered.State != evidence.StateDelivered || delivered.Attestation == nil || delivered.Attestation.Commit != head {
			t.Fatalf("after a code change task 1.1 reads %s at %+v, want delivered at %s", delivered.State, delivered.Attestation, head)
		}
		if delivered.Explanation != nil || len(delivered.Gaps) != 0 {
			t.Errorf("a delivered task is explained: gaps %v, explanation %+v", delivered.Gaps, delivered.Explanation)
		}
		if entries["1.2"].State != evidence.StateStaleCode || entries["1.2"].Explanation == nil || entries["1.2"].Explanation.Code == nil {
			t.Errorf("the regression task: state %s, explanation %+v", entries["1.2"].State, entries["1.2"].Explanation)
		}
		text = statusText(t)
		if lines := linesOf(text, "1.1"); len(lines) != 1 || !strings.Contains(lines[0], "recorded duration") {
			t.Errorf("the lines under a delivered task: %q, want its recorded duration alone", lines)
		}
		if !strings.Contains(text, "1 delivered, 1 stale-code\n") {
			t.Errorf("the summary:\n%s", text)
		}
	})

	t.Run("sealed", func(t *testing.T) {
		_, head := deliveredStatusRepo(t)
		if line := statusLine(t, statusText(t), "1.1"); strings.Contains(line, "sealed") {
			t.Fatalf("an attestation written by an execution is shown as sealed: %s", line)
		}
		rewriteLedger(t, ".", func(tasks map[string]any) {
			tasks["1.1"].(map[string]any)["attestation"].(map[string]any)["sealed"] = true
		})
		entry := statusEntries(t)["1.1"]
		if entry.State != evidence.StateDelivered || entry.Attestation == nil || !entry.Attestation.Sealed {
			t.Fatalf("task 1.1: state %s, attestation %+v, want a sealed attestation", entry.State, entry.Attestation)
		}
		if want := "- 1.1: delivered at " + head + " (sealed by adoption) ["; !strings.HasPrefix(statusLine(t, statusText(t), "1.1"), want) {
			t.Errorf("entry line %q does not start with %q", statusLine(t, statusText(t), "1.1"), want)
		}
	})

	t.Run("not-attested", func(t *testing.T) {
		verify := "; a pass of walden verify " + verifyTaskFeature + " on a tree without uncommitted changes attests it"
		adopt := "; walden adopt " + verifyTaskFeature + " shows whether the pass can be sealed and --apply seals it; a pass left unsealed is proven again with walden verify " + verifyTaskFeature + " --task 1.1"

		check := func(name, reason, detail, marker string) {
			t.Helper()
			entry := statusEntries(t)["1.1"]
			explanation := attestationExplanation(t, entry)
			if explanation.Unavailable != reason || explanation.Detail != detail {
				t.Errorf("%s:\n got %s: %s\nwant %s: %s", name, explanation.Unavailable, explanation.Detail, reason, detail)
			}
			if entry.State == evidence.StateDelivered || entry.Attestation != nil || !entry.Delivery {
				t.Errorf("%s: state %s, attestation %+v, delivery=%t", name, entry.State, entry.Attestation, entry.Delivery)
			}
			text := statusText(t)
			if line := statusLine(t, text, "1.1"); !strings.Contains(line, marker+" [") {
				t.Errorf("%s: entry line %q does not hold %q", name, line, marker)
			}
			if want := "  not attested: " + detail; !strings.Contains(strings.Join(linesOf(text, "1.1"), "\n"), want) {
				t.Errorf("%s: the text does not hold %q:\n%s", name, want, text)
			}
		}

		// A pass on a tree with uncommitted paths.
		root := deliveryStatusFixture(t, deliveryStatusTasks)
		gitify(t, root)
		diagnosticsFile(t, root, "draft.txt", "not committed\n")
		completeStatusTask(t, "1.1")
		check("uncommitted paths", evidence.ReasonUncommittedPaths, "the proof ran on a tree with 1 uncommitted path(s)"+verify, "verified (delivery proof)")

		// The same record once it keeps no tree and no statement facts, as
		// one written before records kept them.
		rewriteLedger(t, root, func(tasks map[string]any) {
			delete(tasks["1.1"].(map[string]any), "tree")
			delete(tasks["1.1"].(map[string]any), "statements")
		})
		check("no tree facts", evidence.ReasonNoTree, "the record keeps no commit and no paths, so it does not show whether its tree was committed"+verify, "verified (delivery proof)")

		// A pass in a repository without a commit.
		root = deliveryStatusFixture(t, deliveryStatusTasks)
		integrityGit(t, root, "init", "-q", "-b", "main")
		completeStatusTask(t, "1.1")
		check("no commit", evidence.ReasonNoCommit, "no commit could be read when the proof ran"+verify, "verified (delivery proof)")

		// A pass recorded on a commit before the plan declared the delivery:
		// it awaits adoption.
		root = deliveryStatusFixture(t, strings.Replace(deliveryStatusTasks, deliveryDeclarationLine, "", 1))
		gitify(t, root)
		head := integrityGit(t, root, "rev-parse", "HEAD")
		completeStatusTask(t, "1.1")
		if entry := statusEntries(t)["1.1"]; entry.State != evidence.StateVerified || entry.Delivery || entry.Explanation != nil {
			t.Fatalf("before the declaration task 1.1 reads %s, delivery=%t, explanation %+v", entry.State, entry.Delivery, entry.Explanation)
		}
		reapprove(t, root, "tasks.md", func(content string) string {
			return strings.Replace(content, "    - Design: Task Store\n    - Verification:\n      - command: [\"grep\"", "    - Design: Task Store\n"+deliveryDeclarationLine+"    - Verification:\n      - command: [\"grep\"", 1)
		})
		check("recorded before the declaration", evidence.ReasonUndeclaredRun,
			"the pass was recorded on commit "+head+" by a run that did not read the task's proof as a delivery proof"+adopt,
			"stale-spec (delivery proof, awaits adoption)")
		if entry := statusEntries(t)["1.1"]; !entry.AwaitsAdoption {
			t.Errorf("the pass does not await adoption: %+v", entry)
		}
		text := statusText(t)
		if want := "Next action: Review walden adopt " + verifyTaskFeature + " before any walden verify"; !strings.Contains(text, want) {
			t.Errorf("the next action does not name adoption first:\n%s", text)
		}
	})

	t.Run("statement-changed", func(t *testing.T) {
		root, head := deliveredStatusRepo(t)
		// The task names R1.AC1 and R2.AC1. Rewording NFR1, which it does not
		// name, keeps it delivered.
		reapprove(t, root, "requirements.md", func(content string) string {
			return strings.Replace(content, "Parsing stays deterministic.", "Parsing stays deterministic on every platform.", 1)
		})
		entries := statusEntries(t)
		if entries["1.1"].State != evidence.StateDelivered {
			t.Fatalf("after rewording a statement the task does not name it reads %s, want delivered", entries["1.1"].State)
		}
		if entries["1.2"].State != evidence.StateStaleSpec {
			t.Fatalf("the regression task that names it reads %s, want stale-spec", entries["1.2"].State)
		}

		// Rewording R1.AC1, which it names.
		reapprove(t, root, "requirements.md", func(content string) string {
			return strings.Replace(content, "the system SHALL parse it.", "the system SHALL parse it at once.", 1)
		})
		entry := statusEntries(t)["1.1"]
		if entry.State != evidence.StateStaleSpec || entry.Attestation != nil {
			t.Fatalf("after rewording R1.AC1 task 1.1 reads %s, attestation %+v, want stale-spec", entry.State, entry.Attestation)
		}
		gap := ""
		for _, candidate := range entry.Gaps {
			if candidate.Kind == "attested-statements-changed" {
				gap = candidate.Message
			}
		}
		if want := "statements attested at commit " + head + " read differently now: R1.AC1 changed"; gap != want {
			t.Errorf("gap %q, want %q", gap, want)
		}
		if entry.Explanation == nil || entry.Explanation.Spec == nil || len(entry.Explanation.Spec.Statements) != 1 || entry.Explanation.Spec.Statements[0].ID != "R1.AC1" {
			t.Errorf("the explanation does not name R1.AC1: %+v", entry.Explanation)
		}
		text := strings.Join(linesOf(statusText(t), "1.1"), "\n")
		for _, want := range []string{"attested-statements-changed: statements attested at commit " + head + " read differently now: R1.AC1 changed", "changed statements: R1.AC1 changed"} {
			if !strings.Contains(text, want) {
				t.Errorf("the text does not hold %q:\n%s", want, text)
			}
		}
		if strings.Contains(text, "R2.AC1") {
			t.Errorf("the text names R2.AC1, which reads as it did:\n%s", text)
		}
	})

	t.Run("json-matches-text", func(t *testing.T) {
		// Every fact the text shows for a delivery task is in the JSON, and
		// the other way round: state, commit, sealing mark, the two marks of
		// a delivery task without an attestation, and the reason.
		facts := func() (fromJSON, fromText []string) {
			t.Helper()
			var stdout, stderr bytes.Buffer
			if code := Run([]string{"evidence", "status", verifyTaskFeature, "--json"}, &stdout, &stderr); code != 0 {
				t.Fatalf("evidence status --json exited %d: %s", code, stderr.String())
			}
			var envelope output.Envelope
			if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
				t.Fatalf("decode: %v", err)
			}
			text := statusText(t)
			for _, entry := range envelope.Result.Evidence {
				fromJSON = append(fromJSON, entry.TaskID+" is "+entry.State)
				switch {
				case entry.Attestation != nil && entry.Attestation.Sealed:
					fromJSON = append(fromJSON, entry.TaskID+" at "+entry.Attestation.Commit+" (sealed by adoption)")
				case entry.Attestation != nil:
					fromJSON = append(fromJSON, entry.TaskID+" at "+entry.Attestation.Commit)
				case entry.Delivery && entry.AwaitsAdoption:
					fromJSON = append(fromJSON, entry.TaskID+" (delivery proof, awaits adoption)")
				case entry.Delivery:
					fromJSON = append(fromJSON, entry.TaskID+" (delivery proof)")
				}
				if entry.Explanation != nil && entry.Explanation.Attestation != nil {
					fromJSON = append(fromJSON, entry.TaskID+" not attested: "+entry.Explanation.Attestation.Detail)
				}

				line := statusLine(t, text, entry.TaskID)
				rest := strings.TrimPrefix(line, "- "+entry.TaskID+": ")
				state, after, _ := strings.Cut(rest, " ")
				fromText = append(fromText, entry.TaskID+" is "+state)
				after, _, _ = strings.Cut(after, " [")
				if strings.HasPrefix(after, "at ") || strings.HasPrefix(after, "(delivery proof") {
					fromText = append(fromText, entry.TaskID+" "+after)
				}
				for _, under := range linesOf(text, entry.TaskID) {
					if detail, found := strings.CutPrefix(under, "  not attested: "); found {
						fromText = append(fromText, entry.TaskID+" not attested: "+detail)
					}
				}
			}
			return fromJSON, fromText
		}
		compare := func(name string, wants ...string) {
			t.Helper()
			fromJSON, fromText := facts()
			if strings.Join(fromJSON, "\n") != strings.Join(fromText, "\n") {
				t.Errorf("%s: the JSON and the text disagree\nJSON:\n%s\ntext:\n%s", name, strings.Join(fromJSON, "\n"), strings.Join(fromText, "\n"))
			}
			for _, want := range wants {
				if !strings.Contains(strings.Join(fromJSON, "\n"), want) {
					t.Errorf("%s: no fact %q in\n%s", name, want, strings.Join(fromJSON, "\n"))
				}
			}
		}

		root, head := deliveredStatusRepo(t)
		compare("delivered", "1.1 is delivered", "1.1 at "+head, "1.2 is verified")
		rewriteLedger(t, root, func(tasks map[string]any) {
			tasks["1.1"].(map[string]any)["attestation"].(map[string]any)["sealed"] = true
		})
		compare("sealed", "1.1 at "+head+" (sealed by adoption)")

		root = deliveryStatusFixture(t, deliveryStatusTasks)
		gitify(t, root)
		diagnosticsFile(t, root, "draft.txt", "not committed\n")
		completeStatusTask(t, "1.1")
		compare("not attested", "1.1 is verified", "1.1 (delivery proof)", "1.1 not attested: the proof ran on a tree with 1 uncommitted path(s)")

		root = deliveryStatusFixture(t, strings.Replace(deliveryStatusTasks, deliveryDeclarationLine, "", 1))
		gitify(t, root)
		completeStatusTask(t, "1.1")
		reapprove(t, root, "tasks.md", func(content string) string {
			return strings.Replace(content, "    - Design: Task Store\n    - Verification:\n      - command: [\"grep\"", "    - Design: Task Store\n"+deliveryDeclarationLine+"    - Verification:\n      - command: [\"grep\"", 1)
		})
		compare("awaiting adoption", "1.1 is stale-spec", "1.1 (delivery proof, awaits adoption)", "1.1 not attested: the pass was recorded on commit")
	})
}
