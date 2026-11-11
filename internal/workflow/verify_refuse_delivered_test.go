package workflow

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/testutil"
)

// TestVerifyTaskRefusesDelivered: naming a delivered task is refused before
// any environment probe, any proof and any ledger write.
func TestVerifyTaskRefusesDelivered(t *testing.T) {
	root, head := deliveryRepo(t)
	completeDelivery(t, root, "1")
	completeDelivery(t, root, "2")
	writeProbes(t, root, "- toolchain: [\"toolchain\", \"--version\"]\n")
	ledgerPath := evidence.DocumentPath(root, deliveryFeature)

	// The completions above captured a profile for this root. The capture is
	// taken once for each root in a process, so without a fresh one the
	// probe could not run whatever the command does: overrideProbeRunner
	// resets it, and the control below shows that the probe is reachable.
	probes := 0
	overrideProbeRunner(t, probeFunc(func(ctx context.Context, name string, args ...string) (shell.Response, error) {
		probes++
		return shell.Response{Stdout: "toolchain 1.0\n"}, nil
	}))

	for _, check := range []bool{false, true} {
		before, err := os.ReadFile(ledgerPath)
		if err != nil {
			t.Fatalf("read the ledger: %v", err)
		}
		runner := testutil.NewFakeRunner(testutil.Response{Stdout: "ok"})

		_, err = VerifyTask(context.Background(), root, deliveryFeature, "1", check, runner)
		if err == nil {
			t.Fatalf("check=%t: a run that names a delivered task is accepted", check)
		}
		for _, want := range []string{
			`task "1" is delivered`, head,
			"declaration", "proof", "approved plan", "statement it names",
		} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("check=%t: the refusal %q does not hold %q", check, err, want)
			}
		}
		if probes != 0 {
			t.Errorf("check=%t: the environment probe ran %d time(s)", check, probes)
		}
		if calls := runner.Calls(); len(calls) != 0 {
			t.Errorf("check=%t: proofs ran: %v", check, calls)
		}
		after, err := os.ReadFile(ledgerPath)
		if err != nil || !bytes.Equal(before, after) {
			t.Errorf("check=%t: the ledger changed (%v)", check, err)
		}
	}

	// Control: on the same fixture a run that names the regression task
	// reaches the probe, the proof and the ledger, so each absence above
	// could have failed.
	before, _ := os.ReadFile(ledgerPath)
	runner := testutil.NewFakeRunner(testutil.Response{Stdout: "ok"})
	if _, err := VerifyTask(context.Background(), root, deliveryFeature, "2", false, runner); err != nil {
		t.Fatalf("the control run failed: %v", err)
	}
	if probes != 1 {
		t.Errorf("control: the probe ran %d time(s), want once", probes)
	}
	if len(runner.Calls()) != 1 {
		t.Errorf("control: proofs run = %v, want one", runner.Calls())
	}
	if after, _ := os.ReadFile(ledgerPath); bytes.Equal(before, after) {
		t.Error("control: the ledger did not change")
	}

	// A delivery task that holds no attestation is not refused.
	root, _ = deliveryRepo(t)
	factsFile(t, root, "draft.txt", "not committed\n")
	completeDelivery(t, root, "1")
	if _, err := VerifyTask(context.Background(), root, deliveryFeature, "1", true, passing()); err != nil {
		t.Errorf("a delivery task without an attestation is refused: %v", err)
	}
}
