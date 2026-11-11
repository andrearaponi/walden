package workflow

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/testutil"
)

// rawDeliveryRecord returns the bytes of one record as the ledger file holds
// them: what "the record is unchanged" means.
func rawDeliveryRecord(t *testing.T, root, id string) string {
	t.Helper()
	data, err := os.ReadFile(evidence.DocumentPath(root, deliveryFeature))
	if err != nil {
		t.Fatalf("read the ledger: %v", err)
	}
	var ledger struct {
		Tasks map[string]json.RawMessage `json:"tasks"`
	}
	if err := json.Unmarshal(data, &ledger); err != nil {
		t.Fatalf("decode the ledger: %v", err)
	}
	if _, recorded := ledger.Tasks[id]; !recorded {
		t.Fatalf("task %s has no record", id)
	}
	return string(ledger.Tasks[id])
}

// provedCommands returns the first argument of each proof the runner ran:
// the delivery proof runs grep, the regression proof runs go.
func provedCommands(runner *testutil.FakeRunner) []string {
	names := []string{}
	for _, call := range runner.Calls() {
		names = append(names, call.Name)
	}
	return names
}

func verifyFeature(t *testing.T, root string, all bool, runner *testutil.FakeRunner) VerifyResult {
	t.Helper()
	result, err := Verify(context.Background(), root, deliveryFeature, all, false, runner)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	return result
}

// undeclaredDeliveryPlan is deliveryPlan before it declared the delivery.
var undeclaredDeliveryPlan = strings.Replace(deliveryPlan, "  - Proof: delivery\n", "", 1)

// awaitingRepo is a committed repository whose task 1 was completed before
// the plan declared its proof as a delivery proof: its recorded pass awaits
// adoption. Task 2 is a verified regression task.
func awaitingRepo(t *testing.T) string {
	t.Helper()
	root := factsRepo(t)
	factsFile(t, root, "main.go", "package main\n")
	factsFile(t, root, "notes.txt", "released\n")
	factsGit(t, root, "add", ".")
	factsGit(t, root, "commit", "-qm", "base")
	setupApprovedFeature(t, root, deliveryFeature, undeclaredDeliveryPlan)
	completeDelivery(t, root, "1")
	completeDelivery(t, root, "2")

	// The plan now declares the delivery, and both tasks stay completed.
	declared := strings.NewReplacer("- [ ] 1.", "- [x] 1.", "- [ ] 2.", "- [x] 2.").Replace(deliveryPlan)
	setupApprovedFeature(t, root, deliveryFeature, declared)
	if record := deliveryRecord(t, root, "1"); record.Attestation != nil {
		t.Fatal("the fixture record must hold no attestation")
	}
	return root
}

// TestVerifyKeepsDelivered: no run without a named task executes the proof
// of a delivered task, with or without every task asked for.
func TestVerifyKeepsDelivered(t *testing.T) {
	for _, all := range []bool{false, true} {
		root, head := deliveryRepo(t)
		completeDelivery(t, root, "1")
		completeDelivery(t, root, "2")
		// The delivered content moves on: the delivery proof would fail now,
		// and the regression task is stale.
		factsFile(t, root, "notes.txt", "superseded\n")
		factsGit(t, root, "add", "-A")
		factsGit(t, root, "commit", "-qm", "the notes move on")
		before := rawDeliveryRecord(t, root, "1")

		runner := testutil.NewFakeRunner(testutil.Response{Stdout: "ok"})
		result := verifyFeature(t, root, all, runner)

		if got := provedCommands(runner); !reflect.DeepEqual(got, []string{"go"}) {
			t.Errorf("all=%t: proofs run = %v, want the regression proof alone", all, got)
		}
		if want := []KeptTask{{TaskID: "1", Commit: head}}; !reflect.DeepEqual(result.Kept, want) {
			t.Errorf("all=%t: kept = %+v, want %+v", all, result.Kept, want)
		}
		if len(result.Outcomes) != 1 || result.Outcomes[0].TaskID != "2" || len(result.Skipped) != 0 || len(result.Left) != 0 {
			t.Errorf("all=%t: outcomes %+v, skipped %v, left %v", all, result.Outcomes, result.Skipped, result.Left)
		}
		if after := rawDeliveryRecord(t, root, "1"); after != before {
			t.Errorf("all=%t: the record of the delivered task changed:\n%s\nwas\n%s", all, after, before)
		}
	}
}

// TestVerifyLeavesAwaitingPass: a pass that awaits adoption is replaced only
// by a run that names its task.
func TestVerifyLeavesAwaitingPass(t *testing.T) {
	for _, tc := range []struct {
		name      string
		all       bool
		committed bool
	}{
		{"default, committed tree", false, true},
		{"every task, committed tree", true, true},
		{"default, uncommitted paths", false, false},
		{"every task, uncommitted paths", true, false},
	} {
		root := awaitingRepo(t)
		if !tc.committed {
			factsFile(t, root, "draft.txt", "not committed\n")
		}
		before := rawDeliveryRecord(t, root, "1")

		// One response at most is needed: the regression proof, when it runs.
		runner := testutil.NewFakeRunner(testutil.Response{Stdout: "ok"})
		result := verifyFeature(t, root, tc.all, runner)

		for _, name := range provedCommands(runner) {
			if name == "grep" {
				t.Errorf("%s: the proof of the awaiting pass ran", tc.name)
			}
		}
		if !reflect.DeepEqual(result.Left, []string{"1"}) {
			t.Errorf("%s: left = %v, want task 1", tc.name, result.Left)
		}
		for _, outcome := range result.Outcomes {
			if outcome.TaskID == "1" {
				t.Errorf("%s: the run reports an outcome for task 1: %+v", tc.name, outcome)
			}
		}
		if after := rawDeliveryRecord(t, root, "1"); after != before {
			t.Errorf("%s: the awaiting record changed:\n%s\nwas\n%s", tc.name, after, before)
		}
	}

	// A run that names the task executes its proof and replaces the pass.
	root := awaitingRepo(t)
	before := rawDeliveryRecord(t, root, "1")
	runner := passing()
	verifyDelivery(t, root, "1", runner)
	if got := provedCommands(runner); !reflect.DeepEqual(got, []string{"grep"}) {
		t.Errorf("a named run: proofs run = %v, want the delivery proof", got)
	}
	if after := rawDeliveryRecord(t, root, "1"); after == before {
		t.Error("a named run left the record as it was")
	}
	if record := deliveryRecord(t, root, "1"); record.Attestation == nil {
		t.Errorf("a named run on a committed tree did not attest the delivery: %+v", record.Tree)
	}
}

// TestVerifyAttestsOnCommittedTree: a delivery task that holds no attestation
// is proven by walden verify once the tree is committed, although it reads
// verified. The verify run of adoption does not prove it.
func TestVerifyAttestsOnCommittedTree(t *testing.T) {
	root, _ := deliveryRepo(t)
	factsFile(t, root, "notes.txt", "released\nand announced\n")
	completeDelivery(t, root, "1")
	completeDelivery(t, root, "2")
	if record := deliveryRecord(t, root, "1"); record.Attestation != nil {
		t.Fatal("the fixture must complete the delivery on uncommitted work")
	}

	// Before the commit the task is verified and skipped, like task 2.
	runner := testutil.NewFakeRunner()
	result := verifyFeature(t, root, false, runner)
	if len(runner.Calls()) != 0 || !reflect.DeepEqual(result.Skipped, []string{"1", "2"}) {
		t.Fatalf("with an uncommitted path: proofs run %v, skipped %v, want none run and both skipped", provedCommands(runner), result.Skipped)
	}

	// The work is committed unchanged: both tasks still read verified.
	factsGit(t, root, "add", "-A")
	factsGit(t, root, "commit", "-qm", "the delivery")
	head := factsGit(t, root, "rev-parse", "HEAD")

	// The verify run of adoption proves only tasks that are not verified.
	runner = testutil.NewFakeRunner()
	adoption, err := VerifyWithOptions(context.Background(), root, deliveryFeature, false, false, runner, VerifyOptions{})
	if err != nil {
		t.Fatalf("the verify run of adoption: %v", err)
	}
	if len(runner.Calls()) != 0 || !reflect.DeepEqual(adoption.Skipped, []string{"1", "2"}) {
		t.Fatalf("the verify run of adoption: proofs run %v, skipped %v", provedCommands(runner), adoption.Skipped)
	}

	// walden verify proves the delivery task and attests it.
	runner = passing()
	result = verifyFeature(t, root, false, runner)
	if got := provedCommands(runner); !reflect.DeepEqual(got, []string{"grep"}) {
		t.Fatalf("on the committed tree: proofs run = %v, want the delivery proof alone", got)
	}
	if !reflect.DeepEqual(result.Skipped, []string{"2"}) || len(result.Outcomes) != 1 {
		t.Errorf("skipped %v, outcomes %+v", result.Skipped, result.Outcomes)
	}
	if outcome := result.Outcomes[0]; outcome.TaskID != "1" || outcome.State != evidence.StateDelivered || !outcome.Passed {
		t.Errorf("outcome %+v, want task 1 delivered", outcome)
	}
	if record := deliveryRecord(t, root, "1"); record.Attestation == nil || record.Attestation.Commit != head {
		t.Errorf("attestation %+v, want the commit %s", record.Attestation, head)
	}

	// From now on the task is kept.
	runner = testutil.NewFakeRunner()
	result = verifyFeature(t, root, false, runner)
	if len(runner.Calls()) != 0 || len(result.Kept) != 1 || result.Kept[0].TaskID != "1" {
		t.Errorf("after the attestation: proofs run %v, kept %+v", provedCommands(runner), result.Kept)
	}
}

// TestVerifyOpenLeavesDeliveredOut: after a run that names a task, a
// delivered task is not among the completed tasks that are not verified.
func TestVerifyOpenLeavesDeliveredOut(t *testing.T) {
	root, _ := deliveryRepo(t)
	completeDelivery(t, root, "1")
	completeDelivery(t, root, "2")

	result := verifyDelivery(t, root, "2", testutil.NewFakeRunner(testutil.Response{Stdout: "ok"}))
	if len(result.Open) != 0 {
		t.Errorf("open = %v: the delivered task is reported as not verified", result.Open)
	}
	for _, warning := range result.Warnings {
		if strings.Contains(warning, "not verified") {
			t.Errorf("the run warns about tasks that are not verified: %q", warning)
		}
	}
}
