package app

import (
	"errors"
	"strings"
	"testing"
)

func TestExecuteStepsStopsAtFirstFailure(t *testing.T) {
	executed := make([]string, 0, 3)

	err := ExecuteSteps(
		Step{
			Name: "validate repo",
			Run: func() error {
				executed = append(executed, "validate repo")
				return nil
			},
		},
		Step{
			Name: "create branch",
			Run: func() error {
				executed = append(executed, "create branch")
				return errors.New("git failed")
			},
		},
		Step{
			Name: "open pr",
			Run: func() error {
				executed = append(executed, "open pr")
				return nil
			},
		},
	)

	if err == nil {
		t.Fatal("expected error from execute steps")
	}
	if !strings.Contains(err.Error(), "create branch") {
		t.Fatalf("expected error to mention failing step, got %v", err)
	}
	if strings.Contains(strings.Join(executed, ","), "open pr") {
		t.Fatalf("expected execution to stop before final step, got %#v", executed)
	}
}
