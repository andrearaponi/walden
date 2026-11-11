package workflow

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/spec"
	"github.com/andrearaponi/walden/internal/testutil"
)

const deliveryFeature = "delivery-demo"

// deliveryPlan holds a delivery leaf and a regression leaf.
const deliveryPlan = `---
status: approved
approved_at: 2026-03-23T10:00:00Z
last_modified: 2026-03-23T10:00:00Z
source_design_approved_at: 2026-03-23T09:30:00Z
---

# Implementation Plan

- [ ] 1. Publish the notes
  - Requirements: ` + "`R1.AC1`" + `
  - Design: Overview
  - Proof: delivery
  - Verification:
    - command: ["grep", "-q", "released", "notes.txt"]
      covers: ["R1.AC1"]
- [ ] 2. Keep the parser working
  - Requirements: ` + "`R1.AC2`" + `
  - Design: Overview
  - Verification:
    - command: ["go", "test", "./..."]
      covers: ["R1.AC2"]
`

// deliveryRepo is an isolated repository with one commit of code, an ignore
// rule for *.log, and the delivery feature under .walden/, which is outside
// the code identity whether it is committed or not.
func deliveryRepo(t *testing.T) (root, head string) {
	t.Helper()
	root = factsRepo(t)
	factsFile(t, root, "main.go", "package main\n")
	factsFile(t, root, "notes.txt", "released\n")
	factsFile(t, root, ".gitignore", "*.log\n")
	factsGit(t, root, "add", ".")
	factsGit(t, root, "commit", "-qm", "base")
	setupApprovedFeature(t, root, deliveryFeature, deliveryPlan)
	return root, factsGit(t, root, "rev-parse", "HEAD")
}

func deliveryRecord(t *testing.T, root, id string) evidence.Record {
	t.Helper()
	ledger, err := evidence.Load(root, deliveryFeature)
	if err != nil {
		t.Fatalf("load evidence: %v", err)
	}
	record, recorded := ledger.Tasks[id]
	if !recorded {
		t.Fatalf("task %s has no record", id)
	}
	return record
}

func passing() *testutil.FakeRunner {
	return testutil.NewFakeRunner(testutil.Response{Stdout: "ok", ExitCode: 0})
}

func completeDelivery(t *testing.T, root, id string) {
	t.Helper()
	if _, err := CompleteTask(context.Background(), root, deliveryFeature, id, passing()); err != nil {
		t.Fatalf("complete task %s: %v", id, err)
	}
}

func verifyDelivery(t *testing.T, root, id string, runner *testutil.FakeRunner) VerifyResult {
	t.Helper()
	result, err := VerifyTask(context.Background(), root, deliveryFeature, id, false, runner)
	if err != nil {
		t.Fatalf("verify task %s: %v", id, err)
	}
	return result
}

// declaredFingerprint is the fingerprint of the delivery leaf as the plan
// declares it: what an attestation written by an execution attests.
func declaredFingerprint(t *testing.T, root string) string {
	t.Helper()
	tree, err := spec.LoadTaskTree(root, deliveryFeature)
	if err != nil {
		t.Fatalf("load the plan: %v", err)
	}
	return spec.TaskDefinitionFingerprint(tree.LeafTasks()[0])
}

func requireAttested(t *testing.T, label string, record evidence.Record, commit, fingerprint string) {
	t.Helper()
	if record.Result != evidence.ResultPassed {
		t.Fatalf("%s: the record holds %s, want a pass", label, record.Result)
	}
	attestation := record.Attestation
	if attestation == nil {
		t.Fatalf("%s: the pass on a committed tree is not attested (tree %+v)", label, record.Tree)
	}
	if attestation.Commit != commit {
		t.Errorf("%s: attested at %s, want HEAD %s", label, attestation.Commit, commit)
	}
	if attestation.TaskFingerprint != fingerprint || attestation.Sealed {
		t.Errorf("%s: attestation %+v, want the declared definition %s, not sealed", label, *attestation, fingerprint)
	}
	if record.Statements == nil || attestation.Statements.Scheme != record.Statements.Scheme {
		t.Errorf("%s: attested statements %+v, record statements %+v", label, attestation.Statements, record.Statements)
	}
}

func requireUnattested(t *testing.T, label string, record evidence.Record, result string) {
	t.Helper()
	if record.Result != result {
		t.Errorf("%s: the record holds %s, want %s", label, record.Result, result)
	}
	if record.Attestation != nil {
		t.Errorf("%s: attested at %s", label, record.Attestation.Commit)
	}
}

func TestDeliveryAttestation(t *testing.T) {
	t.Run("committed", func(t *testing.T) {
		// task complete on a committed tree. An ignored file and the changes
		// under .walden/ do not make the tree uncommitted.
		root, head := deliveryRepo(t)
		factsFile(t, root, "build.log", "noise\n")
		completeDelivery(t, root, "1")
		requireAttested(t, "task complete", deliveryRecord(t, root, "1"), head, declaredFingerprint(t, root))

		// verify, naming a task that was completed on uncommitted work and
		// committed since: it holds a pass and no attestation.
		root, _ = deliveryRepo(t)
		factsFile(t, root, "notes.txt", "released\nand announced\n")
		completeDelivery(t, root, "1")
		requireUnattested(t, "completed on uncommitted work", deliveryRecord(t, root, "1"), evidence.ResultPassed)
		factsGit(t, root, "add", "-A")
		factsGit(t, root, "commit", "-qm", "the delivery")
		head = factsGit(t, root, "rev-parse", "HEAD")
		factsFile(t, root, "build.log", "noise\n")
		factsFile(t, root, ".walden/scratch.txt", "local\n")
		verifyDelivery(t, root, "1", passing())
		requireAttested(t, "verify", deliveryRecord(t, root, "1"), head, declaredFingerprint(t, root))
	})

	t.Run("uncommitted", func(t *testing.T) {
		root, _ := deliveryRepo(t)
		factsFile(t, root, "main.go", "package main // edited\n")
		factsFile(t, root, "new.txt", "new\n")

		completeDelivery(t, root, "1")
		record := deliveryRecord(t, root, "1")
		requireUnattested(t, "task complete", record, evidence.ResultPassed)
		if record.Tree == nil || len(record.Tree.Paths) != 2 {
			t.Fatalf("the fixture must record two differing paths: %+v", record.Tree)
		}

		verifyDelivery(t, root, "1", passing())
		record = deliveryRecord(t, root, "1")
		requireUnattested(t, "verify", record, evidence.ResultPassed)
		if record.Execution == nil || record.Execution.Origin != "verify" {
			t.Fatalf("the second record is not a verify record: %+v", record.Execution)
		}
	})

	t.Run("no-commit", func(t *testing.T) {
		root := factsRepo(t)
		factsFile(t, root, "notes.txt", "released\n")
		setupApprovedFeature(t, root, deliveryFeature, deliveryPlan)

		completeDelivery(t, root, "1")
		record := deliveryRecord(t, root, "1")
		requireUnattested(t, "task complete", record, evidence.ResultPassed)
		if record.Tree != nil {
			t.Errorf("a repository without a commit records tree facts: %+v", record.Tree)
		}
	})

	t.Run("no-git", func(t *testing.T) {
		root := t.TempDir()
		t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
		t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
		// Keep git from finding a repository above the temporary directory.
		t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))
		factsFile(t, root, "notes.txt", "released\n")
		setupApprovedFeature(t, root, deliveryFeature, deliveryPlan)

		completeDelivery(t, root, "1")
		record := deliveryRecord(t, root, "1")
		requireUnattested(t, "task complete", record, evidence.ResultPassed)
		if record.CodeIdentity != "" {
			t.Errorf("a directory without git records a code identity: %s", record.CodeIdentity)
		}
	})

	t.Run("never attested", func(t *testing.T) {
		// A regression proof on a committed tree.
		root, _ := deliveryRepo(t)
		completeDelivery(t, root, "1")
		completeDelivery(t, root, "2")
		requireUnattested(t, "a regression task", deliveryRecord(t, root, "2"), evidence.ResultPassed)
		verifyDelivery(t, root, "2", passing())
		requireUnattested(t, "a regression task after verify", deliveryRecord(t, root, "2"), evidence.ResultPassed)

		// A delivery proof that fails on a committed tree.
		root, _ = deliveryRepo(t)
		factsFile(t, root, "notes.txt", "released\nand announced\n")
		completeDelivery(t, root, "1")
		factsGit(t, root, "add", "-A")
		factsGit(t, root, "commit", "-qm", "the delivery")
		result := verifyDelivery(t, root, "1", testutil.NewFakeRunner(testutil.Response{Stderr: "no match", ExitCode: 1}))
		if len(result.Failed) != 1 {
			t.Fatalf("the proof must fail: %+v", result)
		}
		requireUnattested(t, "a failed delivery proof", deliveryRecord(t, root, "1"), evidence.ResultFailed)
	})
}
