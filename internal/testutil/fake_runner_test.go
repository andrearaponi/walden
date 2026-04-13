package testutil

import (
	"context"
	"errors"
	"testing"
)

func TestFakeRunnerReturnsConfiguredResponsesAndRecordsCalls(t *testing.T) {
	runner := NewFakeRunner(
		Response{Stdout: "ok", ExitCode: 0},
		Response{Err: errors.New("boom"), ExitCode: 1},
	)

	first, err := runner.Run(context.Background(), "git", "status")
	if err != nil {
		t.Fatalf("expected first response to succeed, got %v", err)
	}
	if first.Stdout != "ok" || first.ExitCode != 0 {
		t.Fatalf("unexpected first response: %#v", first)
	}

	second, err := runner.Run(context.Background(), "gh", "repo", "view")
	if err == nil {
		t.Fatal("expected second response to fail")
	}
	if second.ExitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", second.ExitCode)
	}

	calls := runner.Calls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}
	if calls[0].Name != "git" || len(calls[0].Args) != 1 || calls[0].Args[0] != "status" {
		t.Fatalf("unexpected first call: %#v", calls[0])
	}
	if calls[1].Name != "gh" || len(calls[1].Args) != 2 || calls[1].Args[0] != "repo" {
		t.Fatalf("unexpected second call: %#v", calls[1])
	}
}
