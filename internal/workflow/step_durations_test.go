package workflow

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/spec"
)

// scriptedStep is what the fixture runner does for one proof step: it takes
// a known time and exits, or it waits for the step's deadline.
type scriptedStep struct {
	takes         time.Duration
	exit          int
	untilDeadline bool
}

// timedRunner plays its steps in order. Inside a synctest bubble its sleeps
// and the step deadlines run on the bubble's clock, so a duration is exact
// and nothing waits in real time.
type timedRunner struct {
	steps []scriptedStep
	next  int
}

func (r *timedRunner) Run(ctx context.Context, _ string, _ ...string) (shell.Response, error) {
	step := r.steps[r.next]
	r.next++
	if step.untilDeadline {
		<-ctx.Done()
		return shell.Response{ExitCode: -1}, nil
	}
	time.Sleep(step.takes)
	return shell.Response{Stdout: "ok", ExitCode: step.exit}, nil
}

func proofStep(timeout string) spec.VerificationStep {
	step := spec.VerificationStep{Argv: []string{"prove"}}
	if timeout != "" {
		step.Timeout = &timeout
	}
	return step
}

func proofTask(steps ...spec.VerificationStep) ExecutableTask {
	return ExecutableTask{ID: "1", Proof: spec.VerificationSpec{Steps: steps}}
}

func assertDurations(t *testing.T, steps []evidence.StepResult, want ...time.Duration) {
	t.Helper()
	if len(steps) != len(want) {
		t.Fatalf("recorded %d step(s), want %d: a step that never ran must be absent", len(steps), len(want))
	}
	for i, step := range steps {
		if step.DurationMS == nil {
			t.Fatalf("step %d carries no duration", i+1)
		}
		if got := time.Duration(*step.DurationMS) * time.Millisecond; got != want[i] {
			t.Fatalf("step %d lasted %s, recorded %s", i+1, want[i], got)
		}
	}
}

func TestStepDurations(t *testing.T) {
	t.Run("pass", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			runner := &timedRunner{steps: []scriptedStep{{takes: 200 * time.Millisecond}, {takes: 300 * time.Millisecond}}}
			run, err := executeProof(context.Background(), runner, proofTask(proofStep(""), proofStep("")))
			if err != nil {
				t.Fatalf("passing proof failed: %v", err)
			}
			assertDurations(t, run.steps, 200*time.Millisecond, 300*time.Millisecond)
		})
	})

	t.Run("fail", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			runner := &timedRunner{steps: []scriptedStep{{takes: 150 * time.Millisecond, exit: 1}, {takes: time.Second}}}
			run, err := executeProof(context.Background(), runner, proofTask(proofStep(""), proofStep("")))
			if err == nil {
				t.Fatal("a step exiting 1 passed")
			}
			// The failing step keeps its measurement; the step after it
			// never ran and is not in the record.
			assertDurations(t, run.steps, 150*time.Millisecond)
		})
	})

	t.Run("timeout", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			runner := &timedRunner{steps: []scriptedStep{{untilDeadline: true}}}
			run, err := executeProof(context.Background(), runner, proofTask(proofStep("30s")))
			if err == nil {
				t.Fatal("a step past its timeout passed")
			}
			assertDurations(t, run.steps, 30*time.Second)
		})
	})

	t.Run("default-timeout", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			runner := &timedRunner{steps: []scriptedStep{{untilDeadline: true}}}
			run, err := executeProof(context.Background(), runner, proofTask(proofStep("")))
			if err == nil {
				t.Fatal("a step past the default timeout passed")
			}
			assertDurations(t, run.steps, defaultProofStepTimeout)
		})
	})

	t.Run("record", func(t *testing.T) {
		root := t.TempDir()
		writeVerifyFixture(t, root)
		overrideIdentityRunner(t, identityYielding("100644 blob aaa\tmain.go\n"))
		completeBoth(t, root)

		synctest.Test(t, func(t *testing.T) {
			runner := &timedRunner{steps: []scriptedStep{{takes: 200 * time.Millisecond}, {takes: 300 * time.Millisecond}}}
			result, err := Verify(context.Background(), root, "todo-app-demo", true, false, runner)
			if err != nil || len(result.Failed) != 0 {
				t.Fatalf("verify --all: %v, %+v", err, result)
			}
		})

		ledger, err := evidence.Load(root, "todo-app-demo")
		if err != nil {
			t.Fatal(err)
		}
		assertDurations(t, ledger.Tasks["1.1"].Steps, 200*time.Millisecond)
		assertDurations(t, ledger.Tasks["1.2"].Steps, 300*time.Millisecond)
	})
}

// withoutDurations copies recorded steps without their measurements, for
// tests that compare what two runs recorded apart from how long they took.
func withoutDurations(steps []evidence.StepResult) []evidence.StepResult {
	stripped := make([]evidence.StepResult, len(steps))
	for i, step := range steps {
		step.DurationMS = nil
		stripped[i] = step
	}
	return stripped
}
