package adopt

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/spec"
	"github.com/andrearaponi/walden/internal/testutil"
	"github.com/andrearaponi/walden/internal/workflow"
)

const passFeature = "delivery-pass"

// passRequirements defines the criterion the delivery task names, another
// one, and a statement no task of the fixture names.
const passRequirements = "# Requirements Document\n\n## Requirements\n\n### R1 Notes\n\n#### Acceptance Criteria\n\n" +
	"1. `R1.AC1` WHEN the notes are published, the system SHALL name the release.\n" +
	"2. `R1.AC2` WHEN the parser runs, the system SHALL stay deterministic.\n\n" +
	"## Non-Functional Requirements\n\n- `NFR1` The notes stay readable.\n"

// passPlan holds a leaf that publishes the notes and a regression leaf. The
// first becomes a delivery task when the plan declares it.
func passPlan(declared, completed bool, command string) string {
	marker := " "
	if completed {
		marker = "x"
	}
	declaration := ""
	if declared {
		declaration = "  - Proof: delivery\n"
	}
	return fmt.Sprintf("# Implementation Plan\n\n"+
		"- [%s] 1. Publish the notes\n  - Requirements: `R1.AC1`\n  - Design: Overview\n%s  - Verification:\n    - command: %s\n      covers: [\"R1.AC1\"]\n"+
		"- [%s] 2. Keep the parser\n  - Requirements: `R1.AC2`\n  - Design: Overview\n  - Verification:\n    - command: [\"go\", \"test\", \"./...\"]\n      covers: [\"R1.AC2\"]\n",
		marker, declaration, command, marker)
}

const passCommand = `["grep", "-q", "released", "notes.txt"]`

// isolatedAdoptRepo is a repository with one commit of code that reads no
// global or system git configuration: the machine's commit signing, hooks,
// file-system monitor and line-ending conversion cannot reach the test, and
// the git that adoption and the verify machinery start inherits the same
// environment. Tests that use it set process environment and are not
// parallel.
func isolatedAdoptRepo(t *testing.T) string {
	t.Helper()
	requireGit(t)
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	root := t.TempDir()
	gitIn(t, root, "init", "-q", "-b", "main")
	gitIn(t, root, "config", "user.email", "adopt@walden.test")
	gitIn(t, root, "config", "user.name", "Adopt")
	writeRepoFile(t, root, "main.go", "package main\n")
	writeRepoFile(t, root, "notes.txt", "released\n")
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-qm", "code")
	return root
}

func writeRepoFile(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func headOf(t *testing.T, root string) string {
	t.Helper()
	command := exec.Command("git", "rev-parse", "HEAD")
	command.Dir = root
	out, err := command.Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func commitRepo(t *testing.T, root, message string) string {
	t.Helper()
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-qm", message)
	return headOf(t, root)
}

// writeSealedFeature writes the three documents approved and sealed, with a
// consistent chain, as the review gates leave them.
func writeSealedFeature(t *testing.T, root, name, requirements, plan string) {
	t.Helper()
	const design = "# Feature Design\n"
	requirementsFP := spec.Fingerprint("requirements.md", requirements)
	designFP := spec.Fingerprint("design.md", design)
	writeDoc(t, root, name, "requirements.md", fmt.Sprintf("---\nstatus: approved\napproved_at: 2026-07-01T10:00:00Z\nlast_modified: 2026-07-01T10:00:00Z\napproved_fingerprint: %s\n---\n\n%s", requirementsFP, requirements))
	writeDoc(t, root, name, "design.md", fmt.Sprintf("---\nstatus: approved\napproved_at: 2026-07-01T10:10:00Z\nlast_modified: 2026-07-01T10:10:00Z\napproved_fingerprint: %s\nsource_requirements_approved_at: 2026-07-01T10:00:00Z\nsource_requirements_fingerprint: %s\n---\n\n%s", designFP, requirementsFP, design))
	writeDoc(t, root, name, "tasks.md", fmt.Sprintf("---\nstatus: approved\napproved_at: 2026-07-01T10:20:00Z\nlast_modified: 2026-07-01T10:20:00Z\napproved_fingerprint: %s\nsource_design_approved_at: 2026-07-01T10:10:00Z\nsource_design_fingerprint: %s\n---\n\n%s", spec.Fingerprint("tasks.md", plan), designFP, plan))
}

func completePassTask(t *testing.T, root, name, id string) {
	t.Helper()
	runner := testutil.NewFakeRunner(testutil.Response{Stdout: "ok", ExitCode: 0})
	if _, err := workflow.CompleteTask(context.Background(), root, name, id, runner); err != nil {
		t.Fatalf("complete task %s of %s: %v", id, name, err)
	}
}

// recordedBeforeDeclaration builds the feature with a plan that declares no
// delivery, completes both tasks on the tree as it is, and then declares the
// delivery: the recorded pass of task 1 predates the declaration.
func recordedBeforeDeclaration(t *testing.T, root, name string) {
	t.Helper()
	writeSealedFeature(t, root, name, passRequirements, passPlan(false, false, passCommand))
	completePassTask(t, root, name, "1")
	completePassTask(t, root, name, "2")
	writeSealedFeature(t, root, name, passRequirements, passPlan(true, true, passCommand))
}

// editPassRecord changes one record of the ledger as a hand would.
func editPassRecord(t *testing.T, root, name, id string, change func(record map[string]any)) {
	t.Helper()
	path := evidence.DocumentPath(root, name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the ledger: %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("decode the ledger: %v", err)
	}
	change(document["tasks"].(map[string]any)[id].(map[string]any))
	data, err = json.MarshalIndent(document, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write the ledger: %v", err)
	}
}

func planOf(t *testing.T, root, name string) (PlanReport, FeaturePlan) {
	t.Helper()
	report, err := Plan(context.Background(), root, name)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(report.Features) != 1 {
		t.Fatalf("the plan holds %d feature(s), want one", len(report.Features))
	}
	return report, report.Features[0]
}

func unsealedOf(t *testing.T, plan FeaturePlan, id string) UnsealedPass {
	t.Helper()
	for _, pass := range plan.UnsealedPasses {
		if pass.TaskID == id {
			return pass
		}
	}
	t.Fatalf("task %s is not among the unsealed passes: sealable %v, unsealed %v", id, plan.SealablePasses, plan.UnsealedPasses)
	return UnsealedPass{}
}

const proveAgain = "walden verify " + passFeature + " --task 1"

func TestPlanSealablePasses(t *testing.T) {
	t.Run("sealable", func(t *testing.T) {
		root := isolatedAdoptRepo(t)
		head := headOf(t, root)
		recordedBeforeDeclaration(t, root, passFeature)

		report, plan := planOf(t, root, passFeature)
		if want := []PassToSeal{{TaskID: "1", Commit: head}}; !reflect.DeepEqual(plan.SealablePasses, want) {
			t.Errorf("sealable passes = %+v, want %+v (unsealed %+v)", plan.SealablePasses, want, plan.UnsealedPasses)
		}
		if len(plan.UnsealedPasses) != 0 {
			t.Errorf("unsealed passes = %+v", plan.UnsealedPasses)
		}
		// A pass to seal is adoption work: the feature is not complete, and
		// the totals count it. Nothing is proven again.
		if plan.Class != ClassBackfill || report.Totals.Backfill != 1 || report.Totals.SealablePasses != 1 {
			t.Errorf("class %s, totals %+v, want backfill with one pass to seal", plan.Class, report.Totals)
		}
		if plan.ReproveCount != 0 || plan.Workload.Tasks != 0 || plan.Workload.Steps != 0 || report.Totals.ReproveTasks != 0 {
			t.Errorf("re-prove count %d, workload %+v: sealing executes nothing", plan.ReproveCount, plan.Workload)
		}

		// A pass recorded after the plan declared the delivery is not
		// offered: walden verify attests it on a committed tree.
		root = isolatedAdoptRepo(t)
		writeSealedFeature(t, root, passFeature, passRequirements, passPlan(true, false, passCommand))
		writeRepoFile(t, root, "draft.txt", "not committed\n")
		completePassTask(t, root, passFeature, "1")
		completePassTask(t, root, passFeature, "2")
		commitRepo(t, root, "the draft, unchanged")
		report, plan = planOf(t, root, passFeature)
		if len(plan.SealablePasses) != 0 || len(plan.UnsealedPasses) != 0 || report.Totals.SealablePasses != 0 {
			t.Errorf("a pass recorded after the declaration is offered: sealable %+v, unsealed %+v", plan.SealablePasses, plan.UnsealedPasses)
		}
		if plan.Class != ClassComplete {
			t.Errorf("class %s, want complete: both tasks are verified", plan.Class)
		}
	})

	t.Run("read-only", func(t *testing.T) {
		first := isolatedAdoptRepo(t)
		recordedBeforeDeclaration(t, first, passFeature)

		// The completions above captured a profile for that root, so a probe
		// could never run there. The repository is moved to a root this
		// process has not profiled, and declares a probe that writes a file.
		root := filepath.Join(t.TempDir(), "repo")
		if err := os.Rename(first, root); err != nil {
			t.Fatalf("move the repository: %v", err)
		}
		sentinel := filepath.Join(t.TempDir(), "probe-ran")
		probe, _ := json.Marshal([]string{"touch", sentinel})
		writeRepoFile(t, root, ".walden/environment.md", "- sentinel: "+string(probe)+"\n")

		before := hashWaldenTree(t, root)
		_, plan := planOf(t, root, passFeature)
		if len(plan.SealablePasses) != 1 {
			t.Fatalf("the fixture must hold a pass to seal: %+v", plan)
		}
		if after := hashWaldenTree(t, root); after != before {
			t.Error("the plan changed the ledger or the documents")
		}
		if _, err := os.Stat(sentinel); err == nil {
			t.Error("the plan ran the environment probe")
		}
	})

	t.Run("no-commit", func(t *testing.T) {
		// The pass ran on a tree with an uncommitted file, and the file was
		// changed before it was committed: no commit holds that tree.
		root := isolatedAdoptRepo(t)
		writeRepoFile(t, root, "draft.txt", "as proven\n")
		writeSealedFeature(t, root, passFeature, passRequirements, passPlan(false, false, passCommand))
		completePassTask(t, root, passFeature, "1")
		completePassTask(t, root, passFeature, "2")
		writeRepoFile(t, root, "draft.txt", "changed before the commit\n")
		commitRepo(t, root, "the draft, changed")
		writeSealedFeature(t, root, passFeature, passRequirements, passPlan(true, true, passCommand))

		_, plan := planOf(t, root, passFeature)
		pass := unsealedOf(t, plan, "1")
		if pass.Reason != ReasonNoCommit || !strings.Contains(pass.Detail, proveAgain) || !strings.Contains(pass.Detail, "2 commit(s)") {
			t.Errorf("unsealed pass = %+v, want no-commit over the two commits, with %q", pass, proveAgain)
		}
		if strings.Contains(pass.Detail, "limit") {
			t.Errorf("a history shorter than the limit is reported as cut: %s", pass.Detail)
		}
		if len(plan.SealablePasses) != 0 {
			t.Errorf("sealable passes = %+v", plan.SealablePasses)
		}

		// The same pass, with the file committed unchanged, is sealable at
		// the commit that holds it.
		root = isolatedAdoptRepo(t)
		writeRepoFile(t, root, "draft.txt", "as proven\n")
		writeSealedFeature(t, root, passFeature, passRequirements, passPlan(false, false, passCommand))
		completePassTask(t, root, passFeature, "1")
		completePassTask(t, root, passFeature, "2")
		head := commitRepo(t, root, "the draft, unchanged")
		writeSealedFeature(t, root, passFeature, passRequirements, passPlan(true, true, passCommand))
		_, plan = planOf(t, root, passFeature)
		if want := []PassToSeal{{TaskID: "1", Commit: head}}; !reflect.DeepEqual(plan.SealablePasses, want) {
			t.Errorf("committed unchanged: sealable passes = %+v, want %+v (unsealed %+v)", plan.SealablePasses, want, plan.UnsealedPasses)
		}
	})

	t.Run("limit", func(t *testing.T) {
		// Two features hold a pass no commit holds. A wrapper on gitRunner
		// answers the history listing with one commit more than the limit
		// and the tree listing of each with a tree of its own; everything
		// else goes to git.
		root := isolatedAdoptRepo(t)
		writeRepoFile(t, root, "draft.txt", "as proven\n")
		for _, name := range []string{"first-pass", "second-pass"} {
			writeSealedFeature(t, root, name, passRequirements, passPlan(false, false, passCommand))
			completePassTask(t, root, name, "1")
			completePassTask(t, root, name, "2")
		}
		writeRepoFile(t, root, "draft.txt", "changed before the commit\n")
		commitRepo(t, root, "the draft, changed")
		for _, name := range []string{"first-pass", "second-pass"} {
			writeSealedFeature(t, root, name, passRequirements, passPlan(true, true, passCommand))
		}

		synthetic := map[string]bool{}
		ids := make([]string, evidence.SealSearchLimit+1)
		for index := range ids {
			ids[index] = fmt.Sprintf("%040x", index+1)
			synthetic[ids[index]] = true
		}
		inner := gitRunner
		histories, trees := 0, 0
		gitRunner = diagnosticsRunnerFunc(func(ctx context.Context, name string, args ...string) (shell.Response, error) {
			if len(args) >= 4 && args[3] == "log" {
				histories++
				return shell.Response{Stdout: strings.Join(ids, "\n") + "\n"}, nil
			}
			if len(args) >= 4 && args[3] == "ls-tree" && synthetic[args[len(args)-1]] {
				trees++
				return shell.Response{Stdout: "100644 blob " + args[len(args)-1] + "\tmain.go\n"}, nil
			}
			return inner.Run(ctx, name, args...)
		})
		t.Cleanup(func() { gitRunner = inner })

		report, err := Plan(context.Background(), root, "")
		if err != nil {
			t.Fatalf("Plan: %v", err)
		}
		if len(report.Features) != 2 || report.Totals.UnsealedPasses != 2 {
			t.Fatalf("features %d, totals %+v, want two features with an unsealed pass each", len(report.Features), report.Totals)
		}
		for _, plan := range report.Features {
			pass := unsealedOf(t, plan, "1")
			if pass.Reason != ReasonNoCommit {
				t.Errorf("%s: reason %s, want no-commit", plan.Name, pass.Reason)
			}
			for _, want := range []string{fmt.Sprint(evidence.SealSearchLimit) + " commit(s)", "limit", "walden verify " + plan.Name + " --task 1"} {
				if !strings.Contains(pass.Detail, want) {
					t.Errorf("%s: the detail %q does not hold %q", plan.Name, pass.Detail, want)
				}
			}
		}
		// One pass over the history serves both features.
		if histories != 1 || trees != evidence.SealSearchLimit {
			t.Errorf("%d history listing(s) and %d tree listing(s), want one and %d", histories, trees, evidence.SealSearchLimit)
		}
	})

	t.Run("proof-changed", func(t *testing.T) {
		for name, command := range map[string]string{
			"a changed step":        `["grep", "-q", "announced", "notes.txt"]`,
			"an added expectation":  passCommand + "\n      expect_exit: 0",
			"a changed expectation": passCommand + "\n      expect_output: \"released\"",
		} {
			root := isolatedAdoptRepo(t)
			writeSealedFeature(t, root, passFeature, passRequirements, passPlan(false, false, passCommand))
			completePassTask(t, root, passFeature, "1")
			completePassTask(t, root, passFeature, "2")
			writeSealedFeature(t, root, passFeature, passRequirements, passPlan(true, true, command))

			report, plan := planOf(t, root, passFeature)
			pass := unsealedOf(t, plan, "1")
			if pass.Reason != ReasonProofChanged || len(plan.SealablePasses) != 0 {
				t.Errorf("%s: unsealed %+v, sealable %+v, want proof-changed", name, pass, plan.SealablePasses)
			}
			// The task is proven again by apply, as any stale task.
			if plan.Class != ClassReprove || plan.ReproveCount != 1 || report.Totals.ReproveTasks != 1 {
				t.Errorf("%s: class %s, re-prove count %d", name, plan.Class, plan.ReproveCount)
			}
			if strings.Contains(pass.Detail, "--task") || !strings.Contains(pass.Detail, "--apply") {
				t.Errorf("%s: the detail %q must say that apply proves the task again", name, pass.Detail)
			}
		}

		// A covered statement changed with the declaration.
		root := isolatedAdoptRepo(t)
		writeSealedFeature(t, root, passFeature, passRequirements, passPlan(false, false, passCommand))
		completePassTask(t, root, passFeature, "1")
		completePassTask(t, root, passFeature, "2")
		changed := strings.Replace(passPlan(true, true, passCommand), `covers: ["R1.AC1"]`, `covers: ["R1.AC1", "NFR1"]`, 1)
		writeSealedFeature(t, root, passFeature, passRequirements, changed)
		if _, plan := planOf(t, root, passFeature); unsealedOf(t, plan, "1").Reason != ReasonProofChanged {
			t.Errorf("a changed covered statement: %+v", plan.UnsealedPasses)
		}
	})

	t.Run("no-provenance", func(t *testing.T) {
		root := isolatedAdoptRepo(t)
		recordedBeforeDeclaration(t, root, passFeature)
		editPassRecord(t, root, passFeature, "1", func(record map[string]any) { delete(record, "execution") })

		report, plan := planOf(t, root, passFeature)
		pass := unsealedOf(t, plan, "1")
		if pass.Reason != ReasonNoProvenance || len(plan.SealablePasses) != 0 {
			t.Errorf("unsealed %+v, sealable %+v, want no-provenance", pass, plan.SealablePasses)
		}
		// The task is counted among those to prove again, as today.
		if plan.Class != ClassReprove || plan.ReproveCount != 1 || report.Totals.ReproveTasks != 1 || plan.Workload.Steps != 1 {
			t.Errorf("class %s, re-prove count %d, workload %+v", plan.Class, plan.ReproveCount, plan.Workload)
		}

		// A failed record is offered nothing: it holds no pass.
		root = isolatedAdoptRepo(t)
		recordedBeforeDeclaration(t, root, passFeature)
		editPassRecord(t, root, passFeature, "1", func(record map[string]any) {
			record["result"] = "failed"
			record["execution"].(map[string]any)["assertion_result"] = "failed"
		})
		_, plan = planOf(t, root, passFeature)
		if len(plan.SealablePasses) != 0 || len(plan.UnsealedPasses) != 0 {
			t.Errorf("a failed record: sealable %+v, unsealed %+v, want neither", plan.SealablePasses, plan.UnsealedPasses)
		}
		if plan.ReproveCount != 1 {
			t.Errorf("a failed record: re-prove count %d, want one", plan.ReproveCount)
		}
	})

	t.Run("statements", func(t *testing.T) {
		// The requirements are approved again after the pass. NFR1, which
		// the task does not name, is reworded.
		reworded := strings.Replace(passRequirements, "The notes stay readable.", "The notes stay readable on every screen.", 1)
		// R1.AC1, which it names, is reworded.
		renamed := strings.Replace(passRequirements, "the system SHALL name the release.", "the system SHALL name the release and its date.", 1)

		cases := []struct {
			name         string
			requirements string
			keepFacts    bool
			sealable     bool
		}{
			{"under the current approval, without statement facts", passRequirements, false, true},
			{"an earlier approval, without statement facts", reworded, false, false},
			{"an earlier approval, with equal statement fingerprints", reworded, true, true},
			{"an earlier approval, a named statement reworded", renamed, true, false},
		}
		for _, tc := range cases {
			root := isolatedAdoptRepo(t)
			head := headOf(t, root)
			writeSealedFeature(t, root, passFeature, passRequirements, passPlan(false, false, passCommand))
			completePassTask(t, root, passFeature, "1")
			completePassTask(t, root, passFeature, "2")
			if !tc.keepFacts {
				editPassRecord(t, root, passFeature, "1", func(record map[string]any) { delete(record, "statements") })
			}
			writeSealedFeature(t, root, passFeature, tc.requirements, passPlan(true, true, passCommand))

			_, plan := planOf(t, root, passFeature)
			if tc.sealable {
				if want := []PassToSeal{{TaskID: "1", Commit: head}}; !reflect.DeepEqual(plan.SealablePasses, want) {
					t.Errorf("%s: sealable passes = %+v, want %+v (unsealed %+v)", tc.name, plan.SealablePasses, want, plan.UnsealedPasses)
				}
				continue
			}
			pass := unsealedOf(t, plan, "1")
			if pass.Reason != ReasonStatementsUnknown || !strings.Contains(pass.Detail, proveAgain) || len(plan.SealablePasses) != 0 {
				t.Errorf("%s: unsealed %+v, sealable %+v, want statements-unknown with %q", tc.name, pass, plan.SealablePasses, proveAgain)
			}
		}
	})

	t.Run("unsealed alone", func(t *testing.T) {
		// A feature whose only open work is a pass left unsealed: it is not
		// complete, and apply has nothing to prove for it.
		root := isolatedAdoptRepo(t)
		writeRepoFile(t, root, "draft.txt", "as proven\n")
		writeSealedFeature(t, root, passFeature, passRequirements, passPlan(false, false, passCommand))
		completePassTask(t, root, passFeature, "1")
		writeRepoFile(t, root, "draft.txt", "changed before the commit\n")
		commitRepo(t, root, "the draft, changed")
		completePassTask(t, root, passFeature, "2") // verified on the final tree
		writeSealedFeature(t, root, passFeature, passRequirements, passPlan(true, true, passCommand))

		report, plan := planOf(t, root, passFeature)
		if unsealedOf(t, plan, "1").Reason != ReasonNoCommit {
			t.Fatalf("the fixture must hold a pass no commit holds: %+v", plan.UnsealedPasses)
		}
		if plan.Class != ClassReprove || report.Totals.Reprove != 1 || report.Totals.Complete != 0 {
			t.Errorf("class %s, totals %+v, want re-prove", plan.Class, report.Totals)
		}
		if plan.ReproveCount != 0 || plan.Workload.Tasks != 0 || plan.Workload.Steps != 0 || !plan.Workload.Available {
			t.Errorf("re-prove count %d, workload %+v: apply executes nothing for an unsealed pass", plan.ReproveCount, plan.Workload)
		}
	})
}
