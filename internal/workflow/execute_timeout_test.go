package workflow

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/spec"
)

type runnerFunc func(ctx context.Context, name string, args ...string) (shell.Response, error)

func (f runnerFunc) Run(ctx context.Context, name string, args ...string) (shell.Response, error) {
	return f(ctx, name, args...)
}

func timeoutTask(declared string) ExecutableTask {
	step := spec.VerificationStep{Argv: []string{"go", "test", "./pkg/"}}
	if declared != "" {
		step.Timeout = &declared
	}
	proof := spec.VerificationSpec{Steps: []spec.VerificationStep{step}}
	return ExecutableTask{
		ID:           "1.1",
		Title:        "Step",
		Verification: proof.Display(),
		Proof:        proof,
	}
}

func TestExecuteProofDefaultTimeout(t *testing.T) {
	var remaining time.Duration
	runner := runnerFunc(func(ctx context.Context, name string, args ...string) (shell.Response, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("expected the proof step context to carry a deadline")
		}
		remaining = time.Until(deadline)
		return shell.Response{ExitCode: 0}, nil
	})

	if _, _, err := executeProof(context.Background(), runner, timeoutTask("")); err != nil {
		t.Fatalf("executeProof returned error: %v", err)
	}
	if remaining <= 9*time.Minute || remaining > 10*time.Minute {
		t.Fatalf("expected a ~10m default deadline, got %s remaining", remaining)
	}
}

func TestExecuteProofDeclaredTimeoutHonored(t *testing.T) {
	var remaining time.Duration
	runner := runnerFunc(func(ctx context.Context, name string, args ...string) (shell.Response, error) {
		deadline, _ := ctx.Deadline()
		remaining = time.Until(deadline)
		return shell.Response{ExitCode: 0}, nil
	})

	if _, _, err := executeProof(context.Background(), runner, timeoutTask("30s")); err != nil {
		t.Fatalf("executeProof returned error: %v", err)
	}
	if remaining <= 0 || remaining > 30*time.Second {
		t.Fatalf("expected a ≤30s declared deadline, got %s remaining", remaining)
	}
}

func TestExecuteProofTimeoutFailure(t *testing.T) {
	runner := runnerFunc(func(ctx context.Context, name string, args ...string) (shell.Response, error) {
		<-ctx.Done()
		return shell.Response{ExitCode: -1}, nil
	})

	_, _, err := executeProof(context.Background(), runner, timeoutTask("50ms"))
	if err == nil {
		t.Fatal("expected a timeout failure")
	}
	if !strings.Contains(err.Error(), "exceeded timeout 50ms") {
		t.Fatalf("expected failure naming the exceeded timeout, got %v", err)
	}
	if !strings.Contains(err.Error(), `task "1.1"`) {
		t.Fatalf("expected failure naming the task, got %v", err)
	}
}

func TestExecuteProofInvalidTimeoutRefused(t *testing.T) {
	calls := 0
	runner := runnerFunc(func(ctx context.Context, name string, args ...string) (shell.Response, error) {
		calls++
		return shell.Response{ExitCode: 0}, nil
	})

	_, _, err := executeProof(context.Background(), runner, timeoutTask("banana"))
	if err == nil || !strings.Contains(err.Error(), "invalid timeout value") {
		t.Fatalf("expected invalid-timeout refusal, got %v", err)
	}
	if calls != 0 {
		t.Fatalf("expected no proof execution after refusal, ran %d step(s)", calls)
	}
}
