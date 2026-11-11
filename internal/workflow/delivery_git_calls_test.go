package workflow

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/testutil"
)

// countedPlan holds the given number of delivery leaves and one regression
// leaf after them.
func countedPlan(deliveries int) string {
	var plan strings.Builder
	plan.WriteString("---\nstatus: approved\napproved_at: 2026-03-23T10:00:00Z\nlast_modified: 2026-03-23T10:00:00Z\nsource_design_approved_at: 2026-03-23T09:30:00Z\n---\n\n# Implementation Plan\n\n")
	for id := 1; id <= deliveries; id++ {
		fmt.Fprintf(&plan, "- [ ] %d. Publish part %d\n  - Requirements: `R1.AC1`\n  - Design: Overview\n  - Proof: delivery\n  - Verification:\n    - command: [\"grep\", \"-q\", \"released\", \"notes.txt\"]\n      covers: [\"R1.AC1\"]\n", id, id)
	}
	fmt.Fprintf(&plan, "- [ ] %d. Keep the parser working\n  - Requirements: `R1.AC2`\n  - Design: Overview\n  - Verification:\n    - command: [\"go\", \"test\", \"./...\"]\n      covers: [\"R1.AC2\"]\n", deliveries+1)
	return plan.String()
}

// countedRepo is an isolated repository whose feature holds the given number
// of delivered tasks and a verified regression task. With distinct set, each
// delivery is attested on a commit of its own: a git call made for each
// delivered task, or for each attested commit, then grows with their number.
func countedRepo(t *testing.T, deliveries int, distinct bool) string {
	t.Helper()
	root := factsRepo(t)
	factsFile(t, root, "main.go", "package main\n")
	factsFile(t, root, "notes.txt", "released\n")
	factsGit(t, root, "add", ".")
	factsGit(t, root, "commit", "-qm", "base")
	setupApprovedFeature(t, root, deliveryFeature, countedPlan(deliveries))
	for id := 1; id <= deliveries; id++ {
		if distinct && id > 1 {
			factsFile(t, root, "notes.txt", fmt.Sprintf("released\npart %d\n", id))
			factsGit(t, root, "add", "notes.txt")
			factsGit(t, root, "commit", "-qm", fmt.Sprintf("part %d", id))
		}
		completeDelivery(t, root, fmt.Sprint(id))
		if record := deliveryRecord(t, root, fmt.Sprint(id)); record.Attestation == nil {
			t.Fatalf("task %d of the fixture is not attested", id)
		}
	}
	completeDelivery(t, root, fmt.Sprint(deliveries+1))
	return root
}

type countingGit struct {
	inner shell.Runner
	calls int
}

func (c *countingGit) Run(ctx context.Context, name string, args ...string) (shell.Response, error) {
	c.calls++
	return c.inner.Run(ctx, name, args...)
}

// gitCalls counts the git invocations of the three readings of a feature
// that derive its states: the evidence report, the readiness of task status
// and a verify run in check mode.
func gitCalls(t *testing.T, root string) map[string]int {
	t.Helper()
	counts := map[string]int{}
	count := func(name string, reading func()) {
		t.Helper()
		counter := &countingGit{inner: shell.NewExecRunner()}
		overrideIdentityRunner(t, counter)
		reading()
		counts[name] = counter.calls
	}
	count("evidence status", func() {
		if _, err := ReadEvidence(context.Background(), root, deliveryFeature); err != nil {
			t.Fatalf("read evidence: %v", err)
		}
	})
	count("task status", func() {
		if _, err := LoadExecutionReadiness(root, deliveryFeature); err != nil {
			t.Fatalf("load readiness: %v", err)
		}
	})
	count("verify --check", func() {
		// Enough responses for every proof the run may execute.
		responses := make([]testutil.Response, 8)
		for i := range responses {
			responses[i] = testutil.Response{Stdout: "ok"}
		}
		if _, err := Verify(context.Background(), root, deliveryFeature, false, true, testutil.NewFakeRunner(responses...)); err != nil {
			t.Fatalf("verify --check: %v", err)
		}
	})
	return counts
}

func removeAttestations(t *testing.T, root string) {
	t.Helper()
	ledger, err := evidence.Load(root, deliveryFeature)
	if err != nil {
		t.Fatalf("load evidence: %v", err)
	}
	for id, record := range ledger.Tasks {
		record.Attestation = nil
		ledger.Tasks[id] = record
	}
	if err := evidence.Save(root, ledger); err != nil {
		t.Fatalf("save evidence: %v", err)
	}
}

// TestDeliveredTasksAddNoGitCalls: deriving the states of a feature costs the
// same git invocations with one delivered task and with three, and no more
// than for the same feature without the attestations.
func TestDeliveredTasksAddNoGitCalls(t *testing.T) {
	t.Run("identity unchanged", func(t *testing.T) {
		one := gitCalls(t, countedRepo(t, 1, false))
		rootOfThree := countedRepo(t, 3, false)
		three := gitCalls(t, rootOfThree)
		for reading, calls := range one {
			if calls == 0 {
				t.Errorf("%s: no git invocation was counted: the fixture measures nothing", reading)
			}
			if three[reading] != calls {
				t.Errorf("%s: %d git invocation(s) with three delivered tasks, %d with one", reading, three[reading], calls)
			}
		}

		// Without the attestations the tasks read verified. The two readings
		// cost the same. A verify run costs more: on a committed tree it
		// proves the delivery tasks that hold no attestation, to attest them.
		removeAttestations(t, rootOfThree)
		without := gitCalls(t, rootOfThree)
		for _, reading := range []string{"evidence status", "task status"} {
			if three[reading] != without[reading] {
				t.Errorf("%s: %d git invocation(s) with attestations, %d without", reading, three[reading], without[reading])
			}
		}
		if three["verify --check"] > without["verify --check"] {
			t.Errorf("verify --check: %d git invocation(s) with attestations, more than the %d without", three["verify --check"], without["verify --check"])
		}
	})

	t.Run("after a code change", func(t *testing.T) {
		// Each delivery is attested on a commit of its own, and the code
		// then changes once more: every delivered task has stale code, on a
		// different recorded commit.
		counts := map[int]map[string]int{}
		for _, deliveries := range []int{1, 3} {
			root := countedRepo(t, deliveries, true)
			factsFile(t, root, "main.go", "package main // changed\n")
			factsGit(t, root, "add", "main.go")
			factsGit(t, root, "commit", "-qm", "the code moves on")
			counts[deliveries] = gitCalls(t, root)

			// The fixture holds what it claims.
			reading, err := ReadEvidence(context.Background(), root, deliveryFeature)
			if err != nil {
				t.Fatalf("read evidence: %v", err)
			}
			delivered := 0
			for _, entry := range reading.Entries {
				if entry.State == evidence.StateDelivered && entry.CodeFreshness == "stale" {
					delivered++
				}
			}
			if delivered != deliveries {
				t.Fatalf("%d delivered task(s) with stale code, want %d", delivered, deliveries)
			}
		}
		for reading, calls := range counts[1] {
			if counts[3][reading] != calls {
				t.Errorf("%s: %d git invocation(s) with three delivered tasks, %d with one", reading, counts[3][reading], calls)
			}
		}
	})
}
