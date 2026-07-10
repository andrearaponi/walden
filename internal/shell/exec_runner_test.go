package shell

import (
	"context"
	"strings"
	"testing"
)

func TestExecRunnerReportsSuccess(t *testing.T) {
	runner := NewExecRunner()

	resp, err := runner.Run(context.Background(), "sh", "-c", "printf hello")
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if resp.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", resp.ExitCode)
	}
	if resp.Stdout != "hello" {
		t.Fatalf("Stdout = %q, want %q", resp.Stdout, "hello")
	}
	if resp.Err != nil {
		t.Fatalf("Response.Err = %v, want nil", resp.Err)
	}
}

func TestExecRunnerCapturesStderr(t *testing.T) {
	runner := NewExecRunner()

	resp, err := runner.Run(context.Background(), "sh", "-c", "printf oops 1>&2")
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if resp.Stderr != "oops" {
		t.Fatalf("Stderr = %q, want %q", resp.Stderr, "oops")
	}
}

func TestExecRunnerReportsNonZeroExit(t *testing.T) {
	runner := NewExecRunner()

	// A command that exits non-zero is not a runner error: the exit code is
	// captured and returned with a nil error so callers can inspect it.
	resp, err := runner.Run(context.Background(), "sh", "-c", "exit 3")
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if resp.ExitCode != 3 {
		t.Fatalf("ExitCode = %d, want 3", resp.ExitCode)
	}
	if resp.Err != nil {
		t.Fatalf("Response.Err = %v, want nil", resp.Err)
	}
}

func TestExecRunnerReportsLaunchFailure(t *testing.T) {
	runner := NewExecRunner()

	// A command that cannot be launched (not found) is a genuine runner error.
	resp, err := runner.Run(context.Background(), "walden-nonexistent-binary-xyz")
	if err == nil {
		t.Fatal("Run returned nil error for a missing binary, want an error")
	}
	if resp.ExitCode != 1 {
		t.Fatalf("ExitCode = %d, want 1", resp.ExitCode)
	}
	if resp.Err == nil {
		t.Fatal("Response.Err = nil, want the launch error")
	}
}

func TestExecRunnerHonorsCancelledContext(t *testing.T) {
	runner := NewExecRunner()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := runner.Run(ctx, "sh", "-c", "printf hello")
	if err == nil {
		t.Fatal("Run with a cancelled context returned nil error, want an error")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("error = %v, want it to mention context cancellation", err)
	}
}
