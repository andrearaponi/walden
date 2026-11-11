package release

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/shell"
)

type countingGateGit struct {
	inner shell.Runner
	calls int
}

func (c *countingGateGit) Run(ctx context.Context, name string, args ...string) (shell.Response, error) {
	c.calls++
	return c.inner.Run(ctx, name, args...)
}

// countedGateTasks holds the given number of delivery leaves and one
// regression leaf after them.
func countedGateTasks(deliveries int) string {
	var plan strings.Builder
	plan.WriteString(draftTasksHeader + "# Implementation Plan\n\n- [ ] 1. Markers\n")
	for id := 1; id <= deliveries; id++ {
		fmt.Fprintf(&plan, "  - [ ] 1.%d Publish part %d\n    - Requirements: `R1.AC1`\n    - Design: Overview\n    - Proof: delivery\n    - Verification:\n      - command: [\"grep\", \"-q\", \"released\", \"notes.txt\"]\n        covers: [\"R1.AC1\"]\n", id, id)
	}
	fmt.Fprintf(&plan, "  - [ ] 1.%d Keep the marker\n    - Requirements: `R1.AC2`, `R1.AC3`\n    - Design: Overview\n    - Verification:\n      - command: [\"sh\", \"-c\", \"true\"]\n        covers: [\"R1.AC2\", \"R1.AC3\"]\n", deliveries+1)
	return plan.String()
}

// countedGateRepo holds the given number of delivered tasks and a verified
// regression task, with the evidence committed.
func countedGateRepo(t *testing.T, deliveries int) string {
	t.Helper()
	requireGit(t)
	root := isolatedGateRepoWith(t, countedGateTasks(deliveries))
	for id := 1; id <= deliveries+1; id++ {
		completeGateTask(t, root, fmt.Sprintf("1.%d", id))
	}
	commitAll(t, root, "evidence")
	return root
}

// gateGitCalls counts the git invocations of one verdict and returns the
// number of delivered tasks it counted.
func gateGitCalls(t *testing.T, root string) (calls, delivered int) {
	t.Helper()
	counter := &countingGateGit{inner: gitRunner}
	previous := gitRunner
	gitRunner = counter
	defer func() { gitRunner = previous }()
	_, cert := gateCertification(t, root, Options{})
	for _, entry := range cert.Evidence {
		if entry.State == evidence.StateDelivered {
			delivered++
		}
	}
	return counter.calls, delivered
}

// TestReleaseCheckDeliveredGitCalls: a verdict costs the same git
// invocations with one delivered task and with three, before and after a
// code change, and the same as without the attestations.
func TestReleaseCheckDeliveredGitCalls(t *testing.T) {
	roots := map[int]string{1: countedGateRepo(t, 1), 3: countedGateRepo(t, 3)}
	unchanged := map[int]int{}
	for deliveries, root := range roots {
		calls, delivered := gateGitCalls(t, root)
		if delivered != deliveries || calls == 0 {
			t.Fatalf("%d delivered task(s) and %d git invocation(s), want %d delivered", delivered, calls, deliveries)
		}
		unchanged[deliveries] = calls
	}
	if unchanged[1] != unchanged[3] {
		t.Errorf("%d git invocation(s) with three delivered tasks, %d with one", unchanged[3], unchanged[1])
	}

	// After the delivered content moves on, in a new commit.
	changed := map[int]int{}
	for deliveries, root := range roots {
		write(t, root, "notes.txt", "superseded\n")
		commitAll(t, root, "the notes move on")
		calls, delivered := gateGitCalls(t, root)
		if delivered != deliveries {
			t.Fatalf("after the change %d delivered task(s), want %d", delivered, deliveries)
		}
		changed[deliveries] = calls
	}
	if changed[1] != changed[3] {
		t.Errorf("after a code change: %d git invocation(s) with three delivered tasks, %d with one", changed[3], changed[1])
	}

	// The same feature without the attestations, on an unchanged identity.
	root := countedGateRepo(t, 3)
	with, _ := gateGitCalls(t, root)
	ledger, err := evidence.Load(root, deliveryGateFeature)
	if err != nil {
		t.Fatalf("load the ledger: %v", err)
	}
	for id, record := range ledger.Tasks {
		record.Attestation = nil
		ledger.Tasks[id] = record
	}
	if err := evidence.Save(root, ledger); err != nil {
		t.Fatalf("save the ledger: %v", err)
	}
	commitAll(t, root, "attestations removed")
	without, delivered := gateGitCalls(t, root)
	if delivered != 0 || with != without {
		t.Errorf("%d git invocation(s) with attestations, %d without (%d delivered task(s) left)", with, without, delivered)
	}
}
