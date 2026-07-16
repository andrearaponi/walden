//go:build unix

package shell

import (
	"context"
	"testing"
	"time"
)

// The group kill must reap the whole tree well before the WaitDelay backstop:
// a long backstop here proves termination is doing the work.
func TestRunnerKillsProcessGroupOnDeadline(t *testing.T) {
	previous := waitDelay
	waitDelay = 10 * time.Second
	defer func() { waitDelay = previous }()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	started := time.Now()
	response, _ := NewExecRunner().Run(ctx, "sh", "-c", "sleep 60 & wait")
	elapsed := time.Since(started)

	if elapsed > 3*time.Second {
		t.Fatalf("expected prompt return after group kill, took %s", elapsed)
	}
	if response.ExitCode == 0 {
		t.Fatalf("expected non-zero exit for killed proof, got %d", response.ExitCode)
	}
}

// A proof that exits successfully but leaves an orphan holding the output
// pipes must return within the WaitDelay bound as an explicit error — before
// this containment it hung until the orphan exited.
func TestRunnerReturnsDespiteOrphanPipe(t *testing.T) {
	previous := waitDelay
	waitDelay = 1 * time.Second
	defer func() { waitDelay = previous }()

	started := time.Now()
	_, err := NewExecRunner().Run(context.Background(), "sh", "-c", "(sleep 60 &); exit 0")
	elapsed := time.Since(started)

	if elapsed > 5*time.Second {
		t.Fatalf("expected return within the wait-delay bound, took %s", elapsed)
	}
	if err == nil {
		t.Fatal("expected an explicit error for an orphan holding the pipes")
	}
}
