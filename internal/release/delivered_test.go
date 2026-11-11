package release

import (
	"context"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/testutil"
	"github.com/andrearaponi/walden/internal/workflow"
)

const deliveryGateFeature = "delivery-gate"

// deliveryGateRequirements defines three criteria: one that only the
// delivery proof covers, one that only the regression proof covers, one that
// both cover.
func deliveryGateRequirements() string {
	return draftReqHeader + "# Requirements Document\n\n## Introduction\n\nGate fixture.\n\n## Requirements\n\n### R1 Markers\n\n" +
		"**User Story:** As a tester, I want markers, so that the gate has proofs to judge.\n\n#### Acceptance Criteria\n\n" +
		"1. `R1.AC1` WHEN the notes are published, the system SHALL name the release.\n" +
		"2. `R1.AC2` WHEN the second proof runs, the system SHALL succeed.\n" +
		"3. `R1.AC3` WHEN either proof runs, the system SHALL succeed.\n"
}

func deliveryGateTasks() string {
	return draftTasksHeader + "# Implementation Plan\n\n- [ ] 1. Markers\n" +
		"  - [ ] 1.1 Publish the notes\n    - Requirements: `R1.AC1`, `R1.AC3`\n    - Design: Overview\n    - Proof: delivery\n    - Verification:\n" +
		"      - command: [\"grep\", \"-q\", \"released\", \"notes.txt\"]\n        covers: [\"R1.AC1\", \"R1.AC3\"]\n" +
		"  - [ ] 1.2 Keep the marker\n    - Requirements: `R1.AC2`, `R1.AC3`\n    - Design: Overview\n    - Verification:\n" +
		"      - command: [\"sh\", \"-c\", \"true\"]\n        covers: [\"R1.AC2\", \"R1.AC3\"]\n"
}

// isolatedGateRepo is a repository that reads no global or system git
// configuration, with one commit of code and the delivery feature approved
// under .walden/. The machine's commit signing, hooks, file-system monitor
// and line-ending conversion cannot reach the test, and the git the gate
// starts inherits the same environment. Tests that use it set process
// environment and are not parallel.
func isolatedGateRepo(t *testing.T) string {
	t.Helper()
	return isolatedGateRepoWith(t, deliveryGateTasks())
}

// isolatedGateRepoWith is isolatedGateRepo with a plan of the caller.
func isolatedGateRepoWith(t *testing.T, tasks string) string {
	t.Helper()
	requireGit(t)
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	root := t.TempDir()
	gitIn(t, root, "init", "-q", "-b", "main")
	gitIn(t, root, "config", "user.email", "gate@walden.test")
	gitIn(t, root, "config", "user.name", "Gate")
	write(t, root, "main.go", "package main\n")
	write(t, root, "notes.txt", "released\n")
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-qm", "code")
	addFeature(t, root, deliveryGateFeature, deliveryGateRequirements(), certifiableDesign(""), tasks)
	return root
}

func gitHead(t *testing.T, root string) string {
	t.Helper()
	command := exec.Command("git", "rev-parse", "HEAD")
	command.Dir = root
	out, err := command.Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func completeGateTask(t *testing.T, root, id string) {
	t.Helper()
	runner := testutil.NewFakeRunner(testutil.Response{Stdout: "ok", ExitCode: 0})
	if _, err := workflow.CompleteTask(context.Background(), root, deliveryGateFeature, id, runner); err != nil {
		t.Fatalf("complete task %s: %v", id, err)
	}
}

func commitAll(t *testing.T, root, message string) string {
	t.Helper()
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-qm", message)
	return gitHead(t, root)
}

func gateCertification(t *testing.T, root string, opts Options) (ReleaseReport, FeatureCertification) {
	t.Helper()
	report, err := ReleaseCheck(context.Background(), root, deliveryGateFeature, opts)
	if err != nil {
		t.Fatalf("ReleaseCheck: %v", err)
	}
	if len(report.Features) != 1 {
		t.Fatalf("certified %d feature(s), want one", len(report.Features))
	}
	return report, report.Features[0]
}

func gateStates(cert FeatureCertification) map[string]string {
	states := map[string]string{}
	for _, entry := range cert.Evidence {
		states[entry.TaskID] = entry.State
	}
	return states
}

func TestReleaseCheckDelivered(t *testing.T) {
	t.Run("evidence-criterion", func(t *testing.T) {
		// The delivery task is completed on the committed tree: attested.
		root := isolatedGateRepo(t)
		head := gitHead(t, root)
		completeGateTask(t, root, "1.1")
		completeGateTask(t, root, "1.2")
		commitAll(t, root, "evidence")

		report, cert := gateCertification(t, root, Options{})
		if want := map[string]string{"1.1": evidence.StateDelivered, "1.2": evidence.StateVerified}; !reflect.DeepEqual(gateStates(cert), want) {
			t.Fatalf("states = %v, want %v", gateStates(cert), want)
		}
		if evidenceCriterion := criterion(t, cert, "evidence"); !evidenceCriterion.Passed {
			t.Errorf("a delivered task blocks the evidence criterion: %v", evidenceCriterion.Blockers)
		}
		if !report.Releasable() {
			t.Errorf("the feature is not releasable: %+v", report)
		}
		for _, entry := range cert.Evidence {
			if entry.TaskID == "1.1" && (entry.Attestation == nil || entry.Attestation.Commit != head) {
				t.Errorf("the delivered task names %+v, want the commit %s", entry.Attestation, head)
			}
		}

		// After the delivered content moves on, the delivery proof would
		// fail. The delivered task still satisfies the criterion; the
		// regression task is stale and blocks alone.
		write(t, root, "notes.txt", "superseded\n")
		commitAll(t, root, "the notes move on")
		_, cert = gateCertification(t, root, Options{})
		if want := map[string]string{"1.1": evidence.StateDelivered, "1.2": evidence.StateStaleCode}; !reflect.DeepEqual(gateStates(cert), want) {
			t.Fatalf("states = %v, want %v", gateStates(cert), want)
		}
		blockers := criterion(t, cert, "evidence").Blockers
		if len(blockers) != 1 || !strings.Contains(blockers[0], "task 1.2 is stale-code") {
			t.Errorf("blockers = %v, want the regression task alone", blockers)
		}

		// The same feature with the delivery task completed on uncommitted
		// work: it holds no attestation, goes stale with the code and blocks
		// as today.
		root = isolatedGateRepo(t)
		write(t, root, "draft.txt", "not committed\n")
		completeGateTask(t, root, "1.1")
		completeGateTask(t, root, "1.2")
		commitAll(t, root, "evidence and the draft")
		write(t, root, "notes.txt", "superseded\n")
		commitAll(t, root, "the notes move on")
		report, cert = gateCertification(t, root, Options{})
		if want := map[string]string{"1.1": evidence.StateStaleCode, "1.2": evidence.StateStaleCode}; !reflect.DeepEqual(gateStates(cert), want) {
			t.Fatalf("states = %v, want %v", gateStates(cert), want)
		}
		blockers = criterion(t, cert, "evidence").Blockers
		if len(blockers) != 2 || !strings.Contains(blockers[0], "task 1.1 is stale-code") || report.Releasable() {
			t.Errorf("blockers = %v, releasable=%t: an unattested delivery task must block", blockers, report.Releasable())
		}
	})

	t.Run("pending and coverage", func(t *testing.T) {
		// A pending delivery task blocks as any pending task, and is waived
		// as any pending task.
		root := isolatedGateRepo(t)
		commitAll(t, root, "the plan")
		_, cert := gateCertification(t, root, Options{})
		blockers := criterion(t, cert, "evidence").Blockers
		if len(blockers) != 2 || !strings.Contains(blockers[0], "task 1.1 is pending") {
			t.Errorf("blockers = %v, want the two pending tasks", blockers)
		}
		report, cert := gateCertification(t, root, Options{AllowPending: true, WaiverReason: "delivery follows"})
		if !criterion(t, cert, "evidence").Passed || !report.Releasable() {
			t.Errorf("a waived pending delivery task still blocks: %v", criterion(t, cert, "evidence").Blockers)
		}
		if waived := report.WaivedTasks(); len(waived) != 2 {
			t.Errorf("waived tasks = %v, want both", waived)
		}

		// The feature carries the statements only delivery proofs cover,
		// whatever the state of its tasks.
		if want := []string{"R1.AC1"}; !reflect.DeepEqual(cert.DeliveryOnly, want) {
			t.Errorf("delivery-only statements = %v, want %v", cert.DeliveryOnly, want)
		}
		_, plain := func() (ReleaseReport, FeatureCertification) {
			other := gateRepo(t)
			report, err := ReleaseCheck(context.Background(), other, "gate-demo", Options{})
			if err != nil {
				t.Fatalf("ReleaseCheck: %v", err)
			}
			return report, report.Features[0]
		}()
		if len(plain.DeliveryOnly) != 0 {
			t.Errorf("a feature without delivery proofs lists %v", plain.DeliveryOnly)
		}
	})
}
