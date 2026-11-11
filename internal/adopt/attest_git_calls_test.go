package adopt

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/shell"
)

// countedAdoptPlan holds the given number of delivery leaves and one
// regression leaf after them.
func countedAdoptPlan(deliveries int) string {
	var plan strings.Builder
	plan.WriteString("# Implementation Plan\n\n")
	for id := 1; id <= deliveries; id++ {
		fmt.Fprintf(&plan, "- [ ] %d. Publish part %d\n  - Requirements: `R1.AC1`\n  - Design: Overview\n  - Proof: delivery\n  - Verification:\n    - command: %s\n      covers: [\"R1.AC1\"]\n", id, id, passCommand)
	}
	fmt.Fprintf(&plan, "- [ ] %d. Keep the parser\n  - Requirements: `R1.AC2`\n  - Design: Overview\n  - Verification:\n    - command: [\"go\", \"test\", \"./...\"]\n      covers: [\"R1.AC2\"]\n", deliveries+1)
	return plan.String()
}

// countedAdoptRepo holds the given number of delivered tasks, attested by
// their completion on the committed tree, and a verified regression task.
func countedAdoptRepo(t *testing.T, deliveries int) string {
	t.Helper()
	root := isolatedAdoptRepo(t)
	writeSealedFeature(t, root, passFeature, passRequirements, countedAdoptPlan(deliveries))
	for id := 1; id <= deliveries+1; id++ {
		completePassTask(t, root, passFeature, fmt.Sprint(id))
	}
	return root
}

// planGitCalls counts the git invocations of one adoption plan and returns
// the number of delivered tasks it holds.
func planGitCalls(t *testing.T, root string) (calls, delivered int) {
	t.Helper()
	inner := gitRunner
	gitRunner = diagnosticsRunnerFunc(func(ctx context.Context, name string, args ...string) (shell.Response, error) {
		calls++
		return inner.Run(ctx, name, args...)
	})
	defer func() { gitRunner = inner }()
	_, plan := planOf(t, root, passFeature)
	for _, entry := range plan.Evidence {
		if entry.State == evidence.StateDelivered {
			delivered++
		}
	}
	return calls, delivered
}

// TestPlanDeliveredGitCalls: an adoption plan costs the same git invocations
// with one delivered task and with three, before and after a code change,
// and the same as without the attestations.
func TestPlanDeliveredGitCalls(t *testing.T) {
	roots := map[int]string{1: countedAdoptRepo(t, 1), 3: countedAdoptRepo(t, 3)}
	unchanged := map[int]int{}
	for deliveries, root := range roots {
		calls, delivered := planGitCalls(t, root)
		if delivered != deliveries || calls == 0 {
			t.Fatalf("%d delivered task(s) and %d git invocation(s), want %d delivered", delivered, calls, deliveries)
		}
		unchanged[deliveries] = calls
	}
	if unchanged[1] != unchanged[3] {
		t.Errorf("%d git invocation(s) with three delivered tasks, %d with one", unchanged[3], unchanged[1])
	}

	changed := map[int]int{}
	for deliveries, root := range roots {
		writeRepoFile(t, root, "notes.txt", "superseded\n")
		commitRepo(t, root, "the notes move on")
		calls, delivered := planGitCalls(t, root)
		if delivered != deliveries {
			t.Fatalf("after the change %d delivered task(s), want %d", delivered, deliveries)
		}
		changed[deliveries] = calls
	}
	if changed[1] != changed[3] {
		t.Errorf("after a code change: %d git invocation(s) with three delivered tasks, %d with one", changed[3], changed[1])
	}

	// The same feature without the attestations, on an unchanged identity:
	// the passes were recorded after the declaration, so nothing is searched.
	root := countedAdoptRepo(t, 3)
	with, _ := planGitCalls(t, root)
	ledger, err := evidence.Load(root, passFeature)
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
	without, delivered := planGitCalls(t, root)
	if delivered != 0 || with != without {
		t.Errorf("%d git invocation(s) with attestations, %d without (%d delivered task(s) left)", with, without, delivered)
	}
}
