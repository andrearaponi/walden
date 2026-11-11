package adopt

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/spec"
	"github.com/andrearaponi/walden/internal/testutil"
)

func applyTo(t *testing.T, root, name string, runner *testutil.FakeRunner) (ApplyReport, FeatureAdoption) {
	t.Helper()
	report, err := Apply(context.Background(), root, name, runner, nil)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(report.Features) != 1 {
		t.Fatalf("the run holds %d feature(s), want one", len(report.Features))
	}
	return report, report.Features[0]
}

// rawPassRecord returns the fields of one record as the ledger file holds
// them, each as raw JSON.
func rawPassRecord(t *testing.T, root, name, id string) map[string]json.RawMessage {
	t.Helper()
	data, err := os.ReadFile(evidence.DocumentPath(root, name))
	if err != nil {
		t.Fatalf("read the ledger: %v", err)
	}
	var ledger struct {
		Tasks map[string]map[string]json.RawMessage `json:"tasks"`
	}
	if err := json.Unmarshal(data, &ledger); err != nil {
		t.Fatalf("decode the ledger: %v", err)
	}
	record, recorded := ledger.Tasks[id]
	if !recorded {
		t.Fatalf("task %s has no record", id)
	}
	return record
}

func provedBy(runner *testutil.FakeRunner) []string {
	names := []string{}
	for _, call := range runner.Calls() {
		names = append(names, call.Name)
	}
	return names
}

func stateOf(t *testing.T, root, name, id string) evidence.TaskEvidence {
	t.Helper()
	_, plan := planOf(t, root, name)
	for _, entry := range plan.Evidence {
		if entry.TaskID == id {
			return entry
		}
	}
	t.Fatalf("task %s has no evidence entry", id)
	return evidence.TaskEvidence{}
}

func TestApplySealsPasses(t *testing.T) {
	t.Run("sealed-without-execution", func(t *testing.T) {
		root := isolatedAdoptRepo(t)
		head := headOf(t, root)
		recordedBeforeDeclaration(t, root, passFeature)
		_, planned := planOf(t, root, passFeature)

		runner := testutil.NewFakeRunner()
		report, adoption := applyTo(t, root, passFeature, runner)
		if calls := runner.Calls(); len(calls) != 0 {
			t.Errorf("apply ran proofs: %v", calls)
		}
		// The task reads delivered, with the commit the plan named.
		if !reflect.DeepEqual(adoption.SealedPasses, planned.SealablePasses) || len(adoption.SealedPasses) != 1 || adoption.SealedPasses[0].Commit != head {
			t.Errorf("sealed %+v, planned %+v, want task 1 at %s", adoption.SealedPasses, planned.SealablePasses, head)
		}
		if report.Totals.SealedPasses != 1 || report.Totals.Failed != 0 || report.Totals.Errors != 0 || len(adoption.UnsealedPasses) != 0 {
			t.Errorf("totals %+v, unsealed %+v", report.Totals, adoption.UnsealedPasses)
		}
		entry := stateOf(t, root, passFeature, "1")
		if entry.State != evidence.StateDelivered || entry.Attestation == nil || entry.Attestation.Commit != head || !entry.Attestation.Sealed {
			t.Errorf("after apply task 1 reads %s at %+v, want delivered at %s, sealed", entry.State, entry.Attestation, head)
		}

		// The attestation holds the declared definition and the statements
		// the task names, and the ledger carries the later version.
		ledger, err := evidence.Load(root, passFeature)
		if err != nil {
			t.Fatalf("load the ledger: %v", err)
		}
		tree, err := spec.LoadTaskTree(root, passFeature)
		if err != nil {
			t.Fatalf("load the plan: %v", err)
		}
		attestation := ledger.Tasks["1"].Attestation
		if attestation == nil || attestation.TaskFingerprint != spec.TaskDefinitionFingerprint(tree.LeafTasks()[0]) {
			t.Fatalf("attestation %+v does not attest the declared definition", attestation)
		}
		if want := spec.StatementFingerprints(passRequirements)["R1.AC1"]; len(attestation.Statements.Fingerprints) != 1 || attestation.Statements.Fingerprints["R1.AC1"] != want {
			t.Errorf("attested statements %v, want R1.AC1 as it reads", attestation.Statements.Fingerprints)
		}
		if ledger.SchemaVersion != evidence.AttestedSchemaVersion {
			t.Errorf("the ledger is %s, want %s", ledger.SchemaVersion, evidence.AttestedSchemaVersion)
		}
		if ledger.Tasks["2"].Attestation != nil {
			t.Error("the regression task was sealed")
		}
	})

	t.Run("record-unchanged", func(t *testing.T) {
		root := isolatedAdoptRepo(t)
		recordedBeforeDeclaration(t, root, passFeature)
		before := rawPassRecord(t, root, passFeature, "1")
		sibling := rawPassRecord(t, root, passFeature, "2")

		applyTo(t, root, passFeature, testutil.NewFakeRunner())

		after := rawPassRecord(t, root, passFeature, "1")
		if _, sealed := after["attestation"]; !sealed {
			t.Fatal("the record holds no attestation after apply")
		}
		for _, field := range []string{"result", "steps", "verified_at", "execution"} {
			if _, present := before[field]; !present {
				t.Fatalf("the fixture record holds no %s", field)
			}
		}
		// Every field the record held is byte-identical: sealing adds the
		// attestation and changes nothing else.
		for field, value := range before {
			if !bytes.Equal(after[field], value) {
				t.Errorf("sealing changed %s:\n%s\nwas\n%s", field, after[field], value)
			}
		}
		if len(after) != len(before)+1 {
			t.Errorf("the record gained %d field(s), want the attestation alone", len(after)-len(before))
		}
		if !reflect.DeepEqual(rawPassRecord(t, root, passFeature, "2"), sibling) {
			t.Error("sealing changed the record of another task")
		}
	})

	t.Run("second-apply", func(t *testing.T) {
		root := isolatedAdoptRepo(t)
		recordedBeforeDeclaration(t, root, passFeature)
		applyTo(t, root, passFeature, testutil.NewFakeRunner())
		before, err := os.ReadFile(evidence.DocumentPath(root, passFeature))
		if err != nil {
			t.Fatal(err)
		}

		runner := testutil.NewFakeRunner()
		report, adoption := applyTo(t, root, passFeature, runner)
		after, _ := os.ReadFile(evidence.DocumentPath(root, passFeature))
		if !bytes.Equal(before, after) {
			t.Error("a second apply changed the ledger")
		}
		if len(runner.Calls()) != 0 || len(adoption.SealedPasses) != 0 || report.Totals.SealedPasses != 0 || adoption.Class != ClassComplete {
			t.Errorf("a second apply: proofs %v, sealed %+v, class %s", runner.Calls(), adoption.SealedPasses, adoption.Class)
		}
	})

	t.Run("unsealed-not-proven", func(t *testing.T) {
		reworded := strings.Replace(passRequirements, "the system SHALL name the release.", "the system SHALL name the release and its date.", 1)
		for name, tc := range map[string]struct {
			reason  string
			arrange func(t *testing.T, root string)
		}{
			// No commit holds the tree of the pass. The regression task was
			// completed on the same tree and is stale after the commit.
			"no commit holds the tree": {ReasonNoCommit, func(t *testing.T, root string) {
				writeRepoFile(t, root, "draft.txt", "as proven\n")
				writeSealedFeature(t, root, passFeature, passRequirements, passPlan(false, false, passCommand))
				completePassTask(t, root, passFeature, "1")
				completePassTask(t, root, passFeature, "2")
				writeRepoFile(t, root, "draft.txt", "changed before the commit\n")
				commitRepo(t, root, "the draft, changed")
				writeSealedFeature(t, root, passFeature, passRequirements, passPlan(true, true, passCommand))
			}},
			// The statement the task names was reworded after the pass. The
			// requirements were approved again, so the regression task is
			// stale too.
			"the statements cannot be established": {ReasonStatementsUnknown, func(t *testing.T, root string) {
				writeSealedFeature(t, root, passFeature, passRequirements, passPlan(false, false, passCommand))
				completePassTask(t, root, passFeature, "1")
				completePassTask(t, root, passFeature, "2")
				writeSealedFeature(t, root, passFeature, reworded, passPlan(true, true, passCommand))
			}},
		} {
			root := isolatedAdoptRepo(t)
			tc.arrange(t, root)
			before := rawPassRecord(t, root, passFeature, "1")
			if entry := stateOf(t, root, passFeature, "2"); entry.State == evidence.StateVerified {
				t.Fatalf("%s: the fixture needs a regression task that is not verified", name)
			}

			runner := testutil.NewFakeRunner(testutil.Response{Stdout: "ok", ExitCode: 0})
			report, adoption := applyTo(t, root, passFeature, runner)

			// The other task is proven; the task of the unsealed pass is not.
			if got := provedBy(runner); !reflect.DeepEqual(got, []string{"go"}) {
				t.Errorf("%s: proofs run = %v, want the regression proof alone", name, got)
			}
			if !reflect.DeepEqual(adoption.Verified, []string{"2"}) || len(adoption.Failed) != 0 {
				t.Errorf("%s: verified %v, failed %v", name, adoption.Verified, adoption.Failed)
			}
			if len(adoption.UnsealedPasses) != 1 || adoption.UnsealedPasses[0].TaskID != "1" || adoption.UnsealedPasses[0].Reason != tc.reason {
				t.Errorf("%s: unsealed %+v, want task 1 for %s", name, adoption.UnsealedPasses, tc.reason)
			}
			if len(adoption.SealedPasses) != 0 || report.Totals.UnsealedPasses != 1 || report.Totals.Failed != 0 || report.Totals.Errors != 0 {
				t.Errorf("%s: sealed %+v, totals %+v", name, adoption.SealedPasses, report.Totals)
			}
			if after := rawPassRecord(t, root, passFeature, "1"); !reflect.DeepEqual(after, before) {
				t.Errorf("%s: the record of the unsealed pass changed", name)
			}
		}
	})

	t.Run("proven again as today", func(t *testing.T) {
		// A pass whose proof changed with the declaration, and a pass without
		// supported provenance, do not await adoption: apply proves their
		// task again, and a pass on the committed tree attests it.
		for name, arrange := range map[string]func(t *testing.T, root string){
			"the proof changed": func(t *testing.T, root string) {
				writeSealedFeature(t, root, passFeature, passRequirements, passPlan(false, false, passCommand))
				completePassTask(t, root, passFeature, "1")
				completePassTask(t, root, passFeature, "2")
				writeSealedFeature(t, root, passFeature, passRequirements, passPlan(true, true, `["grep", "-q", "announced", "notes.txt"]`))
			},
			"no supported provenance": func(t *testing.T, root string) {
				recordedBeforeDeclaration(t, root, passFeature)
				editPassRecord(t, root, passFeature, "1", func(record map[string]any) { delete(record, "execution") })
			},
		} {
			root := isolatedAdoptRepo(t)
			head := headOf(t, root)
			arrange(t, root)

			runner := testutil.NewFakeRunner(testutil.Response{Stdout: "ok", ExitCode: 0})
			_, adoption := applyTo(t, root, passFeature, runner)
			if got := provedBy(runner); !reflect.DeepEqual(got, []string{"grep"}) {
				t.Errorf("%s: proofs run = %v, want the delivery proof", name, got)
			}
			if len(adoption.SealedPasses) != 0 || !reflect.DeepEqual(adoption.Verified, []string{"1"}) {
				t.Errorf("%s: sealed %+v, verified %v", name, adoption.SealedPasses, adoption.Verified)
			}
			entry := stateOf(t, root, passFeature, "1")
			if entry.State != evidence.StateDelivered || entry.Attestation == nil || entry.Attestation.Commit != head || entry.Attestation.Sealed {
				t.Errorf("%s: after apply task 1 reads %s at %+v, want delivered at %s by execution", name, entry.State, entry.Attestation, head)
			}
		}
	})
}
